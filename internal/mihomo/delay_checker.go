package mihomo

import (
	"context"
	"sync"
	"time"

	"github.com/nemu-x/mihomo-yaml-exporter/internal/config"
	"github.com/nemu-x/mihomo-yaml-exporter/internal/subscription"
)

type DelayChecker struct {
	Process     *Process
	DelayURL    string
	Timeout     time.Duration
	Concurrency int
}

func NewDelayChecker(cfg config.Config, proc *Process) *DelayChecker {
	return &DelayChecker{
		Process:     proc,
		DelayURL:    cfg.MihomoDelayURL,
		Timeout:     cfg.CheckTimeout,
		Concurrency: cfg.CheckConcurrency,
	}
}

func (c *DelayChecker) CheckAll(ctx context.Context, proxies []subscription.Proxy) []subscription.CheckResult {
	if len(proxies) == 0 {
		return nil
	}
	client := c.Process.Client()
	timeoutMs := int(c.Timeout.Milliseconds())
	if timeoutMs < 1000 {
		timeoutMs = 5000
	}

	sem := make(chan struct{}, c.Concurrency)
	results := make([]subscription.CheckResult, len(proxies))
	var wg sync.WaitGroup

	for i, p := range proxies {
		wg.Add(1)
		go func(idx int, proxy subscription.Proxy) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			results[idx] = c.checkOne(ctx, client, proxy, timeoutMs)
		}(i, p)
	}
	wg.Wait()
	return results
}

func (c *DelayChecker) checkOne(ctx context.Context, client *Client, p subscription.Proxy, timeoutMs int) subscription.CheckResult {
	delay, err := client.Delay(ctx, p.Name, c.DelayURL, timeoutMs)
	if err != nil {
		return subscription.CheckResult{Proxy: p, Up: false, LatencyMs: 0}
	}
	return subscription.CheckResult{
		Proxy:     p,
		Up:        true,
		LatencyMs: float64(delay),
	}
}
