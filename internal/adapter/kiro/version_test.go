package kiro

import (
	"context"
	"strings"
	"testing"

	"github.com/dungnt/dntproxy/internal/domain"
)

func TestSDKUserAgentCarriesIDEVersion(t *testing.T) {
	if !strings.HasSuffix(SDKUserAgent, "kiro-ide/"+IDEVersion) {
		t.Fatalf("SDKUserAgent = %q, want suffix kiro-ide/%s", SDKUserAgent, IDEVersion)
	}
}

func TestKiroRequestUsesSDKUserAgent(t *testing.T) {
	creds := &domain.Credentials{AccessToken: "t", ProviderSpecificData: map[string]interface{}{"authMethod": "idc"}}
	req, err := buildKiroRequest(context.Background(), kiroCodeWhispererHost+generateAssistantResponsePath, []byte("{}"), creds, 1)
	if err != nil {
		t.Fatal(err)
	}
	for _, h := range []string{"User-Agent", "X-Amz-User-Agent"} {
		if got := req.Header.Get(h); got != SDKUserAgent {
			t.Errorf("%s = %q, want %q", h, got, SDKUserAgent)
		}
	}
}
