package telegram

import (
	"fmt"
	"strings"
	"time"

	"github.com/dungnt/dntproxy/internal/port"
)

// FormatAlert formats an alert for Telegram MarkdownV2.
func FormatAlert(alert port.Alert) string {
	var icon, title string

	switch alert.Type {
	case port.AlertQuotaExhausted:
		icon, title = "🚫", "Quota Exhausted"
	case port.AlertTokenExpired:
		icon, title = "🔑", "Token Expired"
	case port.AlertConnectionDown:
		icon, title = "⚠️", "Connection Down"
	case port.AlertAllDown:
		icon, title = "🔴", "ALL CONNECTIONS DOWN"
	case port.AlertRateLimited:
		icon, title = "⏳", "Rate Limited"
	case port.AlertComboExhausted:
		icon, title = "💥", "Combo Exhausted"
	case port.AlertConnectionRecovered:
		icon, title = "✅", "Connection Recovered"
	default:
		icon, title = "ℹ️", "Alert"
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("%s *%s*\n", icon, escapeMarkdown(title)))

	if alert.Connection != "" {
		sb.WriteString(fmt.Sprintf("Connection: `%s`\n", escapeMarkdown(alert.Connection)))
	}
	if alert.Provider != "" {
		sb.WriteString(fmt.Sprintf("Provider: `%s`\n", escapeMarkdown(alert.Provider)))
	}
	if alert.Model != "" {
		sb.WriteString(fmt.Sprintf("Model: `%s`\n", escapeMarkdown(alert.Model)))
	}
	if alert.Combo != "" {
		sb.WriteString(fmt.Sprintf("Combo: `%s`\n", escapeMarkdown(alert.Combo)))
	}
	if alert.Message != "" {
		sb.WriteString(fmt.Sprintf("\n%s", escapeMarkdown(alert.Message)))
	}

	sb.WriteString(fmt.Sprintf("\n_%s_", escapeMarkdown(time.Now().Format("15:04:05"))))
	return sb.String()
}

// FormatStatus formats connection status list for Telegram.
func FormatStatus(connections []ConnectionStatus) string {
	if len(connections) == 0 {
		return "No connections configured\\."
	}

	var sb strings.Builder
	sb.WriteString("*Connection Status*\n\n")

	for _, c := range connections {
		var icon string
		switch c.Status {
		case "ok":
			icon = "🟢"
		case "rate_limited":
			icon = "🟡"
		case "error":
			icon = "🔴"
		case "expired":
			icon = "⚫"
		default:
			icon = "⚪"
		}

		sb.WriteString(fmt.Sprintf("%s `%s` \\(%s\\)\n", icon, escapeMarkdown(c.Name), escapeMarkdown(c.Provider)))
		if c.Error != "" {
			sb.WriteString(fmt.Sprintf("   └ %s\n", escapeMarkdown(c.Error)))
		}
	}
	return sb.String()
}

// FormatUsage formats usage stats for Telegram.
func FormatUsage(stats UsageStats) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("*Usage \\- %s*\n\n", escapeMarkdown(stats.Period)))
	sb.WriteString(fmt.Sprintf("Requests: `%d`\n", stats.Requests))
	sb.WriteString(fmt.Sprintf("Tokens in: `%d`\n", stats.TokensIn))
	sb.WriteString(fmt.Sprintf("Tokens out: `%d`\n", stats.TokensOut))
	sb.WriteString(fmt.Sprintf("Cost: `$%.4f`\n", stats.Cost))
	return sb.String()
}

// FormatConnections formats detailed connection info.
func FormatConnections(connections []ConnectionDetail) string {
	if len(connections) == 0 {
		return "No connections configured\\."
	}

	var sb strings.Builder
	sb.WriteString("*Connections Detail*\n\n")

	for _, c := range connections {
		sb.WriteString(fmt.Sprintf("*%s* \\(%s\\)\n", escapeMarkdown(c.Name), escapeMarkdown(c.Provider)))
		sb.WriteString(fmt.Sprintf("  Models: `%d`\n", c.ModelCount))
		if c.BackoffLevel > 0 {
			sb.WriteString(fmt.Sprintf("  Backoff: `%d/7`\n", c.BackoffLevel))
		}
		if c.CooldownRemaining != "" {
			sb.WriteString(fmt.Sprintf("  Cooldown: `%s`\n", escapeMarkdown(c.CooldownRemaining)))
		}
		if c.LastError != "" {
			sb.WriteString(fmt.Sprintf("  Error: %s\n", escapeMarkdown(c.LastError)))
		}
		sb.WriteString("\n")
	}
	return sb.String()
}

// FormatHelp returns the help message.
func FormatHelp() string {
	return `*Available Commands*

/status \- Connection health overview
/usage \- Today's usage stats
/usage 7d \- Last 7 days usage
/connections \- Detailed connection info
/mute 2h \- Suppress alerts \(e\.g\. 1h, 30m, 2h\)
/unmute \- Resume alerts
/help \- Show this message`
}

// FormatMuted formats the mute confirmation.
func FormatMuted(until time.Time) string {
	return fmt.Sprintf("🔇 Alerts muted until `%s`", escapeMarkdown(until.Format("15:04:05")))
}

// FormatUnmuted formats the unmute confirmation.
func FormatUnmuted() string {
	return "🔔 Alerts resumed"
}

// ConnectionStatus holds status info for /status command.
type ConnectionStatus struct {
	Name     string
	Provider string
	Status   string // ok, rate_limited, error, expired
	Error    string
}

// UsageStats holds usage data for /usage command.
type UsageStats struct {
	Period    string
	Requests int
	TokensIn int64
	TokensOut int64
	Cost     float64
}

// ConnectionDetail holds detailed connection info for /connections command.
type ConnectionDetail struct {
	Name              string
	Provider          string
	ModelCount        int
	BackoffLevel      int
	CooldownRemaining string
	LastError         string
}

// escapeMarkdown escapes special characters for Telegram MarkdownV2.
func escapeMarkdown(s string) string {
	replacer := strings.NewReplacer(
		"_", "\\_",
		"*", "\\*",
		"[", "\\[",
		"]", "\\]",
		"(", "\\(",
		")", "\\)",
		"~", "\\~",
		"`", "\\`",
		">", "\\>",
		"#", "\\#",
		"+", "\\+",
		"-", "\\-",
		"=", "\\=",
		"|", "\\|",
		"{", "\\{",
		"}", "\\}",
		".", "\\.",
		"!", "\\!",
	)
	return replacer.Replace(s)
}
