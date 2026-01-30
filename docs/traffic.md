# Traffic Monitoring

The bot collects traffic statistics from multiple sources for cost monitoring.

## Overview

| Source | What is measured | How it's collected |
|--------|------------------|-------------------|
| Edge Gateway | Outbound cloud traffic | SSH → script |
| VPS Upstreams | Proxy traffic by mode | switch-gate API |

## Traffic Command

Use `/traffic` or the 📊 Traffic button to view statistics.

Example output:

```
📈 Traffic Statistics

Cloud Provider:
├ Direct: 70.56 MB
├ VPN: 126.04 MB
├ Total: 0.19 GB
└ 💰 Free (0.19/10 GB)

Primary (current):
├ Direct: 150.50 MB
├ WARP: 2300.00 MB
├ Home: 45.30 / 100 MB
└ 💰 Cost: $0.16

Secondary:
├ Direct: 0.00 MB
├ WARP: 0.00 MB
├ Home: 0.00 / 100 MB
└ 💰 Cost: $0.00
```

## Edge Gateway Traffic

The edge gateway provides outbound traffic statistics from the cloud provider.

### Data Format

The bot calls a script on the edge gateway via SSH:

```bash
/usr/local/bin/yc-traffic.sh
```

Expected JSON output:

```json
{
  "timestamp": "2026-01-29T12:00:00Z",
  "interfaces": {
    "eth0": {
      "name": "External",
      "tx_bytes": 206438400,
      "tx_mb": 196.85
    },
    "wg0": {
      "name": "WireGuard VPN",
      "tx_bytes": 132120576,
      "tx_mb": 126.04
    }
  },
  "summary": {
    "direct_mb": 70.81,
    "vpn_mb": 126.04,
    "total_mb": 196.85,
    "total_gb": 0.19
  },
  "billing": {
    "free_quota_gb": 10,
    "billable_gb": 0,
    "rate_rub_per_gb": 0.96,
    "cost_rub": 0.00
  }
}
```

### Fields

| Field | Description |
|-------|-------------|
| `summary.direct_mb` | Traffic from edge gateway itself |
| `summary.vpn_mb` | Traffic from VPN clients |
| `summary.total_gb` | Total outbound traffic |
| `billing.free_quota_gb` | Free quota per month |
| `billing.billable_gb` | Traffic exceeding free quota |
| `billing.cost_rub` | Estimated cost |

## VPS Traffic (switch-gate)

Each VPS with switch-gate provides traffic statistics via API.

### API Endpoint

```
GET http://127.0.0.1:9090/status
```

### Response Format

```json
{
  "mode": "warp",
  "traffic": {
    "direct_mb": 150.5,
    "warp_mb": 2300.0,
    "home_mb": 45.3,
    "total_mb": 2495.8
  },
  "home": {
    "limit_mb": 100,
    "used_mb": 45.3,
    "remaining_mb": 54.7,
    "cost_usd": 0.16
  }
}
```

### Traffic Modes

| Mode | Description | Cost |
|------|-------------|------|
| Direct | VPS IP address | Usually free |
| WARP | Cloudflare WARP tunnel | Free |
| Home | Residential proxy | ~$3.50/GB |

## Cost Calculation

### Edge Gateway (Cloud)

Typical cloud pricing structure:
- Free quota: 10 GB/month
- After quota: ~$0.01/GB

### VPS Home Proxy

Residential proxy traffic is typically expensive:
- Rate: ~$3.50/GB
- Daily limit: configurable (e.g., 100 MB)

The bot shows:
- Current usage vs limit
- Estimated cost in USD
