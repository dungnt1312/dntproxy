package domain

// UsageStatsResponse is the aggregated usage stats payload for GET /api/usage/stats.
type UsageStatsResponse struct {
	Period           string                       `json:"period"`
	TotalRequests    int                          `json:"totalRequests"`
	TotalPromptTokens int                         `json:"totalPromptTokens"`
	TotalCompletionTokens int                     `json:"totalCompletionTokens"`
	TotalCost        float64                      `json:"totalCost"`
	ByProvider       []UsageGroup                 `json:"byProvider"`
	ByModel          []UsageGroup                 `json:"byModel"`
	ByConnection     []UsageGroup                 `json:"byConnection"`
}

// UsageGroup is one row in a grouped usage table.
type UsageGroup struct {
	Key              string  `json:"key"`
	Label            string  `json:"label,omitempty"`
	Requests         int     `json:"requests"`
	PromptTokens     int     `json:"promptTokens"`
	CompletionTokens int     `json:"completionTokens"`
	TotalTokens      int     `json:"totalTokens"`
	InputCost        float64 `json:"inputCost"`
	OutputCost       float64 `json:"outputCost"`
	TotalCost        float64 `json:"totalCost"`
}

// ChartPoint is one data point for the usage chart.
type ChartPoint struct {
	Label  string  `json:"label"`
	Tokens int     `json:"tokens"`
	Cost   float64 `json:"cost"`
}

// RequestDetail is one row in the paginated request details table.
type RequestDetail struct {
	Timestamp        string  `json:"timestamp"`
	Model            string  `json:"model"`
	Provider         string  `json:"provider"`
	ConnectionName   string  `json:"connectionName,omitempty"`
	Status           string  `json:"status"`
	PromptTokens     int     `json:"promptTokens"`
	CompletionTokens int     `json:"completionTokens"`
	TotalTokens      int     `json:"totalTokens"`
	Cost             float64 `json:"cost"`
	DurationMs       int64   `json:"durationMs,omitempty"`
}

// RequestDetailsResponse is the paginated response for GET /api/usage/request-details.
type RequestDetailsResponse struct {
	Details    []RequestDetail `json:"details"`
	Pagination Pagination      `json:"pagination"`
}

// Pagination holds page metadata.
type Pagination struct {
	Page       int `json:"page"`
	PageSize   int `json:"pageSize"`
	TotalItems int `json:"totalItems"`
	TotalPages int `json:"totalPages"`
}
