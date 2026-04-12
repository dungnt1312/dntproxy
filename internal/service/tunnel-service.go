package service

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"os"
	"sync"
	"time"

	"github.com/dungnt/dntproxy/internal/adapter/tunnel"
	"github.com/dungnt/dntproxy/internal/domain"
	"github.com/dungnt/dntproxy/internal/port"
)

// TunnelStore defines the storage methods tunnel service needs.
type TunnelStore interface {
	GetSettings() (*domain.Settings, error)
	Update(fn func(cfg *domain.AppConfig)) error
}

// TunnelService implements port.TunnelManager.
type TunnelService struct {
	store       TunnelStore
	state       *tunnel.StateManager
	cloudflared *tunnel.Cloudflared
	ctx         context.Context
	cancel      context.CancelFunc
	mu          sync.Mutex
}

// NewTunnelService creates a new tunnel service.
func NewTunnelService(store TunnelStore) (*TunnelService, error) {
	state, err := tunnel.NewStateManager()
	if err != nil {
		return nil, fmt.Errorf("init tunnel state: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	s := &TunnelService{
		store:  store,
		state:  state,
		ctx:    ctx,
		cancel: cancel,
	}

	// Create cloudflared with URL callback
	s.cloudflared = tunnel.NewCloudflared(state, s.onURLDetected)

	return s, nil
}

func (s *TunnelService) onURLDetected(url string) {
	log.Printf("[tunnel] URL detected: %s", url)

	// Load state to get shortId
	tunnelState, err := s.state.LoadState()
	if err != nil {
		log.Printf("[tunnel] Failed to load state: %v", err)
		return
	}

	// Save tunnel URL
	tunnelState.TunnelURL = url
	if err := s.state.SaveState(tunnelState); err != nil {
		log.Printf("[tunnel] Failed to save state: %v", err)
	}

	// Update settings
	s.store.Update(func(cfg *domain.AppConfig) {
		cfg.Settings.TunnelEnabled = true
		cfg.Settings.TunnelURL = url
		cfg.Settings.TunnelProvider = "cloudflare"
		cfg.Settings.TunnelRunning = true
	})

	log.Printf("[tunnel] Tunnel started: %s", url)
}

// Enable starts a cloudflared quick tunnel.
func (s *TunnelService) Enable(localPort int) error {
	s.mu.Lock()
	if s.cloudflared.IsRunning() {
		s.mu.Unlock()
		return nil // Already running
	}
	s.mu.Unlock()

	// Kill any existing process
	if err := s.cloudflared.Kill(); err != nil {
		log.Printf("[tunnel] Kill existing: %v", err)
	}

	// Generate IDs
	tunnelState, err := s.state.LoadState()
	if err != nil {
		return err
	}

	if tunnelState.ShortID == "" {
		tunnelState.ShortID = generateShortID()
	}
	if tunnelState.MachineID == "" {
		tunnelState.MachineID = generateMachineID()
	}
	if err := s.state.SaveState(tunnelState); err != nil {
		return err
	}

	// Ensure binary exists
	if err := s.cloudflared.EnsureBinary(); err != nil {
		return fmt.Errorf("ensure cloudflared binary: %w", err)
	}

	// Spawn tunnel
	log.Printf("[tunnel] Starting cloudflared quick tunnel on port %d...", localPort)
	if err := s.cloudflared.Spawn(s.ctx, localPort); err != nil {
		return fmt.Errorf("spawn cloudflared: %w", err)
	}

	// Wait for URL detection (timeout 30s)
	for i := 0; i < 30; i++ {
		time.Sleep(1 * time.Second)
		ts, err := s.state.LoadState()
		if err == nil && ts.TunnelURL != "" {
			break
		}
		if i == 29 {
			return fmt.Errorf("timeout waiting for tunnel URL")
		}
	}

	// DNS warmup
	log.Printf("[tunnel] Waiting 8s for DNS propagation...")
	time.Sleep(8 * time.Second)

	log.Printf("[tunnel] Tunnel started successfully")
	return nil
}

// Disable stops the running tunnel.
func (s *TunnelService) Disable() error {
	s.cancel()

	// Create new context for future re-enables
	s.ctx, s.cancel = context.WithCancel(context.Background())

	if err := s.cloudflared.Kill(); err != nil {
		log.Printf("[tunnel] Kill error: %v", err)
	}

	// Clear tunnel URL from state but keep shortId/machineId
	ts, err := s.state.LoadState()
	if err == nil {
		ts.TunnelURL = ""
		s.state.SaveState(ts)
	}

	// Update settings
	s.store.Update(func(cfg *domain.AppConfig) {
		cfg.Settings.TunnelEnabled = false
		cfg.Settings.TunnelURL = ""
		cfg.Settings.TunnelRunning = false
	})

	log.Printf("[tunnel] Tunnel stopped")
	return nil
}

// Status returns current tunnel status.
func (s *TunnelService) Status() port.TunnelStatus {
	settings, err := s.store.GetSettings()
	if err != nil {
		return port.TunnelStatus{}
	}

	ts, _ := s.state.LoadState()
	running := s.cloudflared.IsRunning()

	return port.TunnelStatus{
		Enabled:   settings.TunnelEnabled,
		Running:   running,
		Provider:  settings.TunnelProvider,
		TunnelURL: ts.TunnelURL,
		ShortID:   ts.ShortID,
		PublicURL: ts.TunnelURL, // trycloudflare.com is already public
	}
}

// IsRunning checks if the tunnel process is currently alive.
func (s *TunnelService) IsRunning() bool {
	return s.cloudflared.IsRunning()
}

// Stop gracefully stops the tunnel on shutdown.
func (s *TunnelService) Stop() {
	s.cancel()
	s.cloudflared.Kill()
}

// generateShortID creates a 6-char alphanumeric ID.
func generateShortID() string {
	const chars = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, 6)
	for i := range b {
		b[i] = chars[rand.Intn(len(chars))]
	}
	return string(b)
}

// generateMachineID creates a machine identifier.
func generateMachineID() string {
	hostname, _ := os.Hostname()
	return fmt.Sprintf("%s-%d", hostname, time.Now().Unix())
}
