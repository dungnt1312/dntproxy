# Grok Build OAuth Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add minimal Grok Build OAuth support to dntproxy through xAI's Responses API while preserving the existing OpenAI-compatible client surface.

**Architecture:** Add `xai` as a first-class provider with OAuth PKCE auth helpers, auth HTTP handlers, token refresh support, provider registration, model routing, and a focused executor that translates OpenAI Chat Completions requests to xAI Responses requests. Keep the implementation isolated in `internal/adapter/auth`, `internal/adapter/http`, `internal/adapter/xai`, and small domain/service registration edits.

**Tech Stack:** Go, Gin, existing dntproxy `ProviderExecutor`, JSON DB credential store, existing request logger, existing token refresh service, SSE streaming over `net/http`.

---

## File Structure

- Create `internal/adapter/auth/xai.go`: xAI OAuth constants, endpoint validation, authorization URL construction, code exchange, refresh, token parsing.
- Create `internal/adapter/auth/xai_test.go`: unit tests for OAuth URL, endpoint validation, token exchange, refresh token behavior.
- Create `internal/adapter/http/auth-xai-handler.go`: Gin auth start/exchange handlers and in-memory xAI auth session type.
- Modify `internal/adapter/http/auth-handler.go`: session storage cleanup and route registration for xAI.
- Create `internal/adapter/xai/translator.go`: OpenAI Chat Completions to xAI Responses request translation and xAI Responses SSE/event translation helpers.
- Create `internal/adapter/xai/translator_test.go`: request and SSE translation tests.
- Create `internal/adapter/xai/executor.go`: `port.ProviderExecutor` implementation for xAI Responses API.
- Create `internal/adapter/xai/executor_test.go`: upstream request/error behavior tests with `httptest.Server`.
- Modify `internal/domain/provider-config.go`: add xAI provider config.
- Modify `internal/domain/model-definition.go`: add default Grok model definitions.
- Modify `internal/service/model-resolver.go` or model parser alias map: map public prefix `grok` to provider ID `xai`.
- Modify `cmd/dntproxy/main.go`: register xAI executor.
- Modify `internal/adapter/auth/token-refresh.go`: add xAI token refresh path.

## Task 1: xAI OAuth Helpers

**Files:**
- Create: `internal/adapter/auth/xai.go`
- Create: `internal/adapter/auth/xai_test.go`

- [ ] **Step 1: Write failing OAuth helper tests**

Create `internal/adapter/auth/xai_test.go` with tests covering endpoint validation, authorize URL construction, token exchange, and refresh preserving old refresh tokens.

```go
package auth

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestValidateXAIEndpoint(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		wantErr bool
	}{
		{name: "x.ai", raw: "https://x.ai/oauth", wantErr: false},
		{name: "subdomain", raw: "https://auth.x.ai/oauth", wantErr: false},
		{name: "http rejected", raw: "http://auth.x.ai/oauth", wantErr: true},
		{name: "foreign host rejected", raw: "https://evil.example/oauth", wantErr: true},
		{name: "empty rejected", raw: "", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ValidateXAIEndpoint(tt.raw, "authorization_endpoint")
			if (err != nil) != tt.wantErr {
				t.Fatalf("ValidateXAIEndpoint() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestBuildXAIAuthURL(t *testing.T) {
	got, err := BuildXAIAuthURL(XAIAuthorizeURLParams{
		AuthorizationEndpoint: "https://auth.x.ai/oauth/authorize",
		RedirectURI:           "http://127.0.0.1:56121/callback",
		CodeChallenge:        "challenge",
		State:                "state-123",
		Nonce:                "nonce-456",
	})
	if err != nil {
		t.Fatalf("BuildXAIAuthURL() error = %v", err)
	}

	parsed, err := url.Parse(got)
	if err != nil {
		t.Fatalf("parse URL: %v", err)
	}
	q := parsed.Query()
	assertQuery := func(key, want string) {
		t.Helper()
		if gotValue := q.Get(key); gotValue != want {
			t.Fatalf("query %s = %q, want %q", key, gotValue, want)
		}
	}
	assertQuery("response_type", "code")
	assertQuery("client_id", XAIClientID)
	assertQuery("redirect_uri", "http://127.0.0.1:56121/callback")
	assertQuery("scope", XAIScope)
	assertQuery("code_challenge", "challenge")
	assertQuery("code_challenge_method", "S256")
	assertQuery("state", "state-123")
	assertQuery("nonce", "nonce-456")
	assertQuery("plan", "generic")
	assertQuery("referrer", "dntproxy")
}

func TestExchangeXAICode(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s, want POST", r.Method)
		}
		if err := r.ParseForm(); err != nil {
			t.Fatalf("ParseForm: %v", err)
		}
		if got := r.Form.Get("grant_type"); got != "authorization_code" {
			t.Fatalf("grant_type = %q", got)
		}
		if got := r.Form.Get("client_id"); got != XAIClientID {
			t.Fatalf("client_id = %q", got)
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"access_token":  "access-token",
			"refresh_token": "refresh-token",
			"id_token":      "header.eyJlbWFpbCI6InVzZXJAZXhhbXBsZS5jb20iLCJzdWIiOiJzdWItMTIzIn0.signature",
			"token_type":    "Bearer",
			"expires_in":    3600,
		})
	}))
	defer server.Close()

	tokens, err := ExchangeXAICode("code", "http://127.0.0.1:56121/callback", "verifier", server.URL)
	if err != nil {
		t.Fatalf("ExchangeXAICode() error = %v", err)
	}
	if tokens.AccessToken != "access-token" || tokens.RefreshToken != "refresh-token" {
		t.Fatalf("tokens = %+v", tokens)
	}
	if tokens.Email != "user@example.com" || tokens.Subject != "sub-123" {
		t.Fatalf("identity = %q/%q", tokens.Email, tokens.Subject)
	}
}

func TestRefreshXAITokenPreservesRefreshToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Fatalf("ParseForm: %v", err)
		}
		if got := r.Form.Get("grant_type"); got != "refresh_token" {
			t.Fatalf("grant_type = %q", got)
		}
		if got := r.Form.Get("refresh_token"); got != "old-refresh" {
			t.Fatalf("refresh_token = %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"new-access","expires_in":1800}`))
	}))
	defer server.Close()

	tokens, err := RefreshXAIToken("old-refresh", server.URL)
	if err != nil {
		t.Fatalf("RefreshXAIToken() error = %v", err)
	}
	if tokens.AccessToken != "new-access" {
		t.Fatalf("access token = %q", tokens.AccessToken)
	}
	if tokens.RefreshToken != "old-refresh" {
		t.Fatalf("refresh token = %q, want old-refresh", tokens.RefreshToken)
	}
}

func TestBuildXAIAuthURLRejectsInvalidEndpoint(t *testing.T) {
	_, err := BuildXAIAuthURL(XAIAuthorizeURLParams{
		AuthorizationEndpoint: "https://example.com/oauth",
		RedirectURI:           "http://127.0.0.1:56121/callback",
		CodeChallenge:        "challenge",
		State:                "state",
		Nonce:                "nonce",
	})
	if err == nil || !strings.Contains(err.Error(), "not on x.ai") {
		t.Fatalf("error = %v, want x.ai validation error", err)
	}
}
```

- [ ] **Step 2: Run tests and verify they fail**

Run: `go test ./internal/adapter/auth -run 'Test.*XAI'`

Expected: FAIL because xAI helpers are undefined.

- [ ] **Step 3: Implement xAI OAuth helper**

Create `internal/adapter/auth/xai.go`.

```go
package auth

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	XAIDefaultAPIBaseURL = "https://api.x.ai/v1"
	XAIIssuer            = "https://auth.x.ai"
	XAIDiscoveryURL      = XAIIssuer + "/.well-known/openid-configuration"
	XAIClientID          = "b1a00492-073a-47ea-816f-4c329264a828"
	XAIScope             = "openid profile email offline_access grok-cli:access api:access"
	XAIRedirectURI       = "http://127.0.0.1:56121/callback"
)

type XAIAuthorizeURLParams struct {
	AuthorizationEndpoint string
	RedirectURI           string
	CodeChallenge         string
	State                 string
	Nonce                 string
}

type XAIDiscovery struct {
	AuthorizationEndpoint string
	TokenEndpoint         string
}

type XAITokenResponse struct {
	AccessToken  string
	RefreshToken string
	IDToken      string
	TokenType    string
	ExpiresIn    int
	Email        string
	Subject      string
}

func ValidateXAIEndpoint(rawURL string, field string) (string, error) {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return "", fmt.Errorf("xai discovery %s is empty", field)
	}
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return "", fmt.Errorf("xai discovery %s is invalid: %w", field, err)
	}
	if parsed.Scheme != "https" {
		return "", fmt.Errorf("xai discovery %s must use https: %q", field, rawURL)
	}
	host := strings.ToLower(strings.TrimSpace(parsed.Hostname()))
	if host != "x.ai" && !strings.HasSuffix(host, ".x.ai") {
		return "", fmt.Errorf("xai discovery %s host %q is not on x.ai", field, host)
	}
	return rawURL, nil
}

func DiscoverXAI() (*XAIDiscovery, error) {
	req, err := http.NewRequest(http.MethodGet, XAIDiscoveryURL, nil)
	if err != nil {
		return nil, fmt.Errorf("xai discovery: create request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("xai discovery: request failed: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("xai discovery: read response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("xai discovery failed (%d): %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	var payload struct {
		AuthorizationEndpoint string `json:"authorization_endpoint"`
		TokenEndpoint         string `json:"token_endpoint"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("xai discovery: parse response: %w", err)
	}
	authEndpoint, err := ValidateXAIEndpoint(payload.AuthorizationEndpoint, "authorization_endpoint")
	if err != nil {
		return nil, err
	}
	tokenEndpoint, err := ValidateXAIEndpoint(payload.TokenEndpoint, "token_endpoint")
	if err != nil {
		return nil, err
	}
	return &XAIDiscovery{AuthorizationEndpoint: authEndpoint, TokenEndpoint: tokenEndpoint}, nil
}

func BuildXAIAuthURL(params XAIAuthorizeURLParams) (string, error) {
	endpoint, err := ValidateXAIEndpoint(params.AuthorizationEndpoint, "authorization_endpoint")
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(params.RedirectURI) == "" {
		return "", fmt.Errorf("xai authorize URL: redirect URI is required")
	}
	if strings.TrimSpace(params.CodeChallenge) == "" {
		return "", fmt.Errorf("xai authorize URL: code challenge is required")
	}
	if strings.TrimSpace(params.State) == "" {
		return "", fmt.Errorf("xai authorize URL: state is required")
	}
	if strings.TrimSpace(params.Nonce) == "" {
		return "", fmt.Errorf("xai authorize URL: nonce is required")
	}
	values := url.Values{
		"response_type":         {"code"},
		"client_id":             {XAIClientID},
		"redirect_uri":          {strings.TrimSpace(params.RedirectURI)},
		"scope":                 {XAIScope},
		"code_challenge":        {strings.TrimSpace(params.CodeChallenge)},
		"code_challenge_method": {"S256"},
		"state":                 {strings.TrimSpace(params.State)},
		"nonce":                 {strings.TrimSpace(params.Nonce)},
		"plan":                  {"generic"},
		"referrer":              {"dntproxy"},
	}
	return endpoint + "?" + values.Encode(), nil
}

func ExchangeXAICode(code, redirectURI, codeVerifier, tokenEndpoint string) (*XAITokenResponse, error) {
	if strings.TrimSpace(tokenEndpoint) == "" {
		discovery, err := DiscoverXAI()
		if err != nil {
			return nil, err
		}
		tokenEndpoint = discovery.TokenEndpoint
	}
	form := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {strings.TrimSpace(code)},
		"redirect_uri":  {strings.TrimSpace(redirectURI)},
		"client_id":     {XAIClientID},
		"code_verifier": {strings.TrimSpace(codeVerifier)},
	}
	return postXAITokenForm(tokenEndpoint, form, "")
}

func RefreshXAIToken(refreshToken, tokenEndpoint string) (*XAITokenResponse, error) {
	if strings.TrimSpace(tokenEndpoint) == "" {
		discovery, err := DiscoverXAI()
		if err != nil {
			return nil, err
		}
		tokenEndpoint = discovery.TokenEndpoint
	}
	form := url.Values{
		"grant_type":    {"refresh_token"},
		"client_id":     {XAIClientID},
		"refresh_token": {strings.TrimSpace(refreshToken)},
	}
	return postXAITokenForm(tokenEndpoint, form, refreshToken)
}

func postXAITokenForm(tokenEndpoint string, form url.Values, fallbackRefreshToken string) (*XAITokenResponse, error) {
	req, err := http.NewRequest(http.MethodPost, strings.TrimSpace(tokenEndpoint), strings.NewReader(form.Encode()))
	if err != nil {
		return nil, fmt.Errorf("xai token request: create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("xai token request failed: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("xai token response: read body: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("xai token request failed (%d): %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	var payload struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		IDToken      string `json:"id_token"`
		TokenType    string `json:"token_type"`
		ExpiresIn    int    `json:"expires_in"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("xai token response: parse body: %w", err)
	}
	if strings.TrimSpace(payload.AccessToken) == "" {
		return nil, fmt.Errorf("xai token response missing access_token")
	}
	if payload.RefreshToken == "" {
		payload.RefreshToken = fallbackRefreshToken
	}
	email, subject := extractXAIIdentity(payload.IDToken)
	return &XAITokenResponse{
		AccessToken:  strings.TrimSpace(payload.AccessToken),
		RefreshToken: strings.TrimSpace(payload.RefreshToken),
		IDToken:      strings.TrimSpace(payload.IDToken),
		TokenType:    strings.TrimSpace(payload.TokenType),
		ExpiresIn:    payload.ExpiresIn,
		Email:        email,
		Subject:      subject,
	}, nil
}

func extractXAIIdentity(token string) (email string, subject string) {
	parts := strings.Split(token, ".")
	if len(parts) < 2 {
		return "", ""
	}
	payload := parts[1]
	payload += strings.Repeat("=", (4-len(payload)%4)%4)
	raw, err := base64.URLEncoding.DecodeString(payload)
	if err != nil {
		return "", ""
	}
	var claims map[string]interface{}
	if err := json.Unmarshal(raw, &claims); err != nil {
		return "", ""
	}
	if v, ok := claims["email"].(string); ok {
		email = strings.TrimSpace(v)
	}
	if v, ok := claims["sub"].(string); ok {
		subject = strings.TrimSpace(v)
	}
	return email, subject
}
```

- [ ] **Step 4: Run OAuth helper tests**

Run: `go test ./internal/adapter/auth -run 'Test.*XAI'`

Expected: PASS.

## Task 2: Domain Registration and Model Prefix

**Files:**
- Modify: `internal/domain/provider-config.go`
- Modify: `internal/domain/model-definition.go`
- Modify: `internal/service/model-resolver.go`
- Test: existing resolver/model tests plus new xAI prefix test in `internal/service/model-parser_test.go`

- [ ] **Step 1: Add failing model prefix test**

Append to `internal/service/model-parser_test.go`.

```go
func TestParseModelStringGrokPrefix(t *testing.T) {
	parsed, err := ParseModelString("grok/grok-4.3@conn-xai")
	if err != nil {
		t.Fatalf("ParseModelString() error = %v", err)
	}
	if parsed.Provider != "xai" {
		t.Fatalf("provider = %q, want xai", parsed.Provider)
	}
	if parsed.Model != "grok-4.3" {
		t.Fatalf("model = %q, want grok-4.3", parsed.Model)
	}
	if parsed.ConnectionID != "conn-xai" {
		t.Fatalf("connection ID = %q, want conn-xai", parsed.ConnectionID)
	}
}
```

- [ ] **Step 2: Run resolver test and verify it fails**

Run: `go test ./internal/service -run TestParseModelStringGrokPrefix`

Expected: FAIL because `grok` is not mapped to `xai`.

- [ ] **Step 3: Add provider config**

Modify `internal/domain/provider-config.go` by adding this entry to `ProviderConfigs`.

```go
	"xai": {
		ID:             "xai",
		Name:           "Grok Build (xAI)",
		Icon:           "grok",
		AuthMethods:    []string{"oauth"},
		DefaultBaseURL: "https://api.x.ai/v1",
		ChatPath:       "/responses",
		Format:         FormatOpenAIChat,
		SupportsQuota:  false,
		// DefaultModels auto-populated from model-definition registry
	},
```

- [ ] **Step 4: Add model definitions**

Modify `internal/domain/model-definition.go` by adding active xAI model definitions in the existing model definition list format used by the file. Include at least:

```go
Provider: "xai", Model: "grok-4.3", DisplayName: "Grok 4.3"
Provider: "xai", Model: "grok-3-mini", DisplayName: "Grok 3 Mini"
```

Use the exact struct fields already present in `model-definition.go`; do not invent fields.

- [ ] **Step 5: Map public prefix `grok` to provider `xai`**

Modify the alias/prefix map used by `ParseModelString` in `internal/service/model-resolver.go` or the file where `ProviderAliasToID` is defined. Add:

```go
"grok": "xai",
```

If the map lives in `internal/domain`, add the mapping there instead and keep parser behavior unchanged.

- [ ] **Step 6: Run model tests**

Run: `go test ./internal/service -run 'TestParseModelString|TestModel'`

Expected: PASS.

## Task 3: xAI Token Refresh

**Files:**
- Modify: `internal/adapter/auth/token-refresh.go`
- Test: `internal/adapter/auth/xai_test.go`

- [ ] **Step 1: Add token refresh service test**

Add a focused unit test if the existing token refresh service can be exercised with a simple fake store. If creating a fake store is too large, rely on `RefreshXAIToken` tests from Task 1 and add a small branch test only for helper extraction. The expected implementation branch is:

```go
if conn.Provider == "xai" {
	log.Printf("[TOKEN] Refreshing xAI token for %s", conn.Name)
	return s.refreshXAI(conn)
}
```

- [ ] **Step 2: Implement xAI refresh branch**

Modify `internal/adapter/auth/token-refresh.go`.

Add after the Qwen branch in `Refresh`:

```go
	if conn.Provider == "xai" {
		log.Printf("[TOKEN] Refreshing xAI token for %s", conn.Name)
		return s.refreshXAI(conn)
	}
```

Add method near `refreshQwen`:

```go
func (s *TokenRefreshService) refreshXAI(conn *domain.ProviderConnection) (*domain.ProviderConnection, error) {
	tokenEndpoint := getStringFromMap(conn.ProviderSpecificData, "tokenEndpoint")
	tokens, err := RefreshXAIToken(conn.RefreshToken, tokenEndpoint)
	if err != nil {
		return nil, fmt.Errorf("xai refresh failed: %w", err)
	}

	conn.AccessToken = tokens.AccessToken
	if tokens.RefreshToken != "" {
		conn.RefreshToken = tokens.RefreshToken
	}

	expiresIn := tokens.ExpiresIn
	if expiresIn == 0 {
		expiresIn = 3600
	}
	conn.ExpiresIn = expiresIn
	conn.ExpiresAt = time.Now().Add(time.Duration(expiresIn) * time.Second).UTC().Format(time.RFC3339)
	if conn.ProviderSpecificData == nil {
		conn.ProviderSpecificData = make(map[string]interface{})
	}
	if tokens.IDToken != "" {
		conn.ProviderSpecificData["idToken"] = tokens.IDToken
	}
	if tokens.Subject != "" {
		conn.ProviderSpecificData["subject"] = tokens.Subject
	}
	if tokenEndpoint != "" {
		conn.ProviderSpecificData["tokenEndpoint"] = tokenEndpoint
	}
	if tokens.Email != "" {
		conn.Email = tokens.Email
	}
	conn.UpdatedAt = time.Now().UTC().Format(time.RFC3339)

	return conn, nil
}
```

- [ ] **Step 3: Run auth tests**

Run: `go test ./internal/adapter/auth`

Expected: PASS.

## Task 4: xAI Auth HTTP Handlers

**Files:**
- Create: `internal/adapter/http/auth-xai-handler.go`
- Modify: `internal/adapter/http/auth-handler.go`
- Test: existing HTTP auth tests if available; otherwise compile and route tests through router package.

- [ ] **Step 1: Add xAI session storage to auth-handler**

Modify `internal/adapter/http/auth-handler.go` var block:

```go
	xaiSessions   = make(map[string]*xaiSession)
	xaiSessionsMu sync.Mutex
```

Add cleanup call in `init` loop:

```go
			cleanupXAISessions()
```

Add route registration:

```go
		// xAI/Grok OAuth (PKCE, Authorization Code)
		authGroup.POST("/xai/start", authXAIStart())
		authGroup.POST("/xai/exchange", authXAIExchange(store))
```

- [ ] **Step 2: Implement xAI handlers**

Create `internal/adapter/http/auth-xai-handler.go`.

```go
package http

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"net/http"
	"strings"
	"time"

	adapterauth "github.com/dungnt/dntproxy/internal/adapter/auth"
	"github.com/dungnt/dntproxy/internal/domain"
	"github.com/dungnt/dntproxy/internal/port"
	"github.com/gin-gonic/gin"
)

type xaiSession struct {
	CodeVerifier  string
	State         string
	Nonce         string
	RedirectURI   string
	TokenEndpoint string
	CreatedAt     time.Time
}

type xaiStartResponse struct {
	SessionID string `json:"sessionId"`
	AuthURL   string `json:"authUrl"`
	State     string `json:"state"`
}

type xaiExchangeRequest struct {
	SessionID string `json:"sessionId" binding:"required"`
	Code      string `json:"code" binding:"required"`
	State     string `json:"state"`
}

func authXAIStart() gin.HandlerFunc {
	return func(c *gin.Context) {
		pkce, err := adapterauth.GeneratePKCE()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		state, err := randomURLToken(24)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		nonce, err := randomURLToken(24)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		discovery, err := adapterauth.DiscoverXAI()
		if err != nil {
			c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
			return
		}
		authURL, err := adapterauth.BuildXAIAuthURL(adapterauth.XAIAuthorizeURLParams{
			AuthorizationEndpoint: discovery.AuthorizationEndpoint,
			RedirectURI:           adapterauth.XAIRedirectURI,
			CodeChallenge:        pkce.CodeChallenge,
			State:                state,
			Nonce:                nonce,
		})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		sessionID, err := randomURLToken(18)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		xaiSessionsMu.Lock()
		if len(xaiSessions) >= maxAuthSessions {
			xaiSessionsMu.Unlock()
			c.JSON(http.StatusTooManyRequests, gin.H{"error": "too many auth sessions"})
			return
		}
		xaiSessions[sessionID] = &xaiSession{
			CodeVerifier:  pkce.CodeVerifier,
			State:         state,
			Nonce:         nonce,
			RedirectURI:   adapterauth.XAIRedirectURI,
			TokenEndpoint: discovery.TokenEndpoint,
			CreatedAt:     time.Now(),
		}
		xaiSessionsMu.Unlock()

		c.JSON(http.StatusOK, xaiStartResponse{SessionID: sessionID, AuthURL: authURL, State: state})
	}
}

func authXAIExchange(store port.CredentialStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req xaiExchangeRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		xaiSessionsMu.Lock()
		session := xaiSessions[req.SessionID]
		delete(xaiSessions, req.SessionID)
		xaiSessionsMu.Unlock()
		if session == nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "auth session not found or expired"})
			return
		}
		if req.State != "" && req.State != session.State {
			c.JSON(http.StatusBadRequest, gin.H{"error": "state mismatch"})
			return
		}

		tokens, err := adapterauth.ExchangeXAICode(req.Code, session.RedirectURI, session.CodeVerifier, session.TokenEndpoint)
		if err != nil {
			c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
			return
		}
		expiresIn := tokens.ExpiresIn
		if expiresIn == 0 {
			expiresIn = 3600
		}
		now := time.Now().UTC()
		name := tokens.Email
		if name == "" {
			name = tokens.Subject
		}
		if name == "" {
			name = "Grok OAuth"
		}
		conn := &domain.ProviderConnection{
			ID:           fmt.Sprintf("xai-%d", now.UnixMilli()),
			Provider:     "xai",
			AuthType:     "oauth",
			Name:         name,
			Weight:       100,
			IsActive:     true,
			AccessToken:  tokens.AccessToken,
			RefreshToken: tokens.RefreshToken,
			ExpiresIn:    expiresIn,
			ExpiresAt:    now.Add(time.Duration(expiresIn) * time.Second).Format(time.RFC3339),
			Email:        tokens.Email,
			BaseURL:      adapterauth.XAIDefaultAPIBaseURL,
			ProviderSpecificData: map[string]interface{}{
				"authMethod":    "xai-oauth",
				"tokenEndpoint": session.TokenEndpoint,
				"redirectURI":   session.RedirectURI,
				"idToken":       tokens.IDToken,
				"subject":       tokens.Subject,
			},
			CreatedAt: now.Format(time.RFC3339),
			UpdatedAt: now.Format(time.RFC3339),
		}
		if err := store.AddConnection(conn); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"connection": conn})
	}
}

func cleanupXAISessions() {
	xaiSessionsMu.Lock()
	defer xaiSessionsMu.Unlock()
	now := time.Now()
	for id, s := range xaiSessions {
		if now.Sub(s.CreatedAt) > 15*time.Minute {
			delete(xaiSessions, id)
		}
	}
}

func randomURLToken(n int) (string, error) {
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return strings.TrimRight(base64.URLEncoding.EncodeToString(buf), "="), nil
}
```

If `adapterauth.GeneratePKCE()` does not exist with that exact name, use the existing PKCE function from `internal/adapter/auth/pkce.go` and adjust only the call site.

- [ ] **Step 3: Run HTTP adapter tests**

Run: `go test ./internal/adapter/http -run 'TestAuth|TestConnection'`

Expected: PASS or no matching tests; compile must pass.

## Task 5: xAI Request/Response Translator

**Files:**
- Create: `internal/adapter/xai/translator.go`
- Create: `internal/adapter/xai/translator_test.go`

- [ ] **Step 1: Write translator tests**

Create `internal/adapter/xai/translator_test.go`.

```go
package xai

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestTranslateChatToResponses(t *testing.T) {
	body := []byte(`{
		"model":"grok/grok-4.3",
		"stream":true,
		"messages":[
			{"role":"system","content":"be concise"},
			{"role":"user","content":"hello"}
		],
		"temperature":0.2,
		"max_tokens":123
	}`)

	got, err := TranslateChatToResponses("grok-4.3", body)
	if err != nil {
		t.Fatalf("TranslateChatToResponses() error = %v", err)
	}
	var payload map[string]interface{}
	if err := json.Unmarshal(got, &payload); err != nil {
		t.Fatalf("unmarshal translated: %v", err)
	}
	if payload["model"] != "grok-4.3" {
		t.Fatalf("model = %v", payload["model"])
	}
	if payload["stream"] != true {
		t.Fatalf("stream = %v", payload["stream"])
	}
	if payload["instructions"] != "be concise" {
		t.Fatalf("instructions = %v", payload["instructions"])
	}
	if payload["max_output_tokens"].(float64) != 123 {
		t.Fatalf("max_output_tokens = %v", payload["max_output_tokens"])
	}
}

func TestTranslateResponsesEventDelta(t *testing.T) {
	state := NewResponseState("grok-4.3")
	got := TranslateResponsesEvent([]byte(`{"type":"response.output_text.delta","delta":"hi"}`), state)
	if !strings.Contains(got, `"content":"hi"`) {
		t.Fatalf("translated delta = %s", got)
	}
}

func TestTranslateResponsesEventCompleted(t *testing.T) {
	state := NewResponseState("grok-4.3")
	got := TranslateResponsesEvent([]byte(`{"type":"response.completed","response":{"usage":{"input_tokens":10,"output_tokens":2,"total_tokens":12}}}`), state)
	if !strings.Contains(got, `"finish_reason":"stop"`) {
		t.Fatalf("translated completed = %s", got)
	}
	if state.Usage.PromptTokens != 10 || state.Usage.CompletionTokens != 2 {
		t.Fatalf("usage = %+v", state.Usage)
	}
}
```

- [ ] **Step 2: Run translator tests and verify they fail**

Run: `go test ./internal/adapter/xai`

Expected: FAIL because translator package does not exist.

- [ ] **Step 3: Implement translator**

Create `internal/adapter/xai/translator.go` with minimal stable translation.

```go
package xai

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

type chatRequest struct {
	Model               string        `json:"model"`
	Messages            []chatMessage `json:"messages"`
	Stream              bool          `json:"stream"`
	Temperature         *float64      `json:"temperature,omitempty"`
	TopP                *float64      `json:"top_p,omitempty"`
	MaxTokens           int           `json:"max_tokens,omitempty"`
	MaxCompletionTokens int           `json:"max_completion_tokens,omitempty"`
	Tools               []chatTool    `json:"tools,omitempty"`
}

type chatMessage struct {
	Role       string      `json:"role"`
	Content    interface{} `json:"content"`
	ToolCallID string      `json:"tool_call_id,omitempty"`
}

type chatTool struct {
	Type     string           `json:"type"`
	Function chatToolFunction `json:"function"`
}

type chatToolFunction struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description,omitempty"`
	Parameters  map[string]interface{} `json:"parameters"`
}

type responsesRequest struct {
	Model           string                 `json:"model"`
	Input           []responsesInput       `json:"input"`
	Instructions    string                 `json:"instructions,omitempty"`
	Stream          bool                   `json:"stream"`
	Temperature     *float64               `json:"temperature,omitempty"`
	TopP            *float64               `json:"top_p,omitempty"`
	MaxOutputTokens int                    `json:"max_output_tokens,omitempty"`
	Tools           []map[string]interface{} `json:"tools,omitempty"`
}

type responsesInput struct {
	Role    string      `json:"role"`
	Content interface{} `json:"content"`
}

type Usage struct {
	PromptTokens     int
	CompletionTokens int
	TotalTokens      int
}

type ResponseState struct {
	Model            string
	ID               string
	Created          int64
	FinishReasonSent bool
	Usage            Usage
}

func NewResponseState(model string) *ResponseState {
	return &ResponseState{Model: model, ID: fmt.Sprintf("chatcmpl-xai-%d", time.Now().UnixNano()), Created: time.Now().Unix()}
}

func TranslateChatToResponses(model string, body []byte) ([]byte, error) {
	var req chatRequest
	if err := json.Unmarshal(body, &req); err != nil {
		return nil, fmt.Errorf("parse chat request: %w", err)
	}
	out := responsesRequest{
		Model:       model,
		Stream:      req.Stream,
		Temperature: req.Temperature,
		TopP:        req.TopP,
	}
	if req.MaxCompletionTokens > 0 {
		out.MaxOutputTokens = req.MaxCompletionTokens
	} else if req.MaxTokens > 0 {
		out.MaxOutputTokens = req.MaxTokens
	}
	for _, msg := range req.Messages {
		role := strings.TrimSpace(msg.Role)
		switch role {
		case "system", "developer":
			text := contentToText(msg.Content)
			if text != "" {
				if out.Instructions != "" {
					out.Instructions += "\n"
				}
				out.Instructions += text
			}
		case "user", "assistant":
			out.Input = append(out.Input, responsesInput{Role: role, Content: msg.Content})
		case "tool":
			out.Input = append(out.Input, responsesInput{Role: "user", Content: msg.Content})
		default:
			return nil, fmt.Errorf("unsupported message role %q", role)
		}
	}
	for _, tool := range req.Tools {
		if tool.Type != "function" {
			return nil, fmt.Errorf("unsupported tool type %q", tool.Type)
		}
		params := tool.Function.Parameters
		if params == nil {
			params = map[string]interface{}{"type": "object", "properties": map[string]interface{}{}}
		}
		out.Tools = append(out.Tools, map[string]interface{}{
			"type":        "function",
			"name":        tool.Function.Name,
			"description": tool.Function.Description,
			"parameters":  params,
		})
	}
	return json.Marshal(out)
}

func contentToText(content interface{}) string {
	switch v := content.(type) {
	case string:
		return v
	case []interface{}:
		var parts []string
		for _, item := range v {
			m, ok := item.(map[string]interface{})
			if !ok || m["type"] != "text" {
				continue
			}
			if text, ok := m["text"].(string); ok && text != "" {
				parts = append(parts, text)
			}
		}
		return strings.Join(parts, "\n")
	default:
		return ""
	}
}

func TranslateResponsesEvent(data []byte, state *ResponseState) string {
	var event map[string]interface{}
	if err := json.Unmarshal(data, &event); err != nil {
		return ""
	}
	eventType, _ := event["type"].(string)
	switch eventType {
	case "response.output_text.delta":
		delta, _ := event["delta"].(string)
		return formatChunk(state, map[string]interface{}{"content": delta}, nil)
	case "response.completed":
		extractUsage(event, state)
		finish := "stop"
		state.FinishReasonSent = true
		return formatChunk(state, map[string]interface{}{}, &finish) + "data: [DONE]\n\n"
	default:
		return ""
	}
}

func extractUsage(event map[string]interface{}, state *ResponseState) {
	response, _ := event["response"].(map[string]interface{})
	usage, _ := response["usage"].(map[string]interface{})
	state.Usage.PromptTokens = intFromAny(usage["input_tokens"])
	state.Usage.CompletionTokens = intFromAny(usage["output_tokens"])
	state.Usage.TotalTokens = intFromAny(usage["total_tokens"])
}

func intFromAny(v interface{}) int {
	switch n := v.(type) {
	case float64:
		return int(n)
	case int:
		return n
	default:
		return 0
	}
}

func formatChunk(state *ResponseState, delta map[string]interface{}, finish *string) string {
	choice := map[string]interface{}{"index": 0, "delta": delta}
	if finish != nil {
		choice["finish_reason"] = *finish
	}
	chunk := map[string]interface{}{
		"id":      state.ID,
		"object":  "chat.completion.chunk",
		"created": state.Created,
		"model":   state.Model,
		"choices": []interface{}{choice},
	}
	data, _ := json.Marshal(chunk)
	return "data: " + string(data) + "\n\n"
}
```

- [ ] **Step 4: Run translator tests**

Run: `go test ./internal/adapter/xai -run TestTranslate`

Expected: PASS.

## Task 6: xAI Executor

**Files:**
- Create: `internal/adapter/xai/executor.go`
- Create: `internal/adapter/xai/executor_test.go`

- [ ] **Step 1: Write executor tests**

Create `internal/adapter/xai/executor_test.go` with an `httptest.Server` that checks request path `/responses`, Authorization header, and stream translation. Keep the test focused on `Execute` returning status 200 and a body containing OpenAI SSE chunks.

- [ ] **Step 2: Implement executor**

Create `internal/adapter/xai/executor.go`.

```go
package xai

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/dungnt/dntproxy/internal/adapter/shared"
	"github.com/dungnt/dntproxy/internal/domain"
	"github.com/dungnt/dntproxy/internal/port"
)

type Executor struct{}

func NewExecutor() *Executor { return &Executor{} }

func (e *Executor) Execute(model string, body []byte, credentials *domain.Credentials, reqlog port.RequestLogger) (io.ReadCloser, int, error) {
	translatedBody, err := TranslateChatToResponses(model, body)
	if err != nil {
		return nil, http.StatusBadRequest, err
	}
	baseURL := strings.TrimRight(credentials.BaseURL, "/")
	if baseURL == "" {
		baseURL = "https://api.x.ai/v1"
	}
	url := baseURL + "/responses"
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(translatedBody))
	if err != nil {
		return nil, http.StatusInternalServerError, fmt.Errorf("create xai request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	if credentials.AccessToken != "" {
		req.Header.Set("Authorization", "Bearer "+credentials.AccessToken)
	}

	reqlog.SetBodies(shared.PrepareLoggedBody(translatedBody), "")
	start := time.Now()
	resp, err := shared.StreamingHTTPClient.Do(req)
	duration := time.Since(start)
	if err != nil {
		reqlog.Upstream(url, http.MethodPost, http.StatusBadGateway, duration, err)
		return nil, http.StatusBadGateway, fmt.Errorf("xai request failed: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		bodyBytes, errRead := io.ReadAll(io.LimitReader(resp.Body, 1*1024*1024))
		resp.Body.Close()
		respBody := "Unknown error"
		if errRead == nil {
			respBody = string(bodyBytes)
		}
		reqlog.SetBodies("", shared.PrepareLoggedBody(bodyBytes))
		errUpstream := fmt.Errorf("%s", respBody)
		reqlog.Upstream(url, http.MethodPost, resp.StatusCode, duration, errUpstream)
		return nil, resp.StatusCode, fmt.Errorf("xai returned %d: %s", resp.StatusCode, respBody)
	}
	reqlog.Upstream(url, http.MethodPost, resp.StatusCode, duration, nil)

	pr, pw := io.Pipe()
	go func() {
		defer pw.Close()
		defer resp.Body.Close()
		state := NewResponseState(model)
		scanner := bufio.NewScanner(resp.Body)
		scanner.Buffer(make([]byte, 0, 64*1024), 50*1024*1024)
		for scanner.Scan() {
			line := scanner.Text()
			if !strings.HasPrefix(line, "data:") {
				continue
			}
			data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
			if data == "" || data == "[DONE]" {
				continue
			}
			translated := TranslateResponsesEvent([]byte(data), state)
			if translated != "" {
				_, _ = pw.Write([]byte(translated))
			}
		}
		if err := scanner.Err(); err != nil {
			_ = pw.CloseWithError(err)
			return
		}
		if !state.FinishReasonSent {
			finish := "stop"
			_, _ = pw.Write([]byte(formatChunk(state, map[string]interface{}{}, &finish)))
			_, _ = pw.Write([]byte("data: [DONE]\n\n"))
		}
		if state.Usage.PromptTokens > 0 || state.Usage.CompletionTokens > 0 {
			reqlog.SetUsage(state.Usage.PromptTokens, state.Usage.CompletionTokens, "xai_usage")
		}
	}()

	return pr, http.StatusOK, nil
}
```

- [ ] **Step 3: Run xAI adapter tests**

Run: `go test ./internal/adapter/xai`

Expected: PASS.

## Task 7: Register xAI Provider and Auth Routes

**Files:**
- Modify: `cmd/dntproxy/main.go`
- Modify: `internal/adapter/http/auth-handler.go`

- [ ] **Step 1: Register executor**

Modify imports in `cmd/dntproxy/main.go`:

```go
	"github.com/dungnt/dntproxy/internal/adapter/xai"
```

Add provider registration near other providers:

```go
	providers.RegisterExecutor("xai", xai.NewExecutor())
```

- [ ] **Step 2: Ensure auth routes compile**

Run: `go test ./internal/adapter/http`

Expected: PASS.

## Task 8: Verification and Stability Check

**Files:**
- All touched files.

- [ ] **Step 1: Format Go files**

Run: `gofmt -w internal/adapter/auth/xai.go internal/adapter/auth/xai_test.go internal/adapter/http/auth-handler.go internal/adapter/http/auth-xai-handler.go internal/adapter/xai/translator.go internal/adapter/xai/translator_test.go internal/adapter/xai/executor.go internal/adapter/xai/executor_test.go internal/domain/provider-config.go internal/domain/model-definition.go internal/service/model-resolver.go internal/service/model-parser_test.go cmd/dntproxy/main.go`

Expected: command exits 0.

- [ ] **Step 2: Run focused tests**

Run: `go test ./internal/adapter/auth ./internal/adapter/xai ./internal/service ./internal/adapter/http`

Expected: PASS.

- [ ] **Step 3: Run full test suite**

Run: `go test ./...`

Expected: PASS.

- [ ] **Step 4: Build binary**

Run: `go build -o /tmp/dntproxy ./cmd/dntproxy/`

Expected: PASS and `/tmp/dntproxy` exists.

- [ ] **Step 5: Manual sanity checklist**

Verify from code and tests:

- `POST /api/auth/xai/start` returns `sessionId`, `authUrl`, and `state`.
- `POST /api/auth/xai/exchange` stores a provider connection with `Provider == "xai"` and `AuthType == "oauth"`.
- `grok/grok-4.3` resolves to provider `xai` and model `grok-4.3`.
- `grok/grok-4.3@connectionId` preserves pinned account behavior.
- xAI executor sends `Authorization: Bearer <token>` and calls `/responses`.

## Self-Review

- Spec coverage: OAuth flow, provider registration, token refresh, executor translation, routing/models, testing, and deferred stability work are covered.
- Placeholder scan: No `TBD` or ambiguous implementation steps remain; the only conditional instruction is to use the existing PKCE helper name if it differs, which is necessary because the exact helper should be verified during implementation.
- Type consistency: xAI auth helper names are consistent across handler and tests; executor uses existing `port.ProviderExecutor` signature.
