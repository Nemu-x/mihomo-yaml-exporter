package subscription

import "strings"

func BuildProtocolLabel(p Proxy) string {
	var parts []string
	add := func(s string) {
		s = strings.TrimSpace(strings.ToLower(s))
		if s == "" {
			return
		}
		for _, x := range parts {
			if x == s {
				return
			}
		}
		parts = append(parts, s)
	}

	add(p.Type)
	if p.Network != "" {
		add(p.Network)
	} else if p.Type != "" {
		add("tcp")
	}
	if p.Flow != "" {
		f := strings.ToLower(p.Flow)
		switch {
		case strings.Contains(f, "vision"):
			add("vision")
		case strings.Contains(f, "xudp"):
			add("xudp")
		default:
			if i := strings.LastIndex(f, "-"); i >= 0 && len(f[i+1:]) <= 12 {
				add(f[i+1:])
			} else {
				add(f)
			}
		}
	}
	if p.PacketEncoding != "" {
		add(strings.ToLower(p.PacketEncoding))
	}
	if p.Plugin != "" {
		add(strings.ToLower(p.Plugin))
	}
	if p.Cipher != "" {
		add(strings.ToLower(p.Cipher))
	}
	if p.TLS {
		add("tls")
	}
	if p.UDP {
		add("udp")
	}
	if len(parts) == 0 {
		return "unknown"
	}
	return strings.Join(parts, " · ")
}
