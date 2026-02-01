package telegram

import (
	"fmt"
	"sort"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// buildStatusKeyboard builds inline keyboard for /status command
func (b *Bot) buildStatusKeyboard(edgeMode, upstream, vpsMode string) tgbotapi.InlineKeyboardMarkup {
	return b.buildStatusKeyboardWithHealth(edgeMode, upstream, vpsMode, true)
}

// buildStatusKeyboardWithHealth builds inline keyboard with health indicator
// Checkmark is always on the real mode, warning shown if unhealthy
func (b *Bot) buildStatusKeyboardWithHealth(edgeMode, upstream, vpsMode string, vpsHealthy bool) tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		// Edge-gateway modes
		b.buildEdgeRow(edgeMode),
		// Upstream VPS selection
		b.buildUpstreamRow(upstream),
		// VPS modes (switch-gate) with health indicator
		b.buildVPSRowWithHealth(vpsMode, vpsHealthy),
		// Action buttons
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🔄 Refresh", "action:refresh"),
			tgbotapi.NewInlineKeyboardButtonData("📊 Traffic", "action:traffic"),
		),
	)
}

// buildEdgeRow builds edge-gateway mode buttons
func (b *Bot) buildEdgeRow(currentMode string) []tgbotapi.InlineKeyboardButton {
	modes := []struct {
		mode  string
		icon  string
		label string
	}{
		{"direct", "🟡", "Direct"},
		{"full", "🔵", "Full"},
		{"split", "🟢", "Split"},
	}

	var buttons []tgbotapi.InlineKeyboardButton
	for _, m := range modes {
		label := fmt.Sprintf("%s %s", m.icon, m.label)
		if strings.EqualFold(currentMode, m.mode) {
			label += " ✓"
		}
		buttons = append(buttons, tgbotapi.NewInlineKeyboardButtonData(label, "edge:"+m.mode))
	}
	return buttons
}

// buildUpstreamRow builds upstream VPS selection buttons
// All upstreams from config, sorted alphabetically for consistent order
func (b *Bot) buildUpstreamRow(currentUpstream string) []tgbotapi.InlineKeyboardButton {
	// Get all upstream names and sort alphabetically
	names := make([]string, 0, len(b.config.Upstreams))
	for name := range b.config.Upstreams {
		names = append(names, name)
	}
	sort.Strings(names)

	var buttons []tgbotapi.InlineKeyboardButton
	for _, name := range names {
		label := fmt.Sprintf("📍 %s", capitalize(name))
		if strings.EqualFold(currentUpstream, name) {
			label += " ✓"
		}
		buttons = append(buttons, tgbotapi.NewInlineKeyboardButtonData(label, "upstream:"+name))
	}
	return buttons
}

// buildVPSRowWithHealth builds VPS mode buttons with health indicator
// Checkmark is always on currentMode, with optional warning if unhealthy
func (b *Bot) buildVPSRowWithHealth(currentMode string, healthy bool) []tgbotapi.InlineKeyboardButton {
	modes := []struct {
		mode  string
		icon  string
		label string
	}{
		{"direct", "🖥️", "Direct"},
		{"warp", "☁️", "WARP"},
		{"home", "🏠", "Home"},
	}

	var buttons []tgbotapi.InlineKeyboardButton
	for _, m := range modes {
		label := fmt.Sprintf("%s %s", m.icon, m.label)
		if strings.EqualFold(currentMode, m.mode) {
			// Always show checkmark on current mode
			if healthy {
				label += " ✓"
			} else {
				// Current mode but unhealthy - show warning
				label += " ⚠️"
			}
		}
		buttons = append(buttons, tgbotapi.NewInlineKeyboardButtonData(label, "vps:"+m.mode))
	}
	return buttons
}

// buildTrafficKeyboard builds keyboard for /traffic command
func (b *Bot) buildTrafficKeyboard() tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🔄 Refresh", "action:traffic"),
			tgbotapi.NewInlineKeyboardButtonData("📊 Status", "action:refresh"),
		),
	)
}

// buildRestartKeyboard builds keyboard for /restart command
func (b *Bot) buildRestartKeyboard() tgbotapi.InlineKeyboardMarkup {
	// Get all upstream names and sort alphabetically
	names := make([]string, 0, len(b.config.Upstreams))
	for name := range b.config.Upstreams {
		names = append(names, name)
	}
	sort.Strings(names)

	// Build buttons for each upstream
	var buttons []tgbotapi.InlineKeyboardButton
	for _, name := range names {
		label := fmt.Sprintf("🔁 SG %s", capitalize(name))
		buttons = append(buttons, tgbotapi.NewInlineKeyboardButtonData(label, "restart:sg:"+name))
	}

	return tgbotapi.NewInlineKeyboardMarkup(buttons)
}
