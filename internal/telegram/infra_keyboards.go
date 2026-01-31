package telegram

import (
	"fmt"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"github.com/scinfra-pro/scinfra-bot/internal/health"
)

// buildInfraKeyboard builds the infrastructure overview keyboard
func (b *Bot) buildInfraKeyboard() tgbotapi.InlineKeyboardMarkup {
	var rows [][]tgbotapi.InlineKeyboardButton
	var row []tgbotapi.InlineKeyboardButton

	// Collect all server buttons (not grouped by cloud)
	for _, cloud := range b.config.Infrastructure.Clouds {
		for _, server := range cloud.Servers {
			label := fmt.Sprintf("%s %s", server.Icon, server.Name)
			// Add source=overview so Back knows where to return
			callback := fmt.Sprintf("infra:server:%s:overview", server.ID)
			row = append(row, tgbotapi.NewInlineKeyboardButtonData(label, callback))

			// Max 3 buttons per row
			if len(row) >= 3 {
				rows = append(rows, row)
				row = nil
			}
		}
	}
	// Add remaining buttons
	if len(row) > 0 {
		rows = append(rows, row)
	}

	// Action buttons
	rows = append(rows, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("🔄 Refresh", "infra:overview"),
		tgbotapi.NewInlineKeyboardButtonData("📊 Health", "infra:health"),
	))

	return tgbotapi.NewInlineKeyboardMarkup(rows...)
}

// buildHealthKeyboard builds the health status keyboard (same style as infra overview)
func (b *Bot) buildHealthKeyboard(_ []*health.ServerStatus) tgbotapi.InlineKeyboardMarkup {
	var rows [][]tgbotapi.InlineKeyboardButton
	var row []tgbotapi.InlineKeyboardButton

	// Collect all server buttons (same as infra overview - status is shown in dashboard text)
	for _, cloud := range b.config.Infrastructure.Clouds {
		for _, server := range cloud.Servers {
			label := fmt.Sprintf("%s %s", server.Icon, server.Name)
			// Add source=health so Back knows where to return
			callback := fmt.Sprintf("infra:server:%s:health", server.ID)
			row = append(row, tgbotapi.NewInlineKeyboardButtonData(label, callback))

			// Max 3 buttons per row
			if len(row) >= 3 {
				rows = append(rows, row)
				row = nil
			}
		}
	}
	// Add remaining buttons
	if len(row) > 0 {
		rows = append(rows, row)
	}

	// Action buttons
	rows = append(rows, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("← Back", "infra:overview"),
		tgbotapi.NewInlineKeyboardButtonData("🔄 Refresh", "infra:refresh"),
	))

	return tgbotapi.NewInlineKeyboardMarkup(rows...)
}

// buildServerDetailKeyboard builds the server detail view keyboard
// source is "overview" or "health" - determines where Back button leads
func (b *Bot) buildServerDetailKeyboard(serverID, source string) tgbotapi.InlineKeyboardMarkup {
	backCallback := "infra:overview"
	if source == "health" {
		backCallback = "infra:health_back" // uses cache, not force refresh
	}

	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("← Back", backCallback),
			tgbotapi.NewInlineKeyboardButtonData("🔄 Refresh", fmt.Sprintf("infra:server_refresh:%s:%s", serverID, source)),
		),
	)
}
