package telegram

import (
	"fmt"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"github.com/scinfra-pro/scinfra-bot/internal/health"
)

// handleInfra handles the /infra command - infrastructure overview
func (b *Bot) handleInfra(msg *tgbotapi.Message) {
	if !b.config.IsInfrastructureEnabled() {
		b.reply(msg.Chat.ID, "❌ Infrastructure monitoring is not configured.\n\nAdd <code>infrastructure</code> section to config.yaml")
		return
	}

	text, keyboard := b.buildInfraMessage()
	b.replyWithKeyboard(msg.Chat.ID, text, keyboard)
}

// handleHealth handles the /health command - infrastructure health status
func (b *Bot) handleHealth(msg *tgbotapi.Message) {
	if !b.config.IsInfrastructureEnabled() {
		b.reply(msg.Chat.ID, "❌ Infrastructure monitoring is not configured.\n\nAdd <code>infrastructure</code> section to config.yaml")
		return
	}

	if b.healthChecker == nil {
		b.reply(msg.Chat.ID, "❌ Health checker not initialized")
		return
	}

	// Check Prometheus connectivity first
	if err := b.healthChecker.Ping(); err != nil {
		b.reply(msg.Chat.ID, fmt.Sprintf("❌ Prometheus not reachable: %v\n\nCheck if Prometheus is running on monitoring-server.", err))
		return
	}

	text, keyboard := b.buildHealthMessage(true) // force refresh on /health command
	b.replyWithKeyboard(msg.Chat.ID, text, keyboard)
}

// buildInfraMessage builds the infrastructure overview message
func (b *Bot) buildInfraMessage() (string, tgbotapi.InlineKeyboardMarkup) {
	var sb strings.Builder

	sb.WriteString("🏗️ <b>Infrastructure</b>\n")

	for _, cloud := range b.config.Infrastructure.Clouds {
		sb.WriteString(fmt.Sprintf("\n%s <b>%s</b>\n", cloud.Icon, cloud.Name))
		for _, server := range cloud.Servers {
			sb.WriteString(fmt.Sprintf("  • %s %s (<code>%s</code>)\n", server.Icon, server.Name, server.IP))
		}
	}

	keyboard := b.buildInfraKeyboard()
	return sb.String(), keyboard
}

// buildHealthMessage builds the health status message
// force=true bypasses cache and fetches fresh data
func (b *Bot) buildHealthMessage(force bool) (string, tgbotapi.InlineKeyboardMarkup) {
	var statuses []*health.ServerStatus
	var err error

	if force {
		statuses, err = b.healthChecker.CheckAllForce()
	} else {
		statuses, err = b.healthChecker.CheckAll()
	}

	if err != nil {
		return fmt.Sprintf("❌ Error checking health: %v", err), tgbotapi.InlineKeyboardMarkup{}
	}

	var sb strings.Builder
	sb.WriteString("📊 <b>Infrastructure Health</b>\n")

	// Group by cloud
	cloudStatuses := make(map[string][]*health.ServerStatus)
	for _, status := range statuses {
		cloudStatuses[status.CloudName] = append(cloudStatuses[status.CloudName], status)
	}

	// Iterate over clouds in order
	for _, cloud := range b.config.Infrastructure.Clouds {
		servers := cloudStatuses[cloud.Name]
		if len(servers) == 0 {
			continue
		}

		sb.WriteString(fmt.Sprintf("\n%s <b>%s</b>\n", cloud.Icon, cloud.Name))

		for _, status := range servers {
			statusIcon := status.GetStatusIcon()
			externalIcon := status.GetExternalIcon()
			sb.WriteString(fmt.Sprintf("  %s %s %s\n", statusIcon, status.Name, externalIcon))
		}
	}

	// Add Grafana VPN link
	sb.WriteString("\n🔗 <b>Grafana:</b> <code>http://10.0.5.10:3000</code> (VPN)")

	keyboard := b.buildHealthKeyboard(statuses)
	return sb.String(), keyboard
}

// buildServerDetailMessage builds detailed server status message
// source is "overview" or "health" - determines where Back button leads
// force=true bypasses cache and fetches fresh data
func (b *Bot) buildServerDetailMessage(serverID, source string, force bool) (string, tgbotapi.InlineKeyboardMarkup) {
	var status *health.ServerStatus
	var err error

	if force {
		status, err = b.healthChecker.CheckServerForce(serverID)
	} else {
		status, err = b.healthChecker.CheckServer(serverID)
	}

	if err != nil {
		return fmt.Sprintf("❌ Error: %v", err), tgbotapi.InlineKeyboardMarkup{}
	}

	var sb strings.Builder

	// Header
	sb.WriteString(fmt.Sprintf("%s <b>%s</b> (<code>%s</code>)\n", status.Icon, status.Name, status.IP))
	sb.WriteString(fmt.Sprintf("Status: %s %s\n", status.GetStatusIcon(), status.GetStatusLevel()))

	// External access
	if status.ExternalAccess {
		sb.WriteString(fmt.Sprintf("External: %s accessible", status.GetExternalIcon()))
		if status.ExternalLatency > 0 {
			sb.WriteString(fmt.Sprintf(" (%dms)", status.ExternalLatency.Milliseconds()))
		}
		sb.WriteString("\n")
	} else {
		sb.WriteString(fmt.Sprintf("External: %s not accessible", status.GetExternalIcon()))
		if status.ExternalError != "" {
			sb.WriteString(fmt.Sprintf(" (%s)", status.ExternalError))
		}
		sb.WriteString("\n")
	}

	// Services
	if len(status.Services) > 0 {
		sb.WriteString("\n📦 <b>Services:</b>\n")
		for _, svc := range status.Services {
			icon := "✅"
			if !svc.IsUp {
				icon = "❌"
			}
			sb.WriteString(fmt.Sprintf("  • %s %s", svc.Name, icon))
			if svc.Port > 0 {
				sb.WriteString(fmt.Sprintf(" (:%d)", svc.Port))
			}
			sb.WriteString("\n")
		}
	}

	// Resources (only if server is up)
	if status.IsUp {
		sb.WriteString("\n💻 <b>Resources:</b>\n")

		// CPU
		cpuBar := health.FormatProgressBar(status.CPU, 10)
		sb.WriteString(fmt.Sprintf("• CPU: %.0f%% %s\n", status.CPU, cpuBar))

		// Memory
		memBar := health.FormatProgressBar(status.Memory, 10)
		sb.WriteString(fmt.Sprintf("• RAM: %.0f%% %s (%.1f/%.1f GB)\n",
			status.Memory, memBar, status.MemoryUsedGB, status.MemoryTotalGB))

		// Disk
		diskBar := health.FormatProgressBar(status.Disk, 10)
		sb.WriteString(fmt.Sprintf("• Disk: %.0f%% %s (%.1f/%.1f GB)\n",
			status.Disk, diskBar, status.DiskUsedGB, status.DiskTotalGB))

		// Uptime
		sb.WriteString(fmt.Sprintf("\n⏱️ <b>Uptime:</b> %s\n", status.FormatUptime()))
	}

	keyboard := b.buildServerDetailKeyboard(serverID, source)
	return sb.String(), keyboard
}

// handleInfraCallback handles infrastructure-related callbacks
func (b *Bot) handleInfraCallback(callback *tgbotapi.CallbackQuery, parts []string) {
	if len(parts) < 2 {
		b.answerCallback(callback.ID, "❌ Invalid callback")
		return
	}

	action := parts[1]

	switch action {
	case "health":
		// Show health view (uses cache if valid, otherwise fetches)
		text, keyboard := b.buildHealthMessage(false)
		b.editMessageWithKeyboard(callback.Message.Chat.ID, callback.Message.MessageID, text, keyboard)
		b.answerCallback(callback.ID, "📊 Health")

	case "health_back":
		// Back to health view (uses cache for fast navigation)
		text, keyboard := b.buildHealthMessage(false)
		b.editMessageWithKeyboard(callback.Message.Chat.ID, callback.Message.MessageID, text, keyboard)
		b.answerCallback(callback.ID, "← Back")

	case "overview":
		// Show infrastructure overview (no metrics needed, instant)
		text, keyboard := b.buildInfraMessage()
		b.editMessageWithKeyboard(callback.Message.Chat.ID, callback.Message.MessageID, text, keyboard)
		b.answerCallback(callback.ID, "🏗️ Infrastructure")

	case "refresh":
		// Refresh current view (health) - force refresh
		text, keyboard := b.buildHealthMessage(true)
		b.editMessageWithKeyboard(callback.Message.Chat.ID, callback.Message.MessageID, text, keyboard)
		b.answerCallback(callback.ID, "🔄 Refreshed")

	case "server":
		// Show server details (format: infra:server:serverID:source)
		// Uses cache for fast navigation
		if len(parts) < 3 {
			b.answerCallback(callback.ID, "❌ Invalid server")
			return
		}
		serverID := parts[2]
		source := "overview" // default
		if len(parts) >= 4 {
			source = parts[3]
		}
		text, keyboard := b.buildServerDetailMessage(serverID, source, false)
		b.editMessageWithKeyboard(callback.Message.Chat.ID, callback.Message.MessageID, text, keyboard)
		b.answerCallback(callback.ID, "🖥️ "+serverID)

	case "server_refresh":
		// Refresh server details (format: infra:server_refresh:serverID:source)
		// Force refresh
		if len(parts) < 3 {
			b.answerCallback(callback.ID, "❌ Invalid server")
			return
		}
		serverID := parts[2]
		source := "overview" // default
		if len(parts) >= 4 {
			source = parts[3]
		}
		text, keyboard := b.buildServerDetailMessage(serverID, source, true)
		b.editMessageWithKeyboard(callback.Message.Chat.ID, callback.Message.MessageID, text, keyboard)
		b.answerCallback(callback.ID, "🔄 Refreshed")

	default:
		b.answerCallback(callback.ID, "❌ Unknown action")
	}
}
