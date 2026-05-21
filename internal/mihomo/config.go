package mihomo

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

func NormalizeController(addr string) string {
	addr = strings.TrimSpace(addr)
	if addr == "" {
		return "127.0.0.1:9090"
	}
	if strings.Contains(addr, "://") {
		u, err := url.Parse(addr)
		if err == nil && u.Host != "" {
			return u.Host
		}
	}
	return strings.TrimPrefix(strings.TrimPrefix(addr, "https://"), "http://")
}

func PatchConfig(raw []byte, controller, secret string) ([]byte, error) {
	var doc map[string]interface{}
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		return nil, fmt.Errorf("yaml unmarshal: %w", err)
	}
	if doc == nil {
		doc = make(map[string]interface{})
	}

	doc["external-controller"] = NormalizeController(controller)
	if secret != "" {
		doc["secret"] = secret
	}
	if _, ok := doc["mixed-port"]; !ok {
		doc["mixed-port"] = 17890
	}
	if _, ok := doc["socks-port"]; !ok {
		doc["socks-port"] = 0
	}
	if _, ok := doc["port"]; !ok {
		doc["port"] = 0
	}

	return yaml.Marshal(doc)
}

func WriteConfig(dir string, content []byte) (string, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, content, 0o644); err != nil {
		return "", err
	}
	return path, nil
}
