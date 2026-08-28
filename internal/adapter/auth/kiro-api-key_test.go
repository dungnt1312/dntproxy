package auth

import "testing"

func TestParseKiroModelList(t *testing.T) {
	body := []byte(`{"models":[
		{"modelId":"claude-sonnet-5"},
		{"modelId":"claude-opus-5"},
		{"modelId":"claude-sonnet-5"},
		{"id":"glm-5"},
		{"modelName":"qwen3-coder-next"},
		{}
	]}`)

	got := parseKiroModelList(body)
	want := []string{"claude-sonnet-5", "claude-opus-5", "glm-5", "qwen3-coder-next"}

	if len(got) != len(want) {
		t.Fatalf("models = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("models = %v, want %v", got, want)
		}
	}
}

func TestParseKiroModelListRejectsGarbage(t *testing.T) {
	if got := parseKiroModelList([]byte(`not json`)); got != nil {
		t.Fatalf("models = %v, want nil", got)
	}
	if got := parseKiroModelList([]byte(`{"models":[]}`)); len(got) != 0 {
		t.Fatalf("models = %v, want empty", got)
	}
}

func TestValidateKiroAPIKeyRejectsEmptyAndBadRegion(t *testing.T) {
	if _, err := ValidateKiroAPIKey("   ", "us-east-1"); err == nil {
		t.Fatal("expected error for empty apiKey")
	}
	// A malformed region must be rejected locally rather than interpolated into
	// a hostname, which would let a caller point the probe at any host.
	if _, err := ValidateKiroAPIKey("ksk_test", "evil.example.com/"); err == nil {
		t.Fatal("expected error for invalid region")
	}
}
