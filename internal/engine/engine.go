package engine

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/nemu-x/mihomo-yaml-exporter/internal/checker"
	"github.com/nemu-x/mihomo-yaml-exporter/internal/config"
	"github.com/nemu-x/mihomo-yaml-exporter/internal/metrics"
	"github.com/nemu-x/mihomo-yaml-exporter/internal/mihomo"
	"github.com/nemu-x/mihomo-yaml-exporter/internal/subscription"
)

type Snapshot struct {
	Results         []subscription.CheckResult
	ProxiesTotal    int
	ProxiesOnline   int
	LastCheck       time.Time
	SubLoadOK       bool
	SubLastSuccess  time.Time
	LastSubError    string
	HasProxies      bool
	LastCheckFailed bool
	CheckInterval   time.Duration
	CheckInProgress bool
	CheckMode       string
}

type Engine struct {
	cfg       config.Config
	fetcher   *subscription.Fetcher
	checker   checker.Checker
	mihomo    *mihomo.Process
	metrics   *metrics.Registry
	checkMode string

	mu              sync.RWMutex
	proxies         []subscription.Proxy
	results         []subscription.CheckResult
	subLoadOK       bool
	subLastSuccess  time.Time
	lastSubError    string
	lastCheck       time.Time
	hasProxies      bool
	lastCheckFailed bool
	checking        bool
	refreshCh chan struct{}
}

func New(cfg config.Config, reg *metrics.Registry) *Engine {
	e := &Engine{
		cfg:       cfg,
		fetcher:   subscription.NewFetcher(cfg.SubscriptionURL),
		metrics:   reg,
		checkMode: cfg.CheckMode,
		refreshCh: make(chan struct{}, 1),
	}

	switch cfg.CheckMode {
	case "mihomo":
		proc := mihomo.NewProcess(
			cfg.MihomoBinary,
			cfg.MihomoConfigDir,
			cfg.MihomoController,
			cfg.MihomoSecret,
		)
		e.mihomo = proc
		e.checker = mihomo.NewDelayChecker(cfg, proc)
	default:
		e.checker = &checker.TCPChecker{
			Timeout:     cfg.CheckTimeout,
			Concurrency: cfg.CheckConcurrency,
			Retries:     cfg.CheckRetries,
			TLSPorts:    cfg.TLSPorts,
		}
	}

	return e
}

func (e *Engine) Run(ctx context.Context) {
	defer e.shutdown()
	e.tick(ctx)

	ticker := time.NewTicker(e.cfg.CheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			e.tick(ctx)
		case <-e.refreshCh:
			e.tick(ctx)
		}
	}
}

func (e *Engine) shutdown() {
	if e.mihomo != nil {
		_ = e.mihomo.Stop()
	}
}

func (e *Engine) TriggerRefresh() bool {
	select {
	case e.refreshCh <- struct{}{}:
		return true
	default:
		return false
	}
}

func (e *Engine) tick(ctx context.Context) {
	e.mu.Lock()
	if e.checking {
		e.mu.Unlock()
		return
	}
	e.checking = true
	e.mu.Unlock()

	defer func() {
		e.mu.Lock()
		e.checking = false
		e.mu.Unlock()
	}()

	subOK := false
	var subErr string
	var newProxies []subscription.Proxy
	var configRaw []byte

	body, err := e.fetcher.Fetch(ctx)
	if err != nil {
		subErr = err.Error()
		log.Printf("subscription fetch failed: %s (url=%s)", err, subscription.RedactURL(e.cfg.SubscriptionURL))
	} else {
		configRaw = body
		parsed, perr := subscription.Parse(
			body,
			e.cfg.IncludeGroups,
			e.cfg.ExcludeGroups,
			e.cfg.ExcludeProxyRe,
			e.cfg.IncludeProxyRe,
		)
		if perr != nil {
			subErr = perr.Error()
			log.Printf("subscription parse failed: %v", perr)
		} else {
			newProxies = parsed
			subOK = true
		}
	}

	if subOK && e.mihomo != nil {
		startCtx, cancel := context.WithTimeout(ctx, e.cfg.MihomoStartupTimeout)
		if err := e.mihomo.EnsureRunning(startCtx, configRaw); err != nil {
			subErr = err.Error()
			subOK = false
			log.Printf("mihomo start/reload failed: %v", err)
		}
		cancel()
	}

	e.mu.Lock()
	if subOK {
		e.proxies = newProxies
		e.subLoadOK = true
		e.subLastSuccess = time.Now()
		e.lastSubError = ""
		e.hasProxies = len(e.proxies) > 0
	} else {
		e.subLoadOK = false
		e.lastSubError = subErr
		if !e.hasProxies {
			subLast := e.subLastSuccess
			e.mu.Unlock()
			e.metrics.Update(nil, false, subLast, time.Now())
			return
		}
		log.Printf("using last successful proxy list (%d proxies)", len(e.proxies))
	}
	proxies := append([]subscription.Proxy(nil), e.proxies...)
	subLast := e.subLastSuccess
	subLoadOK := e.subLoadOK
	e.mu.Unlock()

	if len(proxies) == 0 {
		return
	}

	checkCtx, cancel := context.WithTimeout(ctx, e.cfg.CheckTimeout*time.Duration(len(proxies)/e.cfg.CheckConcurrency+1))
	defer cancel()

	results := e.checker.CheckAll(checkCtx, proxies)
	checkTime := time.Now()

	online := 0
	for _, r := range results {
		if r.Up {
			online++
		}
	}

	e.mu.Lock()
	e.results = results
	e.lastCheck = checkTime
	e.hasProxies = true
	allDown := len(results) > 0 && online == 0
	e.lastCheckFailed = allDown
	e.mu.Unlock()

	e.metrics.Update(results, subLoadOK, subLast, checkTime)

	if allDown {
		log.Printf("warn: all %d proxies are down after %s check", len(results), e.checkMode)
	} else {
		log.Printf("check done: %d/%d up via %s", online, len(results), e.checkMode)
	}
}

func (e *Engine) Snapshot() Snapshot {
	e.mu.RLock()
	defer e.mu.RUnlock()

	online := 0
	for _, r := range e.results {
		if r.Up {
			online++
		}
	}

	return Snapshot{
		Results:         append([]subscription.CheckResult(nil), e.results...),
		ProxiesTotal:    len(e.results),
		ProxiesOnline:   online,
		LastCheck:       e.lastCheck,
		SubLoadOK:       e.subLoadOK,
		SubLastSuccess:  e.subLastSuccess,
		LastSubError:    e.lastSubError,
		HasProxies:      e.hasProxies,
		LastCheckFailed: e.lastCheckFailed,
		CheckInterval:   e.cfg.CheckInterval,
		CheckInProgress: e.checking,
		CheckMode:       e.checkMode,
	}
}
