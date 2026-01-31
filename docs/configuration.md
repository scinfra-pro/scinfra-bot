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

### Minimal (S3 Dynamic Mode)

```yaml
# Secrets only - infrastructure from S3
telegram:
  token: "${TELEGRAM_BOT_TOKEN}"
  allowed_chat_ids:
    - 123456789

s3:
  enabled: true
  bucket: "my-storage"
  endpoint: "https://s3.example.com"
  region: "us-east-1"
  providers: ["cloud.json", "upstream1.json"]

webhooks:
  enabled: true
  secret: "${WEBHOOK_SECRET}"

infrastructure:
  prometheus_url: "http://localhost:9090"
```

### Full (Static YAML Mode)

```yaml
telegram:
  token: "${TELEGRAM_BOT_TOKEN}"
  allowed_chat_ids:
    - 123456789

# s3.enabled: false (or omit s3 section entirely)

edge:
  name: "Cloud Provider"
  host: "user@gateway.example.com"
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

infrastructure:
  enabled: true
  prometheus_url: "http://localhost:9090"
  clouds:
    - name: "Production"
      servers:
        - id: gateway
          ip: "10.0.1.10"
```

## Configuration Modes

The bot supports two configuration modes:

| Mode | Description |
|------|-------------|
| **Static (YAML)** | All configuration in YAML file |
| **Dynamic (S3)** | Infrastructure loaded from S3, secrets in YAML |

### Static Mode (Default)

Configure everything in YAML. Simple and straightforward.

### Dynamic Mode (S3)

Infrastructure metadata (edge, upstreams, servers) is loaded from S3-compatible storage. Useful when infrastructure is managed by Terraform - metadata is generated and uploaded automatically after `terraform apply`.

**Benefits:**
- Single source of truth (Terraform)
- No manual config updates when infrastructure changes
- Automatic fallback to YAML if S3 is unavailable

## Sections

### s3

Dynamic configuration from S3-compatible storage.

| Field | Required | Default | Description |
|-------|----------|---------|-------------|
| `enabled` | No | `false` | Enable S3 metadata loading |
| `bucket` | Yes* | - | S3 bucket name |
| `prefix` | No | `metadata/` | Object key prefix |
| `endpoint` | Yes* | - | S3-compatible endpoint URL |
| `region` | Yes* | - | S3 region |
| `profile` | No | `default` | AWS CLI profile name |
| `providers` | Yes* | - | List of metadata JSON files to load |

*Required when `enabled: true`

Example:

```yaml
s3:
  enabled: true
  bucket: "my-storage"
  prefix: "metadata/"
  endpoint: "https://s3.example.com"
  region: "us-east-1"
  profile: "default"
  providers:
    - "cloud.json"      # Main cloud (edge + servers)
    - "upstream1.json"  # VPS upstream
    - "upstream2.json"  # VPS upstream
```

**Behavior:**

| `s3.enabled` | S3 Available | Result |
|--------------|--------------|--------|
| `false` | - | YAML only |
| `true` | Yes | S3 data (edge, upstreams, clouds) replaces YAML |
| `true` | No | Warning + YAML fallback |

**Metadata JSON format:**

Each provider file should contain:

```json
{
  "schema_version": "1.0",
  "provider": "cloud-name",
  "cloud": {"name": "Cloud Name", "icon": "☁️"},
  "servers": [...],
  "edge": {...},
  "upstream": {...}
}
```

See Terraform integration documentation for generating metadata files.

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

### infrastructure

Infrastructure monitoring configuration. Enables `/infra` and `/health` commands.

| Field | Required | Default | Description |
|-------|----------|---------|-------------|
| `enabled` | No | `false` | Enable infrastructure monitoring |
| `prometheus_url` | No | `http://localhost:9090` | Prometheus API URL |
| `clouds` | No | `[]` | List of cloud providers with servers |

#### Cloud Configuration

| Field | Required | Default | Description |
|-------|----------|---------|-------------|
| `name` | Yes | - | Cloud provider name (e.g., "Production", "Staging") |
| `icon` | No | `☁️` | Emoji icon for the cloud |
| `servers` | Yes | - | List of servers |

#### Server Configuration

| Field | Required | Default | Description |
|-------|----------|---------|-------------|
| `id` | Yes | - | Unique server identifier |
| `name` | No | Same as `id` | Display name |
| `icon` | No | `🖥️` | Emoji icon |
| `ip` | Yes | - | Internal IP address for Prometheus queries |
| `external_check` | No | - | URL for external accessibility check |
| `services` | No | `[]` | List of services to monitor |

**External check formats:**
- `https://example.com` - HTTPS check (accepts 2xx/3xx)
- `http://example.com` - HTTP check
- `tcp://1.2.3.4:443` - TCP port check

#### Service Configuration

| Field | Required | Default | Description |
|-------|----------|---------|-------------|
| `name` | Yes | - | Service display name |
| `job` | No | - | Prometheus job name for health check |
| `port` | No | - | Port number (for display) |

Example infrastructure configuration:

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
              
        - id: web-server
          name: "web-server"
          icon: "🌐"
          ip: "10.0.2.10"
          external_check: "https://web.example.com"
          
    - name: "Remote 1"
      icon: "☁️"
      servers:
        - id: vps-primary
          name: "vps-primary"
          icon: "📍"
          ip: "1.2.3.4"
          external_check: "tcp://1.2.3.4:443"
          
    - name: "Remote 2"
      icon: "☁️"
      servers:
        - id: vps-secondary
          name: "vps-secondary"
          icon: "📍"
          ip: "5.6.7.8"
          external_check: "tcp://5.6.7.8:443"
```

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
