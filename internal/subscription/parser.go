package subscription

import (
	"fmt"
	"log"
	"regexp"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

type parseOptions struct {
	includeGroups []string
	excludeGroups map[string]struct{}
	excludeRe     *regexp.Regexp
	includeRe     *regexp.Regexp
}

type rawConfig struct {
	Proxies     []rawProxy      `yaml:"proxies"`
	ProxyGroups []rawProxyGroup `yaml:"proxy-groups"`
}

type rawProxy struct {
	Name            string       `yaml:"name"`
	Type            string       `yaml:"type"`
	Server          string       `yaml:"server"`
	Port            flexiblePort `yaml:"port"`
	Network         string       `yaml:"network"`
	Flow            string       `yaml:"flow"`
	PacketEncoding  string       `yaml:"packet-encoding"`
	UDP             bool         `yaml:"udp"`
	TLS             bool         `yaml:"tls"`
	Cipher          string       `yaml:"cipher"`
	Plugin          string       `yaml:"plugin"`
}

type rawProxyGroup struct {
	Name    string   `yaml:"name"`
	Type    string   `yaml:"type"`
	Proxies []string `yaml:"proxies"`
}

type flexiblePort struct {
	value int
	ok    bool
}

var reservedMembers = map[string]struct{}{
	"DIRECT": {}, "REJECT": {}, "GLOBAL": {}, "PASS": {}, "RETURN": {},
}

func (p *flexiblePort) UnmarshalYAML(value *yaml.Node) error {
	if value == nil {
		return nil
	}
	switch value.Kind {
	case yaml.ScalarNode:
		switch value.Tag {
		case "!!int":
			var n int
			if err := value.Decode(&n); err != nil {
				return err
			}
			p.value = n
			p.ok = true
		default:
			var s string
			if err := value.Decode(&s); err != nil {
				return err
			}
			s = strings.TrimSpace(s)
			if s == "" {
				return nil
			}
			n, err := strconv.Atoi(s)
			if err != nil {
				return fmt.Errorf("invalid port %q", s)
			}
			p.value = n
			p.ok = true
		}
	}
	return nil
}

func Parse(data []byte, includeGroups, excludeGroups []string, excludeRe, includeRe *regexp.Regexp) ([]Proxy, error) {
	var cfg rawConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("yaml parse: %w", err)
	}

	excluded := make(map[string]struct{})
	for _, g := range excludeGroups {
		excluded[strings.TrimSpace(g)] = struct{}{}
	}

	opts := parseOptions{
		includeGroups: includeGroups,
		excludeGroups: excluded,
		excludeRe:     excludeRe,
		includeRe:     includeRe,
	}

	byName := make(map[string]Proxy)
	for _, rp := range cfg.Proxies {
		name := strings.TrimSpace(rp.Name)
		if name == "" {
			log.Printf("warn: skip proxy with empty name")
			continue
		}
		server := strings.TrimSpace(rp.Server)
		if server == "" || !rp.Port.ok || rp.Port.value <= 0 {
			log.Printf("warn: skip proxy %q: missing server or port", name)
			continue
		}
		p := Proxy{
			Name:           name,
			Type:           strings.TrimSpace(rp.Type),
			Server:         server,
			Port:           rp.Port.value,
			Network:        strings.TrimSpace(rp.Network),
			Flow:           strings.TrimSpace(rp.Flow),
			PacketEncoding: strings.TrimSpace(rp.PacketEncoding),
			UDP:            rp.UDP,
			TLS:            rp.TLS,
			Cipher:         strings.TrimSpace(rp.Cipher),
			Plugin:         strings.TrimSpace(rp.Plugin),
		}
		p.ProtocolLabel = BuildProtocolLabel(p)
		byName[name] = p
	}

	groupDefs := make(map[string][]string)
	for _, g := range cfg.ProxyGroups {
		gname := strings.TrimSpace(g.Name)
		if gname == "" {
			continue
		}
		if _, skip := excluded[gname]; skip {
			continue
		}
		members := make([]string, 0, len(g.Proxies))
		for _, m := range g.Proxies {
			m = strings.TrimSpace(m)
			if m == "" {
				continue
			}
			if _, skip := reservedMembers[strings.ToUpper(m)]; skip {
				continue
			}
			if _, skip := excluded[m]; skip {
				continue
			}
			members = append(members, m)
		}
		groupDefs[gname] = members
	}

	// direct proxy -> groups (only explicit list membership)
	proxyGroups := make(map[string][]string)
	for gname, members := range groupDefs {
		for _, m := range members {
			if _, ok := byName[m]; ok {
				proxyGroups[m] = append(proxyGroups[m], gname)
			}
		}
	}

	includeSet := make(map[string]struct{})
	for _, g := range includeGroups {
		includeSet[strings.TrimSpace(g)] = struct{}{}
	}

	var selected []Proxy

	if len(includeGroups) == 0 {
		for name, p := range byName {
			p.Groups = uniqueStrings(proxyGroups[name])
			if passFilters(p, opts) {
				selected = append(selected, p)
			}
		}
	} else {
		selectedNames := make(map[string]struct{})
		sourceGroups := make(map[string][]string)

		for inc := range includeSet {
			if _, skip := excluded[inc]; skip {
				continue
			}
			for _, pname := range expandGroup(inc, groupDefs, byName, make(map[string]struct{})) {
				selectedNames[pname] = struct{}{}
				sourceGroups[pname] = append(sourceGroups[pname], inc)
			}
		}

		for pname := range selectedNames {
			p, ok := byName[pname]
			if !ok {
				continue
			}
			groups := uniqueStrings(append(proxyGroups[pname], sourceGroups[pname]...))
			p.Groups = groups
			if passFilters(p, opts) {
				selected = append(selected, p)
			}
		}
	}

	if len(selected) == 0 {
		return nil, fmt.Errorf("no proxies matched filters")
	}
	return selected, nil
}

// expandGroup resolves nested proxy-groups to leaf proxy names.
func expandGroup(name string, groupDefs map[string][]string, byName map[string]Proxy, visiting map[string]struct{}) []string {
	if _, ok := visiting[name]; ok {
		return nil
	}
	visiting[name] = struct{}{}
	defer delete(visiting, name)

	if _, ok := byName[name]; ok {
		return []string{name}
	}

	members, ok := groupDefs[name]
	if !ok {
		return nil
	}

	var out []string
	seen := make(map[string]struct{})
	for _, m := range members {
		var leaves []string
		if _, isProxy := byName[m]; isProxy {
			leaves = []string{m}
		} else {
			leaves = expandGroup(m, groupDefs, byName, visiting)
		}
		for _, leaf := range leaves {
			if _, dup := seen[leaf]; dup {
				continue
			}
			seen[leaf] = struct{}{}
			out = append(out, leaf)
		}
	}
	return out
}

func passFilters(p Proxy, opts parseOptions) bool {
	if opts.excludeRe != nil && opts.excludeRe.MatchString(p.Name) {
		return false
	}
	if opts.includeRe != nil && !opts.includeRe.MatchString(p.Name) {
		return false
	}
	t := strings.ToLower(p.Type)
	if t == "direct" || t == "reject" || t == "block" || t == "dns" {
		return false
	}
	return true
}

func uniqueStrings(in []string) []string {
	if len(in) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, s := range in {
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	return out
}
