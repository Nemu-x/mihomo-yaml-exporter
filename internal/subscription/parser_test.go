package subscription

import (
	"regexp"
	"testing"
)

const sampleYAML = `
proxies:
  - name: de-freakhost
    type: vless
    server: example.com
    port: 443
  - name: DIRECT
    type: direct
  - name: bad-no-port
    type: vmess
    server: x.example.com

proxy-groups:
  - name: Auto
    type: url-test
    proxies:
      - de-freakhost
      - nl-vdsina
  - name: DIRECT
    type: select
    proxies:
      - DIRECT
  - name: Proxy
    type: select
    proxies:
      - de-freakhost
`

func TestParseIncludeGroups(t *testing.T) {
	proxies, err := Parse([]byte(sampleYAML), []string{"Auto", "Proxy"}, []string{"DIRECT", "REJECT", "GLOBAL"}, nil, nil)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(proxies) != 1 {
		t.Fatalf("expected 1 proxy, got %d", len(proxies))
	}
	if proxies[0].Name != "de-freakhost" {
		t.Fatalf("unexpected proxy %q", proxies[0].Name)
	}
}

func TestParseAllProxies(t *testing.T) {
	proxies, err := Parse([]byte(sampleYAML), nil, []string{"DIRECT"}, nil, nil)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(proxies) != 1 {
		t.Fatalf("expected 1 proxy without direct, got %d", len(proxies))
	}
}

func TestExcludeRegex(t *testing.T) {
	re := regexp.MustCompile(`(?i)hidden`)
	yaml := `
proxies:
  - name: hidden-node
    type: vless
    server: a.com
    port: 443
  - name: visible
    type: vless
    server: b.com
    port: 443
proxy-groups: []
`
	proxies, err := Parse([]byte(yaml), nil, nil, re, nil)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(proxies) != 1 || proxies[0].Name != "visible" {
		t.Fatalf("unexpected proxies: %+v", proxies)
	}
}

const nestedYAML = `
proxies:
  - name: "🇩🇪 Германия🥬🚀"
    type: vless
    server: de.example.com
    port: 443
  - name: "🇺🇸 США ⚡🚀"
    type: vless
    server: us.example.com
    port: 443

proxy-groups:
  - name: "🌍 Сеть"
    type: select
    proxies:
      - "🧠 Умный режим"
      - "⚡ Авто"
      - "🚫 Без VPN"
  - name: "🧠 Умный режим"
    type: fallback
    proxies:
      - "⚡ Авто"
  - name: "⚡ Авто"
    type: fallback
    proxies:
      - "🇩🇪 Германия🥬🚀"
      - "🇺🇸 США ⚡🚀"
  - name: "🚫 Без VPN"
    type: select
    proxies:
      - DIRECT
`

func TestParseNestedGroups(t *testing.T) {
	proxies, err := Parse([]byte(nestedYAML), []string{"🌍 Сеть"}, []string{"🚫 Без VPN", "DIRECT", "REJECT", "GLOBAL"}, nil, nil)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(proxies) != 2 {
		t.Fatalf("expected 2 proxies from nested group, got %d", len(proxies))
	}
	names := map[string]bool{proxies[0].Name: true, proxies[1].Name: true}
	if !names["🇩🇪 Германия🥬🚀"] || !names["🇺🇸 США ⚡🚀"] {
		t.Fatalf("unexpected proxy set: %+v", proxies)
	}
}

func TestParseNestedAutoGroup(t *testing.T) {
	proxies, err := Parse([]byte(nestedYAML), []string{"⚡ Авто"}, nil, nil, nil)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(proxies) != 2 {
		t.Fatalf("expected 2 proxies, got %d", len(proxies))
	}
}
