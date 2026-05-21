package telegram

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/dungnt/dntproxy/internal/domain"
)

func (b *Bot) cmdStatus() {
	cfg, err := b.store.Load()
	if err != nil {
		b.sendMarkdown(escapeMarkdown("Failed to load config: " + err.Error()))
		return
	}

	muteUntil, muted := b.MuteState()
	statuses := make([]ConnectionStatus, 0, len(cfg.ProviderConnections))
	for _, conn := range cfg.ProviderConnections {
		if !conn.IsActive {
			continue
		}
		cs := ConnectionStatus{
			Name:     conn.Name,
			Provider: conn.Provider,
			Status:   "ok",
		}

		now := time.Now()

		// Check rate-limited
		if conn.RateLimitedUntil != "" {
			if t, err := time.Parse(time.RFC3339, conn.RateLimitedUntil); err == nil && t.After(now) {
				cs.Status = "rate_limited"
				cs.Error = fmt.Sprintf("until %s", t.Format("15:04:05"))
			}
		}

		// Check token expired
		if conn.ExpiresAt != "" {
			if t, err := time.Parse(time.RFC3339, conn.ExpiresAt); err == nil && t.Before(now) {
				if conn.RefreshToken != "" {
					cs.Status = "refreshable"
					cs.Error = "token expired; refresh token available"
				} else {
					cs.Status = "expired"
					cs.Error = "token expired; no refresh token"
				}
			}
		}

		// Check last error (overrides if backoff is high)
		if conn.BackoffLevel >= 4 {
			cs.Status = "error"
			cs.Error = conn.LastError
		} else if conn.LastError != "" && cs.Status == "ok" {
			// Has error but low backoff — still show it
			if conn.BackoffLevel > 0 {
				cs.Status = "error"
				cs.Error = conn.LastError
			}
		}

		statuses = append(statuses, cs)
	}

	b.sendMarkdown(FormatStatus(statuses, cfg.Settings.Telegram.Enabled, muted, muteUntil, SuppressionWindow()))
}

func (b *Bot) cmdUsage(args []string) {
	period := "today"
	if len(args) > 0 {
		period = strings.ToLower(args[0])
	}

	// Map user-friendly periods to LogStore period format
	queryPeriod := "24h"
	displayPeriod := "Today"
	switch period {
	case "today", "1d":
		queryPeriod = "24h"
		displayPeriod = "Today"
	case "7d", "week":
		queryPeriod = "7d"
		displayPeriod = "Last 7 days"
	case "30d", "month":
		queryPeriod = "30d"
		displayPeriod = "Last 30 days"
	}

	if b.logStore == nil {
		b.sendMarkdown(escapeMarkdown("Log store not available"))
		return
	}

	summary, err := b.logStore.Summary(context.Background(), domain.LogQuery{Range: queryPeriod})
	if err != nil {
		b.sendMarkdown(escapeMarkdown("Failed to query usage: " + err.Error()))
		return
	}

	stats := UsageStats{
		Period:      displayPeriod,
		Requests:    summary.Requests,
		Errors:      summary.Errors,
		TokensIn:    int64(summary.InputTokens),
		TokensOut:   int64(summary.OutputTokens),
		TokensTotal: int64(summary.TotalTokens),
		Cost:        summary.CostTotal,
	}
	stats.Quotas = b.collectQuotaSummaries()

	b.sendMarkdown(FormatUsage(stats))
}

func (b *Bot) collectQuotaSummaries() []QuotaSummary {
	cfg, err := b.store.Load()
	if err != nil {
		return nil
	}

	summaries := make([]QuotaSummary, 0, len(cfg.ProviderConnections))
	for i := range cfg.ProviderConnections {
		conn := &cfg.ProviderConnections[i]
		if !conn.IsActive || !supportsTelegramQuota(conn) {
			continue
		}

		usage, err := fetchQuotaUsage(conn)
		if err != nil {
			summaries = append(summaries, QuotaSummary{Name: conn.Name, Provider: conn.Provider, Message: err.Error()})
			continue
		}
		if usage == nil {
			continue
		}

		summary := QuotaSummary{Name: conn.Name, Provider: conn.Provider, Plan: usage.Plan, LimitReached: usage.LimitReached, Message: usage.Message}
		for _, q := range usage.Quotas {
			summary.Buckets = append(summary.Buckets, QuotaBucketSummary{
				Label:     q.Label,
				Used:      q.Used,
				Total:     q.Total,
				Remaining: q.Remaining,
				Pct:       q.Pct,
				ResetAt:   q.ResetAt,
			})
		}
		summaries = append(summaries, summary)
	}
	return summaries
}

func supportsTelegramQuota(conn *domain.ProviderConnection) bool {
	return conn.Provider == "kiro" || conn.Provider == "openai" || conn.Provider == "minimax"
}

func (b *Bot) cmdConnections() {
	cfg, err := b.store.Load()
	if err != nil {
		b.sendMarkdown(escapeMarkdown("Failed to load config: " + err.Error()))
		return
	}

	details := make([]ConnectionDetail, 0, len(cfg.ProviderConnections))
	for _, conn := range cfg.ProviderConnections {
		if !conn.IsActive {
			continue
		}
		cd := ConnectionDetail{
			Name:         conn.Name,
			Provider:     conn.Provider,
			ModelCount:   len(conn.SupportedModels),
			BackoffLevel: conn.BackoffLevel,
			LastError:    conn.LastError,
		}

		// Calculate cooldown remaining
		if conn.RateLimitedUntil != "" {
			if t, err := time.Parse(time.RFC3339, conn.RateLimitedUntil); err == nil && t.After(time.Now()) {
				remaining := time.Until(t).Round(time.Second)
				cd.CooldownRemaining = remaining.String()
			}
		}

		details = append(details, cd)
	}

	b.sendMarkdown(FormatConnections(details))
}

func (b *Bot) cmdMute(args []string) {
	if len(args) == 0 {
		until, muted := b.MuteState()
		b.sendMarkdown(FormatMuteStatus(muted, until))
		return
	}

	duration, err := parseDuration(args[0])
	if err != nil {
		b.sendMarkdown(escapeMarkdown("Invalid duration: " + args[0] + ". Use e.g. 1h, 30m, 2h"))
		return
	}

	until := time.Now().Add(duration)
	b.setMute(until)
	b.sendMarkdown(FormatMuted(until))
}

// MuteState returns the current mute state and clears expired persisted mutes.
func (b *Bot) MuteState() (time.Time, bool) {
	b.muteMu.RLock()
	until := b.mutedUntil
	b.muteMu.RUnlock()
	if until.IsZero() {
		return time.Time{}, false
	}
	if time.Now().Before(until) {
		return until, true
	}
	b.clearMute()
	return time.Time{}, false
}

func (b *Bot) cmdUnmute() {
	b.clearMute()
	b.sendMarkdown(FormatUnmuted())
}

// parseDuration parses shorthand durations like "2h", "30m", "1h30m".
func parseDuration(s string) (time.Duration, error) {
	// Try standard Go duration first
	d, err := time.ParseDuration(s)
	if err == nil && d > 0 {
		return d, nil
	}

	// Try simple number + suffix
	s = strings.TrimSpace(strings.ToLower(s))
	if len(s) < 2 {
		return 0, fmt.Errorf("invalid duration: %s", s)
	}

	suffix := s[len(s)-1]
	numStr := s[:len(s)-1]
	num, err := strconv.Atoi(numStr)
	if err != nil || num <= 0 {
		return 0, fmt.Errorf("invalid duration: %s", s)
	}

	switch suffix {
	case 'h':
		return time.Duration(num) * time.Hour, nil
	case 'm':
		return time.Duration(num) * time.Minute, nil
	case 's':
		return time.Duration(num) * time.Second, nil
	default:
		return 0, fmt.Errorf("invalid duration suffix: %c", suffix)
	}
}
