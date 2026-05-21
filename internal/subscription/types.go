package subscription

type Proxy struct {
	Name   string
	Type   string
	Server string
	Port   int
	Groups []string
}

type CheckResult struct {
	Proxy     Proxy
	Up        bool
	LatencyMs float64
}
