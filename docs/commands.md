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
