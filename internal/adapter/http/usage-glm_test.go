package http

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/dungnt/dntproxy/internal/domain"
)

// proQuotaBody mirrors a live GLM Pro coding-plan response: two TOKENS_LIMIT
// windows (5-hour and weekly) plus a TIME_LIMIT tool-call window.
const proQuotaBody = `{"code":200,"msg":"Operation successful","data":{"limits":[
{"type":"TOKENS_LIMIT","unit":3,"number":5,"percentage":3,"nextResetTime":1787917128885},
{"type":"TOKENS_LIMIT","unit":6,"number":1,"percentage":20,"nextResetTime":1788344835998},
{"type":"TIME_LIMIT","unit":5,"number":1,"usage":1000,"currentValue":40,"remaining":960,"percentage":4,"nextResetTime":1790332035996,
 "usageDetails":[{"modelCode":"search-prime","usage":25},{"modelCode":"web-reader","usage":15},{"modelCode":"zread","usage":0}]}
],"level":"pro"},"success":true}`

func TestFetchGLMUsage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != glmQuotaPath {
			t.Errorf("path = %q, want %q", r.URL.Path, glmQuotaPath)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer glm-key" {
			t.Errorf("Authorization = %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(proQuotaBody))
	}))
	defer server.Close()

	result, err := fetchGLMUsage(&domain.ProviderConnection{APIKey: "glm-key", BaseURL: server.URL})
	if err != nil {
		t.Fatalf("fetchGLMUsage() error = %v", err)
	}
	if result.Provider != "glm" || result.Plan != "Pro" || result.LimitReached {
		t.Fatalf("result = %+v", result)
	}
	if len(result.Quotas) != 3 {
		t.Fatalf("quota count = %d, want 3", len(result.Quotas))
	}

	// Percentage-only windows are scaled against a total of 100.
	if got := result.Quotas[0]; got.Key != "5-hour-tokens" || got.Label != "5-hour tokens" ||
		got.Used != 3 || got.Total != 100 || got.Remaining != 97 || got.Pct != 3 ||
		got.ResetAt != "2026-08-28T11:38:48Z" {
		t.Fatalf("5-hour bucket = %+v", got)
	}
	if got := result.Quotas[1]; got.Key != "week-tokens" || got.Label != "Weekly tokens" ||
		got.Used != 20 || got.Total != 100 || got.Pct != 20 {
		t.Fatalf("weekly bucket = %+v", got)
	}
	// TIME_LIMIT reports absolute counts, so used/total are real call counts.
	if got := result.Quotas[2]; got.Key != "month-tool-calls" || got.Label != "Monthly tool calls" ||
		got.Used != 40 || got.Total != 1000 || got.Remaining != 960 || got.Pct != 4 {
		t.Fatalf("tool-call bucket = %+v", got)
	}

	tools, ok := result.Extras["toolUsage"].(map[string]float64)
	if !ok || tools["search-prime"] != 25 || tools["web-reader"] != 15 {
		t.Fatalf("toolUsage extras = %#v", result.Extras["toolUsage"])
	}
}

// A Lite plan omits nextResetTime on the token window; the bucket must still be
// emitted, just without a reset timestamp.
func TestFetchGLMUsageLitePlanWithoutResetTime(t *testing.T) {
	const body = `{"code":200,"data":{"limits":[
{"type":"TOKENS_LIMIT","unit":3,"number":5,"percentage":0}
],"level":"lite"},"success":true}`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(body))
	}))
	defer server.Close()

	result, err := fetchGLMUsage(&domain.ProviderConnection{APIKey: "k", BaseURL: server.URL})
	if err != nil {
		t.Fatalf("fetchGLMUsage() error = %v", err)
	}
	if result.Plan != "Lite" || len(result.Quotas) != 1 {
		t.Fatalf("result = %+v", result)
	}
	if got := result.Quotas[0]; got.Key != "5-hour-tokens" || got.ResetAt != "" || got.Total != 100 {
		t.Fatalf("bucket = %+v", got)
	}
}

func TestFetchGLMUsageMarksLimitReached(t *testing.T) {
	const body = `{"code":200,"data":{"limits":[
{"type":"TOKENS_LIMIT","unit":3,"number":5,"percentage":100,"nextResetTime":1787917128885}
],"level":"pro"},"success":true}`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(body))
	}))
	defer server.Close()

	result, err := fetchGLMUsage(&domain.ProviderConnection{APIKey: "k", BaseURL: server.URL})
	if err != nil {
		t.Fatalf("fetchGLMUsage() error = %v", err)
	}
	if !result.LimitReached {
		t.Fatalf("LimitReached = false, want true for a fully used window")
	}
}

func TestFetchGLMUsageNoAPIKey(t *testing.T) {
	result, err := fetchGLMUsage(&domain.ProviderConnection{})
	if err != nil {
		t.Fatalf("fetchGLMUsage() error = %v", err)
	}
	if result.Message == "" || len(result.Quotas) != 0 {
		t.Fatalf("result = %+v", result)
	}
}

// An invalid key must surface as an error so the handler can attempt a refresh
// and the UI can show a retry affordance, rather than a silent empty panel.
func TestFetchGLMUsageUnauthorized(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()

	if _, err := fetchGLMUsage(&domain.ProviderConnection{APIKey: "bad", BaseURL: server.URL}); err == nil {
		t.Fatal("fetchGLMUsage() error = nil, want error for HTTP 401")
	}
}

// Other upstream failures degrade to a message: chat can still work when only
// the monitor endpoint is unhappy.
func TestFetchGLMUsageServerErrorDegrades(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	result, err := fetchGLMUsage(&domain.ProviderConnection{APIKey: "k", BaseURL: server.URL})
	if err != nil {
		t.Fatalf("fetchGLMUsage() error = %v", err)
	}
	if result.Message == "" || len(result.Quotas) != 0 {
		t.Fatalf("result = %+v", result)
	}
}

func TestGLMQuotaBaseURL(t *testing.T) {
	tests := []struct{ in, want string }{
		{"", "https://api.z.ai"},
		{"  ", "https://api.z.ai"},
		{"https://api.z.ai", "https://api.z.ai"},
		{"https://api.z.ai/", "https://api.z.ai"},
		{"https://api.z.ai/api/coding/paas/v4/chat/completions", "https://api.z.ai"},
		{"https://api.z.ai/api/coding/paas/v4", "https://api.z.ai"},
		{"https://api.z.ai/api/anthropic/v1/messages", "https://api.z.ai"},
		{"https://open.bigmodel.cn/api/paas/v4", "https://open.bigmodel.cn"},
		{"https://open.bigmodel.cn/v1", "https://open.bigmodel.cn"},
	}
	for _, tt := range tests {
		if got := glmQuotaBaseURL(tt.in); got != tt.want {
			t.Errorf("glmQuotaBaseURL(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

// Unknown window units must not collide into one key, which is the bug in the
// upstream implementation this was modelled on.
func TestGLMWindowNameKeysAreDistinct(t *testing.T) {
	limits := []glmQuotaLimit{
		{Type: "TOKENS_LIMIT", Unit: 3, Number: 5},
		{Type: "TOKENS_LIMIT", Unit: 6, Number: 1},
		{Type: "TOKENS_LIMIT", Unit: 99, Number: 7},
		{Type: "TIME_LIMIT", Unit: 5, Number: 1},
	}
	seen := map[string]bool{}
	for i, limit := range limits {
		key, label := glmWindowName(limit, i)
		if key == "" || label == "" {
			t.Fatalf("limit %d produced empty key/label", i)
		}
		if seen[key] {
			t.Fatalf("duplicate key %q at index %d", key, i)
		}
		seen[key] = true
	}
}
