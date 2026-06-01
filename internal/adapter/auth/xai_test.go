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
		CodeChallenge:         "challenge",
		State:                 "state-123",
		Nonce:                 "nonce-456",
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
		CodeChallenge:         "challenge",
		State:                 "state",
		Nonce:                 "nonce",
	})
	if err == nil || !strings.Contains(err.Error(), "not on x.ai") {
		t.Fatalf("error = %v, want x.ai validation error", err)
	}
}
