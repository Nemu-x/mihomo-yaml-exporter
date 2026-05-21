# mihomo-yaml-exporter

Prometheus exporter for monitoring Mihomo/Clash YAML subscriptions. Downloads your subscription, parses `proxies` and `proxy-groups`, filters nodes by group and regex rules, runs TCP connectivity checks to `server:port`, and exposes metrics for Grafana.

Alternative to [xray-checker](https://github.com/kutovoys/xray-checker) when the source of truth is a Mihomo YAML config (with proper proxy names and groups), not a raw Xray subscription.

## Features (MVP v1)

- Fetch Mihomo/Clash YAML from `SUBSCRIPTION_URL`
- Filter proxies by `INCLUDE_GROUPS`, `EXCLUDE_GROUPS`, and regex
- TCP availability checks with configurable concurrency
- Prometheus metrics on `/metrics`
- `/health` and `/proxies` debug endpoints
- Keeps last successful proxy list if a fetch fails
- Minimal Docker image (Alpine)

## Quick start

Image: `ghcr.io/nemu-x/mihomo-yaml-exporter:latest` (published to GHCR on push to `main`).

### Docker Compose

```bash
cp .env.example .env
# edit .env — set SUBSCRIPTION_URL (required)
docker compose up -d
```

### Docker

```bash
docker run -d \
  -e SUBSCRIPTION_URL=https://your-panel/sub.yaml \
  -p 9123:9123 \
  ghcr.io/nemu-x/mihomo-yaml-exporter:latest
```

## Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `SUBSCRIPTION_URL` | *(required)* | Mihomo/Clash YAML URL |
| `LISTEN_ADDR` | `0.0.0.0:9123` | HTTP listen address |
| `CHECK_MODE` | `mihomo` | `mihomo` (delay-test) or `tcp` |
| `CHECK_INTERVAL` | `60s` | Subscription refresh and check interval |
| `CHECK_TIMEOUT` | `10s` | Per-proxy check timeout |
| `CHECK_CONCURRENCY` | `5` | Max parallel checks |
| `MIHOMO_DELAY_URL` | gstatic generate_204 | Delay-test target URL |
| `INCLUDE_GROUPS` | *(empty = all proxies)* | Comma-separated proxy-group names |
| `EXCLUDE_GROUPS` | `DIRECT,REJECT,GLOBAL` | Groups ignored when building proxy list |
| `EXCLUDE_PROXY_REGEX` | *(empty)* | Drop proxies whose **name** matches |
| `INCLUDE_PROXY_REGEX` | *(empty)* | Keep only matching proxy names |

## HTTP endpoints

- `GET /` or `GET /ui` — simple web dashboard (no Grafana required)
- `GET /metrics` — Prometheus metrics
- `GET /health` — JSON status (`ok` / `degraded`)
- `GET /proxies` — JSON list of proxies with last check results

## Prometheus metrics

| Metric | Description |
|--------|-------------|
| `mihomo_proxy_up` | 1 = TCP OK, 0 = failed |
| `mihomo_proxy_latency_ms` | Connect latency (ms) |
| `mihomo_proxy_check_timestamp` | Last check Unix time |
| `mihomo_subscription_last_success_timestamp` | Last successful fetch |
| `mihomo_subscription_load_success` | 1 if last fetch OK |
| `mihomo_proxy_total` | Monitored proxy count |
| `mihomo_proxy_online_total` | Proxies up |
| `mihomo_proxy_offline_total` | Proxies down |

### Prometheus scrape config

```yaml
scrape_configs:
  - job_name: mihomo_yaml_exporter
    static_configs:
      - targets: ["mihomo-yaml-exporter:9123"]
```

### Grafana ideas

- Availability: `100 * sum(mihomo_proxy_up) / count(mihomo_proxy_up)`
- Online: `sum(mihomo_proxy_up)`
- Offline: `count(mihomo_proxy_up) - sum(mihomo_proxy_up)`
- Failed table: `mihomo_proxy_up == 0`

## INCLUDE_GROUPS examples

The exporter expands **nested** proxy-groups (e.g. `🌍 Network` → `⚡ Auto` → real nodes).

`SUBSCRIPTION_URL` must return **rendered** YAML with a filled `proxies:` section (client subscription link from your panel). Template files without nodes will not work.

### Nested groups (panel-style config)

```env
INCLUDE_GROUPS=⚡ Auto,📄 Emergency,🌎 Country
EXCLUDE_GROUPS=🚫 No VPN,DIRECT,REJECT,GLOBAL
```

- `⚡ Auto` — primary auto/fallback pool
- `📄 Emergency` — backup nodes
- `🌎 Country` — manual country picker (all nodes)

### Template-based config (after render)

```env
INCLUDE_GROUPS=⚙️ Standard Servers,⚡ Auto (recommended),🛡 Whitelist Mode
EXCLUDE_GROUPS=🚫 No VPN,DIRECT,REJECT,GLOBAL
```

Per-region groups (e.g. `🇩🇪 Germany (all servers)`) can be added if you want regional checks.

## Check modes

- **mihomo** (default): embedded Mihomo with external-controller, delay-test per proxy (same idea as the app).
- **tcp**: TCP/TLS reachability to server:port only (CHECK_MODE=tcp).

## License

MIT
