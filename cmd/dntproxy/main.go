package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	httpAdapter "github.com/dungnt/dntproxy/internal/adapter/http"
	"github.com/dungnt/dntproxy/internal/adapter/storage"
	"github.com/dungnt/dntproxy/internal/logger"
	"github.com/dungnt/dntproxy/internal/service"
	"github.com/spf13/cobra"
)

var version = "0.1.0"

var dbFlag string

func main() {
	rootCmd := &cobra.Command{
		Use:   "dntproxy",
		Short: "AI proxy router with Kiro provider support",
		Long:  "dntproxy - Go port of 9Router. Routes OpenAI-compatible requests to Kiro (AWS CodeWhisperer) with multi-account fallback.",
		RunE:  runServe,
	}

	rootCmd.PersistentFlags().StringVar(&dbFlag, "db", "", "Path to database file (default: ~/.dntproxy/db.json)")
	rootCmd.Flags().IntP("port", "p", 0, "Port to listen on (default: from config or 20128)")

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
			fmt.Printf("dntproxy %s\n", version)
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
	go runLogRetention(logStore)

	cfg, err := store.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	port := 20199
	if cfg.Settings.Port > 0 {
		port = cfg.Settings.Port
	}
	if envPort := os.Getenv("PORT"); envPort != "" {
		fmt.Sscanf(envPort, "%d", &port)
	}
	if p, _ := cmd.Flags().GetInt("port"); p > 0 {
		port = p
	}

	router := httpAdapter.NewRouter(store)

	scheduler := service.NewTokenRefreshScheduler(store)
	go scheduler.Start()

	addr := fmt.Sprintf(":%d", port)
	log.Printf("[dntproxy] v%s starting on http://localhost%s", version, addr)
	log.Printf("[dntproxy] OpenAI-compatible API: http://localhost%s/v1", addr)

	errCh := make(chan error, 1)
	go func() {
		errCh <- router.Run(addr)
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-sigCh:
		log.Printf("[dntproxy] Received signal %s, shutting down...", sig)
	case err := <-errCh:
		if err != nil {
			log.Printf("[dntproxy] Server error: %s", err)
		}
	}

	scheduler.Stop()
	log.Printf("[dntproxy] Shutdown complete")
	return nil
}

func runLogRetention(logStore *storage.SQLiteLogStore) {
	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()

	for range ticker.C {
		cutoff := time.Now().AddDate(0, 0, -30).UnixMilli()
		if err := logStore.PurgeOlderThan(context.Background(), cutoff); err != nil {
			log.Printf("[LOG] Failed to purge old logs: %s", err)
		}
	}
}
