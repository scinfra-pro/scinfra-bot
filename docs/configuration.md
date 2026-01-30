# Configuration

The bot is configured via a YAML file. Default location: `/etc/scinfra-bot/config.yaml`

## Quick Start

```bash
# Copy example config
cp configs/config.example.yaml /etc/scinfra-bot/config.yaml

# Edit with your values
nano /etc/scinfra-bot/config.yaml

# Run the bot
./scinfra-bot -config /etc/scinfra-bot/config.yaml
```

## Configuration File Structure

```yaml
telegram:
  token: "${TELEGRAM_BOT_TOKEN}"
  allowed_chat_ids:
    - 123456789

edge:
  name: "Cloud Provider"
  host: "user@10.0.1.11"
  key_path: ""
  vpn_mode_script: "/usr/local/bin/vpn-mode.sh"

upstreams:
  primary:
    name: "Primary VPS"
    ip: "1.2.3.4"
    user: "root"
    switch_gate: true
    switch_gate_port: 9090

webhooks:
  enabled: true
  listen: "0.0.0.0:8080"
  secret: "${WEBHOOK_SECRET}"

logging:
  level: "info"
```

## Sections

### telegram

| Field | Required | Description |
|-------|----------|-------------|
| `token` | Yes | Bot token from @BotFather |
| `allowed_chat_ids` | Yes | List of Telegram chat IDs allowed to use the bot |

### edge

Edge-gateway SSH connection settings.

| Field | Required | Default | Description |
|-------|----------|---------|-------------|
| `name` | No | "Edge Gateway" | Display name in traffic statistics |
| `host` | Yes | - | SSH host in format `user@host` |
| `key_path` | No | - | Path to SSH private key. If not set, uses SSH agent |
| `vpn_mode_script` | No | `/usr/local/bin/vpn-mode.sh` | Path to VPN mode script on edge-gateway |

### upstreams

Map of upstream VPS servers. Each key becomes a command name.

| Field | Required | Default | Description |
|-------|----------|---------|-------------|
| `name` | No | Capitalized key | Display name for the upstream |
| `ip` | Yes | - | VPS IP address |
| `user` | No | `root` | SSH user |
| `switch_gate` | No | `false` | Enable switch-gate API integration |
| `switch_gate_port` | No | `9090` | switch-gate API port |

Example with two upstreams:

```yaml
upstreams:
  primary:
    name: "Primary VPS"
    ip: "${PRIMARY_VPS_IP}"
    user: "root"
    switch_gate: true
  backup:
    name: "Backup VPS"
    ip: "${BACKUP_VPS_IP}"
    user: "root"
    switch_gate: true
```

This creates commands: `/upstream_primary`, `/upstream_backup`

### webhooks

Webhook receiver for notifications from switch-gate.

| Field | Required | Default | Description |
|-------|----------|---------|-------------|
| `enabled` | No | `false` | Enable webhook receiver |
| `listen` | No | `0.0.0.0:8080` | Listen address |
| `secret` | No | - | Shared secret for authentication |

### logging

| Field | Required | Default | Description |
|-------|----------|---------|-------------|
| `level` | No | `info` | Log level: debug, info, warn, error |

## Environment Variables

Configuration supports environment variable expansion using `${VAR}` syntax.

Example:
```yaml
telegram:
  token: "${TELEGRAM_BOT_TOKEN}"

upstreams:
  primary:
    ip: "${PRIMARY_VPS_IP}"
```

Set variables before running:
```bash
export TELEGRAM_BOT_TOKEN="123456789:ABC..."
export PRIMARY_VPS_IP="1.2.3.4"
./scinfra-bot -config config.yaml
```

## Command Line Options

```bash
./scinfra-bot [options]

Options:
  -config string
        Config file path (default "/etc/scinfra-bot/config.yaml")
  -version
        Show version and exit
```

## Systemd Service

Example systemd unit file:

```ini
# /etc/systemd/system/scinfra-bot.service
[Unit]
Description=SCINFRA Telegram Bot
After=network.target

[Service]
Type=simple
User=bot
EnvironmentFile=/etc/scinfra-bot/env
ExecStart=/usr/local/bin/scinfra-bot -config /etc/scinfra-bot/config.yaml
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
```

Environment file:
```bash
# /etc/scinfra-bot/env
TELEGRAM_BOT_TOKEN=123456789:ABC...
WEBHOOK_SECRET=your-secret-here
```
