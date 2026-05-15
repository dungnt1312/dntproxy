package port

// AlertType identifies the kind of alert sent by the notifier.
type AlertType string

const (
	AlertQuotaExhausted      AlertType = "quota_exhausted"
	AlertTokenExpired        AlertType = "token_expired"
	AlertConnectionDown      AlertType = "connection_down"
	AlertAllDown             AlertType = "all_down"
	AlertRateLimited         AlertType = "rate_limited"
	AlertComboExhausted      AlertType = "combo_exhausted"
	AlertConnectionRecovered AlertType = "connection_recovered"
)

// Alert represents a notification to be sent to the user.
type Alert struct {
	Type         AlertType
	ConnectionID string
	Connection   string // display name
	Provider     string
	Model        string
	Message      string
	Combo        string // for combo_exhausted
}

// Notifier manages alert delivery and interactive commands.
type Notifier interface {
	Start() error
	Stop()
	IsRunning() bool
	SendAlert(alert Alert) error
	SendMessage(text string) error
}
