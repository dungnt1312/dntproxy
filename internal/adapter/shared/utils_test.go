package shared

import (
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
	for _, secret := range []string{"data:image", "image_file"} {
		if strings.Contains(got, secret) {
			t.Fatalf("SanitizeBody() = %q, contains %q", got, secret)
		}
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
