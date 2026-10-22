package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/dungnt/dntproxy/internal/adapter/kiro"
	"github.com/dungnt/dntproxy/internal/adapter/shared"
	"github.com/dungnt/dntproxy/internal/domain"
)

// Kiro model catalog (ListAvailableModels), ported from the IDE's
// ModelConfigProvider fetch:
//
//	GET https://management.<region>.kiro.dev/List-Available-Models
//	    ?origin=AI_EDITOR&profileArn=<arn>&nextToken=<t>
//	Authorization: Bearer <token>   TokenType: SSO_OIDC (IdC) | API_KEY | EXTERNAL_IDP
//
// The IDE follows nextToken for at most 10 pages, waits 15s per request and
// retries 3 times. The Amazon Q surface (q.<region>.amazonaws.com) serves the
// same operation and is what dntproxy already used to validate API keys, so it
// is the fallback (and the primary for API keys, which the management surface
// does not reliably accept).

const (
	modelsOrigin   = "AI_EDITOR"
	maxModelPages  = 10 // B7o in the IDE
	modelsTimeout  = 15 * time.Second
	modelsAttempts = 3
)

var modelsHTTP = shared.NewSafeHTTPClient(modelsTimeout)

// managementRegions are the regions that have a management.<region>.kiro.dev
// host. The IdC region of a token (e.g. us-west-1) is unrelated to the Kiro
// service region, which is why anything else falls back to us-east-1.
var managementRegions = map[string]bool{
	"us-east-1": true, "eu-central-1": true, "us-gov-east-1": true, "us-gov-west-1": true,
}

// managementEndpoint builds the Kiro control-plane host. A var so tests can
// point it at a local server.
var managementEndpoint = func(region string) string {
	return "https://management." + region + ".kiro.dev"
}

// OverrideModelEndpoints redirects both catalog surfaces (and the profile
// lookup that shares the Q host) to base, returning a restore func. It exists so
// tests in other packages never reach the real service; production code never
// calls it.
func OverrideModelEndpoints(base string) (restore func()) {
	oldM, oldQ := managementEndpoint, qEndpoint
	managementEndpoint = func(string) string { return base + "/management" }
	qEndpoint = func(string) string { return base + "/q" }
	return func() { managementEndpoint, qEndpoint = oldM, oldQ }
}

// KiroModelInfo is one entry of the catalog.
type KiroModelInfo struct {
	ID              string  `json:"id"`
	Name            string  `json:"name,omitempty"`
	Description     string  `json:"description,omitempty"`
	Provider        string  `json:"provider,omitempty"`
	RateMultiplier  float64 `json:"rateMultiplier,omitempty"`
	RateUnit        string  `json:"rateUnit,omitempty"`
	MaxInputTokens  int     `json:"maxInputTokens,omitempty"`
	MaxOutputTokens int     `json:"maxOutputTokens,omitempty"`
}

// KiroModelCatalog is the account's live model list.
type KiroModelCatalog struct {
	Models       []KiroModelInfo
	DefaultModel string
	Surface      string // "management" | "q"
}

// IDs returns the model ids in server order.
func (c *KiroModelCatalog) IDs() []string {
	if c == nil {
		return nil
	}
	ids := make([]string, 0, len(c.Models))
	for _, m := range c.Models {
		ids = append(ids, m.ID)
	}
	return ids
}

// ModelFetchError is a failed catalog request. Status is the upstream HTTP
// status, or 0 for transport failures.
type ModelFetchError struct {
	Surface string
	Status  int
	Msg     string
	Err     error
}

func (e *ModelFetchError) Error() string {
	var b strings.Builder
	b.WriteString("kiro " + e.Surface + " ListAvailableModels")
	if e.Status != 0 {
		fmt.Fprintf(&b, " HTTP %d", e.Status)
	}
	if e.Msg != "" {
		b.WriteString(": " + e.Msg)
	}
	if e.Err != nil {
		b.WriteString(": " + e.Err.Error())
	}
	return b.String()
}

func (e *ModelFetchError) Unwrap() error { return e.Err }

// IsAuthRejected reports a 401/403, i.e. a token that should be refreshed.
func (e *ModelFetchError) IsAuthRejected() bool { return e.Status == 401 || e.Status == 403 }

// IsModelAuthRejected reports whether err is a credential rejection.
func IsModelAuthRejected(err error) bool {
	for err != nil {
		if fe, ok := err.(*ModelFetchError); ok {
			return fe.IsAuthRejected()
		}
		u, ok := err.(interface{ Unwrap() error })
		if !ok {
			return false
		}
		err = u.Unwrap()
	}
	return false
}

type modelSurface struct {
	name      string
	base      string // URL up to the query string
	tokenType string
	userAgent string
}

func connString(conn *domain.ProviderConnection, key string) string {
	if conn == nil {
		return ""
	}
	return getStringFromMap(conn.ProviderSpecificData, key)
}

// modelsTokenType is the TokenType header the IDE derives from the auth method
// (api_key → API_KEY, external_idp → EXTERNAL_IDP, IdC → SSO_OIDC, social → none).
func modelsTokenType(conn *domain.ProviderConnection) string {
	if kiro.ConnectionUsesAPIKey(conn) {
		return "API_KEY"
	}
	switch connString(conn, "authMethod") {
	case kiro.AuthMethodExternalIDP:
		return "EXTERNAL_IDP"
	case "builder-id", "idc":
		return "SSO_OIDC"
	}
	if connString(conn, "clientId") != "" && connString(conn, "clientSecret") != "" {
		return "SSO_OIDC" // imported IdC token
	}
	return ""
}

func modelSurfaces(conn *domain.ProviderConnection) []modelSurface {
	apiKey := kiro.ConnectionUsesAPIKey(conn)
	region := kiro.DefaultRegion
	if apiKey {
		region = kiro.NormalizeRegion(connString(conn, "region"))
	}
	mgmtRegion := kiro.DefaultRegion
	if managementRegions[region] {
		mgmtRegion = region
	}

	mgmt := modelSurface{
		name:      "management",
		base:      managementEndpoint(mgmtRegion) + "/List-Available-Models",
		tokenType: modelsTokenType(conn),
		userAgent: kiroUserAgent(),
	}
	// The Q surface is the one the rest of dntproxy already talks to with these
	// credentials: it gets only the API-key marker, exactly like ValidateKiroAPIKey
	// and the usage/quota calls.
	q := modelSurface{
		name:      "q",
		base:      qEndpoint(region) + "/ListAvailableModels",
		userAgent: "AWS-SDK-JS/3.0.0 kiro-ide/1.0.0",
	}
	switch {
	case apiKey:
		q.tokenType = "API_KEY"
		return []modelSurface{q, mgmt}
	case connString(conn, "authMethod") == kiro.AuthMethodExternalIDP:
		q.tokenType = "EXTERNAL_IDP"
	}
	return []modelSurface{mgmt, q}
}

type modelsPage struct {
	Models []struct {
		ModelID        string  `json:"modelId"`
		ModelName      string  `json:"modelName"`
		Description    string  `json:"description"`
		ModelProvider  string  `json:"modelProvider"`
		RateMultiplier float64 `json:"rateMultiplier"`
		RateUnit       string  `json:"rateUnit"`
		TokenLimits    *struct {
			MaxInputTokens  int `json:"maxInputTokens"`
			MaxOutputTokens int `json:"maxOutputTokens"`
		} `json:"tokenLimits"`
	} `json:"models"`
	DefaultModel *struct {
		ModelID string `json:"modelId"`
	} `json:"defaultModel"`
	NextToken string `json:"nextToken"`
}

func (s modelSurface) pageURL(profileArn, nextToken string) string {
	q := url.Values{}
	q.Set("origin", modelsOrigin)
	if profileArn != "" {
		q.Set("profileArn", profileArn)
	}
	if nextToken != "" {
		q.Set("nextToken", nextToken)
	}
	return s.base + "?" + q.Encode()
}

func fetchModelsFromSurface(ctx context.Context, s modelSurface, conn *domain.ProviderConnection) (*KiroModelCatalog, *ModelFetchError) {
	bearer := kiro.ConnectionBearer(conn)
	if bearer == "" {
		return nil, &ModelFetchError{Surface: s.name, Status: 401, Msg: "connection has no credential"}
	}
	profileArn := connString(conn, "profileArn")

	cat := &KiroModelCatalog{Surface: s.name}
	seen := map[string]bool{}
	usedTokens := map[string]bool{}
	next := ""

	for page := 0; page < maxModelPages; page++ {
		headers := map[string]string{
			"Authorization": "Bearer " + bearer,
			"Accept":        "application/json",
			"User-Agent":    s.userAgent,
		}
		if s.tokenType != "" {
			headers["TokenType"] = s.tokenType
		}
		res, err := doRequest(ctx, "GET", s.pageURL(profileArn, next), nil, "", headers,
			reqOpts{client: modelsHTTP, maxAttempts: modelsAttempts, retryTimeouts: true, retryStatus: retryableOIDCStatus})
		if err != nil {
			return nil, &ModelFetchError{Surface: s.name, Msg: "request failed", Err: err}
		}
		if res.Status != 200 {
			return nil, &ModelFetchError{Surface: s.name, Status: res.Status, Msg: truncate(strings.TrimSpace(string(res.Body)), 200)}
		}

		var p modelsPage
		if err := json.Unmarshal(res.Body, &p); err != nil {
			return nil, &ModelFetchError{Surface: s.name, Status: 200, Msg: "malformed response", Err: err}
		}
		for _, m := range p.Models {
			id := strings.TrimSpace(m.ModelID)
			if id == "" || seen[id] { // the IDE drops entries without a modelId
				continue
			}
			seen[id] = true
			info := KiroModelInfo{
				ID: id, Name: m.ModelName, Description: m.Description, Provider: m.ModelProvider,
				RateMultiplier: m.RateMultiplier, RateUnit: m.RateUnit,
			}
			if m.TokenLimits != nil {
				info.MaxInputTokens, info.MaxOutputTokens = m.TokenLimits.MaxInputTokens, m.TokenLimits.MaxOutputTokens
			}
			cat.Models = append(cat.Models, info)
		}
		if cat.DefaultModel == "" && p.DefaultModel != nil {
			cat.DefaultModel = strings.TrimSpace(p.DefaultModel.ModelID)
		}
		// Stop at the last page, or if the server hands back a token twice.
		if p.NextToken == "" || usedTokens[p.NextToken] {
			break
		}
		usedTokens[p.NextToken] = true
		next = p.NextToken
	}
	return cat, nil
}

// FetchKiroModels returns the account's live model catalog. Surfaces are tried
// in order; the first one that answers with models wins. An empty answer from
// every surface is returned as an empty catalog, not an error.
func FetchKiroModels(ctx context.Context, conn *domain.ProviderConnection) (*KiroModelCatalog, error) {
	if conn == nil {
		return nil, fmt.Errorf("connection is nil")
	}
	var (
		errs  []*ModelFetchError
		empty *KiroModelCatalog
	)
	for _, s := range modelSurfaces(conn) {
		cat, err := fetchModelsFromSurface(ctx, s, conn)
		if err == nil {
			if len(cat.Models) > 0 {
				return cat, nil
			}
			if empty == nil {
				empty = cat
			}
			continue
		}
		errs = append(errs, err)
		if ctx.Err() != nil {
			break
		}
	}
	if empty != nil {
		return empty, nil
	}
	return nil, pickFetchError(errs)
}

// FetchKiroModelsWithRefresh fetches the catalog and, on a credential
// rejection, refreshes the token once and retries — the same recovery chat
// requests get after an upstream 401. conn is updated in place on refresh.
// It only reads: nothing is written to SupportedModels.
func FetchKiroModelsWithRefresh(ctx context.Context, svc *TokenRefreshService, conn *domain.ProviderConnection) (*KiroModelCatalog, error) {
	cat, err := FetchKiroModels(ctx, conn)
	if err == nil || !IsModelAuthRejected(err) || svc == nil || conn.AuthType != "oauth" || strings.TrimSpace(conn.RefreshToken) == "" {
		return cat, err
	}
	fresh, refErr := svc.ForceRefresh(conn)
	if refErr != nil {
		return nil, &ModelFetchError{Surface: "refresh", Status: 401, Msg: "token expired and refresh failed", Err: refErr}
	}
	*conn = *fresh
	return FetchKiroModels(ctx, conn)
}

// pickFetchError prefers a credential rejection (actionable) over other
// failures, otherwise the primary surface's error.
func pickFetchError(errs []*ModelFetchError) error {
	for _, e := range errs {
		if e.IsAuthRejected() {
			return e
		}
	}
	if len(errs) > 0 {
		return errs[0]
	}
	return fmt.Errorf("no model surface available")
}
