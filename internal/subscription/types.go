package subscription

type Proxy struct {
	Name            string
	Type            string
	Server          string
	Port            int
	Network         string
	Flow            string
	PacketEncoding  string
	UDP             bool
	TLS             bool
	Cipher          string
	Plugin          string
	ProtocolLabel   string
	Groups          []string
}

type CheckResult struct {
	Proxy     Proxy
	Up        bool
	LatencyMs float64
}
