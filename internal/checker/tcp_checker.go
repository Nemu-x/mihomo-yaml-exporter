package checker

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/nemu-x/mihomo-yaml-exporter/internal/subscription"
)

type TCPChecker struct {
	Timeout     time.Duration
	Concurrency int
}

func (c *TCPChecker) CheckAll(ctx context.Context, proxies []subscription.Proxy) []subscription.CheckResult {
	if len(proxies) == 0 {
		return nil
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
			results[idx] = c.checkOne(ctx, proxy)
		}(i, p)
	}

	wg.Wait()
	return results
}

func (c *TCPChecker) checkOne(ctx context.Context, p subscription.Proxy) subscription.CheckResult {
	addr := net.JoinHostPort(p.Server, fmt.Sprintf("%d", p.Port))
	dialer := net.Dialer{Timeout: c.Timeout}

	start := time.Now()
	conn, err := dialer.DialContext(ctx, "tcp", addr)
	latency := time.Since(start).Milliseconds()

	res := subscription.CheckResult{
		Proxy:     p,
		Up:        err == nil,
		LatencyMs: 0,
	}
	if err == nil {
		_ = conn.Close()
		res.LatencyMs = float64(latency)
	}
	return res
}
