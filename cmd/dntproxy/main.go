package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/dungnt/dntproxy/internal/adapter/anthropic"
	httpAdapter "github.com/dungnt/dntproxy/internal/adapter/http"
	"github.com/dungnt/dntproxy/internal/adapter/kiro"
	openaiAdapter "github.com/dungnt/dntproxy/internal/adapter/openai"
	"github.com/dungnt/dntproxy/internal/adapter/provider"
	"github.com/dungnt/dntproxy/internal/adapter/storage"
	"github.com/dungnt/dntproxy/internal/logger"
	"github.com/dungnt/dntproxy/internal/service"
	appversion "github.com/dungnt/dntproxy/internal/version"
	"github.com/spf13/cobra"
)

var dbFlag string

func main() {
	rootCmd := &cobra.Command{
		Use:   "dntproxy",
		Short: "AI proxy router with multi-provider support",
		Long:  "dntproxy - OpenAI-compatible proxy router. Routes requests to Kiro, OpenAI, GLM, MiniMax, Qwen, Anthropic with multi-account fallback.",
		RunE:  runServe,
	}

	rootCmd.PersistentFlags().StringVar(&dbFlag, "db", "", "Path to database file (default: ~/.dntproxy/db.json)")
	rootCmd.Flags().IntP("port", "p", 0, "Port to listen on (default: from config or 20199)")

	// Serve
	serveCmd := &cobra.Command{
		Use:   "serve",
		Short: "Start the proxy server",
		RunE:  runServe,
	}
	serveCmd.Flags().IntP("port", "p", 0, "Port to listen on")
	rootCmd.AddCommand(serveCmd)

	// Version
	rootCmd.AddCommand(&cobra.Command{
		Use:   "version",
		Short: "Print version",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("dntproxy %s\n", appversion.Version)
		},
	})

	// Auth commands
	rootCmd.AddCommand(buildAuthCmd())

	// Combo commands
	rootCmd.AddCommand(buildComboCmd())

	// Alias commands
	rootCmd.AddCommand(buildAliasCmd())

	// Key commands
	rootCmd.AddCommand(buildKeyCmd())

	// Backup commands
	rootCmd.AddCommand(buildBackupCmd())

	// Tunnel commands
	rootCmd.AddCommand(buildTunnelCmd())

	// Profile commands
	rootCmd.AddCommand(buildProfileCmd())

	// Connection commands
	rootCmd.AddCommand(buildConnectionCmd())

	// Tools commands
	rootCmd.AddCommand(buildToolsCmd())

	// Update command
	rootCmd.AddCommand(buildUpdateCmd())

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func getStore() (*storage.JsonDB, error) {
	return storage.NewJsonDB(dbFlag)
}

func runServe(cmd *cobra.Command, args []string) error {
	store, err := getStore()
	if err != nil {
		return fmt.Errorf("init storage: %w", err)
	}
	logStore, err := storage.NewSQLiteLogStore(filepath.Join(store.DataDir(), "logs.db"))
	if err != nil {
		return fmt.Errorf("init log storage: %w", err)
	}
	defer logStore.Close()
	logger.Init(logStore)
	logRetentionDone := make(chan struct{})
	go runLogRetention(logStore, logRetentionDone)

	cfg, err := store.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	port := 20199
	if cfg.Settings.Port > 0 {
		port = cfg.Settings.Port
	}
	if envPort := os.Getenv("PORT"); envPort != "" {
		if _, err := fmt.Sscanf(envPort, "%d", &port); err != nil {
			log.Printf("[dntproxy] Invalid PORT env var %q, using default %d", envPort, port)
		}
	}
	if p, _ := cmd.Flags().GetInt("port"); p > 0 {
		port = p
	}

	// Create provider registry and register all known providers.
	// To add a new provider: just add one more RegisterExecutor call here.
	providers := provider.NewRegistry()
	providers.RegisterExecutor("kiro", kiro.NewExecutor())
	providers.RegisterExecutor("openai", openaiAdapter.NewExecutor())
	providers.RegisterExecutor("openai-compatible", openaiAdapter.NewExecutor())
	providers.RegisterExecutor("glm", openaiAdapter.NewExecutor())
	providers.RegisterExecutor("minimax", openaiAdapter.NewExecutor())
	providers.RegisterExecutor("qwen", openaiAdapter.NewExecutor())
	providers.RegisterExecutor("anthropic", anthropic.NewExecutor())
	providers.RegisterExecutor("gemini", openaiAdapter.NewExecutor())

	// Create tunnel manager (optional - can be nil)
	tunnelService, err := service.NewTunnelService(store)
	if err != nil {
		log.Printf("[dntproxy] Tunnel service init failed: %v", err)
		tunnelService = nil
	}

	router := httpAdapter.NewRouter(store, providers, tunnelService)

	// Set actual server port in router context
	httpAdapter.SetServerPort(router, port)

	// Tunnel auto-start is handled by the dashboard UI only.
	// Reset persisted enabled flag so it doesn't leak across restarts.
	if tunnelService != nil && cfg.Settings.TunnelEnabled {
		cfg.Settings.TunnelEnabled = false
		_ = store.Save(cfg)
		log.Printf("[tunnel] Tunnel will not auto-start. Enable via dashboard.")
	}

	addr := fmt.Sprintf(":%d", port)
	log.Printf("[dntproxy] Starting on http://localhost%s", addr)
	log.Printf("[dntproxy] Dashboard: http://localhost%s/dashboard", addr)
	log.Printf("[dntproxy] OpenAI-compatible API: http://localhost%s/v1/chat/completions", addr)
	log.Printf("[dntproxy] Anthropic Messages API: http://localhost%s/v1/messages", addr)

	srv := &http.Server{
		Addr:    addr,
		Handler: router,
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.ListenAndServe()
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-sigCh:
		log.Printf("[dntproxy] Received signal %s, shutting down...", sig)
	case err := <-errCh:
		if err != nil && err != http.ErrServerClosed {
			log.Printf("[dntproxy] Server error: %s", err)
		}
	}

	// Graceful shutdown: wait up to 10s for active requests to finish
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("[dntproxy] Graceful shutdown error: %s", err)
	}

	close(logRetentionDone)

	// Stop tunnel
	if tunnelService != nil {
		tunnelService.Stop()
	}

	log.Printf("[dntproxy] Shutdown complete")
	return nil
}

func runLogRetention(logStore *storage.SQLiteLogStore, done <-chan struct{}) {
	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-done:
			return
		case <-ticker.C:
			cutoff := time.Now().AddDate(0, 0, -30).UnixMilli()
			if err := logStore.PurgeOlderThan(context.Background(), cutoff); err != nil {
				log.Printf("[LOG] Failed to purge old logs: %s", err)
			}
		}
	}
}
