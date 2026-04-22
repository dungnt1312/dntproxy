package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/dungnt/dntproxy/internal/service/backup"
	"github.com/spf13/cobra"
)

func buildBackupCmd() *cobra.Command {
	backupCmd := &cobra.Command{
		Use:   "backup",
		Short: "Backup and restore dntproxy configuration",
	}

	exportCmd := &cobra.Command{
		Use:   "export [file]",
		Short: "Export all dntproxy configuration to a JSON file",
		Args:  cobra.MaximumNArgs(1),
		RunE:  runExport,
	}

	importCmd := &cobra.Command{
		Use:   "import <file>",
		Short: "Import dntproxy configuration (replaces all existing data)",
		Args:  cobra.ExactArgs(1),
		RunE:  runImport,
	}

	backupCmd.AddCommand(exportCmd)
	backupCmd.AddCommand(importCmd)

	return backupCmd
}

func runExport(cmd *cobra.Command, args []string) error {
	store, err := getStore()
	if err != nil {
		return fmt.Errorf("failed to open store: %w", err)
	}

	data, err := backup.Export(store)
	if err != nil {
		return fmt.Errorf("failed to export backup: %w", err)
	}

	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal backup: %w", err)
	}

	filename := "dntproxy-backup.json"
	if len(args) > 0 {
		filename = args[0]
	}

	if err := os.WriteFile(filename, jsonData, 0644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	fmt.Printf("Backup exported to %s\n", filename)
	return nil
}

func runImport(cmd *cobra.Command, args []string) error {
	filename := args[0]

	rawData, err := os.ReadFile(filename)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	var backupData backup.BackupData
	if err := json.Unmarshal(rawData, &backupData); err != nil {
		return fmt.Errorf("failed to parse backup file: %w", err)
	}

	store, err := getStore()
	if err != nil {
		return fmt.Errorf("failed to open store: %w", err)
	}

	result, err := backup.Import(store, &backupData)
	if err != nil {
		return fmt.Errorf("import failed: %w", err)
	}

	fmt.Printf("Backup imported: %d items (full override)\n", result.Imported)
	return nil
}
