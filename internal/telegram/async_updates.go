package telegram

import (
	"fmt"
	"log"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// handleEdgeModeChangeAsync handles edge mode change asynchronously
// handleRefreshAsync handles refresh button asynchronously
func (b *Bot) handleRefreshAsync(chatID int64, messageID int, callbackID string) {
	// Get current status
	status, err := b.edgeClient.GetStatus()
	if err != nil {
		log.Printf("Refresh: failed to get status: %v", err)
		text := fmt.Sprintf("❌ Failed to get status: %v", err)
		keyboard := b.buildStatusKeyboard("", "", "")
		b.editMessageWithKeyboard(chatID, messageID, text, keyboard)
		b.answerCallback(callbackID, "❌ Failed to get status")
		return
	}

	upstreamName := status.Server

	// Show checking message
	text, keyboard := b.buildStatusMessagePending("checking IP...", upstreamName)
	b.editMessageWithKeyboard(chatID, messageID, text, keyboard)

	// Fetch and update IP (force refresh = true)
	b.updateIPAndRefresh(chatID, messageID, upstreamName, true)

	// Answer callback after refresh is complete
	b.answerCallback(callbackID, "✅ Refreshed")
}

// updateStatusWithIP updates status message with IP check (silent background update)
func (b *Bot) updateStatusWithIP(chatID int64, messageID int, forceRefresh bool) {
	// Get current status
	status, err := b.edgeClient.GetStatus()
	if err != nil {
		log.Printf("updateStatusWithIP: failed to get status: %v", err)
		return
	}

	upstreamName := status.Server

	// Fetch IP (with cache support)
	ip, err := b.fetchIP(upstreamName, forceRefresh)
	if err != nil {
		log.Printf("updateStatusWithIP: failed to fetch IP: %v", err)
		// Don't show error to user - just skip silent update
		return
	}

	// Update message with final IP
	text, keyboard := b.buildStatusMessageWithIP(ip)
	b.editMessageWithKeyboard(chatID, messageID, text, keyboard)
}

// updateIPAndRefresh fetches IP and updates the status message
func (b *Bot) updateIPAndRefresh(chatID int64, messageID int, upstreamName string, forceRefresh bool) {
	ip, err := b.fetchIP(upstreamName, forceRefresh)
	if err != nil {
		log.Printf("Failed to fetch IP for %s: %v", upstreamName, err)
		ip = "❌ IP check failed"
	}

	// Build final status message with IP
	text, keyboard := b.buildStatusMessageWithIP(ip)
	b.editMessageWithKeyboard(chatID, messageID, text, keyboard)
}

// fetchIP retrieves external IP with caching support
func (b *Bot) fetchIP(upstreamName string, forceRefresh bool) (string, error) {
	// Get VPS mode (need it for cache key)
	vpsMode := ""
	sgClient := b.getSwitchGateClient(upstreamName)
	if sgClient != nil {
		if vpsStatus, err := sgClient.GetStatus(); err == nil {
			vpsMode = vpsStatus.Mode
		}
	}

	// Check cache if not forcing refresh
	if !forceRefresh {
		if cachedIP := b.getIPFromCache(upstreamName, vpsMode); cachedIP != "" {
			return cachedIP, nil
		}
	}

	var ip string
	var err error

	if sgClient == nil {
		// No switch-gate - get edge-gateway IP
		ip, err = b.edgeClient.GetExternalIP()
		if err != nil {
			return "", fmt.Errorf("edge IP check: %w", err)
		}
	} else {
		// Get VPS IP through SOCKS proxy
		ip, err = sgClient.GetExternalIP()
		if err != nil {
			return "", fmt.Errorf("VPS IP check: %w", err)
		}
	}

	// Store in cache with VPS mode
	b.setIPCache(upstreamName, vpsMode, ip)

	return ip, nil
}

// buildStatusMessagePending builds status message with pending state
func (b *Bot) buildStatusMessagePending(pendingText, upstreamName string) (string, tgbotapi.InlineKeyboardMarkup) {
	status, err := b.edgeClient.GetStatus()
	if err != nil {
		return fmt.Sprintf("❌ Error getting status: %v", err), tgbotapi.InlineKeyboardMarkup{}
	}

	// Get VPS mode if switch-gate is available
	vpsMode := ""
	vpsModeLine := ""
	if sgClient := b.getSwitchGateClient(status.Server); sgClient != nil {
		if vpsStatus, err := sgClient.GetStatus(); err == nil {
			vpsMode = vpsStatus.Mode
			vpsModeLine = fmt.Sprintf("\n└ VPS Mode: %s %s", b.getVPSModeIcon(vpsStatus.Mode), vpsStatus.Mode)
		} else {
			vpsModeLine = "\n└ VPS Mode: ❌ error"
		}
	}

	modeIcon := b.getModeIcon(status.Mode)

	text := fmt.Sprintf(`ℹ️ <b>VPN Status</b>

<b>Edge-gateway:</b>
├ Mode: %s %s
├ Upstream: %s%s

<b>Current IP:</b> ⏳ %s`,
		modeIcon, status.Mode,
		status.Server,
		vpsModeLine,
		pendingText,
	)

	keyboard := b.buildStatusKeyboard(status.Mode, status.Server, vpsMode)
	return text, keyboard
}

// buildStatusMessageWithIP builds status message with specific IP
func (b *Bot) buildStatusMessageWithIP(ip string) (string, tgbotapi.InlineKeyboardMarkup) {
	status, err := b.edgeClient.GetStatus()
	if err != nil {
		return fmt.Sprintf("❌ Error getting status: %v", err), tgbotapi.InlineKeyboardMarkup{}
	}

	// Get VPS mode if switch-gate is available
	vpsMode := ""
	vpsModeLine := ""
	if sgClient := b.getSwitchGateClient(status.Server); sgClient != nil {
		if vpsStatus, err := sgClient.GetStatus(); err == nil {
			vpsMode = vpsStatus.Mode
			vpsModeLine = fmt.Sprintf("\n└ VPS Mode: %s %s", b.getVPSModeIcon(vpsStatus.Mode), vpsStatus.Mode)
		} else {
			vpsModeLine = "\n└ VPS Mode: ❌ error"
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
