package shared

import "testing"

func TestPrepareLoggedBodyIgnoresRawBodyEnvWhenSettingDisabled(t *testing.T) {
	t.Setenv("DNTPROXY_LOG_RAW_BODIES", "true")
	SetLogBodiesEnabled(false)

	got := PrepareLoggedBody([]byte(`{"message":"secret"}`))
	if got != "" {
		t.Fatalf("PrepareLoggedBody() = %q, want empty body when setting is disabled", got)
	}
}
