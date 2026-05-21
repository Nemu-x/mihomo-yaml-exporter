package checker

import (
	"context"

	"github.com/nemu-x/mihomo-yaml-exporter/internal/subscription"
)

type Checker interface {
	CheckAll(ctx context.Context, proxies []subscription.Proxy) []subscription.CheckResult
}
