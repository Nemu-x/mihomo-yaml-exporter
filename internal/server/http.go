package server

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/nemu-x/mihomo-yaml-exporter/internal/engine"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type HTTPServer struct {
	eng *engine.Engine
}

func New(eng *engine.Engine) *HTTPServer {
	return &HTTPServer{eng: eng}
}

func (s *HTTPServer) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handleWeb)
	mux.HandleFunc("/ui", s.handleWeb)
	mux.Handle("/metrics", promhttp.Handler())
	mux.HandleFunc("/health", s.handleHealth)
	mux.HandleFunc("/proxies", s.handleProxies)
	mux.HandleFunc("/api/meta", s.handleMeta)
	mux.HandleFunc("/api/refresh", s.handleRefresh)
	return mux
}

func (s *HTTPServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	snap := s.eng.Snapshot()

	status := "ok"
	var errMsg string

	if !snap.HasProxies {
		status = "degraded"
		errMsg = "subscription load failed"
	} else if !snap.SubLoadOK && snap.LastSubError != "" {
		status = "degraded"
		errMsg = snap.LastSubError
	} else if snap.LastCheckFailed {
		status = "degraded"
		errMsg = "all proxy checks failed"
	}

	resp := map[string]interface{}{
		"status":          status,
		"proxies_total":   snap.ProxiesTotal,
		"proxies_online":  snap.ProxiesOnline,
		"checking":        snap.CheckInProgress,
	}
	if !snap.LastCheck.IsZero() {
		resp["last_check"] = snap.LastCheck.UTC().Format(time.RFC3339)
	}
	if errMsg != "" {
		resp["error"] = errMsg
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

func (s *HTTPServer) handleProxies(w http.ResponseWriter, r *http.Request) {
	snap := s.eng.Snapshot()
	out := make([]map[string]interface{}, 0, len(snap.Results))

	for _, res := range snap.Results {
		out = append(out, map[string]interface{}{
			"name":       res.Proxy.Name,
			"type":            res.Proxy.Type,
			"protocol_label":  res.Proxy.ProtocolLabel,
			"network":         res.Proxy.Network,
			"flow":            res.Proxy.Flow,
			"tls":             res.Proxy.TLS,
			"udp":             res.Proxy.UDP,
			"server":     res.Proxy.Server,
			"port":       res.Proxy.Port,
			"groups":     res.Proxy.Groups,
			"up":         res.Up,
			"latency_ms": res.LatencyMs,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(out)
}

func (s *HTTPServer) handleMeta(w http.ResponseWriter, r *http.Request) {
	snap := s.eng.Snapshot()
	intervalSec := int(snap.CheckInterval.Seconds())
	if intervalSec < 1 {
		intervalSec = 60
	}

	method := "tcp_with_tls_fallback"
	note := "TCP/TLS reachability from this host. ICMP is not used."
	if snap.CheckMode == "mihomo" {
		method = "mihomo_delay_test"
		note = "Checks use Mihomo external-controller delay test (same idea as the app)."
	}

	resp := map[string]interface{}{
		"check_interval_sec": intervalSec,
		"check_method":       method,
		"check_mode":         snap.CheckMode,
		"check_note":         note,
		"check_icmp":         false,
	}
	if !snap.LastCheck.IsZero() {
		resp["last_check"] = snap.LastCheck.UTC().Format(time.RFC3339)
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

func (s *HTTPServer) handleRefresh(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	queued := s.eng.TriggerRefresh()
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]bool{"queued": queued})
}
