package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/dungnt/dntproxy/internal/domain"
	"github.com/dungnt/dntproxy/internal/service"
	"github.com/spf13/cobra"
)

func buildProfileCmd() *cobra.Command {
	profileCmd := &cobra.Command{
		Use:   "profile",
		Short: "Manage model routing profiles",
		Long:  "Profiles group model aliases for quick provider switching. Activate a profile to route CLI tools (e.g. Claude CLI) through different providers.",
	}

	// --- create ---
	createCmd := &cobra.Command{
		Use:   "create [name]",
		Short: "Create a new profile",
		Long:  "Create a profile with model alias mappings.\n\nExample:\n  dntproxy profile create my-profile --alias claude-sonnet=kr/claude-sonnet-4.5 --alias claude-haiku=kr/claude-haiku-4.5",
		Args:  cobra.ExactArgs(1),
		RunE:  runProfileCreate,
	}
	createCmd.Flags().StringP("desc", "d", "", "Profile description")
	createCmd.Flags().StringArrayP("alias", "a", nil, "Alias mapping in format alias=provider/model (repeatable)")
	profileCmd.AddCommand(createCmd)

	// --- create-from-preset ---
	presetCmd := &cobra.Command{
		Use:   "create-from-preset [preset-name]",
		Short: "Create a profile from a built-in preset",
		Long:  fmt.Sprintf("Create a profile from a preset. Available presets: %s", strings.Join(domain.ListPresetNames(), ", ")),
		Args:  cobra.ExactArgs(1),
		RunE:  runProfileCreateFromPreset,
	}
	profileCmd.AddCommand(presetCmd)

	// --- list ---
	profileCmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List all profiles",
		RunE:  runProfileList,
	})

	// --- show ---
	profileCmd.AddCommand(&cobra.Command{
		Use:   "show [name]",
		Short: "Show profile details",
		Args:  cobra.ExactArgs(1),
		RunE:  runProfileShow,
	})

	// --- activate ---
	profileCmd.AddCommand(&cobra.Command{
		Use:   "activate [name]",
		Short: "Activate a profile (merges its aliases into the global alias map)",
		Args:  cobra.ExactArgs(1),
		RunE:  runProfileActivate,
	})

	// --- deactivate ---
	profileCmd.AddCommand(&cobra.Command{
		Use:   "deactivate",
		Short: "Deactivate the current profile",
		RunE:  runProfileDeactivate,
	})

	// --- edit ---
	editCmd := &cobra.Command{
		Use:   "edit [name]",
		Short: "Edit a profile's aliases",
		Long:  "Add or remove aliases from a profile.\n\nExample:\n  dntproxy profile edit my-profile --alias claude-sonnet=oai/gpt-5.4 --remove claude-haiku",
		Args:  cobra.ExactArgs(1),
		RunE:  runProfileEdit,
	}
	editCmd.Flags().StringArrayP("alias", "a", nil, "Add/update alias: alias=provider/model (repeatable)")
	editCmd.Flags().StringArrayP("remove", "r", nil, "Remove alias by name (repeatable)")
	profileCmd.AddCommand(editCmd)

	// --- delete ---
	profileCmd.AddCommand(&cobra.Command{
		Use:   "delete [name]",
		Short: "Delete a profile",
		Args:  cobra.ExactArgs(1),
		RunE:  runProfileDelete,
	})

	// --- presets ---
	profileCmd.AddCommand(&cobra.Command{
		Use:   "presets",
		Short: "List available built-in presets",
		RunE:  runProfilePresets,
	})

	// --- export ---
	exportCmd := &cobra.Command{
		Use:   "export [name]",
		Short: "Export a profile to JSON",
		Args:  cobra.ExactArgs(1),
		RunE:  runProfileExport,
	}
	exportCmd.Flags().StringP("output", "o", "", "Output file path (default: stdout)")
	profileCmd.AddCommand(exportCmd)

	// --- import ---
	importCmd := &cobra.Command{
		Use:   "import [file.json]",
		Short: "Import a profile from JSON",
		Args:  cobra.ExactArgs(1),
		RunE:  runProfileImport,
	}
	profileCmd.AddCommand(importCmd)

	return profileCmd
}

func runProfileCreate(cmd *cobra.Command, args []string) error {
	name := args[0]
	desc, _ := cmd.Flags().GetString("desc")
	aliasFlags, _ := cmd.Flags().GetStringArray("alias")

	if len(aliasFlags) == 0 {
		return fmt.Errorf("at least one --alias is required. Example: --alias claude-sonnet=kr/claude-sonnet-4.5")
	}

	aliases, err := parseAliasFlags(aliasFlags)
	if err != nil {
		return err
	}

	store, err := getStore()
	if err != nil {
		return err
	}

	svc := service.NewProfileService(store)
	profile, err := svc.CreateProfile(name, desc, aliases)
	if err != nil {
		return err
	}

	fmt.Printf("Profile '%s' created with %d aliases:\n", profile.Name, len(profile.Aliases))
	printAliases(profile.Aliases)
	fmt.Println("\nRun 'dntproxy profile activate " + name + "' to activate it.")
	return nil
}

func runProfileCreateFromPreset(cmd *cobra.Command, args []string) error {
	presetName := args[0]

	store, err := getStore()
	if err != nil {
		return err
	}

	svc := service.NewProfileService(store)
	profile, err := svc.CreateFromPreset(presetName)
	if err != nil {
		return err
	}

	fmt.Printf("Profile '%s' created from preset:\n", profile.Name)
	if profile.Description != "" {
		fmt.Printf("  %s\n", profile.Description)
	}
	fmt.Println()
	printAliases(profile.Aliases)
	fmt.Println("\nRun 'dntproxy profile activate " + profile.Name + "' to activate it.")
	return nil
}

func runProfileList(cmd *cobra.Command, args []string) error {
	store, err := getStore()
	if err != nil {
		return err
	}

	svc := service.NewProfileService(store)
	profiles, activeProfile, err := svc.ListProfiles()
	if err != nil {
		return err
	}

	if len(profiles) == 0 {
		fmt.Println("No profiles configured.")
		fmt.Println("  Create one:        dntproxy profile create <name> --alias <alias>=<model>")
		fmt.Println("  Or from preset:    dntproxy profile create-from-preset <preset>")
		fmt.Printf("  Available presets: %s\n", strings.Join(domain.ListPresetNames(), ", "))
		return nil
	}

	for _, p := range profiles {
		marker := "  "
		if p.Name == activeProfile {
			marker = "★ "
		}
		desc := ""
		if p.Description != "" {
			desc = fmt.Sprintf(" — %s", p.Description)
		}
		fmt.Printf("%s%-20s (%d aliases)%s\n", marker, p.Name, len(p.Aliases), desc)
	}
	return nil
}

func runProfileShow(cmd *cobra.Command, args []string) error {
	name := args[0]

	store, err := getStore()
	if err != nil {
		return err
	}

	svc := service.NewProfileService(store)
	profile, err := svc.GetProfile(name)
	if err != nil {
		return err
	}

	cfg, _ := store.Load()
	isActive := cfg != nil && cfg.Settings.ActiveProfile == name

	status := "inactive"
	if isActive {
		status = "★ active"
	}

	fmt.Printf("Profile: %s  [%s]\n", profile.Name, status)
	if profile.Description != "" {
		fmt.Printf("  %s\n", profile.Description)
	}
	fmt.Println()
	fmt.Println("Aliases:")
	printAliases(profile.Aliases)

	if len(profile.Combos) > 0 {
		fmt.Println("\nEmbedded Combos:")
		for _, combo := range profile.Combos {
			fmt.Printf("  %s → %s\n", combo.Name, strings.Join(combo.Models, " → "))
		}
	}
	return nil
}

func runProfileActivate(cmd *cobra.Command, args []string) error {
	name := args[0]

	store, err := getStore()
	if err != nil {
		return err
	}

	svc := service.NewProfileService(store)
	if err := svc.ActivateProfile(name); err != nil {
		return err
	}

	profile, _ := svc.GetProfile(name)
	fmt.Printf("★ Profile '%s' activated.\n", name)
	if profile != nil {
		fmt.Println("Active aliases:")
		printAliases(profile.Aliases)
	}
	return nil
}

func runProfileDeactivate(cmd *cobra.Command, args []string) error {
	store, err := getStore()
	if err != nil {
		return err
	}

	cfg, err := store.Load()
	if err != nil {
		return err
	}

	if cfg.Settings.ActiveProfile == "" {
		fmt.Println("No profile is currently active.")
		return nil
	}

	activeName := cfg.Settings.ActiveProfile
	svc := service.NewProfileService(store)
	if err := svc.DeactivateProfile(); err != nil {
		return err
	}

	fmt.Printf("Profile '%s' deactivated. Aliases removed.\n", activeName)
	return nil
}

func runProfileEdit(cmd *cobra.Command, args []string) error {
	name := args[0]
	aliasFlags, _ := cmd.Flags().GetStringArray("alias")
	removeFlags, _ := cmd.Flags().GetStringArray("remove")

	if len(aliasFlags) == 0 && len(removeFlags) == 0 {
		return fmt.Errorf("provide --alias or --remove flags")
	}

	addAliases := make(domain.AliasMap)
	if len(aliasFlags) > 0 {
		var err error
		addAliases, err = parseAliasFlags(aliasFlags)
		if err != nil {
			return err
		}
	}

	store, err := getStore()
	if err != nil {
		return err
	}

	svc := service.NewProfileService(store)
	if err := svc.UpdateProfileAliases(name, addAliases, removeFlags); err != nil {
		return err
	}

	fmt.Printf("Profile '%s' updated.\n", name)
	if len(addAliases) > 0 {
		fmt.Println("Added/updated:")
		printAliases(addAliases)
	}
	if len(removeFlags) > 0 {
		fmt.Printf("Removed: %s\n", strings.Join(removeFlags, ", "))
	}
	return nil
}

func runProfileDelete(cmd *cobra.Command, args []string) error {
	name := args[0]

	store, err := getStore()
	if err != nil {
		return err
	}

	svc := service.NewProfileService(store)
	if err := svc.DeleteProfile(name); err != nil {
		return err
	}

	fmt.Printf("Profile '%s' deleted.\n", name)
	return nil
}

func runProfilePresets(cmd *cobra.Command, args []string) error {
	fmt.Println("Available presets:")
	fmt.Println()
	for _, name := range domain.ListPresetNames() {
		preset := domain.BuiltinPresets[name]
		fmt.Printf("  %-20s %s\n", name, preset.Description)
		for alias, model := range preset.Aliases {
			fmt.Printf("    %-30s → %s\n", alias, model)
		}
		fmt.Println()
	}
	fmt.Println("Create from preset: dntproxy profile create-from-preset <name>")
	return nil
}

func runProfileExport(cmd *cobra.Command, args []string) error {
	name := args[0]
	outputPath, _ := cmd.Flags().GetString("output")

	store, err := getStore()
	if err != nil {
		return err
	}

	svc := service.NewProfileService(store)
	profile, err := svc.GetProfile(name)
	if err != nil {
		return err
	}

	data, err := json.MarshalIndent(profile, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}

	if outputPath != "" {
		if err := os.WriteFile(outputPath, data, 0644); err != nil {
			return fmt.Errorf("write file: %w", err)
		}
		fmt.Printf("Profile '%s' exported to %s\n", name, outputPath)
	} else {
		fmt.Println(string(data))
	}
	return nil
}

func runProfileImport(cmd *cobra.Command, args []string) error {
	filePath := args[0]

	data, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("read file: %w", err)
	}

	var profile domain.Profile
	if err := json.Unmarshal(data, &profile); err != nil {
		return fmt.Errorf("parse JSON: %w", err)
	}

	if profile.Name == "" {
		return fmt.Errorf("profile must have a name")
	}
	if len(profile.Aliases) == 0 {
		return fmt.Errorf("profile must have at least one alias")
	}

	store, err := getStore()
	if err != nil {
		return err
	}

	svc := service.NewProfileService(store)
	created, err := svc.CreateProfile(profile.Name, profile.Description, profile.Aliases)
	if err != nil {
		return err
	}

	fmt.Printf("Profile '%s' imported with %d aliases:\n", created.Name, len(created.Aliases))
	printAliases(created.Aliases)
	return nil
}

// --- helpers ---

func parseAliasFlags(flags []string) (domain.AliasMap, error) {
	aliases := make(domain.AliasMap)
	for _, f := range flags {
		parts := strings.SplitN(f, "=", 2)
		if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
			return nil, fmt.Errorf("invalid alias format: %q (expected alias=provider/model)", f)
		}
		aliases[parts[0]] = parts[1]
	}
	return aliases, nil
}

func printAliases(aliases domain.AliasMap) {
	// Sort keys for consistent output
	keys := make([]string, 0, len(aliases))
	for k := range aliases {
		keys = append(keys, k)
	}
	for i := 0; i < len(keys); i++ {
		for j := i + 1; j < len(keys); j++ {
			if keys[j] < keys[i] {
				keys[i], keys[j] = keys[j], keys[i]
			}
		}
	}
	for _, k := range keys {
		fmt.Printf("  %-35s → %s\n", k, aliases[k])
	}
}
