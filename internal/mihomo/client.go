package mihomo

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

type Client struct {
	BaseURL string
	Secret  string
	Client  *http.Client
}

func NewClient(baseURL, secret string) *Client {
	return &Client{
		BaseURL: strings.TrimRight(baseURL, "/"),
		Secret:  secret,
		Client:  &http.Client{},
	}
}

func (c *Client) Ping(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+"/version", nil)
	if err != nil {
		return err
	}
	c.applyAuth(req)
	resp, err := c.Client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("mihomo ping HTTP %d", resp.StatusCode)
	}
	return nil
}

func (c *Client) ReloadConfig(ctx context.Context, configPath string) error {
	body := fmt.Sprintf(`{"path":%q}`, configPath)
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, c.BaseURL+"/configs", strings.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	c.applyAuth(req)
	resp, err := c.Client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("mihomo reload HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(b)))
	}
	return nil
}

func (c *Client) Delay(ctx context.Context, proxyName, testURL string, timeoutMs int) (int, error) {
	seg := escapeProxyName(proxyName)
	u, err := url.Parse(c.BaseURL + "/proxies/" + seg + "/delay")
	if err != nil {
		return 0, err
	}
	q := u.Query()
	q.Set("url", testURL)
	q.Set("timeout", fmt.Sprintf("%d", timeoutMs))
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return 0, err
	}
	c.applyAuth(req)
	resp, err := c.Client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 8192))
	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("delay HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var out struct {
		Delay int `json:"delay"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return 0, err
	}
	if out.Delay <= 0 {
		return 0, fmt.Errorf("delay test failed")
	}
	return out.Delay, nil
}

func (c *Client) applyAuth(req *http.Request) {
	if c.Secret != "" {
		req.Header.Set("Authorization", "Bearer "+c.Secret)
	}
}

func escapeProxyName(name string) string {
	return url.PathEscape(name)
}
