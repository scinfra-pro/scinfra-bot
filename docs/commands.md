# Bot Commands

This document describes all available bot commands and UI elements.

## General Commands

| Command | Description |
|---------|-------------|
| `/start` | Welcome message |
| `/help` | Show available commands |
| `/status` | Full VPN status with inline buttons |
| `/ip` | Current external IP address |
| `/traffic` | Traffic statistics |

## Edge-gateway Commands

Control the edge-gateway VPN mode.

| Command | Description |
|---------|-------------|
| `/edge` | Show current edge-gateway mode |
| `/edge_direct` | Switch to direct mode (no VPN) |
| `/edge_full` | Switch to full VPN mode |
| `/edge_split` | Switch to split tunneling mode |

### Mode Icons

| Mode | Icon | Description |
|------|------|-------------|
| direct | 🟡 | No VPN - direct connection |
| full | 🔵 | All traffic through VPN tunnel |
| split | 🟢 | Split tunneling - optimal mode |

## Upstream Commands

Switch between VPS servers. Commands are generated dynamically from configuration.

| Command | Description |
|---------|-------------|
| `/upstream` | Show current upstream server |
| `/upstream_<name>` | Switch to specified upstream |

Example: If you have upstreams `primary` and `secondary` in config, commands will be `/upstream_primary` and `/upstream_secondary`.

## VPS Commands

Control switch-gate mode on the current upstream VPS.

| Command | Description |
|---------|-------------|
| `/vps` | Show VPS mode and traffic |
| `/vps_direct` | Use VPS direct IP |
| `/vps_warp` | Use Cloudflare WARP |
| `/vps_home` | Use residential IP |

### VPS Mode Icons

| Mode | Icon | Description |
|------|------|-------------|
| direct | 🖥️ | VPS IP address |
| warp | ☁️ | Cloudflare WARP tunnel |
| home | 🏠 | Residential proxy IP |

## Infrastructure Commands

Monitor all servers and services in your infrastructure.

| Command | Description |
|---------|-------------|
| `/infra` | Infrastructure overview with server buttons |
| `/health` | Health status with metrics and external checks |

### Infrastructure View

The `/infra` command shows an overview of all configured servers:

```
🏗️ Infrastructure

☁️ Production
  • 🖥️ gateway (10.0.1.10)
  • 🌐 web-server (10.0.2.10)
  • 🗄️ db-server (10.0.3.10)

☁️ Remote 1
  • 📍 vps-primary (1.2.3.4)

☁️ Remote 2
  • 📍 vps-secondary (5.6.7.8)

[🔄 Refresh] [📊 Health]
```

### Health View

The `/health` command (or Health button) shows server status with indicators:

```
📊 Infrastructure Health

☁️ Production
  🟢 gateway 📶
  🟡 web-server ❌
  🟢 db-server 📶

☁️ Remote 1
  🟢 vps-primary 📶

☁️ Remote 2
  🟢 vps-secondary 📶

🔗 Grafana: http://localhost:3000 (VPN)
```

### Status Icons

| Icon | Meaning |
|------|---------|
| 🟢 | Server is up and healthy |
| 🟡 | Server is up but degraded (high CPU/RAM/Disk or service down) |
| 🛑 | Server is down |
| 📶 | Externally accessible (HTTPS/TCP check passed) |
| ❌ | Not externally accessible |

### Server Detail View

Click on any server button to see detailed information:

```
🖥️ gateway (10.0.1.10)
Status: 🟢 up
External: 📶 accessible (45ms)

📦 Services:
  • Nginx ✅ (:443)
  • WireGuard ✅ (:51820)

💻 Resources:
  • CPU: 15% ▓░░░░░░░░░
  • RAM: 45% ▓▓▓▓░░░░░░ (0.9/2.0 GB)
  • Disk: 35% ▓▓▓░░░░░░░ (3/10 GB)

⏱️ Uptime: 14d 3h 22m

[← Back] [🔄 Refresh]
```

## Admin Commands

| Command | Description |
|---------|-------------|
| `/restart` | Show restart services menu |
| `/restart_sg` | Restart switch-gate on current upstream |
| `/restart_sg_<name>` | Restart switch-gate on specified upstream |

## Inline Keyboard

The `/status` command shows an inline keyboard with buttons:

```
[🟡 Direct] [🔵 Full] [🟢 Split ✓]
[📍 Primary ✓] [📍 Secondary]
[🖥️ Direct] [☁️ WARP ✓] [🏠 Home]
[🔄 Refresh] [📊 Traffic]
```

- Current mode is marked with ✓
- Failed mode is marked with ❌ (when health check fails)
- Refresh button performs a health check on the current VPS mode

## Message Status Icons

| Icon | Meaning |
|------|---------|
| ℹ️ | Information / status |
| ✅ | Success |
| ❌ | Error |
| 🔧 | Help |
| 👋 | Welcome |
| ⏳ | In progress |

## Fallback Behavior

When switching VPS modes, if the requested mode fails to activate:

1. The mode button shows ❌ indicator
2. Toast notification shows the error
3. Traffic falls back to a working mode (usually direct)
4. After 5 seconds, the ❌ indicator is cleared

Use the **Refresh** button to perform a health check and see the actual mode status.
