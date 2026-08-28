package kiro

import (
	"net/http"
	"strings"
	"testing"

	"github.com/dungnt/dntproxy/internal/domain"
)

func apiKeyCreds(region string) *domain.Credentials {
	data := map[string]interface{}{"authMethod": AuthMethodAPIKey}
	if region != "" {
		data["region"] = region
	}
	return &domain.Credentials{
		Provider:             "kiro",
		APIKey:               "ksk_test_key",
		ProviderSpecificData: data,
	}
}

func TestOrderedEndpointsHoistsQForAPIKey(t *testing.T) {
	got := OrderedEndpoints(apiKeyCreds(""))
	if len(got) != 2 {
		t.Fatalf("expected 2 endpoints, got %d: %v", len(got), got)
	}
	// The legacy CodeWhisperer surface authenticates API keys but then rejects the
	// payload with a terminal 400, so it must never be tried before q.*.
	if !strings.Contains(got[0], "://q.") {
		t.Fatalf("expected q host first for api_key, got %v", got)
	}
	if !strings.Contains(got[1], "://codewhisperer.") {
		t.Fatalf("expected codewhisperer second for api_key, got %v", got)
	}
}

func TestOrderedEndpointsUnchangedForOAuth(t *testing.T) {
	for _, method := range []string{"builder-id", "idc", "social", "imported", ""} {
		creds := &domain.Credentials{
			Provider:             "kiro",
			AccessToken:          "token",
			ProviderSpecificData: map[string]interface{}{"authMethod": method},
		}
		got := OrderedEndpoints(creds)
		// OAuth accounts must keep hitting exactly the one host they always have,
		// so a 401 still means "refresh the token" instead of silently retrying.
		want := []string{kiroCodeWhispererHost + generateAssistantResponsePath}
		if len(got) != 1 || got[0] != want[0] {
			t.Fatalf("authMethod %q: endpoints = %v, want %v", method, got, want)
		}
	}
}

func TestOrderedEndpointsRegionalizesAmazonHostsOnly(t *testing.T) {
	got := OrderedEndpoints(apiKeyCreds("eu-west-1"))
	for _, url := range got {
		if !strings.Contains(url, "eu-west-1") {
			t.Fatalf("amazonaws host not regionalized: %s (all: %v)", url, got)
		}
	}
}

func TestOrderedEndpointsIgnoresInvalidRegion(t *testing.T) {
	got := OrderedEndpoints(apiKeyCreds("evil.example.com"))
	for _, url := range got {
		if strings.Contains(url, "evil.example.com") {
			t.Fatalf("invalid region leaked into endpoint: %s", url)
		}
	}
}

func TestShouldTryNextEndpoint(t *testing.T) {
	for _, status := range []int{401, 403, 404} {
		if !shouldTryNextEndpoint(status) {
			t.Fatalf("status %d should advance to the next endpoint", status)
		}
	}
	// 400 means the surface understood the credential but rejected the payload —
	// advancing is pointless for OAuth and required for api_key, but the decision
	// is made by the caller, not here.
	for _, status := range []int{200, 400, 422, 429, 500} {
		if shouldTryNextEndpoint(status) {
			t.Fatalf("status %d should not advance to the next endpoint", status)
		}
	}
}

func TestBuildKiroRequestAPIKeyHeaders(t *testing.T) {
	req, err := buildKiroRequest(nil, kiroQHost+generateAssistantResponsePath, []byte(`{}`), apiKeyCreds(""), 1)
	if err != nil {
		t.Fatalf("buildKiroRequest: %v", err)
	}
	if got := req.Header.Get("Authorization"); got != "Bearer ksk_test_key" {
		t.Fatalf("Authorization = %q", got)
	}
	if got := req.Header.Get("TokenType"); got != "API_KEY" {
		t.Fatalf("TokenType = %q, want API_KEY", got)
	}
	// X-Amz-Target on the q.* host breaks the call.
	if got := req.Header.Get("X-Amz-Target"); got != "" {
		t.Fatalf("X-Amz-Target must be absent on q host, got %q", got)
	}
}

func TestBuildKiroRequestSetsTargetOnCodeWhispererOnly(t *testing.T) {
	req, err := buildKiroRequest(nil, kiroCodeWhispererHost+generateAssistantResponsePath, []byte(`{}`), apiKeyCreds(""), 1)
	if err != nil {
		t.Fatalf("buildKiroRequest: %v", err)
	}
	if got := req.Header.Get("X-Amz-Target"); got == "" {
		t.Fatal("X-Amz-Target must be set on the codewhisperer host")
	}
}

func TestBuildKiroRequestOAuthOmitsTokenType(t *testing.T) {
	creds := &domain.Credentials{
		Provider:             "kiro",
		AccessToken:          "oauth-token",
		ProviderSpecificData: map[string]interface{}{"authMethod": "builder-id"},
	}
	req, err := buildKiroRequest(nil, kiroRuntimeHost+generateAssistantResponsePath, []byte(`{}`), creds, 1)
	if err != nil {
		t.Fatalf("buildKiroRequest: %v", err)
	}
	if got := req.Header.Get("Authorization"); got != "Bearer oauth-token" {
		t.Fatalf("Authorization = %q", got)
	}
	if got := req.Header.Get("TokenType"); got != "" {
		t.Fatalf("TokenType must be absent for builder-id, got %q", got)
	}
}

func TestBuildKiroRequestExternalIDPTokenType(t *testing.T) {
	creds := &domain.Credentials{
		Provider:             "kiro",
		AccessToken:          "entra-token",
		ProviderSpecificData: map[string]interface{}{"authMethod": AuthMethodExternalIDP},
	}
	req, err := buildKiroRequest(nil, kiroCodeWhispererHost+generateAssistantResponsePath, []byte(`{}`), creds, 1)
	if err != nil {
		t.Fatalf("buildKiroRequest: %v", err)
	}
	if got := req.Header.Get("TokenType"); got != "EXTERNAL_IDP" {
		t.Fatalf("TokenType = %q, want EXTERNAL_IDP", got)
	}
}

func TestApplyConnectionAuthAPIKey(t *testing.T) {
	conn := &domain.ProviderConnection{
		Provider:             "kiro",
		AuthType:             "apikey",
		APIKey:               "ksk_conn",
		ProviderSpecificData: map[string]interface{}{"authMethod": AuthMethodAPIKey},
	}
	header := http.Header{}
	ApplyConnectionAuth(header, conn)
	if got := header.Get("Authorization"); got != "Bearer ksk_conn" {
		t.Fatalf("Authorization = %q", got)
	}
	if got := header.Get("TokenType"); got != "API_KEY" {
		t.Fatalf("TokenType = %q, want API_KEY", got)
	}
}

func TestApplyConnectionAuthOAuth(t *testing.T) {
	conn := &domain.ProviderConnection{
		Provider:    "kiro",
		AuthType:    "oauth",
		AccessToken: "oauth-token",
	}
	header := http.Header{}
	ApplyConnectionAuth(header, conn)
	if got := header.Get("Authorization"); got != "Bearer oauth-token" {
		t.Fatalf("Authorization = %q", got)
	}
	if header.Get("TokenType") != "" {
		t.Fatal("TokenType must be absent for oauth connections")
	}
}

func TestBuildKiroPayloadOmitsProfileArnForAPIKey(t *testing.T) {
	// Sending a profileArn the key does not own makes CodeWhisperer answer
	// 403 "bearer token invalid", so api_key connections must send none.
	req := &OpenAIRequest{
		Model:    "claude-sonnet-5",
		Messages: []OpenAIMessage{{Role: "user", Content: []byte(`"hello"`)}},
	}
	payload, err := BuildKiroPayload(req, "claude-sonnet-5", apiKeyCreds(""))
	if err != nil {
		t.Fatalf("BuildKiroPayload: %v", err)
	}
	if payload.ProfileArn != "" {
		t.Fatalf("profileArn must be empty for api_key, got %q", payload.ProfileArn)
	}
}
