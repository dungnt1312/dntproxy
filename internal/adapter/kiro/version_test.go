package kiro

import (
	"context"
	"strings"
	"testing"

	"github.com/dungnt/dntproxy/internal/domain"
)

// The CodeWhisperer backend rejects "kiro-ide/x.y.z" with
// UnsupportedClientVersionException; it accepts the IDE's own
// "KiroIDE-<version>-<machineId>" token.
func TestSDKUserAgentUsesKiroIDEToken(t *testing.T) {
	ua := SDKUserAgent()
	if !strings.Contains(ua, "KiroIDE-"+IDEVersion+"-") {
		t.Fatalf("SDKUserAgent = %q, want KiroIDE-%s-<machineId>", ua, IDEVersion)
	}
	if strings.Contains(ua, "kiro-ide/") {
		t.Fatalf("SDKUserAgent = %q must not use the rejected kiro-ide/ form", ua)
	}
	if MachineID() == "" {
		t.Fatal("machine id is empty")
	}
}

func TestSDKUserAgentVersionOverride(t *testing.T) {
	t.Setenv("KIRO_IDE_VERSION", "9.9.9")
	if ua := SDKUserAgent(); !strings.Contains(ua, "KiroIDE-9.9.9-") {
		t.Fatalf("override ignored: %q", ua)
	}
}

func TestKiroRequestUsesSDKUserAgent(t *testing.T) {
	creds := &domain.Credentials{AccessToken: "t", ProviderSpecificData: map[string]interface{}{"authMethod": "idc"}}
	req, err := buildKiroRequest(context.Background(), kiroCodeWhispererHost+generateAssistantResponsePath, []byte("{}"), creds, 1)
	if err != nil {
		t.Fatal(err)
	}
	for _, h := range []string{"User-Agent", "X-Amz-User-Agent"} {
		if got := req.Header.Get(h); got != SDKUserAgent() {
			t.Errorf("%s = %q, want %q", h, got, SDKUserAgent())
		}
	}
}
