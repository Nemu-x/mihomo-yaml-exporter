package config

import (
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	SubscriptionURL  string
	ListenAddr       string
	CheckMode        string
	CheckInterval    time.Duration
	CheckTimeout     time.Duration
	CheckConcurrency int
	CheckRetries     int
	TLSPorts         map[int]struct{}
	IncludeGroups    []string
	ExcludeGroups    []string
	ExcludeProxyRe   *regexp.Regexp
	IncludeProxyRe   *regexp.Regexp

	MihomoController     string
	MihomoSecret         string
	MihomoDelayURL       string
	MihomoBinary         string
	MihomoConfigDir      string
	MihomoStartupTimeout time.Duration
}

func Load() (Config, error) {
	subURL := os.Getenv("SUBSCRIPTION_URL")
	if subURL == "" {
		return Config{}, fmt.Errorf("SUBSCRIPTION_URL is required")
	}

	cfg := Config{
		SubscriptionURL:  subURL,
		ListenAddr:       envOr("LISTEN_ADDR", "0.0.0.0:9123"),
		CheckMode:        strings.ToLower(envOr("CHECK_MODE", "mihomo")),
		CheckInterval:    durationEnv("CHECK_INTERVAL", 60*time.Second),
		CheckTimeout:     durationEnv("CHECK_TIMEOUT", 10*time.Second),
		CheckConcurrency: intEnv("CHECK_CONCURRENCY", 5),
		CheckRetries:     intEnv("CHECK_RETRIES", 2),
		TLSPorts:         parsePorts(envOr("CHECK_TLS_PORTS", "443,8443")),
		IncludeGroups:    splitCSV(os.Getenv("INCLUDE_GROUPS")),
		ExcludeGroups:    splitCSV(envOr("EXCLUDE_GROUPS", "DIRECT,REJECT,GLOBAL")),

		MihomoController:     envOr("MIHOMO_CONTROLLER", "http://127.0.0.1:9090"),
		MihomoSecret:         os.Getenv("MIHOMO_SECRET"),
		MihomoDelayURL:       envOr("MIHOMO_DELAY_URL", "http://www.gstatic.com/generate_204"),
		MihomoBinary:         envOr("MIHOMO_BINARY", "/usr/local/bin/mihomo"),
		MihomoConfigDir:      envOr("MIHOMO_CONFIG_DIR", "/tmp/mihomo"),
		MihomoStartupTimeout: durationEnv("MIHOMO_STARTUP_TIMEOUT", 45*time.Second),
	}

	if v := os.Getenv("EXCLUDE_PROXY_REGEX"); v != "" {
		re, err := regexp.Compile(v)
		if err != nil {
			return Config{}, fmt.Errorf("EXCLUDE_PROXY_REGEX: %w", err)
		}
		cfg.ExcludeProxyRe = re
	}

	if v := os.Getenv("INCLUDE_PROXY_REGEX"); v != "" {
		re, err := regexp.Compile(v)
		if err != nil {
			return Config{}, fmt.Errorf("INCLUDE_PROXY_REGEX: %w", err)
		}
		cfg.IncludeProxyRe = re
	}

	if cfg.CheckConcurrency < 1 {
		cfg.CheckConcurrency = 1
	}
	if cfg.CheckRetries < 0 {
		cfg.CheckRetries = 0
	}
	if cfg.CheckMode != "mihomo" && cfg.CheckMode != "tcp" {
		return Config{}, fmt.Errorf("CHECK_MODE must be mihomo or tcp")
	}

	return cfg, nil
}

func parsePorts(s string) map[int]struct{} {
	out := make(map[int]struct{})
	for _, p := range splitCSV(s) {
		if n, err := strconv.Atoi(p); err == nil && n > 0 {
			out[n] = struct{}{}
		}
	}
	return out
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func durationEnv(key string, def time.Duration) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return def
	}
	return d
}

func intEnv(key string, def int) int {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}

func splitCSV(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
