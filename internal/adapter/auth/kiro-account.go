package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/dungnt/dntproxy/internal/adapter/kiro"
	"github.com/dungnt/dntproxy/internal/domain"
	"github.com/dungnt/dntproxy/internal/port"
)

// Kiro access tokens (IdC, Builder ID, social) are opaque, not JWTs, so the
// account email cannot be read from them. The IDE gets it from GetUsageLimits
// with isEmailRequired=true; without that flag userInfo.email comes back null.

// FetchKiroAccountEmail returns the account email for a Kiro connection, or ""
// when Kiro does not report one. It only reads and never refreshes the token.
func FetchKiroAccountEmail(ctx context.Context, conn *domain.ProviderConnection) (string, error) {
	bearer := kiro.ConnectionBearer(conn)
	if bearer == "" {
		return "", fmt.Errorf("connection has no credential")
	}
	region := kiro.DefaultRegion
	if kiro.ConnectionUsesAPIKey(conn) {
		region = kiro.NormalizeRegion(connString(conn, "region"))
	}

	q := url.Values{}
	q.Set("origin", modelsOrigin)
	q.Set("resourceType", "AGENTIC_REQUEST")
	q.Set("isEmailRequired", "true")
	if arn := connString(conn, "profileArn"); arn != "" {
		q.Set("profileArn", arn)
	}
	headers := map[string]string{
		"Authorization": "Bearer " + bearer,
		"Accept":        "application/json",
		"User-Agent":    kiroUserAgent(),
	}
	if tt := modelsTokenType(conn); tt != "" {
		headers["TokenType"] = tt
	}

	res, err := doRequest(ctx, "GET", qEndpoint(region)+"/getUsageLimits?"+q.Encode(), nil, "", headers,
		reqOpts{client: profilesHTTP, maxAttempts: 2, retryTimeouts: true, retryStatus: retryableOIDCStatus})
	if err != nil {
		return "", err
	}
	if res.Status != 200 {
		return "", fmt.Errorf("getUsageLimits: HTTP %d", res.Status)
	}
	var body struct {
		UserInfo *struct {
			Email *string `json:"email"`
		} `json:"userInfo"`
	}
	if err := json.Unmarshal(res.Body, &body); err != nil {
		return "", fmt.Errorf("getUsageLimits: malformed response")
	}
	if body.UserInfo == nil || body.UserInfo.Email == nil {
		return "", nil
	}
	email := strings.TrimSpace(*body.UserInfo.Email)
	if !strings.Contains(email, "@") {
		return "", nil
	}
	return email, nil
}

// defaultKiroName matches names dntproxy generated itself, which may be
// replaced by the account email. Anything else was chosen by the operator.
var defaultKiroName = regexp.MustCompile(
	`^(?:(?:AWS Builder ID|AWS IAM Identity Center(?: \(Internal\))?|Google|GitHub|Imported|Kiro) Account \d+` +
		`|Kiro API Key \d+` +
		`|Kiro (?:Builder ID|IdC|Internal|Google|GitHub|Imported|API Key)(?: \([^)]*\))?(?: \d+)?)$`)

// IsDefaultKiroName reports whether name is a dntproxy-generated placeholder.
func IsDefaultKiroName(name string) bool {
	return name == "" || defaultKiroName.MatchString(strings.TrimSpace(name))
}

// kiroNameBase is the fallback label when no email is known, e.g.
// "Kiro Builder ID" or "Kiro IdC (d-9167184e8a)".
func kiroNameBase(conn *domain.ProviderConnection) string {
	if kiro.ConnectionUsesAPIKey(conn) {
		return "Kiro API Key"
	}
	switch connString(conn, "authMethod") {
	case "builder-id":
		return "Kiro Builder ID"
	case "idc":
		if connString(conn, "kiroProvider") == ProviderInternal {
			return "Kiro Internal"
		}
		if org := startURLOrg(connString(conn, "startUrl")); org != "" {
			return "Kiro IdC (" + org + ")"
		}
		return "Kiro IdC"
	case "google":
		return "Kiro Google"
	case "github":
		return "Kiro GitHub"
	}
	return "Kiro Imported"
}

// startURLOrg extracts the organisation label from an IdC start URL
// ("https://acme.awsapps.com/start" -> "acme").
func startURLOrg(startURL string) string {
	u, err := url.Parse(strings.TrimSpace(startURL))
	if err != nil {
		return ""
	}
	host := u.Hostname()
	if host == "" || host == "view.awsapps.com" || host == "amzn.awsapps.com" {
		return ""
	}
	return strings.SplitN(host, ".", 2)[0]
}

// ResolveKiroEmail fills conn.Email for a freshly created connection: from the
// token when it is a JWT, otherwise from GetUsageLimits. Best effort — a
// failure leaves Email empty and the caller falls back to a readable name.
func ResolveKiroEmail(ctx context.Context, conn *domain.ProviderConnection) {
	if conn.Email != "" {
		return
	}
	if email := ExtractEmailFromJWT(conn.AccessToken); strings.Contains(email, "@") {
		conn.Email = email
		return
	}
	if email, err := FetchKiroAccountEmail(ctx, conn); err == nil && email != "" {
		conn.Email = email
	}
}

// BackfillKiroAccountEmails looks up the email of every active Kiro connection
// that has none, stores it, and replaces the name only when it is still a
// dntproxy-generated placeholder. Returns how many connections were updated.
func BackfillKiroAccountEmails(ctx context.Context, store port.CredentialStore) int {
	cfg, err := store.Load()
	if err != nil || cfg == nil {
		return 0
	}
	updated := 0
	for i := range cfg.ProviderConnections {
		conn := cfg.ProviderConnections[i]
		if conn.Provider != "kiro" || !conn.IsActive || conn.Email != "" {
			continue
		}
		if domain.IsAccountUnavailable(conn.RateLimitedUntil) {
			continue
		}
		email, err := FetchKiroAccountEmail(ctx, &conn)
		if err != nil || email == "" {
			continue
		}
		changed := false
		_ = store.Update(func(cfg *domain.AppConfig) {
			for j := range cfg.ProviderConnections {
				c := &cfg.ProviderConnections[j]
				if c.ID != conn.ID || c.Email != "" {
					continue
				}
				c.Email = email
				if IsDefaultKiroName(c.Name) {
					c.Name = email
				}
				changed = true
				return
			}
		})
		if changed {
			updated++
		}
		if ctx.Err() != nil {
			break
		}
	}
	return updated
}

// KiroConnectionName picks the display name for a new Kiro connection: the
// account email when known, otherwise a readable label that is unique among
// existing connection names ("Kiro Builder ID", "Kiro Builder ID 2", ...).
func KiroConnectionName(conn *domain.ProviderConnection, existing []domain.ProviderConnection) string {
	if conn.Email != "" {
		return conn.Email
	}
	base := kiroNameBase(conn)
	taken := make(map[string]bool, len(existing))
	for _, c := range existing {
		taken[c.Name] = true
	}
	if !taken[base] {
		return base
	}
	for n := 2; ; n++ {
		if name := base + " " + strconv.Itoa(n); !taken[name] {
			return name
		}
	}
}
