# Webhook Integration

The bot can receive webhook notifications from switch-gate for real-time alerts.

## Overview

When enabled, the bot runs an HTTP server that receives events from switch-gate instances.

```
switch-gate (VPS) ──POST──> scinfra-bot ──notify──> Telegram
```

## Configuration

### Bot Configuration

```yaml
webhooks:
  enabled: true
  listen: "0.0.0.0:8080"
  secret: "${WEBHOOK_SECRET}"
```

### switch-gate Configuration

Configure each switch-gate instance to send webhooks:

```yaml
webhooks:
  enabled: true
  url: "http://10.0.5.10:8080/webhook/switch-gate"
  secret: "${WEBHOOK_SECRET}"
  source: "primary"
  events:
    mode_changed: false
    limit_reached: true
```

## Events

### mode.changed

Triggered when VPS mode changes.

**Payload:**
```json
{
  "event": "mode.changed",
  "timestamp": "2026-01-28T15:30:00Z",
  "source": "primary",
  "payload": {
    "from": "direct",
    "to": "warp",
    "trigger": "manual"
  }
}
```

**Notification:**
```
🔄 Primary VPS

Mode: direct → warp
```

### limit.reached

Triggered when home proxy limit is reached.

**Payload:**
```json
{
  "event": "limit.reached",
  "timestamp": "2026-01-28T16:00:00Z",
  "source": "primary",
  "payload": {
    "used_mb": 100,
    "limit_mb": 100,
    "switched_to": "warp"
  }
}
```

**Notification:**
```
⚠️ Primary VPS

Home limit reached: 100/100 MB
Auto-switched to: warp
```

## Event Filtering

It's recommended to filter events on the switch-gate side to avoid notification spam.

| Event | Recommended | Reason |
|-------|-------------|--------|
| `mode_changed` | `false` | User already sees mode change via inline buttons |
| `limit_reached` | `true` | Important automatic event |

## Endpoints

### POST /webhook/switch-gate

Receives webhook events from switch-gate.

**Headers:**
- `Content-Type: application/json`
- `X-Webhook-Secret: <secret>` - Required for authentication

**Response:**
- `200 OK` - Event received
- `401 Unauthorized` - Invalid secret
- `400 Bad Request` - Invalid payload

### GET /health

Health check endpoint.

**Response:**
- `200 OK`

## Security

### Authentication

All webhook requests must include the `X-Webhook-Secret` header matching the configured secret.

### Network Isolation

It's recommended to expose the webhook endpoint only on internal networks (e.g., VPN/WireGuard) rather than the public internet.

Example firewall rule:
```bash
# Allow only from VPN subnet
ufw allow from 10.0.100.0/24 to any port 8080
```

## Troubleshooting

### Webhook not received

1. Check if webhook server is enabled: `webhooks.enabled: true`
2. Verify the bot is listening: `netstat -tlnp | grep 8080`
3. Check switch-gate logs for connection errors
4. Verify network connectivity between VPS and bot

### Authentication errors

1. Ensure `X-Webhook-Secret` header is set in switch-gate
2. Verify secrets match in both configurations
3. Check bot logs for "Webhook unauthorized" messages
