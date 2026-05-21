package subscription

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type Fetcher struct {
	URL    string
	Client *http.Client
}

func NewFetcher(url string) *Fetcher {
	return &Fetcher{
		URL: url,
		Client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (f *Fetcher) Fetch(ctx context.Context) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, f.URL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "mihomo-yaml-exporter/1.0")

	resp, err := f.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("subscription HTTP %d", resp.StatusCode)
	}

	const maxBody = 32 << 20
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBody))
	if err != nil {
		return nil, err
	}
	return body, nil
}

// RedactURL masks tokens in subscription URLs for safe logging.
func RedactURL(raw string) string {
	if raw == "" {
		return ""
	}
	// Query params that often carry secrets
	for _, key := range []string{"token", "access_token", "auth", "key", "secret", "password"} {
		if i := strings.Index(strings.ToLower(raw), key+"="); i >= 0 {
			start := i + len(key) + 1
			end := start
			for end < len(raw) && raw[end] != '&' && raw[end] != '#' {
				end++
			}
			return raw[:start] + "***" + raw[end:]
		}
	}
	// Long path segment (common token-in-path pattern)
	if u := strings.Index(raw, "://"); u >= 0 {
		rest := raw[u+3:]
		slash := strings.Index(rest, "/")
		if slash >= 0 {
			path := rest[slash+1:]
			if len(path) > 24 && !strings.Contains(path, ".") {
				return raw[:u+3+slash+1] + "***"
			}
		}
	}
	return raw
}
