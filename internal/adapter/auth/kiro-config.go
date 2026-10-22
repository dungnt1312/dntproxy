package auth

import (
	"bytes"
	"crypto/sha1" //nolint:gosec // mirrors Kiro IDE's clientIdHash derivation
	"encoding/hex"
	"encoding/json"
	"net"
	"net/url"
	"os"
	"strings"
	"time"
)

// Constants below mirror the Kiro IDE auth stack (kiro-shared, v1.0.437).
const (
	// DefaultKiroIDEVersion is reported in the auth service User-Agent
	// (KiroIDE-<version>-<machineId>). Override with KIRO_IDE_VERSION.
	DefaultKiroIDEVersion = "1.0.437"

	BuilderIDStartURL   = "https://view.awsapps.com/start"
	InternalSSOStartURL = "https://amzn.awsapps.com/start"
	DefaultIdCRegion    = "us-east-1"

	idcClientName          = "Kiro IDE"
	idcRedirectURI         = "http://127.0.0.1/oauth/callback"
	idcCallbackPath        = "/oauth/callback"
	externalIdPRedirectURI = "kiro://kiro.oauth/callback"

	defaultAuthEndpoint = "https://prod.us-east-1.auth.desktop.kiro.dev"
	defaultPortalURL    = "https://app.kiro.dev"

	// loginFlowTimeout is how long one browser stage may wait for its callback.
	loginFlowTimeout = 10 * time.Minute
	// clientRegInvalidationOffset treats an OIDC client registration as expired
	// this long before its real expiry.
	clientRegInvalidationOffset = 15 * time.Minute
)

// IdCScopes are the grant scopes Kiro registers (scopePrefix "codewhisperer").
var IdCScopes = []string{
	"codewhisperer:completions",
	"codewhisperer:analysis",
	"codewhisperer:conversations",
	"codewhisperer:transformations",
	"codewhisperer:taskassist",
}

// PortalCallbackPorts is the loopback port list Kiro's PortalAuthServer tries
// in order. A var so tests can use an ephemeral port.
var PortalCallbackPorts = []int{3128, 4649, 6588, 8008, 9091, 49153, 50153, 51153, 52153, 53153}

// LoginFlowOverride returns KIRO_LOGIN_FLOW ("auth-code", "device-code" or
// empty for automatic selection).
func LoginFlowOverride() string { return os.Getenv("KIRO_LOGIN_FLOW") }

// IsLoopbackAddr reports whether an address ("host:port" or bare host) is a
// loopback IP.
func IsLoopbackAddr(addr string) bool {
	host := addr
	if h, _, err := net.SplitHostPort(addr); err == nil {
		host = h
	}
	ip := net.ParseIP(strings.Trim(host, "[]"))
	return ip != nil && ip.IsLoopback()
}

// authServiceEndpoint returns the Kiro auth service base URL
// (kiroAuthConfig.endpoint). Override with KIRO_AUTH_ENDPOINT.
func authServiceEndpoint() string {
	if v := strings.TrimSpace(os.Getenv("KIRO_AUTH_ENDPOINT")); v != "" {
		return strings.TrimRight(v, "/")
	}
	return defaultAuthEndpoint
}

// PortalURL returns the unified auth portal base URL. Override with
// KIRO_AUTH_PORTAL_URL (same variable the IDE honours).
func PortalURL() string {
	if v := strings.TrimSpace(os.Getenv("KIRO_AUTH_PORTAL_URL")); v != "" {
		return strings.TrimRight(v, "/")
	}
	return defaultPortalURL
}

// AuthSuccessPageURL is where the loopback server sends the browser on success.
func AuthSuccessPageURL() string {
	return PortalURL() + "/signin?auth_status=success&redirect_from=KiroIDE"
}

// AuthErrorPageURL is where the loopback server sends the browser on failure.
func AuthErrorPageURL(message string) string {
	enc := strings.ReplaceAll(url.QueryEscape(message), "+", "%20")
	return PortalURL() + "/signin?auth_status=error&redirect_from=KiroIDE&error_message=" + enc
}

// kiroUserAgent builds KiroIDE-<version>-<machineId>.
func kiroUserAgent() string {
	version := strings.TrimSpace(os.Getenv("KIRO_IDE_VERSION"))
	if version == "" {
		version = DefaultKiroIDEVersion
	}
	return "KiroIDE-" + version + "-" + kiroMachineID()
}

// ClientIDHash reproduces Kiro's sha1(JSON.stringify({startUrl})) cache key.
func ClientIDHash(startURL string) string {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false) // JSON.stringify does not escape <, >, &
	_ = enc.Encode(struct {
		StartURL string `json:"startUrl"`
	}{startURL})
	sum := sha1.Sum(bytes.TrimRight(buf.Bytes(), "\n")) //nolint:gosec
	return hex.EncodeToString(sum[:])
}
