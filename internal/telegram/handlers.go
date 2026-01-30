package telegram

import (
	"fmt"
	"log"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// capitalize returns string with first letter uppercased
func capitalize(s string) string {
	if len(s) == 0 {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// handleCommand routes commands to handlers
func (b *Bot) handleCommand(msg *tgbotapi.Message) {
	cmd := msg.Command()
	args := msg.CommandArguments()

	log.Printf("Command: /%s %s (from chat %d)", cmd, args, msg.Chat.ID)

	// Dynamic upstream commands: /upstream_<name>
	if strings.HasPrefix(cmd, "upstream_") {
		name := strings.TrimPrefix(cmd, "upstream_")
		if b.config.IsValidUpstream(name) {
			b.handleUpstream(msg, name)
			return
		}
	}

	// Dynamic restart commands: /restart_sg_<name>
	if strings.HasPrefix(cmd, "restart_sg_") {
		name := strings.TrimPrefix(cmd, "restart_sg_")
		if b.config.IsValidUpstream(name) {
			b.handleRestart(msg, "sg "+name)
			return
		}
	}

	switch cmd {
	case "start":
		b.handleStart(msg)
	case "help":
		b.handleHelp(msg)
	case "status":
		b.handleStatus(msg)
	case "edge":
		b.handleEdge(msg, args)
	case "edge_direct":
		b.handleEdge(msg, "direct")
	case "edge_full":
		b.handleEdge(msg, "full")
	case "edge_split":
		b.handleEdge(msg, "split")
	case "ip":
		b.handleIP(msg)
	case "upstream":
		b.handleUpstream(msg, args)
	case "vps":
		b.handleVPS(msg, args)
	case "vps_direct":
		b.handleVPS(msg, "direct")
	case "vps_warp":
		b.handleVPS(msg, "warp")
	case "vps_home":
		b.handleVPS(msg, "home")
	case "traffic":
		b.handleTraffic(msg)
	case "restart", "restart_sg":
		b.handleRestart(msg, args)
	default:
		b.reply(msg.Chat.ID, fmt.Sprintf("Unknown command: /%s\nUse /help for available commands.", cmd))
	}
}

// handleStart sends welcome message
func (b *Bot) handleStart(msg *tgbotapi.Message) {
	text := `👋 <b>SCINFRA Bot</b>

Welcome! This bot manages VPN infrastructure.

Use /help to see available commands.`
	b.reply(msg.Chat.ID, text)
}

// handleHelp sends help message with dynamic upstream commands
func (b *Bot) handleHelp(msg *tgbotapi.Message) {
	var sb strings.Builder

	sb.WriteString("🔧 <b>Available Commands</b>\n\n")

	// General commands
	sb.WriteString("<b>General:</b>\n")
	sb.WriteString("ℹ️ /status - Full VPN status (with inline buttons)\n")
	sb.WriteString("ℹ️ /ip - Current external IP\n")
	sb.WriteString("📊 /traffic - Traffic statistics\n")
	sb.WriteString("ℹ️ /help - This message\n")

	// Edge-gateway commands
	sb.WriteString("\n<b>Edge-gateway:</b>\n")
	sb.WriteString("ℹ️ /edge - Show current mode\n")
	sb.WriteString("🟡 /edge_direct - Direct mode\n")
	sb.WriteString("🔵 /edge_full - Full VPN mode\n")
	sb.WriteString("🟢 /edge_split - Split tunneling\n")

	// Dynamic upstream commands from config
	sb.WriteString("\n<b>Upstream:</b>\n")
	sb.WriteString("ℹ️ /upstream - Show current upstream\n")
	for _, name := range b.config.GetUpstreamNames() {
		displayName := b.config.GetUpstreamDisplayName(name)
		sb.WriteString(fmt.Sprintf("📍 /upstream_%s - Switch to %s\n", name, displayName))
	}

	// VPS commands
	sb.WriteString("\n<b>VPS (switch-gate):</b>\n")
	sb.WriteString("ℹ️ /vps - Show VPS mode and traffic\n")
	sb.WriteString("🖥️ /vps_direct - VPS Direct IP\n")
	sb.WriteString("☁️ /vps_warp - Cloudflare WARP\n")
	sb.WriteString("🏠 /vps_home - Residential IP\n")

	// Dynamic admin commands
	sb.WriteString("\n<b>Admin:</b>\n")
	sb.WriteString("🔄 /restart - Restart services menu\n")
	sb.WriteString("🔁 /restart_sg - Restart switch-gate (current upstream)\n")
	for _, name := range b.config.GetUpstreamNames() {
		displayName := b.config.GetUpstreamDisplayName(name)
		sb.WriteString(fmt.Sprintf("🔁 /restart_sg_%s - Restart switch-gate on %s\n", name, displayName))
	}

	b.reply(msg.Chat.ID, sb.String())
}

// handleStatus sends full VPN status with inline keyboard
func (b *Bot) handleStatus(msg *tgbotapi.Message) {
	text, keyboard := b.buildStatusMessage()
	b.replyWithKeyboard(msg.Chat.ID, text, keyboard)
}

// buildStatusMessage builds status text and keyboard
func (b *Bot) buildStatusMessage() (string, tgbotapi.InlineKeyboardMarkup) {
	status, err := b.edgeClient.GetStatus()
	if err != nil {
		return fmt.Sprintf("❌ Error getting status: %v", err), tgbotapi.InlineKeyboardMarkup{}
	}

	ip, err := b.edgeClient.GetExternalIP()
	if err != nil {
		ip = "unknown"
	}

	// Get VPS mode if switch-gate is available
	vpsMode := ""
	vpsModeLine := ""
	if sgClient := b.getSwitchGateClient(status.Server); sgClient != nil {
		if vpsStatus, err := sgClient.GetStatus(); err == nil {
			vpsMode = vpsStatus.Mode
			vpsModeLine = fmt.Sprintf("\n└ VPS Mode: %s %s", b.getVPSModeIcon(vpsStatus.Mode), vpsStatus.Mode)
		}
	}

	modeIcon := b.getModeIcon(status.Mode)

	text := fmt.Sprintf(`ℹ️ <b>VPN Status</b>

<b>Edge-gateway:</b>
├ Mode: %s %s
├ Upstream: %s%s

<b>Current IP:</b> <code>%s</code>`,
		modeIcon, status.Mode,
		status.Server,
		vpsModeLine,
		ip,
	)

	keyboard := b.buildStatusKeyboard(status.Mode, status.Server, vpsMode)
	return text, keyboard
}

// buildStatusMessageWithCheck builds status with mode health check
// This takes longer (~8-10 sec) but detects if current mode is not working
func (b *Bot) buildStatusMessageWithCheck() (string, tgbotapi.InlineKeyboardMarkup) {
	status, err := b.edgeClient.GetStatus()
	if err != nil {
		return fmt.Sprintf("❌ Error getting status: %v", err), tgbotapi.InlineKeyboardMarkup{}
	}

	ip, err := b.edgeClient.GetExternalIP()
	if err != nil {
		ip = "unknown"
	}

	// Get VPS status with health check
	vpsMode := ""
	failedVPSMode := ""
	vpsModeLine := ""
	if sgClient := b.getSwitchGateClient(status.Server); sgClient != nil {
		// Use GetStatusWithCheck for health verification
		if vpsStatus, err := sgClient.GetStatusWithCheck(); err == nil {
			// Check if mode is healthy
			if vpsStatus.ModeHealthy != nil && !*vpsStatus.ModeHealthy {
				// Mode is not working - traffic goes through direct (fallback)
				failedVPSMode = vpsStatus.Mode
				vpsMode = "direct" // Fallback mode
				errorInfo := ""
				if vpsStatus.ModeError != nil {
					errorInfo = fmt.Sprintf(" (%s)", *vpsStatus.ModeError)
				}
				vpsModeLine = fmt.Sprintf("\n└ VPS Mode: %s %s ❌%s", b.getVPSModeIcon(vpsStatus.Mode), vpsStatus.Mode, errorInfo)
			} else {
				vpsMode = vpsStatus.Mode
				vpsModeLine = fmt.Sprintf("\n└ VPS Mode: %s %s ✓", b.getVPSModeIcon(vpsStatus.Mode), vpsStatus.Mode)
			}
		}
	}

	modeIcon := b.getModeIcon(status.Mode)

	text := fmt.Sprintf(`ℹ️ <b>VPN Status</b> (checked)

<b>Edge-gateway:</b>
├ Mode: %s %s
├ Upstream: %s%s

<b>Current IP:</b> <code>%s</code>`,
		modeIcon, status.Mode,
		status.Server,
		vpsModeLine,
		ip,
	)

	keyboard := b.buildStatusKeyboardWithFailed(status.Mode, status.Server, vpsMode, failedVPSMode)
	return text, keyboard
}

// handleEdge handles edge-gateway commands
func (b *Bot) handleEdge(msg *tgbotapi.Message, args string) {
	args = strings.TrimSpace(args)

	// No args - show status
	if args == "" {
		status, err := b.edgeClient.GetStatus()
		if err != nil {
			b.reply(msg.Chat.ID, fmt.Sprintf("❌ Error: %v", err))
			return
		}

		// Get VPS mode if switch-gate is available
		vpsModeLine := ""
		if sgClient := b.getSwitchGateClient(status.Server); sgClient != nil {
			if vpsStatus, err := sgClient.GetStatus(); err == nil {
				vpsModeLine = fmt.Sprintf("\nVPS Mode: %s %s", b.getVPSModeIcon(vpsStatus.Mode), vpsStatus.Mode)
			}
		}

		modeIcon := b.getModeIcon(status.Mode)
		text := fmt.Sprintf(`ℹ️ <b>Edge-gateway</b>

Mode: %s %s
Upstream: %s%s
Table: %s

<i>Use /edge &lt;mode&gt; to change</i>
<i>Modes: direct, full, split</i>`,
			modeIcon, status.Mode,
			status.Server,
			vpsModeLine,
			status.Table,
		)
		b.reply(msg.Chat.ID, text)
		return
	}

	// Parse mode
	mode := strings.ToLower(args)
	if mode != "direct" && mode != "full" && mode != "split" {
		b.reply(msg.Chat.ID, fmt.Sprintf("Invalid mode: %s\nValid modes: direct, full, split", mode))
		return
	}

	// Change mode
	b.reply(msg.Chat.ID, fmt.Sprintf("Switching to <b>%s</b> mode...", mode))

	if err := b.edgeClient.SetMode(mode); err != nil {
		b.reply(msg.Chat.ID, fmt.Sprintf("❌ Error: %v", err))
		return
	}

	// Get new status
	status, err := b.edgeClient.GetStatus()
	if err != nil {
		b.reply(msg.Chat.ID, fmt.Sprintf("Mode changed, but failed to get status: %v", err))
		return
	}

	ip, _ := b.edgeClient.GetExternalIP()

	modeIcon := b.getModeIcon(status.Mode)
	text := fmt.Sprintf(`✅ <b>Mode Changed</b>

Mode: %s %s
IP: <code>%s</code>`,
		modeIcon, status.Mode,
		ip,
	)
	b.reply(msg.Chat.ID, text)
}

// handleUpstream handles upstream server commands
func (b *Bot) handleUpstream(msg *tgbotapi.Message, args string) {
	args = strings.TrimSpace(args)

	// No args - show current upstream
	if args == "" {
		status, err := b.edgeClient.GetStatus()
		if err != nil {
			b.reply(msg.Chat.ID, fmt.Sprintf("❌ Error: %v", err))
			return
		}

		upstreamNames := b.config.GetUpstreamNames()
		vpsIP := b.config.GetUpstreamIP(status.Server)

		// Get VPS mode if switch-gate is available
		vpsModeLine := ""
		if sgClient := b.getSwitchGateClient(status.Server); sgClient != nil {
			if vpsStatus, err := sgClient.GetStatus(); err == nil {
				vpsModeLine = fmt.Sprintf("\nVPS Mode: %s %s", b.getVPSModeIcon(vpsStatus.Mode), vpsStatus.Mode)
			}
		}

		text := fmt.Sprintf(`ℹ️ <b>Upstream</b>

Current: <b>%s</b>
Edge Mode: %s %s
VPS IP: <code>%s</code>%s

<i>Available:</i> %s

<i>Use /upstream &lt;name&gt; to change</i>`,
			status.Server,
			b.getModeIcon(status.Mode), status.Mode,
			vpsIP,
			vpsModeLine,
			strings.Join(upstreamNames, ", "),
		)
		b.reply(msg.Chat.ID, text)
		return
	}

	// Validate upstream name
	upstream := strings.ToLower(args)
	if !b.config.IsValidUpstream(upstream) {
		b.reply(msg.Chat.ID, fmt.Sprintf("Invalid upstream: %s\nAvailable: %s",
			upstream, strings.Join(b.config.GetUpstreamNames(), ", ")))
		return
	}

	// Change upstream
	b.reply(msg.Chat.ID, fmt.Sprintf("Switching to <b>%s</b>...", upstream))

	if err := b.edgeClient.SetUpstream(upstream); err != nil {
		b.reply(msg.Chat.ID, fmt.Sprintf("❌ Error: %v", err))
		return
	}

	// Get new status
	status, err := b.edgeClient.GetStatus()
	if err != nil {
		b.reply(msg.Chat.ID, fmt.Sprintf("Upstream changed, but failed to get status: %v", err))
		return
	}

	vpsIP := b.config.GetUpstreamIP(status.Server)

	// Get VPS mode if switch-gate is available
	vpsModeLine := ""
	if sgClient := b.getSwitchGateClient(status.Server); sgClient != nil {
		if vpsStatus, err := sgClient.GetStatus(); err == nil {
			vpsModeLine = fmt.Sprintf("\nVPS Mode: %s %s", b.getVPSModeIcon(vpsStatus.Mode), vpsStatus.Mode)
		}
	}

	text := fmt.Sprintf(`✅ <b>Upstream Changed</b>

Upstream: <b>%s</b>
Edge Mode: %s %s
VPS IP: <code>%s</code>%s`,
		status.Server,
		b.getModeIcon(status.Mode), status.Mode,
		vpsIP,
		vpsModeLine,
	)
	b.reply(msg.Chat.ID, text)
}

// handleVPS handles VPS switch-gate commands
func (b *Bot) handleVPS(msg *tgbotapi.Message, args string) {
	args = strings.TrimSpace(args)

	// Get current upstream
	edgeStatus, err := b.edgeClient.GetStatus()
	if err != nil {
		b.reply(msg.Chat.ID, fmt.Sprintf("❌ Error getting edge status: %v", err))
		return
	}

	upstreamName := edgeStatus.Server
	sgClient := b.getSwitchGateClient(upstreamName)
	if sgClient == nil {
		b.reply(msg.Chat.ID, fmt.Sprintf("No switch-gate configured for upstream: %s", upstreamName))
		return
	}

	// No args - show status
	if args == "" {
		b.reply(msg.Chat.ID, fmt.Sprintf("Loading %s status...", upstreamName))

		status, err := sgClient.GetStatus()
		if err != nil {
			b.reply(msg.Chat.ID, fmt.Sprintf("❌ Error: %v", err))
			return
		}

		ip, err := sgClient.GetExternalIP()
		if err != nil {
			ip = "unknown"
		}

		modeIcon := b.getVPSModeIcon(status.Mode)
		text := fmt.Sprintf(`ℹ️ <b>VPS: %s</b>

Mode: %s %s
Mode IP: <code>%s</code>

<b>Traffic:</b>
├ Direct: %.2f MB
├ WARP: %.2f MB
└ Home: %.2f / %d MB

<i>Use /vps &lt;mode&gt; to change</i>
<i>Modes: direct, warp, home</i>`,
			upstreamName,
			modeIcon, status.Mode,
			ip,
			status.Traffic.DirectMB,
			status.Traffic.WarpMB,
			status.Traffic.HomeMB, status.Home.LimitMB,
		)
		b.reply(msg.Chat.ID, text)
		return
	}

	// Parse mode
	mode := strings.ToLower(args)
	if mode != "direct" && mode != "warp" && mode != "home" {
		b.reply(msg.Chat.ID, fmt.Sprintf("Invalid mode: %s\nValid modes: direct, warp, home", mode))
		return
	}

	// Change mode
	b.reply(msg.Chat.ID, fmt.Sprintf("Switching %s to <b>%s</b>...", upstreamName, mode))

	if err := sgClient.SetMode(mode); err != nil {
		b.reply(msg.Chat.ID, fmt.Sprintf("❌ Error: %v", err))
		return
	}

	// Get new status
	status, err := sgClient.GetStatus()
	if err != nil {
		b.reply(msg.Chat.ID, fmt.Sprintf("Mode changed, but failed to get status: %v", err))
		return
	}

	ip, _ := sgClient.GetExternalIP()

	modeIcon := b.getVPSModeIcon(status.Mode)
	text := fmt.Sprintf(`✅ <b>VPS Mode Changed</b>

VPS: <b>%s</b>
Mode: %s %s
Mode IP: <code>%s</code>`,
		upstreamName,
		modeIcon, status.Mode,
		ip,
	)
	b.reply(msg.Chat.ID, text)
}

// getVPSModeIcon returns emoji for VPS mode
func (b *Bot) getVPSModeIcon(mode string) string {
	switch strings.ToLower(mode) {
	case "direct":
		return "\U0001F5A5" // 🖥️ Computer - VPS IP
	case "warp":
		return "\u2601\uFE0F" // ☁️ Cloud - Cloudflare
	case "home":
		return "\U0001F3E0" // 🏠 House - Residential
	default:
		return "\u2753" // ❓ Question mark
	}
}

// handleIP sends current external IP
func (b *Bot) handleIP(msg *tgbotapi.Message) {
	ip, err := b.edgeClient.GetExternalIP()
	if err != nil {
		b.reply(msg.Chat.ID, fmt.Sprintf("❌ Error: %v", err))
		return
	}

	b.reply(msg.Chat.ID, fmt.Sprintf("ℹ️ <b>Current IP:</b> <code>%s</code>", ip))
}

// getModeIcon returns emoji for mode
func (b *Bot) getModeIcon(mode string) string {
	switch strings.ToLower(mode) {
	case "direct":
		return "\U0001F7E1" // Yellow circle - "Attention" no VPN
	case "full":
		return "\U0001F535" // Blue circle - "VPN" all traffic tunneled
	case "split":
		return "\U0001F7E2" // Green circle - "OK" optimal mode
	default:
		return "\u26AA" // White circle
	}
}

// handleCallback handles inline keyboard button presses
func (b *Bot) handleCallback(callback *tgbotapi.CallbackQuery) {
	// Check cooldown (1 second per chat)
	if b.checkCooldown(callback.Message.Chat.ID) {
		b.answerCallback(callback.ID, "⏳ Please wait...")
		return
	}

	data := callback.Data
	parts := strings.Split(data, ":")

	if len(parts) < 2 {
		b.answerCallback(callback.ID, "❌ Invalid callback data")
		return
	}

	category := parts[0]
	value := parts[1]

	log.Printf("Callback: %s (from chat %d)", data, callback.Message.Chat.ID)

	switch category {
	case "edge":
		b.handleEdgeCallback(callback, value)
	case "upstream":
		b.handleUpstreamCallback(callback, value)
	case "vps":
		b.handleVPSCallback(callback, value)
	case "action":
		b.handleActionCallback(callback, value)
	case "restart":
		b.handleRestartCallback(callback, parts)
	default:
		b.answerCallback(callback.ID, "❌ Unknown action")
	}
}

// handleEdgeCallback handles edge mode button press
func (b *Bot) handleEdgeCallback(callback *tgbotapi.CallbackQuery, mode string) {
	if err := b.edgeClient.SetMode(mode); err != nil {
		b.answerCallback(callback.ID, fmt.Sprintf("❌ Error: %v", err))
		return
	}

	// Update message with new status
	text, keyboard := b.buildStatusMessage()
	b.editMessageWithKeyboard(callback.Message.Chat.ID, callback.Message.MessageID, text, keyboard)
	b.answerCallback(callback.ID, fmt.Sprintf("✅ Edge → %s", mode))
}

// handleUpstreamCallback handles upstream selection button press
func (b *Bot) handleUpstreamCallback(callback *tgbotapi.CallbackQuery, upstream string) {
	if err := b.edgeClient.SetUpstream(upstream); err != nil {
		b.answerCallback(callback.ID, fmt.Sprintf("❌ Error: %v", err))
		return
	}

	// Update message with new status
	text, keyboard := b.buildStatusMessage()
	b.editMessageWithKeyboard(callback.Message.Chat.ID, callback.Message.MessageID, text, keyboard)
	b.answerCallback(callback.ID, fmt.Sprintf("✅ Upstream → %s", upstream))
}

// handleVPSCallback handles VPS mode button press
func (b *Bot) handleVPSCallback(callback *tgbotapi.CallbackQuery, mode string) {
	// Get current upstream
	edgeStatus, err := b.edgeClient.GetStatus()
	if err != nil {
		b.answerCallback(callback.ID, fmt.Sprintf("❌ Error: %v", err))
		return
	}

	sgClient := b.getSwitchGateClient(edgeStatus.Server)
	if sgClient == nil {
		b.answerCallback(callback.ID, "❌ No switch-gate for this upstream")
		return
	}

	if err := sgClient.SetMode(mode); err != nil {
		b.answerCallback(callback.ID, fmt.Sprintf("❌ Error: %v", err))
		return
	}

	// Update message with new status
	text, keyboard := b.buildStatusMessage()
	b.editMessageWithKeyboard(callback.Message.Chat.ID, callback.Message.MessageID, text, keyboard)
	b.answerCallback(callback.ID, fmt.Sprintf("✅ VPS → %s", mode))
}

// handleActionCallback handles action button press (refresh, traffic)
func (b *Bot) handleActionCallback(callback *tgbotapi.CallbackQuery, action string) {
	switch action {
	case "refresh":
		// Use health check version for Refresh button
		text, keyboard := b.buildStatusMessageWithCheck()
		b.editMessageWithKeyboard(callback.Message.Chat.ID, callback.Message.MessageID, text, keyboard)
		b.answerCallback(callback.ID, "🔄 Checked")
	case "traffic":
		text, keyboard := b.buildTrafficMessage()
		b.editMessageWithKeyboard(callback.Message.Chat.ID, callback.Message.MessageID, text, keyboard)
		b.answerCallback(callback.ID, "📊 Traffic")
	default:
		b.answerCallback(callback.ID, "❌ Unknown action")
	}
}

// handleTraffic handles /traffic command
func (b *Bot) handleTraffic(msg *tgbotapi.Message) {
	text, keyboard := b.buildTrafficMessage()
	b.replyWithKeyboard(msg.Chat.ID, text, keyboard)
}

// buildTrafficMessage builds traffic statistics message
func (b *Bot) buildTrafficMessage() (string, tgbotapi.InlineKeyboardMarkup) {
	// Get current upstream
	edgeStatus, err := b.edgeClient.GetStatus()
	if err != nil {
		return fmt.Sprintf("❌ Error: %v", err), tgbotapi.InlineKeyboardMarkup{}
	}

	var sb strings.Builder
	sb.WriteString("📈 <b>Traffic Statistics</b>\n")

	// Edge gateway traffic (cloud provider)
	sb.WriteString(fmt.Sprintf("\n<b>%s:</b>\n", b.config.Edge.Name))
	ycTraffic, err := b.edgeClient.GetTraffic()
	if err != nil {
		sb.WriteString(fmt.Sprintf("└ ❌ Error: %v\n", err))
	} else {
		sb.WriteString(fmt.Sprintf("├ Direct: %.2f MB\n", ycTraffic.Summary.DirectMB))
		sb.WriteString(fmt.Sprintf("├ VPN: %.2f MB\n", ycTraffic.Summary.VpnMB))
		sb.WriteString(fmt.Sprintf("├ Total: %.2f GB\n", ycTraffic.Summary.TotalGB))
		if ycTraffic.Billing.BillableGB > 0 {
			sb.WriteString(fmt.Sprintf("├ Billable: %.2f GB\n", ycTraffic.Billing.BillableGB))
			sb.WriteString(fmt.Sprintf("└ 💰 Cost: ₽%.2f\n", ycTraffic.Billing.CostRub))
		} else {
			sb.WriteString(fmt.Sprintf("└ 💰 Free (%.2f/%.0f GB)\n", ycTraffic.Summary.TotalGB, ycTraffic.Billing.FreeQuotaGB))
		}
	}

	// Iterate over all upstreams (VPS traffic)
	upstreamNames := b.config.GetUpstreamNames()
	for _, name := range upstreamNames {
		sgClient := b.getSwitchGateClient(name)
		if sgClient == nil {
			continue
		}

		status, err := sgClient.GetStatus()
		if err != nil {
			sb.WriteString(fmt.Sprintf("\n<b>%s:</b> ❌ Error\n", name))
			continue
		}

		// Mark current upstream
		marker := ""
		if name == edgeStatus.Server {
			marker = " (current)"
		}

		sb.WriteString(fmt.Sprintf("\n<b>%s%s:</b>\n", capitalize(name), marker))
		sb.WriteString(fmt.Sprintf("├ Direct: %.2f MB\n", status.Traffic.DirectMB))
		sb.WriteString(fmt.Sprintf("├ WARP: %.2f MB\n", status.Traffic.WarpMB))
		sb.WriteString(fmt.Sprintf("├ Home: %.2f / %d MB\n", status.Traffic.HomeMB, status.Home.LimitMB))

		if status.Home.CostUSD > 0 {
			sb.WriteString(fmt.Sprintf("└ 💰 Cost: $%.2f\n", status.Home.CostUSD))
		} else {
			sb.WriteString("└ 💰 Cost: $0.00\n")
		}
	}

	keyboard := b.buildTrafficKeyboard()
	return sb.String(), keyboard
}

// handleRestart handles the /restart command
func (b *Bot) handleRestart(msg *tgbotapi.Message, args string) {
	args = strings.TrimSpace(args)

	// No args — show menu
	if args == "" {
		text := "🔄 <b>Restart</b>\n\nSelect service to restart:"
		keyboard := b.buildRestartKeyboard()
		b.replyWithKeyboard(msg.Chat.ID, text, keyboard)
		return
	}

	// Parse args: "sg" or "sg aeza"
	parts := strings.Fields(args)
	if len(parts) == 0 {
		b.reply(msg.Chat.ID, "Usage: /restart sg [upstream]")
		return
	}

	service := parts[0]
	if service != "sg" {
		b.reply(msg.Chat.ID, "Unknown service. Available: sg (switch-gate)")
		return
	}

	// Determine upstream
	var upstream string
	if len(parts) > 1 {
		upstream = strings.ToLower(parts[1])
		if !b.config.IsValidUpstream(upstream) {
			b.reply(msg.Chat.ID, fmt.Sprintf("❌ Invalid upstream: %s", upstream))
			return
		}
	} else {
		// Use current upstream
		status, err := b.edgeClient.GetStatus()
		if err != nil {
			b.reply(msg.Chat.ID, fmt.Sprintf("❌ Error: %v", err))
			return
		}
		upstream = status.Server
	}

	// Perform restart
	b.restartSwitchGate(msg.Chat.ID, upstream)
}

// handleRestartCallback handles restart button clicks
func (b *Bot) handleRestartCallback(callback *tgbotapi.CallbackQuery, parts []string) {
	// Format: restart:sg:aeza
	if len(parts) < 3 {
		b.answerCallback(callback.ID, "❌ Invalid callback")
		return
	}

	service := parts[1]  // "sg"
	upstream := parts[2] // "aeza"

	if service != "sg" {
		b.answerCallback(callback.ID, "❌ Unknown service")
		return
	}

	if !b.config.IsValidUpstream(upstream) {
		b.answerCallback(callback.ID, "❌ Invalid upstream")
		return
	}

	b.answerCallback(callback.ID, "🔁 Restarting...")
	b.restartSwitchGate(callback.Message.Chat.ID, upstream)
}

// restartSwitchGate restarts switch-gate on specified upstream
func (b *Bot) restartSwitchGate(chatID int64, upstream string) {
	b.reply(chatID, fmt.Sprintf("⏳ Restarting switch-gate on %s...", capitalize(upstream)))

	sgClient := b.getSwitchGateClient(upstream)
	if sgClient == nil {
		b.reply(chatID, fmt.Sprintf("❌ switch-gate not configured for %s", upstream))
		return
	}

	if err := sgClient.Restart(); err != nil {
		b.reply(chatID, fmt.Sprintf("❌ Failed to restart: %v", err))
		return
	}

	b.reply(chatID, fmt.Sprintf("✅ switch-gate restarted (%s)", capitalize(upstream)))
}
