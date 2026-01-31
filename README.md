# SCINFRA Bot

Telegram bot for distributed infrastructure management. Control network modes, switch between servers, monitor traffic, and more — all from Telegram.

## Features

- **Edge-gateway control** — Switch network modes (direct/full/split)
- **Multi-upstream support** — Manage multiple servers
- **Server mode control** — Direct IP, WARP, or residential proxy
- **Traffic monitoring** — Track usage and costs
- **Infrastructure monitoring** — Health checks for all servers via Prometheus
- **Inline keyboard** — Quick access to all controls
- **Webhook notifications** — Real-time alerts

### Inline Keyboard UI

```
[🟡 Direct] [🔵 Full] [🟢 Split ✓]
[📍 Primary ✓] [📍 Secondary]
[🖥️ Direct] [☁️ WARP ✓] [🏠 Home]
[🔄 Refresh] [📊 Traffic]
```

### Infrastructure Health UI

```
📊 Infrastructure Health

☁️ Production
  🟢 gateway 📶
  🟢 web-server 📶
  🟢 db-server 📶

☁️ Remote 1
  🟢 vps-primary 📶

☁️ Remote 2
  🟢 vps-secondary 📶

[← Back] [🔄 Refresh]
```

- 🟢/🟡/🛑 — server health (up/degraded/down)
- 📶/❌ — external accessibility

## Quick Start

```bash
# Download latest release
wget https://github.com/scinfra-pro/scinfra-bot/releases/latest/download/scinfra-bot-linux-amd64
chmod +x scinfra-bot-linux-amd64
sudo mv scinfra-bot-linux-amd64 /usr/local/bin/scinfra-bot

# Configure
cp configs/config.example.yaml /etc/scinfra-bot/config.yaml
nano /etc/scinfra-bot/config.yaml

# Run
scinfra-bot -config /etc/scinfra-bot/config.yaml
```

## Documentation

- [Commands](docs/commands.md) — Bot commands and UI
- [Configuration](docs/configuration.md) — Config file reference
- [Infrastructure](docs/infrastructure.md) — Infrastructure monitoring
- [Traffic Monitoring](docs/traffic.md) — Traffic statistics
- [Webhooks](docs/webhooks.md) — Webhook integration

## Building from Source

```bash
git clone https://github.com/scinfra-pro/scinfra-bot.git
cd scinfra-bot
make build
```

## Requirements

- Go 1.21+
- SSH access to edge-gateway
- [switch-gate](https://github.com/scinfra-pro/switch-gate) v0.5+ on VPS (optional, for VPS mode control)
- Prometheus + node_exporter (optional, for infrastructure monitoring)

## License

MIT
