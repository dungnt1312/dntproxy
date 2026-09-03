package domain

import "testing"

func TestMCPSettingsNormalize(t *testing.T) {
	m := MCPSettings{}
	m.Normalize()
	if m.VideoMaxFrames != 8 {
		t.Fatalf("VideoMaxFrames = %d, want default 8", m.VideoMaxFrames)
	}

	m = MCPSettings{VideoMaxFrames: 99, DefaultModel: "glm/glm-4.5v"}
	m.Normalize()
	if m.VideoMaxFrames != 16 {
		t.Fatalf("VideoMaxFrames = %d, want clamp 16", m.VideoMaxFrames)
	}

	m = MCPSettings{VideoMaxFrames: 3}
	m.Normalize()
	if m.VideoMaxFrames != 3 || m.DefaultModel != "" {
		t.Fatalf("valid values must be kept: %+v", m)
	}
}
