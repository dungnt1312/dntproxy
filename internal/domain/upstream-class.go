package domain

import "strings"

type UpstreamClass string

const (
	UpstreamNonFallback      UpstreamClass = "non_fallback"
	UpstreamRetryable        UpstreamClass = "retryable"
	UpstreamQuota            UpstreamClass = "quota"
	UpstreamAuth             UpstreamClass = "auth"
	UpstreamTransient        UpstreamClass = "transient"
	UpstreamModelEntitlement UpstreamClass = "model_entitlement"
)

func ClassifyUpstream(status int, errorText string) UpstreamClass {
	lower := strings.ToLower(errorText)
	if IsNonFallbackStatus(status) {
		return UpstreamNonFallback
	}
	// client-error body hints (mirror chat-service-error-routing)
	for _, hint := range []string{"invalid request", "improperly formed request", "malformed", "invalid json", "missing required", "unsupported parameter", "tool schema"} {
		if strings.Contains(lower, hint) {
			return UpstreamNonFallback
		}
	}
	if status == 403 && isModelEntitlementError(lower) {
		return UpstreamModelEntitlement
	}
	for _, kw := range []string{"rate limit", "too many requests", "quota exceeded", "capacity", "overloaded"} {
		if strings.Contains(lower, kw) {
			return UpstreamQuota
		}
	}
	switch status {
	case 401:
		return UpstreamAuth
	case 429:
		return UpstreamQuota
	case 402, 403:
		return UpstreamQuota
	case 502, 503, 504:
		// Service-unavailable style: temporary upstream outages.
		return UpstreamTransient
	case 404:
		// Keep legacy CheckFallbackError behavior: still allow account/model failover.
		return UpstreamRetryable
	case 408, 500:
		// Verbatim taxonomy tests treat these as retryable (failover-eligible).
		return UpstreamRetryable
	default:
		if status >= 500 {
			return UpstreamTransient
		}
		// Unknown non-4xx-client statuses remain retryable for multi-account HA.
		return UpstreamRetryable
	}
}

func IsRetryableUpstream(status int, errorText string) bool {
	switch ClassifyUpstream(status, errorText) {
	case UpstreamNonFallback:
		return false
	default:
		return true
	}
}

func IsQuotaExceeded(status int, errorText string) bool {
	return ClassifyUpstream(status, errorText) == UpstreamQuota ||
		ClassifyUpstream(status, errorText) == UpstreamModelEntitlement
}
