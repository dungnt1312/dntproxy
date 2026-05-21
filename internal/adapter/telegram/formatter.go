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
func FormatStatus(connections []ConnectionStatus, enabled bool, muted bool, mutedUntil time.Time, suppressionWindow time.Duration) string {
	if len(connections) == 0 {
		return "No connections configured\\."
	}

	var sb strings.Builder
	sb.WriteString("*Connection Status*\n\n")
	if enabled {
		sb.WriteString("Bot: `enabled`\n")
	} else {
		sb.WriteString("Bot: `disabled` \\(alerts paused\\)\n")
	}
	if muted {
		sb.WriteString(fmt.Sprintf("Mute: `active until %s`\n", escapeMarkdown(mutedUntil.Local().Format("2006-01-02 15:04:05"))))
	} else {
		sb.WriteString("Mute: `off`\n")
	}
	sb.WriteString(fmt.Sprintf("Repeat alerts: `suppressed for %s per issue`\n\n", escapeMarkdown(suppressionWindow.String())))

	for _, c := range connections {
		var icon string
		switch c.Status {
		case "ok":
			icon = "🟢"
		case "rate_limited":
			icon = "🟡"
		case "refreshable":
			icon = "🟠"
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
	if stats.Errors > 0 {
		sb.WriteString(fmt.Sprintf("Errors: `%d`\n", stats.Errors))
	}
	sb.WriteString(fmt.Sprintf("Tokens in: `%d`\n", stats.TokensIn))
	sb.WriteString(fmt.Sprintf("Tokens out: `%d`\n", stats.TokensOut))
	sb.WriteString(fmt.Sprintf("Tokens total: `%d`\n", stats.TokensTotal))
	sb.WriteString(fmt.Sprintf("Cost: `$%.4f`\n", stats.Cost))

	if len(stats.Quotas) == 0 {
		sb.WriteString("\nQuota: `not checked or unsupported`\n")
		return sb.String()
	}

	sb.WriteString("\n*Quota Check*\n")
	for _, q := range stats.Quotas {
		sb.WriteString(fmt.Sprintf("\n`%s` \\(%s\\)", escapeMarkdown(q.Name), escapeMarkdown(q.Provider)))
		if q.Plan != "" {
			sb.WriteString(fmt.Sprintf(" \\- %s", escapeMarkdown(q.Plan)))
		}
		sb.WriteString("\n")
		if q.LimitReached {
			sb.WriteString("  Limit: `reached`\n")
		}
		if q.Message != "" {
			sb.WriteString(fmt.Sprintf("  Note: %s\n", escapeMarkdown(q.Message)))
		}
		for _, bucket := range q.Buckets {
			sb.WriteString(fmt.Sprintf("  %s: `%d/%d used` \\(%d%%\\), `%d left`", escapeMarkdown(bucket.Label), bucket.Used, bucket.Total, bucket.Pct, bucket.Remaining))
			if bucket.ResetAt != "" {
				sb.WriteString(fmt.Sprintf(" reset `%s`", escapeMarkdown(formatResetAt(bucket.ResetAt))))
			}
			sb.WriteString("\n")
		}
	}
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
/usage \- Today's usage stats \+ quota check
/usage 7d \- Last 7 days usage \+ quota check
/connections \- Detailed connection info
/mute \- Show current mute status
/mute 2h \- Suppress alert notifications \(e\.g\. 1h, 30m, 2h\)
/unmute \- Resume alerts
/help \- Show this message`
}

// FormatMuted formats the mute confirmation.
func FormatMuted(until time.Time) string {
	return fmt.Sprintf("🔇 Alerts muted until `%s`\nOnly alert notifications are paused\\. Commands still work\\.", escapeMarkdown(until.Local().Format("2006-01-02 15:04:05")))
}

// FormatMuteStatus formats current mute state.
func FormatMuteStatus(muted bool, until time.Time) string {
	if muted {
		return fmt.Sprintf("🔇 Alerts are muted until `%s`\nUse `/unmute` to resume now\\.", escapeMarkdown(until.Local().Format("2006-01-02 15:04:05")))
	}
	return "🔔 Alerts are not muted\\.\nUse `/mute 30m`, `/mute 2h`, or `/mute 1h30m`\\."
}

// FormatUnmuted formats the unmute confirmation.
func FormatUnmuted() string {
	return "🔔 Alerts resumed"
}

// ConnectionStatus holds status info for /status command.
type ConnectionStatus struct {
	Name     string
	Provider string
	Status   string // ok, rate_limited, refreshable, error, expired
	Error    string
}

// UsageStats holds usage data for /usage command.
type UsageStats struct {
	Period      string
	Requests    int
	Errors      int
	TokensIn    int64
	TokensOut   int64
	TokensTotal int64
	Cost        float64
	Quotas      []QuotaSummary
}

// QuotaSummary is a compact provider quota summary for /usage.
type QuotaSummary struct {
	Name         string
	Provider     string
	Plan         string
	LimitReached bool
	Message      string
	Buckets      []QuotaBucketSummary
}

// QuotaBucketSummary is one quota bucket for /usage.
type QuotaBucketSummary struct {
	Label     string
	Used      int
	Total     int
	Remaining int
	Pct       int
	ResetAt   string
}

// ConnectionDetail holds detailed connection info.
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

func formatResetAt(value string) string {
	if t, err := time.Parse(time.RFC3339, value); err == nil {
		return t.Local().Format("2006-01-02 15:04")
	}
	return value
}
