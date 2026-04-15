package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/dungnt/dntproxy/internal/service"
	"github.com/spf13/cobra"
)

func buildTunnelCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tunnel",
		Short: "Manage Cloudflare tunnel",
		Long:  "Enable/disable Cloudflare quick tunnels for public access to local dntproxy.",
	}

	cmd.AddCommand(&cobra.Command{
		Use:   "enable",
		Short: "Start a Cloudflare quick tunnel",
		RunE:  runTunnelEnable,
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "disable",
		Short: "Stop the running tunnel",
		RunE:  runTunnelDisable,
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "status",
		Short: "Show tunnel status",
		RunE:  runTunnelStatus,
	})

	return cmd
}

func runTunnelEnable(cmd *cobra.Command, args []string) error {
	store, err := getStore()
	if err != nil {
		return err
	}

	cfg, err := store.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	port := cfg.Settings.Port
	if port == 0 {
		port = 20199
	}

	tunnelSvc, err := service.NewTunnelService(store)
	if err != nil {
		return fmt.Errorf("init tunnel service: %w", err)
	}

	// Handle graceful shutdown on Ctrl+C
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		fmt.Println("\nStopping tunnel...")
		tunnelSvc.Stop()
		os.Exit(0)
	}()

	fmt.Println("Starting Cloudflare quick tunnel...")
	if err := tunnelSvc.Enable(port); err != nil {
		return fmt.Errorf("enable tunnel: %w", err)
	}

	status := tunnelSvc.Status()
	fmt.Printf("\nTunnel started!\n")
	fmt.Printf("Public URL: %s\n", status.PublicURL)
	fmt.Printf("Direct URL: %s\n", status.TunnelURL)
	fmt.Println("\nPress Ctrl+C to stop")

	// Keep running
	select {}
}

func runTunnelDisable(cmd *cobra.Command, args []string) error {
	store, err := getStore()
	if err != nil {
		return err
	}

	tunnelSvc, err := service.NewTunnelService(store)
	if err != nil {
		return fmt.Errorf("init tunnel service: %w", err)
	}

	if err := tunnelSvc.Disable(); err != nil {
		return fmt.Errorf("disable tunnel: %w", err)
	}

	fmt.Println("Tunnel stopped")
	return nil
}

func runTunnelStatus(cmd *cobra.Command, args []string) error {
	store, err := getStore()
	if err != nil {
		return err
	}

	tunnelSvc, err := service.NewTunnelService(store)
	if err != nil {
		return fmt.Errorf("init tunnel service: %w", err)
	}

	status := tunnelSvc.Status()
	fmt.Printf("Enabled:  %v\n", status.Enabled)
	fmt.Printf("Running:  %v\n", status.Running)
	fmt.Printf("Provider: %s\n", status.Provider)
	fmt.Printf("Short ID: %s\n", status.ShortID)
	if status.PublicURL != "" {
		fmt.Printf("Public:   %s\n", status.PublicURL)
	}
	if status.TunnelURL != "" {
		fmt.Printf("Direct:   %s\n", status.TunnelURL)
	}
	return nil
}
