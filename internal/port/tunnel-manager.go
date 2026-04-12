package port

// TunnelStatus represents the current state of a tunnel.
type TunnelStatus struct {
	Enabled   bool   `json:"enabled"`
	Running   bool   `json:"running"`
	Provider  string `json:"provider"`
	TunnelURL string `json:"tunnelUrl"`
	ShortID   string `json:"shortId"`
	PublicURL string `json:"publicUrl"`
}

// TunnelManager manages cloudflared tunnel lifecycle.
type TunnelManager interface {
	// Enable starts a cloudflared quick tunnel to the given local port.
	Enable(localPort int) error

	// Disable stops the running tunnel.
	Disable() error

	// Status returns current tunnel status.
	Status() TunnelStatus

	// IsRunning checks if the tunnel process is currently alive.
	IsRunning() bool

	// Stop gracefully stops the tunnel on shutdown.
	Stop()
}
