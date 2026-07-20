package domain

// LogEntry is a structured event persisted in the local log database.
type LogEntry struct {
	ID             string  `json:"id"`
	Timestamp      string  `json:"timestamp"`
	TimestampMs    int64   `json:"timestampMs"`
	Level          string  `json:"level"`
	Provider       string  `json:"provider"`
	Direction      string  `json:"direction"`
	Method         string  `json:"method,omitempty"`
	Path           string  `json:"path,omitempty"`
	StatusCode     int     `json:"statusCode,omitempty"`
	DurationMs     int64   `json:"durationMs,omitempty"`
	ConnectionID   string  `json:"connectionId,omitempty"`
	ConnectionName string  `json:"connectionName,omitempty"`
	Model          string  `json:"model,omitempty"`
	RequestID      string  `json:"requestId,omitempty"`
	Message        string  `json:"message"`
	Error          string  `json:"error,omitempty"`
	BodySize       int     `json:"bodySize,omitempty"`
	RequestBody    string  `json:"requestBody,omitempty"`
	ResponseBody   string  `json:"responseBody,omitempty"`
	InputTokens    int     `json:"inputTokens,omitempty"`
	OutputTokens   int     `json:"outputTokens,omitempty"`
	TotalTokens    int     `json:"totalTokens,omitempty"`
	UsageSource    string  `json:"usageSource,omitempty"`
	CostInput      float64 `json:"costInput,omitempty"`
	CostOutput     float64 `json:"costOutput,omitempty"`
	CostTotal      float64 `json:"costTotal,omitempty"`
	Currency       string  `json:"currency,omitempty"`
	MetadataJSON   string  `json:"metadataJson,omitempty"`
	TenantID       string  `json:"tenantId,omitempty"` // multi-tenancy support for SaaS
}

// CompressionLogMetadata stores per-request token compression savings.
type CompressionLogMetadata struct {
	OriginalBytes       int            `json:"originalBytes"`
	CompressedBytes     int            `json:"compressedBytes"`
	SavedBytes          int            `json:"savedBytes"`
	TokensSavedEstimate int            `json:"tokensSavedEstimate"`
	Ratio               float64        `json:"ratio"`
	Detections          map[string]int `json:"detections,omitempty"`
	Skipped             int            `json:"skipped,omitempty"`
}

// LogQuery filters log list and summary requests.
type LogQuery struct {
	ConnectionID string
	Provider     string
	Level        string
	Search       string
	Range        string
	Limit        int
	TenantID     string // for multi-tenancy filtering
}

// LogSummary contains aggregate usage and cost data for the selected logs.
type LogSummary struct {
	Requests     int     `json:"requests"`
	Errors       int     `json:"errors"`
	InputTokens  int     `json:"inputTokens"`
	OutputTokens int     `json:"outputTokens"`
	TotalTokens  int     `json:"totalTokens"`
	CostTotal    float64 `json:"costTotal"`
	Currency     string  `json:"currency"`
	AvgLatencyMs float64 `json:"avgLatencyMs"`
}

// LogConnectionSummary powers the connection filter and side panel.
type LogConnectionSummary struct {
	ConnectionID   string  `json:"connectionId"`
	ConnectionName string  `json:"connectionName"`
	Provider       string  `json:"provider"`
	Requests       int     `json:"requests"`
	Errors         int     `json:"errors"`
	TotalTokens    int     `json:"totalTokens"`
	InputTokens    int     `json:"inputTokens"`
	OutputTokens   int     `json:"outputTokens"`
	CostTotal      float64 `json:"costTotal"`
	Currency       string  `json:"currency"`
	LastUsedMs     int64   `json:"lastUsedMs"`
	AvgLatencyMs   float64 `json:"avgLatencyMs"`
}

// DailyUsageStat contains aggregated usage metrics for a single calendar day.
type DailyUsageStat struct {
	Date         string         `json:"date"`
	Requests     int            `json:"requests"`
	Errors       int            `json:"errors"`
	InputTokens  int            `json:"inputTokens"`
	OutputTokens int            `json:"outputTokens"`
	TotalTokens  int            `json:"totalTokens"`
	CostTotal    float64        `json:"costTotal"`
	Models       map[string]int `json:"models,omitempty"`
}

// ModelPrice stores an editable price profile used for estimated cost.
type ModelPrice struct {
	ID           string  `json:"id"`
	Provider     string  `json:"provider"`
	ModelPattern string  `json:"modelPattern"`
	InputPer1M   float64 `json:"inputPer1m"`
	OutputPer1M  float64 `json:"outputPer1m"`
	Currency     string  `json:"currency"`
	SourceNote   string  `json:"sourceNote"`
	UpdatedAtMs  int64   `json:"updatedAtMs"`
	IsUserEdited bool    `json:"isUserEdited"`
}
