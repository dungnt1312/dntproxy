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

	// Bound to the dashboard principal that started the flow.
	TenantID  string
	APIKeyID  string
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

// RegisterAuthRoutes adds OAuth enrollment + fetch-models endpoints under the
// protected /api group (dashboardKeyMiddleware). All of these mutate or use
// stored credentials, so they require a valid dashboard API key.
//
// Handler implementations are split across separate files:
//   - auth-kiro-handler.go    — Builder ID, IDC, device code polling
//   - auth-social-handler.go  — Google/GitHub PKCE
//   - auth-openai-handler.go  — OpenAI OAuth (PKCE + callback server)
//   - auth-qwen-handler.go    — Qwen OAuth (device code)
//   - auth-xai-handler.go     — xAI/Grok OAuth + import
//   - auth-models-handler.go  — Fetch models from provider API
func RegisterAuthRoutes(api *gin.RouterGroup, store port.CredentialStore) {
	authGroup := api.Group("/auth")
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
		authGroup.POST("/xai/import-file", authXAIImportFile(store))
	}

	// Fetch models from provider API (uses connection credentials)
	api.POST("/connections/:id/fetch-models", apiFetchConnectionModels(store))
}

// authCallerIDs returns tenant + api key id from the current dashboard request.
func authCallerIDs(c *gin.Context) (tenantID, keyID string) {
	tenantID = GetTenantID(c)
	if v, ok := c.Get("apiKeyID"); ok {
		if s, ok := v.(string); ok {
			keyID = s
		}
	}
	return tenantID, keyID
}

// authSessionAllowed ensures the completing caller matches the starter principal.
func authSessionAllowed(c *gin.Context, sessionTenant, sessionKey string) bool {
	tenantID, keyID := authCallerIDs(c)
	if sessionTenant != tenantID {
		return false
	}
	// If starter recorded a key id, require the same key (prevents session hijack
	// across different dashboard keys in the same tenant).
	if sessionKey != "" && keyID != "" && sessionKey != keyID {
		return false
	}
	return true
}
