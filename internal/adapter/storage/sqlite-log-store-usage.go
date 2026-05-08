package storage

import (
	"context"
	"fmt"
	"time"

	"github.com/dungnt/dntproxy/internal/domain"
)

func periodCutoffMs(period string) int64 {
	now := time.Now()
	switch period {
	case "24h":
		return now.Add(-24 * time.Hour).UnixMilli()
	case "7d":
		return now.AddDate(0, 0, -7).UnixMilli()
	case "30d":
		return now.AddDate(0, 0, -30).UnixMilli()
	case "60d":
		return now.AddDate(0, 0, -60).UnixMilli()
	default:
		return now.AddDate(0, 0, -7).UnixMilli()
	}
}

func periodDays(period string) int {
	switch period {
	case "24h":
		return 1
	case "7d":
		return 7
	case "30d":
		return 30
	case "60d":
		return 60
	default:
		return 7
	}
}

// UsageStats returns aggregated usage grouped by provider, model, and connection.
func (s *SQLiteLogStore) UsageStats(ctx context.Context, period string) (*domain.UsageStatsResponse, error) {
	cutoff := periodCutoffMs(period)

	resp := &domain.UsageStatsResponse{
		Period:       period,
		ByProvider:   []domain.UsageGroup{},
		ByModel:      []domain.UsageGroup{},
		ByConnection: []domain.UsageGroup{},
	}

	// Totals
	row := s.db.QueryRowContext(ctx, `SELECT
		COUNT(*),
		COALESCE(SUM(input_tokens), 0),
		COALESCE(SUM(output_tokens), 0),
		COALESCE(SUM(cost_total), 0)
		FROM request_logs WHERE direction = 'response' AND timestamp_ms >= ?`, cutoff)
	if err := row.Scan(&resp.TotalRequests, &resp.TotalPromptTokens,
		&resp.TotalCompletionTokens, &resp.TotalCost); err != nil {
		return nil, fmt.Errorf("usage totals: %w", err)
	}

	// By provider
	byProvider, err := s.queryUsageGroup(ctx, cutoff, "provider", "provider")
	if err != nil {
		return nil, err
	}
	resp.ByProvider = byProvider

	// By model
	byModel, err := s.queryUsageGroup(ctx, cutoff, "model", "model")
	if err != nil {
		return nil, err
	}
	resp.ByModel = byModel

	// By connection
	byConn, err := s.queryUsageGroup(ctx, cutoff, "COALESCE(connection_name, connection_id)", "connection_name")
	if err != nil {
		return nil, err
	}
	resp.ByConnection = byConn

	return resp, nil
}

func (s *SQLiteLogStore) queryUsageGroup(ctx context.Context, cutoff int64, groupCol, labelCol string) ([]domain.UsageGroup, error) {
	query := fmt.Sprintf(`SELECT %s, %s,
		COUNT(*),
		COALESCE(SUM(input_tokens), 0),
		COALESCE(SUM(output_tokens), 0),
		COALESCE(SUM(total_tokens), 0),
		COALESCE(SUM(cost_input), 0),
		COALESCE(SUM(cost_output), 0),
		COALESCE(SUM(cost_total), 0)
		FROM request_logs
		WHERE direction = 'response' AND timestamp_ms >= ? AND %s != ''
		GROUP BY %s
		ORDER BY COALESCE(SUM(total_tokens), 0) DESC`, groupCol, labelCol, groupCol, groupCol)

	rows, err := s.db.QueryContext(ctx, query, cutoff)
	if err != nil {
		return nil, fmt.Errorf("usage group query: %w", err)
	}
	defer rows.Close()

	result := make([]domain.UsageGroup, 0)
	for rows.Next() {
		var g domain.UsageGroup
		if err := rows.Scan(&g.Key, &g.Label, &g.Requests,
			&g.PromptTokens, &g.CompletionTokens, &g.TotalTokens,
			&g.InputCost, &g.OutputCost, &g.TotalCost); err != nil {
			return nil, err
		}
		result = append(result, g)
	}
	return result, rows.Err()
}

// ChartData returns time-bucketed usage for the chart.
func (s *SQLiteLogStore) ChartData(ctx context.Context, period string) ([]domain.ChartPoint, error) {
	cutoff := periodCutoffMs(period)
	days := periodDays(period)

	if period == "24h" {
		return s.hourlyChartData(ctx, cutoff)
	}
	return s.dailyChartData(ctx, cutoff, days)
}

func (s *SQLiteLogStore) hourlyChartData(ctx context.Context, cutoff int64) ([]domain.ChartPoint, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT
		(strftime('%%H', timestamp_ms/1000, 'unixepoch', 'localtime') || ':00') as bucket,
		COALESCE(SUM(total_tokens), 0),
		COALESCE(SUM(cost_total), 0)
		FROM request_logs
		WHERE direction = 'response' AND timestamp_ms >= ?
		GROUP BY bucket
		ORDER BY bucket ASC`, cutoff)
	if err != nil {
		return nil, fmt.Errorf("hourly chart: %w", err)
	}
	defer rows.Close()

	byHour := make(map[string]domain.ChartPoint)
	for rows.Next() {
		var cp domain.ChartPoint
		if err := rows.Scan(&cp.Label, &cp.Tokens, &cp.Cost); err != nil {
			return nil, err
		}
		byHour[cp.Label] = cp
	}

	result := make([]domain.ChartPoint, 0, 24)
	now := time.Now()
	for i := 23; i >= 0; i-- {
		h := now.Add(-time.Duration(i) * time.Hour)
		label := h.Format("15:04")
		if cp, ok := byHour[label]; ok {
			result = append(result, cp)
		} else {
			result = append(result, domain.ChartPoint{Label: label})
		}
	}
	return result, rows.Err()
}

func (s *SQLiteLogStore) dailyChartData(ctx context.Context, cutoff int64, days int) ([]domain.ChartPoint, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT
		date(timestamp_ms/1000, 'unixepoch', 'localtime') as day,
		COALESCE(SUM(total_tokens), 0),
		COALESCE(SUM(cost_total), 0)
		FROM request_logs
		WHERE direction = 'response' AND timestamp_ms >= ?
		GROUP BY day
		ORDER BY day ASC`, cutoff)
	if err != nil {
		return nil, fmt.Errorf("daily chart: %w", err)
	}
	defer rows.Close()

	byDate := make(map[string]domain.ChartPoint)
	for rows.Next() {
		var cp domain.ChartPoint
		if err := rows.Scan(&cp.Label, &cp.Tokens, &cp.Cost); err != nil {
			return nil, err
		}
		byDate[cp.Label] = cp
	}

	result := make([]domain.ChartPoint, 0, days)
	for i := days - 1; i >= 0; i-- {
		d := time.Now().AddDate(0, 0, -i).Format("2006-01-02")
		if cp, ok := byDate[d]; ok {
			result = append(result, cp)
		} else {
			result = append(result, domain.ChartPoint{Label: d})
		}
	}
	return result, rows.Err()
}

// RequestDetails returns paginated request detail rows.
func (s *SQLiteLogStore) RequestDetails(ctx context.Context, page, pageSize int, provider, startDate, endDate string) (*domain.RequestDetailsResponse, error) {
	where := "WHERE direction = 'response'"
	var args []interface{}

	if provider != "" {
		where += " AND provider = ?"
		args = append(args, provider)
	}
	if startDate != "" {
		where += " AND timestamp_ms >= ?"
		t, err := time.Parse("2006-01-02", startDate)
		if err == nil {
			args = append(args, t.UnixMilli())
		}
	}
	if endDate != "" {
		where += " AND timestamp_ms <= ?"
		t, err := time.Parse("2006-01-02", endDate)
		if err == nil {
			args = append(args, t.Add(24*time.Hour).UnixMilli()-1)
		}
	}

	if pageSize <= 0 {
		pageSize = 20
	}
	if page <= 0 {
		page = 1
	}

	var totalItems int
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM request_logs %s", where)
	if err := s.db.QueryRowContext(ctx, countQuery, args...).Scan(&totalItems); err != nil {
		return nil, fmt.Errorf("count request details: %w", err)
	}

	offset := (page - 1) * pageSize
	dataQuery := fmt.Sprintf(`SELECT timestamp, model, provider,
		COALESCE(connection_name, ''),
		CASE WHEN level = 'ERROR' THEN 'error' ELSE 'ok' END,
		COALESCE(input_tokens, 0), COALESCE(output_tokens, 0),
		COALESCE(total_tokens, 0), COALESCE(cost_total, 0),
		COALESCE(duration_ms, 0)
		FROM request_logs %s
		ORDER BY timestamp_ms DESC LIMIT ? OFFSET ?`, where)

	queryArgs := append(args, pageSize, offset)
	rows, err := s.db.QueryContext(ctx, dataQuery, queryArgs...)
	if err != nil {
		return nil, fmt.Errorf("request details: %w", err)
	}
	defer rows.Close()

	details := make([]domain.RequestDetail, 0)
	for rows.Next() {
		var d domain.RequestDetail
		if err := rows.Scan(&d.Timestamp, &d.Model, &d.Provider,
			&d.ConnectionName, &d.Status, &d.PromptTokens,
			&d.CompletionTokens, &d.TotalTokens, &d.Cost,
			&d.DurationMs); err != nil {
			return nil, err
		}
		details = append(details, d)
	}

	totalPages := (totalItems + pageSize - 1) / pageSize

	return &domain.RequestDetailsResponse{
		Details: details,
		Pagination: domain.Pagination{
			Page:       page,
			PageSize:   pageSize,
			TotalItems: totalItems,
			TotalPages: totalPages,
		},
	}, rows.Err()
}
