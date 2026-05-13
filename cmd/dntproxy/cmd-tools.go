package main

import (
	"fmt"
	"strings"

	"github.com/dungnt/dntproxy/internal/domain"
	"github.com/dungnt/dntproxy/internal/service"
	"github.com/spf13/cobra"
)

func buildToolsCmd() *cobra.Command {
	toolsCmd := &cobra.Command{
		Use:   "tools",
		Short: "Configure AI coding tools to use dntproxy",
		Long:  "Detect, configure, and manage AI coding tools (Claude Code, Cursor, Windsurf, OpenCode, Cline, Continue, Gemini CLI) to route through dntproxy.",
	}

	// --- list ---
	toolsCmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List supported tools and their status",
		RunE:  runToolsList,
	})

	// --- status ---
	toolsCmd.AddCommand(&cobra.Command{
		Use:   "status",
		Short: "Show which tools are configured to use dntproxy",
		RunE:  runToolsStatus,
	})

	// --- configure ---
	configureCmd := &cobra.Command{
		Use:   "configure [tool]",
		Short: "Configure a tool to use dntproxy as its backend",
		Long:  "Auto-configure a tool's config file to route requests through dntproxy.\nUse 'all' to configure all detected tools.",
		Args:  cobra.ExactArgs(1),
		RunE:  runToolsConfigure,
	}
	toolsCmd.AddCommand(configureCmd)

	// --- reset ---
	resetCmd := &cobra.Command{
		Use:   "reset [tool]",
		Short: "Revert a tool's configuration to direct provider access",
		Long:  "Restore the tool's original config from backup, or remove dntproxy fields.\nUse 'all' to reset all configured tools.",
		Args:  cobra.ExactArgs(1),
		RunE:  runToolsReset,
	}
	toolsCmd.AddCommand(resetCmd)

	return toolsCmd
}

func runToolsList(cmd *cobra.Command, args []string) error {
	store, err := getStore()
	if err != nil {
		return err
	}

	svc := service.NewToolsService(store)
	statuses, err := svc.ListTools()
	if err != nil {
		return err
	}

	fmt.Println("Supported AI coding tools:")
	fmt.Println()
	fmt.Printf("  %-15s %-20s %-12s %-12s\n", "ID", "Name", "Installed", "Configured")
	fmt.Printf("  %-15s %-20s %-12s %-12s\n", "──────────────", "───────────────────", "──────────", "──────────")

	for _, s := range statuses {
		installed := "✗"
		if s.Installed {
			installed = "✓"
		}
		configured := "—"
		if s.Configured {
			configured = "★ yes"
		} else if s.Installed {
			configured = "no"
		}
		fmt.Printf("  %-15s %-20s %-12s %-12s\n", s.ID, s.Name, installed, configured)
	}

	fmt.Println()
	fmt.Println("Configure a tool:  dntproxy tools configure <tool-id>")
	fmt.Println("Configure all:     dntproxy tools configure all")
	return nil
}

func runToolsStatus(cmd *cobra.Command, args []string) error {
	store, err := getStore()
	if err != nil {
		return err
	}

	svc := service.NewToolsService(store)
	statuses, err := svc.ListTools()
	if err != nil {
		return err
	}

	configured := 0
	for _, s := range statuses {
		if s.Configured {
			configured++
		}
	}

	if configured == 0 {
		fmt.Println("No tools are currently configured to use dntproxy.")
		fmt.Println("Run 'dntproxy tools configure <tool>' to set one up.")
		return nil
	}

	fmt.Printf("%d tool(s) configured:\n\n", configured)
	for _, s := range statuses {
		if !s.Configured {
			continue
		}
		backup := ""
		if s.BackupExists {
			backup = " (backup saved)"
		}
		fmt.Printf("  ★ %-15s → %s%s\n", s.Name, s.ProxyURL, backup)
		if s.ConfigPath != "" {
			fmt.Printf("    config: %s\n", s.ConfigPath)
		}
	}
	return nil
}

func runToolsConfigure(cmd *cobra.Command, args []string) error {
	toolArg := strings.TrimSpace(args[0])

	store, err := getStore()
	if err != nil {
		return err
	}
	svc := service.NewToolsService(store)

	if toolArg == "all" {
		return configureAllTools(svc)
	}

	id := domain.ToolID(toolArg)
	def := domain.GetToolDefinition(id)
	if def == nil {
		return fmt.Errorf("unknown tool: %q. Run 'dntproxy tools list' to see supported tools", toolArg)
	}

	if err := svc.Configure(id); err != nil {
		return fmt.Errorf("configure %s: %w", def.Name, err)
	}

	fmt.Printf("★ %s configured to use dntproxy.\n", def.Name)

	status, _ := svc.GetStatus(id)
	if status != nil && status.ConfigPath != "" {
		fmt.Printf("  config: %s\n", status.ConfigPath)
	}
	if status != nil && status.ProxyURL != "" {
		fmt.Printf("  proxy:  %s\n", status.ProxyURL)
	}
	fmt.Printf("\nReset with: dntproxy tools reset %s\n", id)
	return nil
}

func runToolsReset(cmd *cobra.Command, args []string) error {
	toolArg := strings.TrimSpace(args[0])

	store, err := getStore()
	if err != nil {
		return err
	}
	svc := service.NewToolsService(store)

	if toolArg == "all" {
		return resetAllTools(svc)
	}

	id := domain.ToolID(toolArg)
	def := domain.GetToolDefinition(id)
	if def == nil {
		return fmt.Errorf("unknown tool: %q. Run 'dntproxy tools list' to see supported tools", toolArg)
	}

	if err := svc.Reset(id); err != nil {
		return fmt.Errorf("reset %s: %w", def.Name, err)
	}

	fmt.Printf("%s configuration reset to defaults.\n", def.Name)
	return nil
}

// configureAllTools configures all detected (installed) tools.
func configureAllTools(svc *service.ToolsService) error {
	statuses, err := svc.ListTools()
	if err != nil {
		return err
	}

	count := 0
	for _, s := range statuses {
		if !s.Installed {
			continue
		}
		if err := svc.Configure(s.ID); err != nil {
			fmt.Printf("  ✗ %s: %v\n", s.Name, err)
			continue
		}
		fmt.Printf("  ★ %s configured\n", s.Name)
		count++
	}

	if count == 0 {
		fmt.Println("No installed tools detected. Run 'dntproxy tools list' to see supported tools.")
	} else {
		fmt.Printf("\n%d tool(s) configured.\n", count)
	}
	return nil
}

// resetAllTools resets all configured tools.
func resetAllTools(svc *service.ToolsService) error {
	statuses, err := svc.ListTools()
	if err != nil {
		return err
	}

	count := 0
	for _, s := range statuses {
		if !s.Configured {
			continue
		}
		if err := svc.Reset(s.ID); err != nil {
			fmt.Printf("  ✗ %s: %v\n", s.Name, err)
			continue
		}
		fmt.Printf("  ✓ %s reset\n", s.Name)
		count++
	}

	if count == 0 {
		fmt.Println("No tools are currently configured.")
	} else {
		fmt.Printf("\n%d tool(s) reset.\n", count)
	}
	return nil
}
