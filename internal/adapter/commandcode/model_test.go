package commandcode

import "testing"

func TestMapModel(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"deepseek-v4-pro", "deepseek/deepseek-v4-pro"},
		{"deepseek-v4", "deepseek/deepseek-v4-pro"},
		{"minimax-m3", "MiniMaxAI/MiniMax-M3"},
		{"MiniMaxAI/MiniMax-M3", "MiniMaxAI/MiniMax-M3"},
		{"qwen3.7-max", "Qwen/Qwen3.7-Max"},
		{"mimo-pro", "xiaomi/mimo-v2.5-pro"},
		{"already/custom", "already/custom"},
	}
	for _, tt := range tests {
		if got := MapModel(tt.in); got != tt.want {
			t.Errorf("MapModel(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
