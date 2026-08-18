package shared

import (
	"net/http"
	"net/url"
	"testing"
)

func TestValidateOutboundURL_PrivateRejectedForTenants(t *testing.T) {
	cases := []string{
		"http://127.0.0.1:11434",
		"http://10.0.0.8/v1",
		"http://192.168.1.1",
		"http://169.254.169.254/latest/meta-data",
		"http://localhost:8080",
		"file:///etc/passwd",
		"http://user:pass@example.com",
	}
	for _, raw := range cases {
		if err := ValidateOutboundURL(raw, false); err == nil {
			t.Errorf("ValidateOutboundURL(%q, tenant) = nil, want error", raw)
		}
	}
}

func TestValidateOutboundURL_PrivateAllowedForAdmin(t *testing.T) {
	if err := ValidateOutboundURL("http://127.0.0.1:11434", true); err != nil {
		t.Fatalf("admin loopback rejected: %v", err)
	}
	if err := ValidateOutboundURL("http://10.0.0.8/v1", true); err != nil {
		t.Fatalf("admin RFC1918 rejected: %v", err)
	}
}

func TestValidateOutboundURL_PublicIPAllowed(t *testing.T) {
	if err := ValidateOutboundURL("https://1.1.1.1/v1", false); err != nil {
		t.Fatalf("public IP rejected: %v", err)
	}
}

func TestAllowPrivateOutbound(t *testing.T) {
	if !AllowPrivateOutbound("") || !AllowPrivateOutbound("global") {
		t.Fatal("legacy/global should allow private URLs")
	}
	if AllowPrivateOutbound("acme") {
		t.Fatal("tenant must not allow private URLs")
	}
}

func TestCheckRedirectSafe_RejectsPrivateHop(t *testing.T) {
	orig, _ := http.NewRequest(http.MethodGet, "https://1.1.1.1/v1", nil)
	next, _ := http.NewRequest(http.MethodGet, "http://169.254.169.254/", nil)
	if err := CheckRedirectSafe(next, []*http.Request{orig}); err == nil {
		t.Fatal("redirect to metadata IP should fail")
	}
}

func TestCheckRedirectSafe_AllowsPrivateToPrivate(t *testing.T) {
	orig, _ := http.NewRequest(http.MethodGet, "http://127.0.0.1:11434/v1", nil)
	next, _ := http.NewRequest(http.MethodGet, "http://127.0.0.1:11434/v2", nil)
	if err := CheckRedirectSafe(next, []*http.Request{orig}); err != nil {
		t.Fatalf("same-host LAN redirect rejected: %v", err)
	}
}

func TestCheckRedirectSafe_CapsHops(t *testing.T) {
	u, _ := url.Parse("https://1.1.1.1/v1")
	req := &http.Request{URL: u}
	via := []*http.Request{req, req, req}
	if err := CheckRedirectSafe(req, via); err == nil {
		t.Fatal("expected too many redirects")
	}
}
