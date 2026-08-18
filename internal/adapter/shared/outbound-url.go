package shared

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/dungnt/dntproxy/internal/domain"
)

const maxOutboundRedirects = 3

// AllowPrivateOutbound reports whether the connection owner may target
// loopback/RFC1918 hosts (self-host Ollama). Tenant-owned connections cannot.
func AllowPrivateOutbound(tenantID string) bool {
	return domain.IsLegacyTenant(tenantID)
}

// ValidateOutboundURL checks scheme/host and, unless allowPrivate, rejects
// loopback, private, link-local, and metadata addresses (including DNS that
// resolves to any of those).
func ValidateOutboundURL(raw string, allowPrivate bool) error {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return errors.New("URL is empty")
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Hostname() == "" {
		return errors.New("invalid URL")
	}
	if parsed.Scheme != "https" && parsed.Scheme != "http" {
		return errors.New("URL must use HTTP or HTTPS")
	}
	if parsed.User != nil {
		return errors.New("URL must not contain credentials")
	}
	if allowPrivate {
		return nil
	}
	return rejectNonPublicHost(parsed.Hostname())
}

func rejectNonPublicHost(host string) error {
	host = strings.TrimSuffix(host, ".")
	if strings.EqualFold(host, "localhost") {
		return errors.New("URL host is not public")
	}
	if ip := net.ParseIP(host); ip != nil {
		if !isPublicImageIP(ip) {
			return errors.New("URL host is not public")
		}
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	ips, err := net.DefaultResolver.LookupIP(ctx, "ip", host)
	if err != nil || len(ips) == 0 {
		return errors.New("resolve URL host failed")
	}
	for _, ip := range ips {
		if !isPublicImageIP(ip) {
			return errors.New("URL host resolves to a non-public address")
		}
	}
	return nil
}

// CheckRedirectSafe follows at most 3 hops. Cross-host hops to private
// addresses are rejected unless the original request was already private
// (admin talking to a LAN service that redirects on the same network).
func CheckRedirectSafe(req *http.Request, via []*http.Request) error {
	if len(via) >= maxOutboundRedirects {
		return errors.New("too many redirects")
	}
	if req == nil || req.URL == nil {
		return errors.New("invalid redirect")
	}
	allowPrivate := false
	if len(via) > 0 && via[0].URL != nil {
		allowPrivate = hostLooksPrivate(via[0].URL.Hostname())
	}
	return ValidateOutboundURL(req.URL.String(), allowPrivate)
}

func hostLooksPrivate(host string) bool {
	host = strings.TrimSuffix(strings.TrimSpace(host), ".")
	if host == "" || strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return false
	}
	return !isPublicImageIP(ip)
}

// NewSafeHTTPClient returns a client that will not follow redirects onto
// non-public hosts.
func NewSafeHTTPClient(timeout time.Duration) *http.Client {
	return &http.Client{
		Timeout:       timeout,
		CheckRedirect: CheckRedirectSafe,
	}
}
