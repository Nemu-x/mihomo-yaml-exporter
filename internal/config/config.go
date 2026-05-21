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
	CheckInterval    time.Duration
	CheckTimeout     time.Duration
	CheckConcurrency int
	IncludeGroups    []string
	ExcludeGroups    []string
	ExcludeProxyRe   *regexp.Regexp
	IncludeProxyRe   *regexp.Regexp
}

func Load() (Config, error) {
	subURL := os.Getenv("SUBSCRIPTION_URL")
	if subURL == "" {
		return Config{}, fmt.Errorf("SUBSCRIPTION_URL is required")
	}

	cfg := Config{
		SubscriptionURL:  subURL,
		ListenAddr:       envOr("LISTEN_ADDR", "0.0.0.0:9123"),
		CheckInterval:    durationEnv("CHECK_INTERVAL", 60*time.Second),
		CheckTimeout:     durationEnv("CHECK_TIMEOUT", 5*time.Second),
		CheckConcurrency: intEnv("CHECK_CONCURRENCY", 20),
		IncludeGroups:    splitCSV(os.Getenv("INCLUDE_GROUPS")),
		ExcludeGroups:    splitCSV(envOr("EXCLUDE_GROUPS", "DIRECT,REJECT,GLOBAL")),
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

	return cfg, nil
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
	if err != nil || n < 1 {
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
