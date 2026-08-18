package auth

import "testing"

func TestNormalizeAWSRegion(t *testing.T) {
	got, err := NormalizeAWSRegion("")
	if err != nil || got != "us-east-1" {
		t.Fatalf("empty = %q, %v", got, err)
	}
	got, err = NormalizeAWSRegion("US-WEST-2")
	if err != nil || got != "us-west-2" {
		t.Fatalf("us-west-2 = %q, %v", got, err)
	}
	for _, bad := range []string{"evil.com/steal", "us-east-1.evil.com", "us-east-1@x", "us east", ".."} {
		if _, err := NormalizeAWSRegion(bad); err == nil {
			t.Errorf("NormalizeAWSRegion(%q) = nil, want error", bad)
		}
	}
}

func TestRefreshTokenSSORejectsHostInjection(t *testing.T) {
	_, err := RefreshTokenSSO("rt", "cid", "sec", "evil.com/steal")
	if err == nil {
		t.Fatal("injected region should fail before HTTP")
	}
}
