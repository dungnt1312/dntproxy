package main

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/dungnt/dntproxy/internal/domain"
	"github.com/google/uuid"
	"github.com/spf13/cobra"
)

func buildKeyCmd() *cobra.Command {
	keyCmd := &cobra.Command{
		Use:   "key",
		Short: "Manage API keys",
	}

	keyCmd.AddCommand(&cobra.Command{
		Use:   "generate [name]",
		Short: "Generate a new API key",
		Args:  cobra.ExactArgs(1),
		RunE:  runKeyGenerate,
	})

	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List API keys",
		RunE:  runKeyList,
	}
	listCmd.Flags().Bool("show-keys", false, "Show full API keys (default: masked)")
	keyCmd.AddCommand(listCmd)

	keyCmd.AddCommand(&cobra.Command{
		Use:   "remove [id]",
		Short: "Remove an API key",
		Args:  cobra.ExactArgs(1),
		RunE:  runKeyRemove,
	})

	return keyCmd
}

func runKeyGenerate(cmd *cobra.Command, args []string) error {
	name := args[0]

	store, err := getStore()
	if err != nil {
		return err
	}

	cfg, err := store.Load()
	if err != nil {
		return err
	}

	// Generate random key
	keyBytes := make([]byte, 24)
	if _, err := rand.Read(keyBytes); err != nil {
		return fmt.Errorf("generate key: %w", err)
	}
	key := "sk-dnt-" + hex.EncodeToString(keyBytes)

	apiKey := domain.APIKey{
		ID:        uuid.New().String(),
		Name:      name,
		Key:       key,
		IsActive:  true,
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
	}

	cfg.APIKeys = append(cfg.APIKeys, apiKey)
	if err := store.Save(cfg); err != nil {
		return fmt.Errorf("save: %w", err)
	}

	fmt.Printf("API key generated:\n")
	fmt.Printf("  Name: %s\n", name)
	fmt.Printf("  Key:  %s\n", key)
	fmt.Println("\nSave this key — it won't be shown again.")
	return nil
}

func runKeyList(cmd *cobra.Command, args []string) error {
	store, err := getStore()
	if err != nil {
		return err
	}

	cfg, err := store.Load()
	if err != nil {
		return err
	}

	if len(cfg.APIKeys) == 0 {
		fmt.Println("No API keys. Run 'dntproxy key generate <name>' to create one.")
		return nil
	}

	showKeys, _ := cmd.Flags().GetBool("show-keys")

	if showKeys {
		fmt.Printf("%-36s  %-20s  %-60s  %-8s\n", "ID", "NAME", "KEY", "ACTIVE")
		for _, k := range cfg.APIKeys {
			active := "yes"
			if !k.IsActive {
				active = "no"
			}
			fmt.Printf("%-36s  %-20s  %-60s  %-8s\n", k.ID, k.Name, k.Key, active)
		}
	} else {
		fmt.Printf("%-36s  %-20s  %-20s  %-8s\n", "ID", "NAME", "KEY (masked)", "ACTIVE")
		for _, k := range cfg.APIKeys {
			masked := k.Key[:10] + "..." + k.Key[len(k.Key)-4:]
			active := "yes"
			if !k.IsActive {
				active = "no"
			}
			fmt.Printf("%-36s  %-20s  %-20s  %-8s\n", k.ID, k.Name, masked, active)
		}
		fmt.Println("\nTip: Use --show-keys to display full API keys")
	}
	return nil
}

func runKeyRemove(cmd *cobra.Command, args []string) error {
	id := args[0]

	store, err := getStore()
	if err != nil {
		return err
	}

	cfg, err := store.Load()
	if err != nil {
		return err
	}

	found := false
	for i, k := range cfg.APIKeys {
		if k.ID == id || k.Name == id {
			fmt.Printf("Removing API key: %s (%s)\n", k.Name, k.ID)
			cfg.APIKeys = append(cfg.APIKeys[:i], cfg.APIKeys[i+1:]...)
			found = true
			break
		}
	}

	if !found {
		return fmt.Errorf("API key not found: %s", id)
	}

	if err := store.Save(cfg); err != nil {
		return fmt.Errorf("save: %w", err)
	}

	fmt.Println("API key removed.")
	return nil
}
