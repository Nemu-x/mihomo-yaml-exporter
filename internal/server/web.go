package server

import (
	"net/http"
)

const dashboardHTML = `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>Mihomo Proxy Status</title>
  <style>
    :root { color-scheme: dark light; --bg: #0f1419; --card: #1a2332; --text: #e7ecf3; --muted: #8b9bb4; --up: #3dd68c; --down: #f87171; --warn: #fbbf24; }
    @media (prefers-color-scheme: light) { :root { --bg: #f4f6f9; --card: #fff; --text: #1a2332; --muted: #5a6b82; } }
    * { box-sizing: border-box; }
    body { font-family: system-ui, sans-serif; background: var(--bg); color: var(--text); margin: 0; padding: 1.5rem; }
    h1 { font-size: 1.35rem; margin: 0 0 1rem; }
    .summary { display: flex; gap: 1rem; flex-wrap: wrap; margin-bottom: 1.5rem; }
    .card { background: var(--card); border-radius: 10px; padding: 1rem 1.25rem; min-width: 140px; }
    .card .label { color: var(--muted); font-size: 0.8rem; }
    .card .value { font-size: 1.6rem; font-weight: 600; }
    .status-ok { color: var(--up); } .status-degraded { color: var(--warn); }
    table { width: 100%; border-collapse: collapse; background: var(--card); border-radius: 10px; overflow: hidden; }
    th, td { padding: 0.65rem 1rem; text-align: left; border-bottom: 1px solid rgba(128,128,128,0.15); }
    th { color: var(--muted); font-size: 0.75rem; text-transform: uppercase; }
    .badge { display: inline-block; padding: 0.15rem 0.5rem; border-radius: 4px; font-size: 0.75rem; font-weight: 600; }
    .badge-up { background: rgba(61,214,140,0.2); color: var(--up); }
    .badge-down { background: rgba(248,113,113,0.2); color: var(--down); }
    .muted { color: var(--muted); font-size: 0.85rem; margin-top: 1rem; }
    #error { color: var(--down); margin-bottom: 1rem; }
  </style>
</head>
<body>
  <h1>⚡ Mihomo YAML Exporter</h1>
  <div id="error"></div>
  <div class="summary" id="summary"></div>
  <table><thead><tr><th>Status</th><th>Name</th><th>Type</th><th>Server</th><th>Latency</th><th>Groups</th></tr></thead>
  <tbody id="rows"></tbody></table>
  <p class="muted">Auto-refresh every 30s · <a href="/metrics">/metrics</a> · <a href="/health">/health</a></p>
  <script>
    function esc(s) { var d = document.createElement('div'); d.textContent = s; return d.innerHTML; }
    function refresh() {
      Promise.all([fetch('/health').then(function(r){return r.json();}), fetch('/proxies').then(function(r){return r.json();})])
      .then(function(data) {
        var health = data[0], proxies = data[1];
        document.getElementById('error').textContent = health.error || '';
        var pct = health.proxies_total ? Math.round(100 * health.proxies_online / health.proxies_total) : 0;
        document.getElementById('summary').innerHTML =
          '<div class="card"><div class="label">Status</div><div class="value status-' + health.status + '">' + health.status.toUpperCase() + '</div></div>' +
          '<div class="card"><div class="label">Online</div><div class="value">' + health.proxies_online + ' / ' + health.proxies_total + '</div></div>' +
          '<div class="card"><div class="label">Availability</div><div class="value">' + pct + '%</div></div>' +
          '<div class="card"><div class="label">Last check</div><div class="value" style="font-size:0.95rem">' + (health.last_check || '—') + '</div></div>';
        proxies.sort(function(a,b){ if (a.up === b.up) return 0; return a.up ? -1 : 1; });
        document.getElementById('rows').innerHTML = proxies.map(function(p) {
          return '<tr><td><span class="badge ' + (p.up ? 'badge-up' : 'badge-down') + '">' + (p.up ? 'UP' : 'DOWN') + '</span></td>' +
            '<td>' + esc(p.name) + '</td><td>' + esc(p.type) + '</td><td>' + esc(p.server) + ':' + p.port + '</td>' +
            '<td>' + (p.up ? p.latency_ms + ' ms' : '—') + '</td><td>' + esc((p.groups || []).join(', ')) + '</td></tr>';
        }).join('');
      }).catch(function(e) { document.getElementById('error').textContent = 'Failed to load: ' + e; });
    }
    refresh(); setInterval(refresh, 30000);
  </script>
</body>
</html>`

func (s *HTTPServer) handleWeb(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" && r.URL.Path != "/ui" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(dashboardHTML))
}
