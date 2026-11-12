package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/dungnt/dntproxy/internal/domain"
)

// profileServer fakes q.<region>.amazonaws.com/ListAvailableProfiles.
func withProfiles(t *testing.T, arns ...string) *atomic.Int32 {
	t.Helper()
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/ListAvailableProfiles") {
			http.NotFound(w, r)
			return
		}
		calls.Add(1)
		// Only us-east-1 answers, so the merged list is exactly arns.
		if !strings.HasPrefix(r.URL.Path, "/us-east-1/") {
			_ = json.NewEncoder(w).Encode(map[string]any{"profiles": []any{}})
			return
		}
		var ps []map[string]string
		for i, a := range arns {
			ps = append(ps, map[string]string{"arn": a, "profileName": "P" + string(rune('A'+i))})
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"profiles": ps})
	}))
	oldQ := qEndpoint
	qEndpoint = func(region string) string { return srv.URL + "/" + region }
	t.Cleanup(func() { qEndpoint = oldQ; srv.Close() })
	return &calls
}

func TestResolveConnectionProfileArnSingleProfile(t *testing.T) {
	withProfiles(t, "arn:aws:codewhisperer:us-east-1:1:profile/ONE")
	conn := kiroConn("p1", "", nil)
	changed, err := ResolveConnectionProfileArn(context.Background(), &conn)
	if err != nil || !changed || conn.ProviderSpecificData["profileArn"] != "arn:aws:codewhisperer:us-east-1:1:profile/ONE" {
		t.Fatalf("changed=%v err=%v pd=%v", changed, err, conn.ProviderSpecificData)
	}
}

func TestResolveConnectionProfileArnLeavesAmbiguousEmpty(t *testing.T) {
	withProfiles(t, "arn:a", "arn:b")
	conn := kiroConn("p2", "", nil)
	if changed, err := ResolveConnectionProfileArn(context.Background(), &conn); changed || err == nil {
		t.Fatalf("two profiles must not be guessed: changed=%v err=%v", changed, err)
	}
	if _, ok := conn.ProviderSpecificData["profileArn"]; ok {
		t.Fatal("profileArn set despite ambiguity")
	}
}

func TestResolveConnectionProfileArnSkipsWhenNotNeeded(t *testing.T) {
	calls := withProfiles(t, "arn:x")
	has := kiroConn("p3", "", nil)
	has.ProviderSpecificData["profileArn"] = "arn:existing"
	bid := kiroConn("p4", "", nil)
	bid.ProviderSpecificData["authMethod"] = "builder-id"
	social := kiroConn("p5", "", nil)
	social.ProviderSpecificData = map[string]interface{}{"authMethod": "google"}
	for _, conn := range []*domain.ProviderConnection{&has, &bid, &social} {
		if changed, err := ResolveConnectionProfileArn(context.Background(), conn); changed || err != nil {
			t.Fatalf("%s: changed=%v err=%v", conn.ID, changed, err)
		}
	}
	if calls.Load() != 0 {
		t.Fatalf("no lookup expected, got %d", calls.Load())
	}
	if has.ProviderSpecificData["profileArn"] != "arn:existing" {
		t.Fatal("existing profileArn changed")
	}
}

func TestBackfillKiroProfileArns(t *testing.T) {
	withProfiles(t, "arn:aws:codewhisperer:us-east-1:1:profile/ONE")
	missing := kiroConn("b1", "", nil)
	has := kiroConn("b2", "", nil)
	has.ProviderSpecificData["profileArn"] = "arn:keep"
	inactive := kiroConn("b3", "", nil)
	inactive.IsActive = false
	store := newRefreshStore(t, missing, has, inactive)

	if n := BackfillKiroProfileArns(context.Background(), store); n != 1 {
		t.Fatalf("updated = %d, want 1", n)
	}
	got, _ := store.GetConnectionByID("b1")
	if got.ProviderSpecificData["profileArn"] != "arn:aws:codewhisperer:us-east-1:1:profile/ONE" {
		t.Fatalf("b1 pd = %v", got.ProviderSpecificData)
	}
	if got.ProviderSpecificData["clientId"] != "cid" {
		t.Fatal("backfill must not drop other provider data")
	}
	if k, _ := store.GetConnectionByID("b2"); k.ProviderSpecificData["profileArn"] != "arn:keep" {
		t.Fatal("existing profileArn overwritten")
	}
	if k, _ := store.GetConnectionByID("b3"); k.ProviderSpecificData["profileArn"] != nil {
		t.Fatal("inactive connection touched")
	}
	if n := BackfillKiroProfileArns(context.Background(), store); n != 0 {
		t.Fatalf("second run updated %d", n)
	}
}
