# Infrastructure Monitoring

This document describes the infrastructure monitoring feature of scinfra-bot.

## Overview

The infrastructure monitoring feature provides real-time visibility into your distributed infrastructure directly through Telegram, without depending on external access to Grafana or other monitoring tools.

**Key benefits:**
- Works even when external services are down (SSL issues, DNS problems)
- No public IP needed for monitoring server
- Bot is always accessible if monitoring-server is running

## Architecture

```
┌─────────────────────────────────────────────────────────────────────┐
│                         Telegram Bot                                 │
│                      (monitoring-server)                             │
├─────────────────────────────────────────────────────────────────────┤
│                                                                      │
│   Local/Cloud Servers           Remote VPS                          │
│   ┌─────────────────┐           ┌─────────────────────────┐         │
│   │   Prometheus    │           │   SSH ProxyJump         │         │
│   │  localhost:9090 │           │  → jump-host → VPS      │         │
│   │                 │           │                         │         │
│   │  node_exporter  │           │  switch-gate API:9090   │         │
│   │  metrics        │           │  node_exporter:9100     │         │
│   └─────────────────┘           └─────────────────────────┘         │
│                                                                      │
└─────────────────────────────────────────────────────────────────────┘
```

## Metrics Collection

### Local/Cloud Servers

Metrics are collected from Prometheus:

| Metric | PromQL Query |
|--------|--------------|
| IsUp | `up{instance="<server-id>",job="node"}` |
| CPU | `100 - avg(rate(node_cpu_seconds_total{mode="idle"}[5m]))*100` |
| Memory | `(1 - node_memory_MemAvailable_bytes/node_memory_MemTotal_bytes)*100` |
| Disk | `(1 - node_filesystem_avail_bytes/node_filesystem_size_bytes)*100` |
| Uptime | `node_time_seconds - node_boot_time_seconds` |

### Remote VPS

Metrics are collected via SSH ProxyJump to each VPS:

1. **switch-gate API** (`curl http://localhost:9090/status`)
   - Server up/down status
   - Uptime
   - Service health

2. **node_exporter** (`curl http://localhost:9100/metrics`)
   - CPU (load average)
   - Memory usage
   - Disk usage

**SSH path:** `bot-server → jump-host → VPS`

### External Accessibility

All servers are checked for external accessibility:

| Check Type | Example |
|------------|---------|
| HTTPS | `https://your-server.example.com` |
| TCP | `tcp://1.2.3.4:443` |

Checks are performed directly from the bot.

## Caching

To improve responsiveness, health data is cached:

| Action | Cache Behavior |
|--------|----------------|
| `/health` command | Force refresh |
| `📊 Health` button | Use cache if valid |
| `🔄 Refresh` button | Force refresh |
| `← Back` button | Use cache |
| Server click | Use cache |

**Cache TTL:** 60 seconds

After TTL expires, next request fetches fresh data.

## Configuration

### Basic Setup

```yaml
infrastructure:
  enabled: true
  prometheus_url: "http://localhost:9090"
  
  clouds:
    - name: "Production"
      icon: "☁️"
      servers:
        - id: gateway
          name: "gateway"
          icon: "🖥️"
          ip: "10.0.1.10"
          external_check: "https://gateway.example.com"
          services:
            - name: "Nginx"
              job: "nginx"
            - name: "WireGuard"
              port: 51820
```

### Server Configuration

| Field | Description |
|-------|-------------|
| `id` | Unique server identifier (must match Prometheus instance label) |
| `name` | Display name |
| `icon` | Emoji icon |
| `ip` | Internal IP address |
| `external_check` | URL for external accessibility check (`https://...` or `tcp://...`) |
| `services` | List of services to monitor |

### Service Configuration

| Field | Description |
|-------|-------------|
| `name` | Service display name |
| `job` | Prometheus job name (optional) |
| `port` | Port number for display (optional) |

### Remote VPS Setup

For remote VPS with switch-gate:

1. Configure in `upstreams` section with `switch_gate: true`
2. Add to `infrastructure.clouds` with matching IP
3. Install node_exporter on VPS (`apt install prometheus-node-exporter`)

```yaml
upstreams:
  primary:
    name: "Primary VPS"
    ip: "1.2.3.4"
    user: "root"
    switch_gate: true
    switch_gate_port: 9090
  secondary:
    name: "Secondary VPS"
    ip: "5.6.7.8"
    user: "root"
    switch_gate: true
    switch_gate_port: 9090

infrastructure:
  clouds:
    - name: "Remote 1"
      servers:
        - id: vps-primary
          ip: "1.2.3.4"  # Must match upstream IP
          external_check: "tcp://1.2.3.4:443"
          
    - name: "Remote 2"
      servers:
        - id: vps-secondary
          ip: "5.6.7.8"  # Must match upstream IP
          external_check: "tcp://5.6.7.8:443"
```

## Commands

| Command | Description |
|---------|-------------|
| `/infra` | Infrastructure overview with server list |
| `/health` | Health status with metrics |

## Status Icons

| Icon | Meaning |
|------|---------|
| 🟢 | Server up and healthy |
| 🟡 | Server up but degraded (high resource usage or service down) |
| 🛑 | Server down |
| 📶 | Externally accessible |
| ❌ | Not externally accessible |

## Troubleshooting

### "Prometheus not reachable"

- Check if Prometheus is running
- Verify `prometheus_url` in config
- Check network connectivity to Prometheus

### VPS showing as down

- Verify SSH ProxyJump works: `ssh -J user@jump-host root@vps-ip`
- Check switch-gate is running: `systemctl status switch-gate`
- Verify node_exporter: `curl http://localhost:9100/metrics`

### Metrics not updating

- Check cache TTL (60 seconds)
- Use 🔄 Refresh button to force update
- Check bot logs for errors
