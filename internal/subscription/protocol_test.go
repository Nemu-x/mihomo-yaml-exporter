package subscription

import "testing"

func TestBuildProtocolLabel(t *testing.T) {
	p := Proxy{
		Type:    "vless",
		Network: "xhttp",
		TLS:     true,
		Flow:    "xtls-rprx-vision",
	}
	got := BuildProtocolLabel(p)
	if got == "" || got == "unknown" {
		t.Fatalf("unexpected label: %q", got)
	}
}
