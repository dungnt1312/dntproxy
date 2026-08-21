package http

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/dungnt/dntproxy/internal/domain"
)

func TestFetchCommandCodeUsage(t *testing.T) {
	var mu sync.Mutex
	queries := map[string]string{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer user_test" {
			t.Errorf("Authorization = %q", got)
		}
		mu.Lock()
		queries[r.URL.Path] = r.URL.RawQuery
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/alpha/whoami":
			_, _ = w.Write([]byte(`{"org":{"id":"org_123"}}`))
		case "/alpha/billing/subscriptions":
			_, _ = w.Write([]byte(`{"success":true,"data":{"status":"active","planId":"individual-go","currentPeriodStart":"2026-08-04T07:00:41Z","currentPeriodEnd":"2026-09-04T07:00:41Z"}}`))
		case "/alpha/billing/credits":
			_, _ = w.Write([]byte(`{"credits":{"belowThreshold":false,"monthlyCredits":3},"windowLimits":{"exceeded":false,"fiveHour":{"used":1,"cap":3,"exceeded":false,"resetAt":1787313905620},"weekly":{"used":6,"cap":6,"exceeded":true,"resetAt":1787892128076}}}`))
		case "/alpha/usage/summary":
			_, _ = w.Write([]byte(`{"totalCount":2451,"totalTokens":259127459,"totalCredits":7}`))
		default:
			t.Errorf("unexpected path %q", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	result, err := fetchCommandCodeUsage(&domain.ProviderConnection{APIKey: "user_test", BaseURL: server.URL})
	if err != nil {
		t.Fatalf("fetchCommandCodeUsage() error = %v", err)
	}
	if result.Provider != "commandcode" || result.Plan != "individual-go" || !result.LimitReached {
		t.Fatalf("result = %+v", result)
	}
	if result.Extras["totalCount"] != 2451 || result.Extras["totalTokens"] != 259127459 || result.Extras["currentPeriodEnd"] != "2026-09-04T07:00:41Z" {
		t.Fatalf("extras = %#v", result.Extras)
	}
	if len(result.Quotas) != 3 {
		t.Fatalf("quota count = %d, want 3", len(result.Quotas))
	}
	if got := result.Quotas[0]; got.Key != "monthly-credits" || got.Used != 7000 || got.Total != 10000 || got.Remaining != 3000 || got.Unit != "credits" || got.Scale != 1000 {
		t.Fatalf("monthly credits bucket = %+v", got)
	}
	if got := result.Quotas[1]; got.Key != "five-hour" || got.Used != 1000 || got.Total != 3000 || got.ResetAt != "2026-08-21T12:05:05Z" || got.Unit != "credits" || got.Scale != 1000 {
		t.Fatalf("five-hour bucket = %+v", got)
	}
	if got := result.Quotas[2]; got.Key != "weekly" || got.Used != 6000 || got.Total != 6000 || got.Unit != "credits" || got.Scale != 1000 {
		t.Fatalf("weekly bucket = %+v", got)
	}
	if queries["/alpha/billing/subscriptions"] != "orgId=org_123" || queries["/alpha/billing/credits"] != "orgId=org_123" {
		t.Fatalf("org queries = %#v", queries)
	}
	if got := queries["/alpha/usage/summary"]; !strings.Contains(got, "orgId=org_123") || !strings.Contains(got, "since=2026-08-04T07%3A00%3A41Z") {
		t.Fatalf("summary query = %q", got)
	}
}

func TestFetchCommandCodeUsageSupportsPersonalAccount(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("x-cli-environment"); got != "production" {
			t.Errorf("x-cli-environment = %q", got)
		}
		if r.URL.Query().Has("orgId") {
			t.Errorf("unexpected orgId query: %q", r.URL.RawQuery)
		}
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/alpha/whoami":
			_, _ = w.Write([]byte(`{"success":true,"org":null}`))
		case "/alpha/billing/subscriptions":
			_, _ = w.Write([]byte(`{"success":true,"data":{"status":"active","planId":"individual-go"}}`))
		case "/alpha/billing/credits":
			_, _ = w.Write([]byte(`{"credits":{"monthlyCredits":1},"windowLimits":{}}`))
		case "/alpha/usage/summary":
			_, _ = w.Write([]byte(`{}`))
		default:
			t.Errorf("unexpected path %q", r.URL.Path)
		}
	}))
	defer server.Close()

	result, err := fetchCommandCodeUsage(&domain.ProviderConnection{APIKey: "user_test", BaseURL: server.URL})
	if err != nil {
		t.Fatalf("fetchCommandCodeUsage() error = %v", err)
	}
	if result.Plan != "individual-go" || len(result.Quotas) != 1 || result.Quotas[0].Key != "monthly-credits" {
		t.Fatalf("result = %+v", result)
	}
}

func TestFetchCommandCodeUsageOmitsEmptySince(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/alpha/whoami":
			_, _ = w.Write([]byte(`{"org":{"id":"org_123"}}`))
		case "/alpha/billing/subscriptions":
			_, _ = w.Write([]byte(`{"success":true,"data":{"status":"active","planId":"individual-go"}}`))
		case "/alpha/billing/credits":
			_, _ = w.Write([]byte(`{"credits":{"monthlyCredits":1},"windowLimits":{}}`))
		case "/alpha/usage/summary":
			if r.URL.Query().Get("since") != "" {
				t.Fatalf("since = %q, want empty", r.URL.Query().Get("since"))
			}
			_, _ = w.Write([]byte(`{}`))
		}
	}))
	defer server.Close()

	if _, err := fetchCommandCodeUsage(&domain.ProviderConnection{APIKey: "user_test", BaseURL: server.URL}); err != nil {
		t.Fatalf("fetchCommandCodeUsage() error = %v", err)
	}
}

func TestFetchCommandCodeUsageErrors(t *testing.T) {
	t.Run("missing key", func(t *testing.T) {
		result, err := fetchCommandCodeUsage(&domain.ProviderConnection{})
		if err != nil || !strings.Contains(result.Message, "No API key") {
			t.Fatalf("result=%+v err=%v", result, err)
		}
	})

	t.Run("upstream failure", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"error":"bad key"}`))
		}))
		defer server.Close()
		_, err := fetchCommandCodeUsage(&domain.ProviderConnection{APIKey: "user_test", BaseURL: server.URL})
		if err == nil || !strings.Contains(err.Error(), "HTTP 401") || !strings.Contains(err.Error(), "bad key") {
			t.Fatalf("err = %v", err)
		}
	})

	t.Run("invalid json", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(`not json`))
		}))
		defer server.Close()
		_, err := fetchCommandCodeUsage(&domain.ProviderConnection{APIKey: "user_test", BaseURL: server.URL})
		if err == nil || !strings.Contains(err.Error(), "parse Command Code /alpha/whoami") {
			t.Fatalf("err = %v", err)
		}
	})
}
