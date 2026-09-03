package auth

import (
	"testing"
	"time"
)

// RFC 4226 Appendix D test vectors (secret = "12345678901234567890").
func TestHOTPSHA1RFC4226Vectors(t *testing.T) {
	key := []byte("12345678901234567890")
	want := []string{
		"755224", "287082", "359152", "969429", "338314",
		"254676", "287922", "162583", "399871", "520489",
	}
	for counter, expected := range want {
		got := hotpSHA1(key, uint64(counter))
		if got != expected {
			t.Errorf("counter %d: got %s, want %s", counter, got, expected)
		}
	}
}

func TestGenerateTOTPMatchesHOTPWindow(t *testing.T) {
	// secret decodes to the RFC 4226 key "12345678901234567890"
	secret := "GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ"
	// 59s / 30 = counter 1 -> 287082
	code, err := GenerateTOTP(secret, time.Unix(59, 0))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if code != "287082" {
		t.Errorf("got %s, want 287082", code)
	}
}

func TestNormalizeTOTPSecret(t *testing.T) {
	cases := map[string]string{
		"jbswy3dpehpk3pxp":         "JBSWY3DPEHPK3PXP",
		"JBSW Y3DP-EHPK 3PXP":      "JBSWY3DPEHPK3PXP",
		"JBSWY3DP":                 "JBSWY3DP",
		"jbswy3dpehpk3pxpi=======": "JBSWY3DPEHPK3PXPI=======",
		"":                         "",
		"   ":                      "",
	}
	for in, want := range cases {
		if got := NormalizeTOTPSecret(in); got != want {
			t.Errorf("NormalizeTOTPSecret(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestGenerateTOTPErrors(t *testing.T) {
	if _, err := GenerateTOTP("", time.Now()); err == nil {
		t.Error("expected error for empty secret")
	}
	if _, err := GenerateTOTP("!!!not-base32!!!", time.Now()); err == nil {
		t.Error("expected error for invalid base32")
	}
}

func TestGenerateTOTPSixDigits(t *testing.T) {
	code, err := GenerateTOTP("JBSWY3DPEHPK3PXP", time.Now())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(code) != 6 {
		t.Errorf("got %s, want 6 digits", code)
	}
}
