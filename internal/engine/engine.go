package engine

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/nemu-x/mihomo-yaml-exporter/internal/checker"
	"github.com/nemu-x/mihomo-yaml-exporter/internal/config"
	"github.com/nemu-x/mihomo-yaml-exporter/internal/metrics"
	"github.com/nemu-x/mihomo-yaml-exporter/internal/subscription"
)

type Snapshot struct {
	Results          []subscription.CheckResult
	ProxiesTotal     int
	ProxiesOnline    int
	LastCheck        time.Time
	SubLoadOK        bool
	SubLastSuccess   time.Time
	LastSubError     string
	HasProxies       bool
	LastCheckFailed  bool
}

type Engine struct {
	cfg     config.Config
	fetcher *subscription.Fetcher
	checker *checker.TCPChecker
	metrics *metrics.Registry

	mu              sync.RWMutex
	proxies         []subscription.Proxy
	results         []subscription.CheckResult
	subLoadOK       bool
	subLastSuccess  time.Time
	lastSubError    string
	lastCheck       time.Time
	hasProxies      bool
	lastCheckFailed bool
}

func New(cfg config.Config, reg *metrics.Registry) *Engine {
	return &Engine{
		cfg:     cfg,
		fetcher: subscription.NewFetcher(cfg.SubscriptionURL),
		checker: &checker.TCPChecker{
			Timeout:     cfg.CheckTimeout,
			Concurrency: cfg.CheckConcurrency,
		},
		metrics: reg,
	}
}

func (e *Engine) Run(ctx context.Context) {
	e.tick(ctx)

	ticker := time.NewTicker(e.cfg.CheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			e.tick(ctx)
		}
	}
}

func (e *Engine) tick(ctx context.Context) {
	subOK := false
	var subErr string
	var newProxies []subscription.Proxy

	body, err := e.fetcher.Fetch(ctx)
	if err != nil {
		subErr = err.Error()
		log.Printf("subscription fetch failed: %s (url=%s)", err, subscription.RedactURL(e.cfg.SubscriptionURL))
	} else {
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

	results := e.checker.CheckAll(ctx, proxies)
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
		log.Printf("warn: all %d proxies are down after TCP check", len(results))
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
	}
}
