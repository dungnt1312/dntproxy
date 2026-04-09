package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/dungnt/dntproxy/internal/domain"
	"github.com/google/uuid"
	"github.com/spf13/cobra"
)

func buildComboCmd() *cobra.Command {
	comboCmd := &cobra.Command{
		Use:   "combo",
		Short: "Manage model combos",
	}

	addCmd := &cobra.Command{
		Use:   "add [name] [model1] [model2] ...",
		Short: "Create a combo",
		Args:  cobra.MinimumNArgs(2),
		RunE:  runComboAdd,
	}
	comboCmd.AddCommand(addCmd)

	comboCmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List combos",
		RunE:  runComboList,
	})

	comboCmd.AddCommand(&cobra.Command{
		Use:   "remove [name]",
		Short: "Remove a combo",
		Args:  cobra.ExactArgs(1),
		RunE:  runComboRemove,
	})

	return comboCmd
}

func runComboAdd(cmd *cobra.Command, args []string) error {
	name := args[0]
	models := args[1:]

	store, err := getStore()
	if err != nil {
		return err
	}

	cfg, err := store.Load()
	if err != nil {
		return err
	}

	// Check for duplicate name
	for _, c := range cfg.Combos {
		if c.Name == name {
			return fmt.Errorf("combo '%s' already exists", name)
		}
	}

	now := time.Now().UTC().Format(time.RFC3339)
	combo := domain.Combo{
		ID:        uuid.New().String(),
		Name:      name,
		Models:    models,
		CreatedAt: now,
		UpdatedAt: now,
	}

	cfg.Combos = append(cfg.Combos, combo)
	if err := store.Save(cfg); err != nil {
		return fmt.Errorf("save: %w", err)
	}

	fmt.Printf("Combo '%s' created with %d models:\n", name, len(models))
	for i, m := range models {
		fmt.Printf("  %d. %s\n", i+1, m)
	}
	return nil
}

func runComboList(cmd *cobra.Command, args []string) error {
	store, err := getStore()
	if err != nil {
		return err
	}

	cfg, err := store.Load()
	if err != nil {
		return err
	}

	if len(cfg.Combos) == 0 {
		fmt.Println("No combos configured. Run 'dntproxy combo add <name> <model1> <model2>' to create one.")
		return nil
	}

	for _, c := range cfg.Combos {
		fmt.Printf("%s  (%d models)\n", c.Name, len(c.Models))
		for i, m := range c.Models {
			fmt.Printf("  %d. %s\n", i+1, m)
		}
	}
	return nil
}

func runComboRemove(cmd *cobra.Command, args []string) error {
	name := args[0]

	store, err := getStore()
	if err != nil {
		return err
	}

	cfg, err := store.Load()
	if err != nil {
		return err
	}

	found := false
	for i, c := range cfg.Combos {
		if c.Name == name || strings.HasPrefix(c.ID, name) {
			fmt.Printf("Removing combo: %s\n", c.Name)
			cfg.Combos = append(cfg.Combos[:i], cfg.Combos[i+1:]...)
			found = true
			break
		}
	}

	if !found {
		return fmt.Errorf("combo not found: %s", name)
	}

	if err := store.Save(cfg); err != nil {
		return fmt.Errorf("save: %w", err)
	}

	fmt.Println("Combo removed.")
	return nil
}
