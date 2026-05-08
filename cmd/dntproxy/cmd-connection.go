package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/dungnt/dntproxy/internal/service/backup"
	"github.com/spf13/cobra"
)

func buildConnectionCmd() *cobra.Command {
	connCmd := &cobra.Command{
		Use:     "connection",
		Short:   "Manage provider connections",
		Aliases: []string{"conn"},
	}

	exportCmd := &cobra.Command{
		Use:   "export <connection-id> [file]",
		Short: "Export a single connection to JSON file",
		Args:  cobra.RangeArgs(1, 2),
		RunE:  runConnectionExport,
	}

	exportMultipleCmd := &cobra.Command{
		Use:   "export-multiple [file]",
		Short: "Export multiple connections to JSON file",
		Long:  "Export multiple connections. Use --ids flag to specify connection IDs, or export all if not specified.",
		Args:  cobra.MaximumNArgs(1),
		RunE:  runConnectionExportMultiple,
	}
	exportMultipleCmd.Flags().StringSlice("ids", []string{}, "Connection IDs to export (comma-separated)")

	importCmd := &cobra.Command{
		Use:   "import <file>",
		Short: "Import connection(s) from JSON file",
		Long:  "Import connection(s) from a JSON file. Supports both single connection and multiple connections format.",
		Args:  cobra.ExactArgs(1),
		RunE:  runConnectionImport,
	}
	importCmd.Flags().String("mode", "add", "Import mode: add (fail if exists), replace (update if exists), merge (skip if exists)")

	connCmd.AddCommand(exportCmd)
	connCmd.AddCommand(exportMultipleCmd)
	connCmd.AddCommand(importCmd)

	return connCmd
}

func runConnectionExport(cmd *cobra.Command, args []string) error {
	connectionID := args[0]

	store, err := getStore()
	if err != nil {
		return fmt.Errorf("failed to open store: %w", err)
	}

	data, err := backup.ExportConnection(store, connectionID)
	if err != nil {
		return fmt.Errorf("failed to export connection: %w", err)
	}

	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal connection: %w", err)
	}

	filename := fmt.Sprintf("dntproxy-connection-%s.json", data.Connection.Name)
	if len(args) > 1 {
		filename = args[1]
	}

	if err := os.WriteFile(filename, jsonData, 0644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	fmt.Printf("Connection exported to %s\n", filename)
	fmt.Printf("  ID: %s\n", data.Connection.ID)
	fmt.Printf("  Name: %s\n", data.Connection.Name)
	fmt.Printf("  Provider: %s\n", data.Connection.Provider)
	return nil
}

func runConnectionExportMultiple(cmd *cobra.Command, args []string) error {
	ids, _ := cmd.Flags().GetStringSlice("ids")

	store, err := getStore()
	if err != nil {
		return fmt.Errorf("failed to open store: %w", err)
	}

	data, err := backup.ExportConnections(store, ids)
	if err != nil {
		return fmt.Errorf("failed to export connections: %w", err)
	}

	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal connections: %w", err)
	}

	filename := "dntproxy-connections.json"
	if len(args) > 0 {
		filename = args[0]
	}

	if err := os.WriteFile(filename, jsonData, 0644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	fmt.Printf("Exported %d connection(s) to %s\n", len(data.ProviderConnections), filename)
	for _, conn := range data.ProviderConnections {
		fmt.Printf("  - %s (%s)\n", conn.Name, conn.Provider)
	}
	return nil
}

func runConnectionImport(cmd *cobra.Command, args []string) error {
	filename := args[0]
	modeStr, _ := cmd.Flags().GetString("mode")

	rawData, err := os.ReadFile(filename)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	store, err := getStore()
	if err != nil {
		return fmt.Errorf("failed to open store: %w", err)
	}

	mode := backup.ImportConnectionMode(modeStr)
	if mode != backup.ImportModeAdd && mode != backup.ImportModeReplace && mode != backup.ImportModeMerge {
		return fmt.Errorf("invalid mode: %s (must be add, replace, or merge)", modeStr)
	}

	// Try to parse as single connection first
	var singleConn backup.ConnectionExportData
	if err := json.Unmarshal(rawData, &singleConn); err == nil && singleConn.Connection.ID != "" {
		result, err := backup.ImportConnection(store, &singleConn, mode)
		if err != nil {
			return fmt.Errorf("import failed: %w", err)
		}

		fmt.Printf("Connection imported (mode: %s)\n", mode)
		fmt.Printf("  Imported: %d\n", result.Imported)
		fmt.Printf("  Updated: %d\n", result.Updated)
		fmt.Printf("  Skipped: %d\n", result.Skipped)
		if len(result.Errors) > 0 {
			fmt.Printf("  Errors:\n")
			for _, e := range result.Errors {
				fmt.Printf("    - %s\n", e)
			}
		}
		return nil
	}

	// Try to parse as multiple connections
	var multipleConns backup.BackupData
	if err := json.Unmarshal(rawData, &multipleConns); err != nil {
		return fmt.Errorf("failed to parse file (not a valid connection export): %w", err)
	}

	result, err := backup.ImportConnections(store, &multipleConns, mode)
	if err != nil {
		return fmt.Errorf("import failed: %w", err)
	}

	fmt.Printf("Connections imported (mode: %s)\n", mode)
	fmt.Printf("  Imported: %d\n", result.Imported)
	fmt.Printf("  Updated: %d\n", result.Updated)
	fmt.Printf("  Skipped: %d\n", result.Skipped)
	if len(result.Errors) > 0 {
		fmt.Printf("  Errors:\n")
		for _, e := range result.Errors {
			fmt.Printf("    - %s\n", e)
		}
	}
	return nil
}
