package storage

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/dungnt/dntproxy/internal/domain"
)

// List returns recent logs that match the query.
func (s *SQLiteLogStore) List(ctx context.Context, query domain.LogQuery) ([]domain.LogEntry, error) {
	where, args := buildLogWhere(query)
	limit := normalizeLimit(query.Limit)
	args = append(args, limit)

	rows, err := s.db.QueryContext(ctx, `SELECT id, timestamp_ms, timestamp, level, provider, direction,
		COALESCE(method, ''), COALESCE(path, ''), status_code, duration_ms,
		COALESCE(connection_id, ''), COALESCE(connection_name, ''), COALESCE(model, ''),
		COALESCE(request_id, ''), COALESCE(message, ''), COALESCE(error, ''), body_size,
		input_tokens, output_tokens, total_tokens, COALESCE(usage_source, ''),
		cost_input, cost_output, cost_total, currency, COALESCE(metadata_json, ''),
		COALESCE(request_body, ''), COALESCE(response_body, '')
		FROM request_logs `+where+` ORDER BY timestamp_ms DESC LIMIT ?`, args...)
	if err != nil {
		return nil, fmt.Errorf("list logs: %w", err)
	}
	defer rows.Close()

	logs := make([]domain.LogEntry, 0)
	for rows.Next() {
		entry, err := scanLogEntry(rows)
		if err != nil {
			return nil, err
		}
		logs = append(logs, *entry)
	}
	return logs, rows.Err()
}

// Summary returns aggregate request, error, token, cost, and latency totals.
func (s *SQLiteLogStore) Summary(ctx context.Context, query domain.LogQuery) (*domain.LogSummary, error) {
	where, args := buildLogWhere(query)
	row := s.db.QueryRowContext(ctx, `SELECT
		COUNT(CASE WHEN direction = 'response' THEN 1 END),
		COUNT(CASE WHEN level = 'ERROR' THEN 1 END),
		COALESCE(SUM(input_tokens), 0),
		COALESCE(SUM(output_tokens), 0),
		COALESCE(SUM(total_tokens), 0),
		COALESCE(SUM(cost_total), 0),
		COALESCE(AVG(CASE WHEN direction = 'response' AND duration_ms > 0 THEN duration_ms END), 0)
		FROM request_logs `+where, args...)

	summary := &domain.LogSummary{Currency: "USD"}
	if err := row.Scan(&summary.Requests, &summary.Errors, &summary.InputTokens,
		&summary.OutputTokens, &summary.TotalTokens, &summary.CostTotal, &summary.AvgLatencyMs); err != nil {
		return nil, fmt.Errorf("summarize logs: %w", err)
	}
	return summary, nil
}

// ConnectionSummaries aggregates usage by connection.
func (s *SQLiteLogStore) ConnectionSummaries(ctx context.Context, query domain.LogQuery) ([]domain.LogConnectionSummary, error) {
	where, args := buildLogWhere(query)
	rows, err := s.db.QueryContext(ctx, `SELECT COALESCE(connection_id, ''), COALESCE(connection_name, ''), provider,
		COUNT(CASE WHEN direction = 'response' THEN 1 END),
		COUNT(CASE WHEN level = 'ERROR' THEN 1 END),
		COALESCE(SUM(total_tokens), 0),
		COALESCE(SUM(input_tokens), 0),
		COALESCE(SUM(output_tokens), 0),
		COALESCE(SUM(cost_total), 0),
		COALESCE(MAX(timestamp_ms), 0),
		COALESCE(AVG(duration_ms), 0)
		FROM request_logs `+where+`
		GROUP BY connection_id, connection_name, provider
		HAVING COALESCE(connection_id, '') <> ''
		ORDER BY COALESCE(SUM(total_tokens), 0) DESC`, args...)
	if err != nil {
		return nil, fmt.Errorf("connection summaries: %w", err)
	}
	defer rows.Close()

	result := make([]domain.LogConnectionSummary, 0)
	for rows.Next() {
		var item domain.LogConnectionSummary
		item.Currency = "USD"
		if err := rows.Scan(&item.ConnectionID, &item.ConnectionName, &item.Provider,
			&item.Requests, &item.Errors, &item.TotalTokens,
			&item.InputTokens, &item.OutputTokens,
			&item.CostTotal, &item.LastUsedMs, &item.AvgLatencyMs); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

// DailyStats returns per-day aggregated usage for the requested range, with gaps filled as zero rows.
func (s *SQLiteLogStore) DailyStats(ctx context.Context, query domain.LogQuery) ([]domain.DailyUsageStat, error) {
	cutoffMs := rangeCutoffMs(query.Range)
	rows, err := s.db.QueryContext(ctx, `
		SELECT
			date(timestamp_ms/1000, 'unixepoch', 'localtime') as day,
			COUNT(CASE WHEN direction = 'response' THEN 1 END),
			COUNT(CASE WHEN level = 'ERROR' THEN 1 END),
			COALESCE(SUM(input_tokens), 0),
			COALESCE(SUM(output_tokens), 0),
			COALESCE(SUM(total_tokens), 0),
			COALESCE(SUM(cost_total), 0)
		FROM request_logs
		WHERE timestamp_ms >= ?
		GROUP BY day
		ORDER BY day ASC`, cutoffMs)
	if err != nil {
		return nil, fmt.Errorf("daily stats: %w", err)
	}
	defer rows.Close()

	byDate := make(map[string]domain.DailyUsageStat)
	for rows.Next() {
		var stat domain.DailyUsageStat
		if err := rows.Scan(&stat.Date, &stat.Requests, &stat.Errors,
			&stat.InputTokens, &stat.OutputTokens, &stat.TotalTokens, &stat.CostTotal); err != nil {
			return nil, err
		}
		byDate[stat.Date] = stat
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	days := rangeDayCount(query.Range)
	result := make([]domain.DailyUsageStat, 0, days)
	for i := days - 1; i >= 0; i-- {
		d := time.Now().AddDate(0, 0, -i).Format("2006-01-02")
		if s, ok := byDate[d]; ok {
			result = append(result, s)
		} else {
			result = append(result, domain.DailyUsageStat{Date: d})
		}
	}
	return result, nil
}

func rangeDayCount(r string) int {
	switch r {
	case "7d":
		return 7
	case "30d":
		return 30
	default:
		return 14
	}
}

func buildLogWhere(query domain.LogQuery) (string, []interface{}) {
	clauses := []string{"timestamp_ms >= ?"}
	args := []interface{}{rangeCutoffMs(query.Range)}

	if query.ConnectionID != "" && query.ConnectionID != "all" {
		clauses = append(clauses, "connection_id = ?")
		args = append(args, query.ConnectionID)
	}
	if query.Provider != "" && query.Provider != "all" {
		clauses = append(clauses, "provider = ?")
		args = append(args, query.Provider)
	}
	if query.Level != "" && query.Level != "all" {
		clauses = append(clauses, "level = ?")
		args = append(args, query.Level)
	}
	if query.Search != "" {
		clauses = append(clauses, "(message LIKE ? ESCAPE '\\' OR error LIKE ? ESCAPE '\\' OR model LIKE ? ESCAPE '\\' OR request_id LIKE ? ESCAPE '\\' OR metadata_json LIKE ? ESCAPE '\\')")
		escaped := escapeLike(query.Search)
		search := "%" + escaped + "%"
		args = append(args, search, search, search, search, search)
	}

	return "WHERE " + strings.Join(clauses, " AND "), args
}

func rangeCutoffMs(value string) int64 {
	now := time.Now()
	switch value {
	case "1h":
		return now.Add(-time.Hour).UnixMilli()
	case "7d":
		return now.AddDate(0, 0, -7).UnixMilli()
	case "30d":
		return now.AddDate(0, 0, -30).UnixMilli()
	default:
		return now.AddDate(0, 0, -1).UnixMilli()
	}
}

func normalizeLimit(limit int) int {
	if limit <= 0 {
		return 200
	}
	if limit > 1000 {
		return 1000
	}
	return limit
}

type logScanner interface {
	Scan(dest ...interface{}) error
}

func scanLogEntry(row logScanner) (*domain.LogEntry, error) {
	var entry domain.LogEntry
	if err := row.Scan(&entry.ID, &entry.TimestampMs, &entry.Timestamp, &entry.Level,
		&entry.Provider, &entry.Direction, &entry.Method, &entry.Path, &entry.StatusCode,
		&entry.DurationMs, &entry.ConnectionID, &entry.ConnectionName, &entry.Model,
		&entry.RequestID, &entry.Message, &entry.Error, &entry.BodySize, &entry.InputTokens,
		&entry.OutputTokens, &entry.TotalTokens, &entry.UsageSource, &entry.CostInput,
		&entry.CostOutput, &entry.CostTotal, &entry.Currency, &entry.MetadataJSON,
		&entry.RequestBody, &entry.ResponseBody); err != nil {
		return nil, err
	}
	return &entry, nil
}

// escapeLike escapes SQL LIKE special characters so user input is treated
// as a literal pattern rather than a wildcard expression.
func escapeLike(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "%", "\\%")
	s = strings.ReplaceAll(s, "_", "\\_")
	return s
}
