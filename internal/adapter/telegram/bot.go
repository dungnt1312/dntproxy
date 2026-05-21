package telegram

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"github.com/dungnt/dntproxy/internal/domain"
	"github.com/dungnt/dntproxy/internal/port"
)

// Bot implements port.Notifier with an embedded Telegram bot.
type Bot struct {
	mu      sync.RWMutex
	api     *tgbotapi.BotAPI
	ownerID int64
	running bool
	cancel  context.CancelFunc

	store    port.CredentialStore
	logStore port.LogStore
	dedup    *DedupStore
	alerter  *Alerter

	// mute state
	muteMu     sync.RWMutex
	mutedUntil time.Time
}

// NewBot creates a new Telegram bot adapter.
func NewBot(store port.CredentialStore, logStore port.LogStore) *Bot {
	return &Bot{
		store:    store,
		logStore: logStore,
		dedup:    NewDedupStore(),
	}
}

// Start initializes the bot API and begins long-polling for commands.
func (b *Bot) Start() error {
	if b.IsRunning() {
		return nil
	}

	settings, err := b.store.GetSettings()
	if err != nil {
		return fmt.Errorf("telegram: failed to load settings: %w", err)
	}

	if settings.Telegram.BotToken == "" {
		return fmt.Errorf("telegram: bot token not configured")
	}
	if settings.Telegram.OwnerID == 0 {
		return fmt.Errorf("telegram: owner ID not configured")
	}

	api, err := tgbotapi.NewBotAPI(settings.Telegram.BotToken)
	if err != nil {
		return fmt.Errorf("telegram: failed to connect: %w", err)
	}

	// Restore mute state
	b.muteMu.Lock()
	b.mutedUntil = time.Time{}
	if settings.Telegram.MutedUntil != "" {
		if t, err := time.Parse(time.RFC3339, settings.Telegram.MutedUntil); err == nil && t.After(time.Now()) {
			b.mutedUntil = t
		}
	}
	b.muteMu.Unlock()

	ctx, cancel := context.WithCancel(context.Background())

	b.mu.Lock()
	b.api = api
	b.ownerID = settings.Telegram.OwnerID
	b.running = true
	b.cancel = cancel
	b.mu.Unlock()

	// Register slash commands with Telegram
	b.registerCommands()

	go b.pollUpdates(ctx)

	log.Printf("[TELEGRAM] Bot started as @%s for owner %d", api.Self.UserName, b.ownerID)
	return nil
}

// SetAlerter stores the alerter reference on the bot for lifecycle management.
func (b *Bot) SetAlerter(a *Alerter) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.alerter = a
}

// GetAlerter returns the current alerter (may be nil).
func (b *Bot) GetAlerter() *Alerter {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.alerter
}

// registerCommands sets the bot's slash command menu in Telegram.
func (b *Bot) registerCommands() {
	commands := tgbotapi.NewSetMyCommands(
		tgbotapi.BotCommand{Command: "status", Description: "Connection health overview"},
		tgbotapi.BotCommand{Command: "usage", Description: "Today's usage stats (or /usage 7d)"},
		tgbotapi.BotCommand{Command: "connections", Description: "Detailed connection info"},
		tgbotapi.BotCommand{Command: "mute", Description: "Suppress alerts (e.g. /mute 2h)"},
		tgbotapi.BotCommand{Command: "unmute", Description: "Resume alerts"},
		tgbotapi.BotCommand{Command: "help", Description: "Show available commands"},
	)
	if _, err := b.api.Request(commands); err != nil {
		log.Printf("[TELEGRAM] Failed to register commands: %v", err)
	}
}

// Stop gracefully shuts down the bot.
func (b *Bot) Stop() {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.cancel != nil {
		b.cancel()
		b.cancel = nil
	}
	if b.api != nil {
		b.api.StopReceivingUpdates()
		b.api = nil
	}
	b.running = false
	log.Printf("[TELEGRAM] Bot stopped")
}

// IsRunning returns whether the bot is currently active.
func (b *Bot) IsRunning() bool {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.running
}

// SendAlert sends a deduplicated alert to the owner.
func (b *Bot) SendAlert(alert port.Alert) error {
	if !b.IsRunning() {
		return nil
	}
	if b.isMuted() {
		return nil
	}

	key := DedupKey(alert)
	if !b.dedup.ShouldSend(key) {
		return nil
	}

	text := FormatAlert(alert)
	return b.sendMarkdown(text)
}

// SendMessage sends a plain text message to the owner.
func (b *Bot) SendMessage(text string) error {
	if !b.IsRunning() {
		return nil
	}

	b.mu.RLock()
	api := b.api
	ownerID := b.ownerID
	b.mu.RUnlock()

	if api == nil {
		return fmt.Errorf("telegram: bot not initialized")
	}

	msg := tgbotapi.NewMessage(ownerID, text)
	_, err := api.Send(msg)
	return err
}

// NotifyRecovered sends a recovery alert and clears dedup state.
func (b *Bot) NotifyRecovered(connectionID, connectionName, provider string) {
	b.dedup.Clear(connectionID)
	_ = b.SendAlert(port.Alert{
		Type:         port.AlertConnectionRecovered,
		ConnectionID: connectionID,
		Connection:   connectionName,
		Provider:     provider,
		Message:      "Connection is back online",
	})
}

// GetBotUsername returns the bot's username (for UI display).
func (b *Bot) GetBotUsername() string {
	b.mu.RLock()
	defer b.mu.RUnlock()
	if b.api != nil {
		return b.api.Self.UserName
	}
	return ""
}

func (b *Bot) pollUpdates(ctx context.Context) {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 30

	b.mu.RLock()
	api := b.api
	ownerID := b.ownerID
	b.mu.RUnlock()

	if api == nil {
		return
	}

	updates := api.GetUpdatesChan(u)

	for {
		select {
		case <-ctx.Done():
			return
		case update, ok := <-updates:
			if !ok {
				return
			}
			if update.Message == nil {
				continue
			}
			// Owner-only auth
			if update.Message.From.ID != ownerID {
				continue
			}
			b.handleCommand(update.Message)
		}
	}
}

func (b *Bot) handleCommand(msg *tgbotapi.Message) {
	text := strings.TrimSpace(msg.Text)
	if text == "" {
		return
	}

	parts := strings.Fields(text)
	cmd := strings.ToLower(parts[0])
	args := parts[1:]

	switch cmd {
	case "/start", "/help":
		b.sendMarkdown(FormatHelp())
	case "/status":
		b.cmdStatus()
	case "/usage":
		b.cmdUsage(args)
	case "/connections":
		b.cmdConnections()
	case "/mute":
		b.cmdMute(args)
	case "/unmute":
		b.cmdUnmute()
	default:
		b.sendMarkdown("Unknown command\\. Use /help for available commands\\.")
	}
}

func (b *Bot) sendMarkdown(text string) error {
	b.mu.RLock()
	api := b.api
	ownerID := b.ownerID
	b.mu.RUnlock()

	if api == nil {
		return fmt.Errorf("telegram: bot not initialized")
	}

	msg := tgbotapi.NewMessage(ownerID, text)
	msg.ParseMode = tgbotapi.ModeMarkdownV2
	_, err := api.Send(msg)
	if err != nil {
		log.Printf("[TELEGRAM] Failed to send message: %v", err)
	}
	return err
}

func (b *Bot) isMuted() bool {
	b.muteMu.RLock()
	defer b.muteMu.RUnlock()
	return time.Now().Before(b.mutedUntil)
}

func (b *Bot) setMute(until time.Time) {
	b.muteMu.Lock()
	b.mutedUntil = until
	b.muteMu.Unlock()

	// Persist mute state
	_ = b.store.Update(func(cfg *domain.AppConfig) {
		cfg.Settings.Telegram.MutedUntil = until.Format(time.RFC3339)
	})
}

func (b *Bot) clearMute() {
	b.muteMu.Lock()
	b.mutedUntil = time.Time{}
	b.muteMu.Unlock()

	_ = b.store.Update(func(cfg *domain.AppConfig) {
		cfg.Settings.Telegram.MutedUntil = ""
	})
}
