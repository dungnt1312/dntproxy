package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

func buildAliasCmd() *cobra.Command {
	aliasCmd := &cobra.Command{
		Use:   "alias",
		Short: "Manage model aliases",
	}

	aliasCmd.AddCommand(&cobra.Command{
		Use:   "set [alias] [provider/model]",
		Short: "Set a model alias",
		Args:  cobra.ExactArgs(2),
		RunE:  runAliasSet,
	})

	aliasCmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List aliases",
		RunE:  runAliasList,
	})

	aliasCmd.AddCommand(&cobra.Command{
		Use:   "remove [alias]",
		Short: "Remove an alias",
		Args:  cobra.ExactArgs(1),
		RunE:  runAliasRemove,
	})

	return aliasCmd
}

func runAliasSet(cmd *cobra.Command, args []string) error {
	alias := args[0]
	model := args[1]

	store, err := getStore()
	if err != nil {
		return err
	}

	cfg, err := store.Load()
	if err != nil {
		return err
	}

	if cfg.ModelAliases == nil {
		cfg.ModelAliases = make(map[string]string)
	}

	cfg.ModelAliases[alias] = model
	if err := store.Save(cfg); err != nil {
		return fmt.Errorf("save: %w", err)
	}

	fmt.Printf("Alias set: %s → %s\n", alias, model)
	return nil
}

func runAliasList(cmd *cobra.Command, args []string) error {
	store, err := getStore()
	if err != nil {
		return err
	}

	cfg, err := store.Load()
	if err != nil {
		return err
	}

	if len(cfg.ModelAliases) == 0 {
		fmt.Println("No aliases configured. Run 'dntproxy alias set <alias> <provider/model>' to create one.")
		return nil
	}

	for alias, model := range cfg.ModelAliases {
		fmt.Printf("  %s → %s\n", alias, model)
	}
	return nil
}

func runAliasRemove(cmd *cobra.Command, args []string) error {
	alias := args[0]

	store, err := getStore()
	if err != nil {
		return err
	}

	cfg, err := store.Load()
	if err != nil {
		return err
	}

	if _, ok := cfg.ModelAliases[alias]; !ok {
		return fmt.Errorf("alias not found: %s", alias)
	}

	delete(cfg.ModelAliases, alias)
	if err := store.Save(cfg); err != nil {
		return fmt.Errorf("save: %w", err)
	}

	fmt.Printf("Alias '%s' removed.\n", alias)
	return nil
}
