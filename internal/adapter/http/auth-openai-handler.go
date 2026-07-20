package http

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/dungnt/dntproxy/internal/adapter/auth"
	"github.com/dungnt/dntproxy/internal/domain"
	"github.com/dungnt/dntproxy/internal/port"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// === OpenAI OAuth (PKCE, Authorization Code Flow) ===
// Mirrors the Codex CLI CODEX_CONFIG implementation.
// client_id: app_EMoamEEZ73f0CkXaXp7hrann
// auth URL:  https://auth.openai.com/oauth/authorize
// token URL: https://auth.openai.com/oauth/token

const (
	openaiClientID     = "app_EMoamEEZ73f0CkXaXp7hrann"
	openaiAuthURL      = "https://auth.openai.com/oauth/authorize"
	openaiTokenURL     = "https://auth.openai.com/oauth/token"
	openaiScope        = "openid profile email offline_access"
	openaiCallbackPort = 1455
)

// openaiSession extends authSession with OpenAI-specific captured code.
type openaiSession struct {
	CodeVerifier string
	State        string
	Code         string // filled when callback received
	Done         bool
	Error        string
	TenantID     string
	APIKeyID     string
	CreatedAt    time.Time
}

func authOpenAIStart() gin.HandlerFunc {
	return func(c *gin.Context) {
		codeVerifier, codeChallenge, state, err := auth.GeneratePKCE()
		if err != nil {
			c.JSON(500, gin.H{"error": "Failed to generate PKCE: " + err.Error()})
			return
		}

		redirectURI := fmt.Sprintf("http://localhost:%d/auth/callback", openaiCallbackPort)

		params := map[string]string{
			"response_type":              "code",
			"client_id":                  openaiClientID,
			"redirect_uri":               redirectURI,
			"scope":                      openaiScope,
			"code_challenge":             codeChallenge,
			"code_challenge_method":      "S256",
			"state":                      state,
			"id_token_add_organizations": "true",
			"originator":                 "openai_native",
		}

		queryParts := []string{}
		for k, v := range params {
			queryParts = append(queryParts, k+"="+url.QueryEscape(v))
		}
		authURL := openaiAuthURL + "?" + strings.Join(queryParts, "&")

		sessionID := uuid.New().String()
		starterTenant, starterKey := authCallerIDs(c)
		sess := &openaiSession{
			CodeVerifier: codeVerifier,
			State:        state,
			TenantID:     starterTenant,
			APIKeyID:     starterKey,
			CreatedAt:    time.Now(),
		}
		openaiSessionsMu.Lock()
		if len(openaiSessions) >= maxAuthSessions {
			openaiSessionsMu.Unlock()
			c.JSON(429, gin.H{"error": "too many pending auth sessions"})
			return
		}
		openaiSessions[sessionID] = sess
		openaiSessionsMu.Unlock()

		// Start one-shot local callback server
		go func() {
			mux := http.NewServeMux()
			server := &http.Server{Addr: fmt.Sprintf(":%d", openaiCallbackPort), Handler: mux}
			mux.HandleFunc("/auth/callback", func(w http.ResponseWriter, r *http.Request) {
				q := r.URL.Query()
				openaiSessionsMu.Lock()
				if q.Get("error") != "" {
					sess.Error = q.Get("error_description")
					if sess.Error == "" {
						sess.Error = q.Get("error")
					}
				} else if st := q.Get("state"); st != "" && st != sess.State {
					sess.Error = "Invalid OAuth state"
				} else {
					sess.Code = q.Get("code")
				}
				sess.Done = true
				openaiSessionsMu.Unlock()

				w.Header().Set("Content-Type", "text/html")
				if sess.Error != "" {
					fmt.Fprintf(w, `<html><body><h2>❌ Authorization failed</h2><p>%s</p><p>You can close this tab.</p></body></html>`, sess.Error)
				} else {
					fmt.Fprintf(w, `<html><body><h2>✅ Connected to OpenAI!</h2><p>You can close this tab and return to dntproxy.</p></body></html>`)
				}
				go func() { time.Sleep(2 * time.Second); server.Close() }()
			})
			// Timeout after 5 min
			go func() {
				time.Sleep(5 * time.Minute)
				openaiSessionsMu.Lock()
				if !sess.Done {
					sess.Done = true
					sess.Error = "Authentication timed out"
				}
				openaiSessionsMu.Unlock()
				server.Close()
			}()
			server.ListenAndServe()
		}()

		c.JSON(200, gin.H{
			"sessionId":   sessionID,
			"authUrl":     authURL,
			"redirectUri": redirectURI,
		})
	}
}

func authOpenAIExchange(store port.CredentialStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			SessionID   string `json:"sessionId"`
			CallbackURL string `json:"callbackUrl"` // optional manual paste
		}
		if err := c.ShouldBindJSON(&req); err != nil || req.SessionID == "" {
			c.JSON(400, gin.H{"error": "sessionId is required"})
			return
		}

		openaiSessionsMu.Lock()
		sess, ok := openaiSessions[req.SessionID]
		openaiSessionsMu.Unlock()
		if !ok {
			c.JSON(404, gin.H{"error": "Session not found or expired"})
			return
		}

		if !authSessionAllowed(c, sess.TenantID, sess.APIKeyID) {
			c.JSON(403, gin.H{"error": "Access denied"})
			return
		}

		// If user manually pasted the callback URL, inject the code into the session
		if req.CallbackURL != "" {
			parsed, parseErr := url.Parse(req.CallbackURL)
			if parseErr != nil {
				c.JSON(400, gin.H{"error": "Invalid callback URL: " + parseErr.Error()})
				return
			}
			manualCode := parsed.Query().Get("code")
			if manualCode == "" {
				c.JSON(400, gin.H{"error": "No 'code' parameter found in the callback URL"})
				return
			}
			openaiSessionsMu.Lock()
			sess.Code = manualCode
			sess.Done = true
			sess.Error = ""
			openaiSessionsMu.Unlock()
		}

		// Check if callback was received (auto or manual)
		openaiSessionsMu.Lock()
		done := sess.Done
		code := sess.Code
		errMsg := sess.Error
		openaiSessionsMu.Unlock()

		if !done {
			c.JSON(200, gin.H{"status": "pending"})
			return
		}
		if errMsg != "" {
			c.JSON(200, gin.H{"status": "error", "error": errMsg})
			return
		}
		if code == "" {
			c.JSON(200, gin.H{"status": "error", "error": "No authorization code received"})
			return
		}

		redirectURI := fmt.Sprintf("http://localhost:%d/auth/callback", openaiCallbackPort)

		// Exchange code for tokens
		formData := url.Values{
			"grant_type":    {"authorization_code"},
			"client_id":     {openaiClientID},
			"code":          {code},
			"redirect_uri":  {redirectURI},
			"code_verifier": {sess.CodeVerifier},
		}

		resp, err := http.Post(openaiTokenURL, "application/x-www-form-urlencoded", strings.NewReader(formData.Encode()))
		if err != nil {
			c.JSON(502, gin.H{"error": "Token exchange request failed: " + err.Error()})
			return
		}
		defer resp.Body.Close()

		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1*1024*1024))
		if resp.StatusCode != 200 {
			c.JSON(400, gin.H{"error": fmt.Sprintf("Token exchange failed (%d): %s", resp.StatusCode, string(body))})
			return
		}

		var tokens struct {
			AccessToken  string `json:"access_token"`
			RefreshToken string `json:"refresh_token"`
			IDToken      string `json:"id_token"`
			ExpiresIn    int    `json:"expires_in"`
			TokenType    string `json:"token_type"`
		}
		if err := json.Unmarshal(body, &tokens); err != nil {
			c.JSON(500, gin.H{"error": "Failed to parse token response: " + err.Error()})
			return
		}

		email := auth.ExtractEmailFromJWT(tokens.IDToken)
		if email == "" {
			email = auth.ExtractEmailFromJWT(tokens.AccessToken)
		}

		cfg, err := store.Load()
		if err != nil {
			c.JSON(500, gin.H{"error": "Failed to load config"})
			return
		}
		name := email
		if name == "" {
			name = fmt.Sprintf("OpenAI Account %d", len(cfg.ProviderConnections)+1)
		}

		expiresIn := tokens.ExpiresIn
		if expiresIn == 0 {
			expiresIn = 3600
		}
		now := time.Now().UTC().Format(time.RFC3339)

		conn := domain.ProviderConnection{
			ID:           uuid.New().String(),
			Provider:     "openai",
			AuthType:     "oauth",
			Name:         name,
			Weight:       100,
			IsActive:     true,
			AccessToken:  tokens.AccessToken,
			RefreshToken: tokens.RefreshToken,
			ExpiresAt:    time.Now().Add(time.Duration(expiresIn) * time.Second).UTC().Format(time.RFC3339),
			ExpiresIn:    expiresIn,
			Email:        email,
			TestStatus:   "active",
			ProviderSpecificData: map[string]interface{}{
				"authMethod": "oauth",
				"idToken":    tokens.IDToken,
			},
			CreatedAt: now,
			UpdatedAt: now,
			TenantID:  GetTenantID(c),
		}

		cfg.ProviderConnections = append(cfg.ProviderConnections, conn)
		if err := store.Save(cfg); err != nil {
			c.JSON(500, gin.H{"error": "Failed to save: " + err.Error()})
			return
		}

		openaiSessionsMu.Lock()
		delete(openaiSessions, req.SessionID)
		openaiSessionsMu.Unlock()

		c.JSON(200, gin.H{
			"status": "success",
			"id":     conn.ID,
			"name":   conn.Name,
			"email":  email,
		})
	}
}
