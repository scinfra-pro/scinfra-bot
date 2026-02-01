package telegram

import (
	"log"
	"sync"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"github.com/scinfra-pro/scinfra-bot/internal/config"
	"github.com/scinfra-pro/scinfra-bot/internal/edge"
	"github.com/scinfra-pro/scinfra-bot/internal/health"
	"github.com/scinfra-pro/scinfra-bot/internal/switchgate"
)

// Bot represents the Telegram bot
type Bot struct {
	api               *tgbotapi.BotAPI
	config            *config.Config
	edgeClient        *edge.Client
	switchGateClients map[string]*switchgate.Client
	healthChecker     *health.Checker

	// Cooldown tracking for callback spam protection
	callbackCooldown map[int64]time.Time
	cooldownMu       sync.Mutex
}

// New creates a new Telegram bot
func New(cfg *config.Config, edgeClient *edge.Client) (*Bot, error) {
	api, err := tgbotapi.NewBotAPI(cfg.Telegram.Token)
	if err != nil {
		return nil, err
	}

	log.Printf("Authorized on account %s", api.Self.UserName)

	// Create switch-gate clients for each upstream
	sgClients := make(map[string]*switchgate.Client)
	for name, upstream := range cfg.Upstreams {
		if upstream.SwitchGate {
			client, err := switchgate.NewClient(switchgate.ClientConfig{
				Name:     name,
				JumpHost: cfg.Edge.Host,
				TargetIP: upstream.IP,
				User:     upstream.User,
				KeyPath:  cfg.Edge.KeyPath,
				APIPort:  upstream.SwitchGatePort,
			})
			if err != nil {
				log.Printf("Warning: failed to create switch-gate client for %s: %v", name, err)
				continue
			}
			sgClients[name] = client
			log.Printf("Created switch-gate client for %s (%s)", name, upstream.IP)
		}
	}

	// Create health checker if infrastructure monitoring is enabled
	var healthChecker *health.Checker
	if cfg.IsInfrastructureEnabled() {
		healthChecker = health.NewChecker(cfg, sgClients)
		// Set edge SSH stats provider
		healthChecker.SetEdgeSSHStatsFunc(func() health.EdgeSSHStats {
			stats := edgeClient.GetSSHStats()
			return health.EdgeSSHStats{
				SuccessCount: stats.SuccessCount,
				ErrorCount:   stats.ErrorCount,
				LastLatency:  stats.LastLatency,
				LastError:    stats.LastError,
				LastErrorAt:  stats.LastErrorAt,
			}
		})
		log.Printf("Infrastructure monitoring enabled with %d clouds", len(cfg.Infrastructure.Clouds))
	}

	return &Bot{
		api:               api,
		config:            cfg,
		edgeClient:        edgeClient,
		switchGateClients: sgClients,
		healthChecker:     healthChecker,
		callbackCooldown:  make(map[int64]time.Time),
	}, nil
}

// getSwitchGateClient returns switch-gate client for upstream name
func (b *Bot) getSwitchGateClient(name string) *switchgate.Client {
	if client, ok := b.switchGateClients[name]; ok {
		return client
	}
	return nil
}

// Start begins polling for updates
func (b *Bot) Start() error {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates := b.api.GetUpdatesChan(u)

	log.Println("Bot started, waiting for messages...")

	for update := range updates {
		// Handle callback queries (inline keyboard buttons)
		if update.CallbackQuery != nil {
			if !b.config.IsAllowedChat(update.CallbackQuery.Message.Chat.ID) {
				log.Printf("Unauthorized callback from chat %d", update.CallbackQuery.Message.Chat.ID)
				continue
			}
			b.handleCallback(update.CallbackQuery)
			continue
		}

		if update.Message == nil {
			continue
		}

		// Check authorization
		if !b.config.IsAllowedChat(update.Message.Chat.ID) {
			log.Printf("Unauthorized access from chat %d", update.Message.Chat.ID)
			continue
		}

		// Handle commands
		if update.Message.IsCommand() {
			b.handleCommand(update.Message)
		}
	}

	return nil
}

// Stop gracefully stops the bot
func (b *Bot) Stop() {
	b.api.StopReceivingUpdates()
}

// reply sends a message to the chat
func (b *Bot) reply(chatID int64, text string) {
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ParseMode = "HTML"
	if _, err := b.api.Send(msg); err != nil {
		log.Printf("Failed to send message: %v", err)
	}
}

// replyWithKeyboard sends a message with inline keyboard
func (b *Bot) replyWithKeyboard(chatID int64, text string, keyboard tgbotapi.InlineKeyboardMarkup) {
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ParseMode = "HTML"
	msg.ReplyMarkup = keyboard
	if _, err := b.api.Send(msg); err != nil {
		log.Printf("Failed to send message with keyboard: %v", err)
	}
}

// editMessageWithKeyboard edits existing message with new text and keyboard
func (b *Bot) editMessageWithKeyboard(chatID int64, messageID int, text string, keyboard tgbotapi.InlineKeyboardMarkup) {
	edit := tgbotapi.NewEditMessageText(chatID, messageID, text)
	edit.ParseMode = "HTML"
	edit.ReplyMarkup = &keyboard
	if _, err := b.api.Send(edit); err != nil {
		log.Printf("Failed to edit message: %v", err)
	}
}

// answerCallback answers callback query with optional toast message
func (b *Bot) answerCallback(callbackID string, text string) {
	callback := tgbotapi.NewCallback(callbackID, text)
	if _, err := b.api.Request(callback); err != nil {
		log.Printf("Failed to answer callback: %v", err)
	}
}

// SendNotification sends a notification to all allowed chats (implements TelegramNotifier)
func (b *Bot) SendNotification(text string) error {
	var lastErr error
	for _, chatID := range b.config.Telegram.AllowedChatIDs {
		msg := tgbotapi.NewMessage(chatID, text)
		msg.ParseMode = "HTML"
		if _, err := b.api.Send(msg); err != nil {
			log.Printf("Failed to send notification to chat %d: %v", chatID, err)
			lastErr = err
		}
	}
	return lastErr
}

// checkCooldown checks if chat is in cooldown period (returns true if should skip)
func (b *Bot) checkCooldown(chatID int64) bool {
	b.cooldownMu.Lock()
	defer b.cooldownMu.Unlock()

	lastTime, exists := b.callbackCooldown[chatID]
	if exists && time.Since(lastTime) < time.Second {
		return true // Still in cooldown
	}

	b.callbackCooldown[chatID] = time.Now()
	return false
}
