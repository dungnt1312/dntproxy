package shared

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestSanitizeBodyRedactsImageInputs(t *testing.T) {
	input := []byte(`{
		"prompt":"edit this",
		"image":"data:image/png;base64,secret",
		"images":[{"image_url":"data:image/png;base64,secret"}],
		"mask":"data:image/png;base64,secret",
		"subject_reference":[{"type":"character","image_file":"data:image/png;base64,secret"}]
	}`)
	got := string(SanitizeBody(input))
	if got == string(input) {
		t.Fatal("SanitizeBody() returned original image payload")
	}
	if strings.Contains(got, "data:image") || strings.Contains(got, "secret") {
		t.Fatalf("SanitizeBody() leaked image payload: %q", got)
	}
}

func TestPrepareLoggedBodyIgnoresRawBodyEnvWhenSettingDisabled(t *testing.T) {
	t.Setenv("DNTPROXY_LOG_RAW_BODIES", "true")
	SetLogBodiesEnabled(false)

	got := PrepareLoggedBody([]byte(`{"message":"secret"}`))
	if got != "" {
		t.Fatalf("PrepareLoggedBody() = %q, want empty body when setting is disabled", got)
	}
}

func TestIsCanceledOrClosedStream(t *testing.T) {
	if !IsCanceledOrClosedStream(errors.New("http2: response body closed")) {
		t.Fatal("should detect http2 response body closed")
	}
	if !IsCanceledOrClosedStream(context.Canceled) {
		t.Fatal("should detect context.Canceled")
	}
	if IsCanceledOrClosedStream(errors.New("returned 429: rate limit")) {
		t.Fatal("should not treat rate limit as abort")
	}
}

func TestIsResponseHeaderTimeout(t *testing.T) {
	if !IsResponseHeaderTimeout(errors.New(`Post "https://codewhisperer.us-east-1.amazonaws.com/generateAssistantResponse": http2: timeout awaiting response headers`)) {
		t.Fatal("should detect response-header timeout")
	}
	if IsResponseHeaderTimeout(errors.New("context deadline exceeded")) {
		t.Fatal("should not detect generic context deadline")
	}
	if IsResponseHeaderTimeout(errors.New("HTTP 403: forbidden")) {
		t.Fatal("should not detect status errors")
	}
	if IsResponseHeaderTimeout(nil) {
		t.Fatal("should not detect nil error")
	}
}
