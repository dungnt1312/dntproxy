package http

import (
	"sync"
	"time"

	"github.com/dungnt/dntproxy/internal/port"
	"github.com/gin-gonic/gin"
)

// In-memory auth session storage (cleaned up after use or expiry)
var (
	authSessions   = make(map[string]*authSession)
	authSessionsMu sync.Mutex

	openaiSessions   = make(map[string]*openaiSession)
	openaiSessionsMu sync.Mutex

	qwenSessions   = make(map[string]*qwenSession)
	qwenSessionsMu sync.Mutex

	xaiSessions   = make(map[string]*xaiSession)
	xaiSessionsMu sync.Mutex

	maxAuthSessions = 1000
)

type authSession struct {
	// Builder ID / IDC device code flow
	ClientID     string
	ClientSecret string
	DeviceCode   string
	Region       string
	StartURL     string
	Interval     int
	AuthMethod   string // "builder-id" or "idc"

	// Social login (PKCE)
	CodeVerifier string
	State        string
	Provider     string // "google" or "github"

	CreatedAt time.Time
}

func init() {
	go func() {
		for {
			time.Sleep(5 * time.Minute)
			cleanupAuthSessions()
			cleanupOpenAISessions()
			cleanupQwenSessions()
			cleanupXAISessions()
		}
	}()
}

func cleanupAuthSessions() {
	authSessionsMu.Lock()
	defer authSessionsMu.Unlock()
	now := time.Now()
	for id, s := range authSessions {
		if now.Sub(s.CreatedAt) > 15*time.Minute {
			delete(authSessions, id)
		}
	}
}

func cleanupOpenAISessions() {
	openaiSessionsMu.Lock()
	defer openaiSessionsMu.Unlock()
	now := time.Now()
	for id, s := range openaiSessions {
		if now.Sub(s.CreatedAt) > 15*time.Minute {
			delete(openaiSessions, id)
		}
	}
}

// RegisterAuthRoutes adds auth flow endpoints.
// Handler implementations are split across separate files:
//   - auth-kiro-handler.go    — Builder ID, IDC, device code polling
//   - auth-social-handler.go  — Google/GitHub PKCE
//   - auth-openai-handler.go  — OpenAI OAuth (PKCE + callback server)
//   - auth-qwen-handler.go    — Qwen OAuth (device code)
//   - auth-models-handler.go  — Fetch models from provider API
func RegisterAuthRoutes(r *gin.Engine, store port.CredentialStore) {
	authGroup := r.Group("/api/auth")
	{
		authGroup.POST("/kiro/start-builderid", authStartBuilderID())
		authGroup.POST("/kiro/start-idc", authStartIDC())
		authGroup.POST("/kiro/poll", authPoll(store))
		authGroup.POST("/kiro/start-social", authStartSocial())
		authGroup.POST("/kiro/exchange-social", authExchangeSocial(store))

		// OpenAI OAuth (PKCE, Authorization Code)
		authGroup.POST("/openai/start", authOpenAIStart())
		authGroup.POST("/openai/exchange", authOpenAIExchange(store))

		// Qwen OAuth (Device Code Flow)
		authGroup.POST("/qwen/start", authQwenStart())
		authGroup.POST("/qwen/poll", authQwenPoll(store))

		// xAI/Grok OAuth (PKCE, Authorization Code)
		authGroup.POST("/xai/start", authXAIStart())
		authGroup.POST("/xai/exchange", authXAIExchange(store))
	}

	// Fetch models from provider API
	r.POST("/api/connections/:id/fetch-models", apiFetchConnectionModels(store))
}
