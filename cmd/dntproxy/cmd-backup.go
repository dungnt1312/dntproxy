package main

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/dungnt/dntproxy/internal/domain"
	"github.com/spf13/cobra"
)

func buildBackupCmd() *cobra.Command {
	backupCmd := &cobra.Command{
		Use:   "backup",
		Short: "Backup and restore dntproxy configuration",
	}

	exportCmd := &cobra.Command{
		Use:   "export [file]",
		Short: "Export dntproxy configuration to a JSON file",
		Args:  cobra.MaximumNArgs(1),
		RunE:  runExport,
	}
	exportCmd.Flags().Bool("mask", false, "Mask sensitive data (tokens, API keys)")

	importCmd := &cobra.Command{
		Use:   "import <file>",
		Short: "Import dntproxy configuration from a JSON file",
		Args:  cobra.ExactArgs(1),
		RunE:  runImport,
	}
	importCmd.Flags().String("mode", "merge", "Import mode: 'replace' (replace all) or 'merge' (add to existing)")

	backupCmd.AddCommand(exportCmd)
	backupCmd.AddCommand(importCmd)

	return backupCmd
}

type BackupData struct {
	Version             string                      `json:"version"`
	ExportedAt          string                      `json:"exportedAt"`
	ProviderConnections []domain.ProviderConnection `json:"providerConnections"`
	Combos              []domain.Combo              `json:"combos"`
	ModelAliases        domain.AliasMap             `json:"modelAliases"`
	APIKeys             []domain.APIKey             `json:"apiKeys"`
	Settings            domain.Settings             `json:"settings"`
}

func runExport(cmd *cobra.Command, args []string) error {
	store, err := getStore()
	if err != nil {
		return fmt.Errorf("failed to open store: %w", err)
	}

	cfg, err := store.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	maskTokens, _ := cmd.Flags().GetBool("mask")

	// Clone and optionally mask sensitive data
	connections := make([]domain.ProviderConnection, len(cfg.ProviderConnections))
	for i, conn := range cfg.ProviderConnections {
		connections[i] = conn
		if maskTokens {
			connections[i].AccessToken = maskString(conn.AccessToken, 4, 4)
			connections[i].RefreshToken = maskString(conn.RefreshToken, 4, 4)
			connections[i].APIKey = maskString(conn.APIKey, 4, 4)
		}
	}

	apiKeys := cfg.APIKeys
	if maskTokens {
		apiKeys = make([]domain.APIKey, len(cfg.APIKeys))
		for i, k := range cfg.APIKeys {
			apiKeys[i] = k
			apiKeys[i].Key = maskString(k.Key, 10, 4)
		}
	}

	backup := BackupData{
		Version:             "1.0",
		ExportedAt:          time.Now().UTC().Format(time.RFC3339),
		ProviderConnections: connections,
		Combos:              cfg.Combos,
		ModelAliases:        cfg.ModelAliases,
		APIKeys:             apiKeys,
		Settings:            cfg.Settings,
	}

	data, err := json.MarshalIndent(backup, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal backup: %w", err)
	}

	filename := "dntproxy-backup.json"
	if len(args) > 0 {
		filename = args[0]
	}

	if err := os.WriteFile(filename, data, 0644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	fmt.Printf("Backup exported to %s\n", filename)
	return nil
}

func runImport(cmd *cobra.Command, args []string) error {
	filename := args[0]

	data, err := os.ReadFile(filename)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	var backup BackupData
	if err := json.Unmarshal(data, &backup); err != nil {
		return fmt.Errorf("failed to parse backup file: %w", err)
	}

	store, err := getStore()
	if err != nil {
		return fmt.Errorf("failed to open store: %w", err)
	}

	cfg, err := store.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	mode, _ := cmd.Flags().GetString("mode")

	imported := 0
	skipped := 0

	if mode == "replace" {
		cfg.ProviderConnections = nil
		cfg.Combos = nil
		cfg.ModelAliases = nil
		cfg.APIKeys = nil
	}

	// Import connections
	for _, conn := range backup.ProviderConnections {
		if conn.ID == "" {
			skipped++
			continue
		}

		if mode == "merge" {
			found := false
			for i, existing := range cfg.ProviderConnections {
				if existing.ID == conn.ID {
					cfg.ProviderConnections[i] = conn
					imported++
					found = true
					break
				}
			}
			if !found {
				cfg.ProviderConnections = append(cfg.ProviderConnections, conn)
				imported++
			}
		} else {
			cfg.ProviderConnections = append(cfg.ProviderConnections, conn)
			imported++
		}
	}

	// Import combos
	for _, combo := range backup.Combos {
		if combo.ID == "" || combo.Name == "" {
			skipped++
			continue
		}

		found := false
		for i, existing := range cfg.Combos {
			if existing.ID == combo.ID || existing.Name == combo.Name {
				cfg.Combos[i] = combo
				imported++
				found = true
				break
			}
		}
		if !found {
			cfg.Combos = append(cfg.Combos, combo)
			imported++
		}
	}

	// Import aliases
	if cfg.ModelAliases == nil {
		cfg.ModelAliases = make(domain.AliasMap)
	}
	for alias, model := range backup.ModelAliases {
		cfg.ModelAliases[alias] = model
		imported++
	}

	// Import API keys (skip masked ones)
	for _, k := range backup.APIKeys {
		if k.ID == "" || k.Key == "" {
			skipped++
			continue
		}

		found := false
		for i, existing := range cfg.APIKeys {
			if existing.ID == k.ID {
				cfg.APIKeys[i] = k
				imported++
				found = true
				break
			}
		}
		if !found {
			cfg.APIKeys = append(cfg.APIKeys, k)
			imported++
		}
	}

	// Import settings
	if backup.Settings.Port > 0 {
		cfg.Settings.Port = backup.Settings.Port
	}
	if backup.Settings.ComboStrategy != "" {
		cfg.Settings.ComboStrategy = backup.Settings.ComboStrategy
	}
	if backup.Settings.RequireAPIKey {
		cfg.Settings.RequireAPIKey = true
	}
	if backup.Settings.StickyRoundRobinLimit > 0 {
		cfg.Settings.StickyRoundRobinLimit = backup.Settings.StickyRoundRobinLimit
	}

	if err := store.Save(cfg); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	fmt.Printf("Backup imported: %d items imported, %d skipped (mode=%s)\n", imported, skipped, mode)
	return nil
}

func maskString(s string, first, last int) string {
	if len(s) <= first+last {
		return "***"
	}
	return s[:first] + "..." + s[len(s)-last:]
}
