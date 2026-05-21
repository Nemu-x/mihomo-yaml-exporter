package checker

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/nemu-x/mihomo-yaml-exporter/internal/subscription"
)

type TCPChecker struct {
	Timeout     time.Duration
	Concurrency int
	Retries     int
	TLSPorts    map[int]struct{}
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
	attempts := c.Retries + 1
	if attempts < 1 {
		attempts = 1
	}

	for i := 0; i < attempts; i++ {
		if ctx.Err() != nil {
			break
		}
		up, latency, err := c.probe(ctx, p)
		if up {
			return subscription.CheckResult{
				Proxy:     p,
				Up:        true,
				LatencyMs: float64(latency),
			}
		}
		_ = err
		if i+1 < attempts {
			select {
			case <-ctx.Done():
				return subscription.CheckResult{Proxy: p, Up: false}
			case <-time.After(250 * time.Millisecond):
			}
		}
	}
	return subscription.CheckResult{Proxy: p, Up: false, LatencyMs: 0}
}

func (c *TCPChecker) probe(ctx context.Context, p subscription.Proxy) (bool, int64, error) {
	addr := net.JoinHostPort(p.Server, fmt.Sprintf("%d", p.Port))
	dialer := net.Dialer{Timeout: c.Timeout}

	start := time.Now()
	conn, err := dialer.DialContext(ctx, "tcp", addr)
	if err == nil {
		_ = conn.Close()
		return true, time.Since(start).Milliseconds(), nil
	}

	if _, useTLS := c.TLSPorts[p.Port]; !useTLS {
		return false, 0, err
	}

	host := p.Server
	if strings.Contains(host, ":") {
		if h, _, splitErr := net.SplitHostPort(host); splitErr == nil {
			host = h
		}
	}

	start = time.Now()
	tlsConn, tlsErr := tls.DialWithDialer(&dialer, "tcp", addr, &tls.Config{
		InsecureSkipVerify: true,
		ServerName:         host,
	})
	if tlsErr == nil {
		_ = tlsConn.Close()
		return true, time.Since(start).Milliseconds(), nil
	}
	return false, 0, tlsErr
}
