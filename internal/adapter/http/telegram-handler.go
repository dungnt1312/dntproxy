package http

import (
	"net/http"

	"github.com/dungnt/dntproxy/internal/adapter/telegram"
	"github.com/dungnt/dntproxy/internal/domain"
	"github.com/dungnt/dntproxy/internal/port"
	"github.com/gin-gonic/gin"
)

// RegisterTelegramRoutes adds /api/telegram/* endpoints.
func RegisterTelegramRoutes(api *gin.RouterGroup, store port.CredentialStore) {
	tg := api.Group("/telegram")
	{
		tg.GET("/status", apiTelegramStatus(store))
		tg.POST("/start", apiTelegramStart(store))
		tg.POST("/stop", apiTelegramStop(store))
		tg.POST("/test", apiTelegramTest(store))
	}
}

func getTelegramBot(c *gin.Context) *telegram.Bot {
	if bot, ok := globalTelegramBot.(*telegram.Bot); ok {
		return bot
	}
	return nil
}

func apiTelegramStatus(store port.CredentialStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		bot := getTelegramBot(c)

		settings, err := store.GetSettings()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load settings"})
			return
		}

		status := gin.H{
			"enabled":    settings.Telegram.Enabled,
			"running":    false,
			"username":   "",
			"ownerID":    settings.Telegram.OwnerID,
			"hasToken":   settings.Telegram.BotToken != "",
			"mutedUntil": settings.Telegram.MutedUntil,
		}

		if bot != nil && bot.IsRunning() {
			status["running"] = true
			status["username"] = bot.GetBotUsername()
		}

		c.JSON(http.StatusOK, status)
	}
}

func apiTelegramStart(store port.CredentialStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		bot := getTelegramBot(c)
		if bot == nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "telegram bot not initialized"})
			return
		}

		if bot.IsRunning() {
			c.JSON(http.StatusOK, gin.H{"message": "bot already running", "username": bot.GetBotUsername()})
			return
		}

		if err := bot.Start(); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		// Start alerter (use existing or create new)
		alerter := bot.GetAlerter()
		if alerter == nil {
			alerter = telegram.NewAlerter(bot, store)
			bot.SetAlerter(alerter)
		}
		alerter.Start()

		// Persist enabled state
		_ = store.Update(func(cfg *domain.AppConfig) {
			cfg.Settings.Telegram.Enabled = true
		})

		c.JSON(http.StatusOK, gin.H{"message": "bot started", "username": bot.GetBotUsername()})
	}
}

func apiTelegramStop(store port.CredentialStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		bot := getTelegramBot(c)
		if bot == nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "telegram bot not initialized"})
			return
		}

		// Stop alerter first
		if alerter := bot.GetAlerter(); alerter != nil {
			alerter.Stop()
		}
		bot.Stop()

		// Persist disabled state so the bot/alerter does not restart on next boot.
		_ = store.Update(func(cfg *domain.AppConfig) {
			cfg.Settings.Telegram.Enabled = false
		})

		c.JSON(http.StatusOK, gin.H{"message": "bot stopped"})
	}
}

func apiTelegramTest(store port.CredentialStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		bot := getTelegramBot(c)
		if bot == nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "telegram bot not initialized"})
			return
		}

		if !bot.IsRunning() {
			// Try to start temporarily for test
			if err := bot.Start(); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "failed to start bot: " + err.Error()})
				return
			}
			defer func() {
				settings, _ := store.GetSettings()
				if settings != nil && !settings.Telegram.Enabled {
					bot.Stop()
				}
			}()
		}

		err := bot.SendMessage("✅ dntproxy Telegram bot is working!")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "failed to send test message: " + err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "test message sent"})
	}
}
