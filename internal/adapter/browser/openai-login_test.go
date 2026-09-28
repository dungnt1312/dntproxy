package browser

import "testing"

func TestPhoneTextsMatchOpenAIScreens(t *testing.T) {
	// Exact strings from the "Phone number required" gate screen.
	texts := []string{
		"Phone number required",
		"Add your phone number to continue. We'll send a one-time code to verify it.",
		"Phone number is required.",
		"Verify your phone to continue",
	}
	for _, s := range texts {
		if !phoneTexts.MatchString(s) {
			t.Errorf("phoneTexts should match %q", s)
		}
	}
}

func TestPhoneTextsNoFalsePositives(t *testing.T) {
	texts := []string{
		"Select a workspace Personal account Continue", // consent screen
		"Welcome to ChatGPT",                           // normal dashboard
		"Incorrect email address or password",          // handled by hardFailTexts
	}
	for _, s := range texts {
		if phoneTexts.MatchString(s) {
			t.Errorf("phoneTexts should NOT match %q", s)
		}
	}
}

func TestHardFailTexts(t *testing.T) {
	if !hardFailTexts.MatchString("Incorrect email address or password") {
		t.Error("hardFailTexts should match incorrect credentials")
	}
	if !hardFailTexts.MatchString("This account has been locked.") {
		t.Error("hardFailTexts should match locked accounts")
	}
}

func TestCaptchaTexts(t *testing.T) {
	if !captchaTexts.MatchString("Verify you're a human") {
		t.Error("captchaTexts should match challenge copy")
	}
}
