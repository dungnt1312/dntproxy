package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/dungnt/dntproxy/internal/adapter/auth"
	"github.com/dungnt/dntproxy/internal/domain"
	"github.com/google/uuid"
	"github.com/spf13/cobra"
)

func buildAuthCmd() *cobra.Command {
	authCmd := &cobra.Command{
		Use:   "auth",
		Short: "Manage Kiro authentication",
	}

	authCmd.AddCommand(&cobra.Command{
		Use:   "add",
		Short: "Add a new Kiro connection (interactive)",
		RunE:  runAuthAdd,
	})

	authCmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List saved connections",
		RunE:  runAuthList,
	})

	removeCmd := &cobra.Command{
		Use:   "remove [id]",
		Short: "Remove a connection",
		Args:  cobra.ExactArgs(1),
		RunE:  runAuthRemove,
	}
	authCmd.AddCommand(removeCmd)

	authCmd.AddCommand(&cobra.Command{
		Use:   "test [id]",
		Short: "Test a connection",
		Args:  cobra.ExactArgs(1),
		RunE:  runAuthTest,
	})

	return authCmd
}

func runAuthAdd(cmd *cobra.Command, args []string) error {
	fmt.Println("Choose authentication method:")
	fmt.Println("\nKiro (AWS CodeWhisperer):")
	fmt.Println("  1. AWS Builder ID (device code)")
	fmt.Println("  2. AWS IAM Identity Center / IDC (device code)")
	fmt.Println("  3. Google social login")
	fmt.Println("  4. GitHub social login")
	fmt.Println("  5. Import refresh token")
	fmt.Println("\nOpenAI:")
	fmt.Println("  6. OpenAI OAuth (Authorization Code + PKCE)")
	fmt.Print("\nChoice [1-6]: ")

	reader := bufio.NewReader(os.Stdin)
	choice, _ := reader.ReadString('\n')
	choice = strings.TrimSpace(choice)

	switch choice {
	case "1":
		return runAuthBuilderID()
	case "2":
		return runAuthIDC(reader)
	case "3":
		return runAuthSocial("google")
	case "4":
		return runAuthSocial("github")
	case "5":
		return runAuthImport(reader)
	case "6":
		return runAuthOpenAI()
	default:
		return fmt.Errorf("invalid choice: %s", choice)
	}
}

func runAuthBuilderID() error {
	fmt.Println("\n[Builder ID] Registering client...")

	client, deviceAuth, err := auth.StartBuilderIDDeviceAuth()
	if err != nil {
		return fmt.Errorf("start device auth: %w", err)
	}

	fmt.Printf("\nOpen this URL in your browser:\n  %s\n", deviceAuth.VerificationURIComplete)
	fmt.Printf("\nOr go to %s and enter code: %s\n", deviceAuth.VerificationURI, deviceAuth.UserCode)
	fmt.Println("\nWaiting for authorization...")

	return pollAndSave(client.ClientID, client.ClientSecret, deviceAuth.DeviceCode, "us-east-1", "builder-id", deviceAuth.Interval)
}

func runAuthIDC(reader *bufio.Reader) error {
	fmt.Print("\nEnter your SSO start URL (e.g. https://mycompany.awsapps.com/start): ")
	startURL, _ := reader.ReadString('\n')
	startURL = strings.TrimSpace(startURL)
	if startURL == "" {
		return fmt.Errorf("start URL is required")
	}

	fmt.Print("Enter region (default: us-east-1): ")
	region, _ := reader.ReadString('\n')
	region = strings.TrimSpace(region)
	if region == "" {
		region = "us-east-1"
	}

	fmt.Println("\n[IDC] Registering client...")

	client, deviceAuth, err := auth.StartIDCDeviceAuth(startURL, region)
	if err != nil {
		return fmt.Errorf("start IDC device auth: %w", err)
	}

	fmt.Printf("\nOpen this URL in your browser:\n  %s\n", deviceAuth.VerificationURIComplete)
	fmt.Printf("\nOr go to %s and enter code: %s\n", deviceAuth.VerificationURI, deviceAuth.UserCode)
	fmt.Println("\nWaiting for authorization...")

	return pollAndSave(client.ClientID, client.ClientSecret, deviceAuth.DeviceCode, region, "idc", deviceAuth.Interval)
}

func pollAndSave(clientID, clientSecret, deviceCode, region, authMethod string, interval int) error {
	if interval < 1 {
		interval = 5
	}

	timeout := time.After(5 * time.Minute)
	ticker := time.NewTicker(time.Duration(interval) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-timeout:
			return fmt.Errorf("authorization timed out (5 minutes)")
		case <-ticker.C:
			result, err := auth.PollDeviceToken(clientID, clientSecret, deviceCode, region)
			if err != nil {
				return fmt.Errorf("poll error: %w", err)
			}

			if result.Pending {
				fmt.Print(".")
				continue
			}

			if !result.Success {
				return fmt.Errorf("authorization failed: %s - %s", result.Error, result.ErrorDescription)
			}

			fmt.Println("\n\nAuthorization successful!")
			return saveConnection(result.Tokens, authMethod, clientID, clientSecret, region)
		}
	}
}

func runAuthSocial(provider string) error {
	codeVerifier, codeChallenge, state, err := auth.GeneratePKCE()
	if err != nil {
		return fmt.Errorf("generate PKCE: %w", err)
	}

	authURL, err := auth.BuildSocialLoginURL(provider, codeChallenge, state)
	if err != nil {
		return fmt.Errorf("build auth URL: %w", err)
	}

	fmt.Printf("\n[%s] Open this URL in your browser:\n  %s\n", provider, authURL)
	fmt.Println("\nAfter login, you'll be redirected to a kiro:// URL.")
	fmt.Println("Copy the 'code' parameter from that URL and paste it here.")
	fmt.Printf("\nState (for verification): %s\n", state)

	reader := bufio.NewReader(os.Stdin)
	fmt.Print("\nEnter authorization code: ")
	code, _ := reader.ReadString('\n')
	code = strings.TrimSpace(code)
	if code == "" {
		return fmt.Errorf("authorization code is required")
	}

	fmt.Println("Exchanging code for tokens...")

	result, err := auth.ExchangeSocialCode(code, codeVerifier)
	if err != nil {
		return fmt.Errorf("exchange code: %w", err)
	}

	result.AuthMethod = provider
	return saveConnection(result, provider, "", "", "us-east-1")
}

func runAuthImport(reader *bufio.Reader) error {
	fmt.Println("\nPaste your Kiro refresh token (starts with aorAAAAAG...):")
	fmt.Print("> ")
	token, _ := reader.ReadString('\n')
	token = strings.TrimSpace(token)
	if token == "" {
		return fmt.Errorf("refresh token is required")
	}

	fmt.Print("Client ID (leave empty to auto-register): ")
	clientID, _ := reader.ReadString('\n')
	clientID = strings.TrimSpace(clientID)

	clientSecret := ""
	if clientID != "" {
		fmt.Print("Client Secret: ")
		clientSecret, _ = reader.ReadString('\n')
		clientSecret = strings.TrimSpace(clientSecret)
	}

	fmt.Print("Region (default: us-east-1): ")
	region, _ := reader.ReadString('\n')
	region = strings.TrimSpace(region)

	fmt.Print("Auth method (builder-id/idc, default: builder-id): ")
	method, _ := reader.ReadString('\n')
	method = strings.TrimSpace(method)
	if method == "" {
		method = "builder-id"
	}

	fmt.Println("\nValidating token...")

	result, err := auth.ValidateAndImportToken(token, clientID, clientSecret, region, method)
	if err != nil {
		return fmt.Errorf("import failed: %w", err)
	}

	return saveConnection(result, result.AuthMethod, result.ClientID, result.ClientSecret, result.Region)
}

func saveConnection(tokens *auth.TokenResult, authMethod, clientID, clientSecret, region string) error {
	store, err := getStore()
	if err != nil {
		return err
	}

	cfg, err := store.Load()
	if err != nil {
		return err
	}

	email := auth.ExtractEmailFromJWT(tokens.AccessToken)

	// Determine display name
	providerLabel := "AWS Builder ID"
	switch authMethod {
	case "idc":
		providerLabel = "AWS IAM Identity Center"
	case "google":
		providerLabel = "Google"
	case "github":
		providerLabel = "GitHub"
	case "imported":
		providerLabel = "Imported"
	}

	name := email
	if name == "" {
		name = fmt.Sprintf("%s Account %d", providerLabel, len(cfg.ProviderConnections)+1)
	}

	now := time.Now().UTC().Format(time.RFC3339)
	expiresIn := tokens.ExpiresIn
	if expiresIn == 0 {
		expiresIn = 3600
	}

	conn := domain.ProviderConnection{
		ID:           uuid.New().String(),
		Provider:     "kiro",
		AuthType:     "oauth",
		Name:         name,
		Weight:       100,
		IsActive:     true,
		AccessToken:  tokens.AccessToken,
		RefreshToken: tokens.RefreshToken,
		ExpiresAt:    time.Now().Add(time.Duration(expiresIn) * time.Second).UTC().Format(time.RFC3339),
		ExpiresIn:    expiresIn,
		Email:        email,
		TestStatus:   "active",
		ProviderSpecificData: map[string]interface{}{
			"profileArn": tokens.ProfileArn,
			"authMethod": authMethod,
			"provider":   providerLabel,
		},
		CreatedAt: now,
		UpdatedAt: now,
	}

	// Persist clientId/clientSecret for future token refreshes
	if clientID != "" {
		conn.ProviderSpecificData["clientId"] = clientID
	}
	if clientSecret != "" {
		conn.ProviderSpecificData["clientSecret"] = clientSecret
	}
	if region != "" {
		conn.ProviderSpecificData["region"] = region
	}

	cfg.ProviderConnections = append(cfg.ProviderConnections, conn)
	if err := store.Save(cfg); err != nil {
		return fmt.Errorf("save connection: %w", err)
	}

	fmt.Printf("\nConnection saved!\n")
	fmt.Printf("  ID:       %s\n", conn.ID)
	fmt.Printf("  Name:     %s\n", conn.Name)
	fmt.Printf("  Method:   %s\n", providerLabel)
	fmt.Printf("  Provider: kiro\n")
	if email != "" {
		fmt.Printf("  Email:    %s\n", email)
	}

	return nil
}

func runAuthList(cmd *cobra.Command, args []string) error {
	store, err := getStore()
	if err != nil {
		return err
	}

	cfg, err := store.Load()
	if err != nil {
		return err
	}

	if len(cfg.ProviderConnections) == 0 {
		fmt.Println("No connections configured. Run 'dntproxy auth add' to add one.")
		return nil
	}

	fmt.Printf("%-36s  %-6s  %-20s  %-15s  %-8s\n", "ID", "WGT", "NAME", "METHOD", "STATUS")
	fmt.Println(strings.Repeat("-", 100))

	for _, c := range cfg.ProviderConnections {
		method := ""
		if c.ProviderSpecificData != nil {
			if m, ok := c.ProviderSpecificData["provider"].(string); ok {
				method = m
			}
		}
		status := "active"
		if !c.IsActive {
			status = "disabled"
		} else if domain.IsAccountUnavailable(c.RateLimitedUntil) {
			status = "cooldown"
		}

		name := c.Name
		if len(name) > 20 {
			name = name[:17] + "..."
		}

		fmt.Printf("%-36s  %-6d  %-20s  %-15s  %-8s\n", c.ID, c.Weight, name, method, status)
	}

	return nil
}

func runAuthRemove(cmd *cobra.Command, args []string) error {
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
	for i, c := range cfg.ProviderConnections {
		if c.ID == id || strings.HasPrefix(c.ID, id) {
			fmt.Printf("Removing connection: %s (%s)\n", c.Name, c.ID)
			cfg.ProviderConnections = append(cfg.ProviderConnections[:i], cfg.ProviderConnections[i+1:]...)
			found = true
			break
		}
	}

	if !found {
		return fmt.Errorf("connection not found: %s", id)
	}

	if err := store.Save(cfg); err != nil {
		return fmt.Errorf("save: %w", err)
	}

	fmt.Println("Connection removed.")
	return nil
}

func runAuthOpenAI() error {
	codeVerifier, codeChallenge, state, err := auth.GeneratePKCE()
	if err != nil {
		return fmt.Errorf("generate PKCE: %w", err)
	}

	// Use localhost callback
	redirectURI := "http://localhost:20129/callback"
	authURL := auth.BuildOpenAIAuthURL(redirectURI, state, codeChallenge)

	fmt.Printf("\n[OpenAI] Open this URL in your browser:\n  %s\n", authURL)
	fmt.Println("\nAfter login, you'll be redirected to localhost.")
	fmt.Println("Copy the 'code' parameter from the URL and paste it here.")
	fmt.Printf("\nState (for verification): %s\n", state)

	reader := bufio.NewReader(os.Stdin)
	fmt.Print("\nEnter authorization code: ")
	code, _ := reader.ReadString('\n')
	code = strings.TrimSpace(code)
	if code == "" {
		return fmt.Errorf("authorization code is required")
	}

	fmt.Println("Exchanging code for tokens...")

	tokens, err := auth.ExchangeOpenAICode(code, redirectURI, codeVerifier)
	if err != nil {
		return fmt.Errorf("exchange code: %w", err)
	}

	return saveOpenAIConnection(tokens)
}

func saveOpenAIConnection(tokens *auth.OpenAITokenResponse) error {
	store, err := getStore()
	if err != nil {
		return err
	}

	cfg, err := store.Load()
	if err != nil {
		return err
	}

	email := auth.ExtractEmailFromJWT(tokens.AccessToken)
	name := email
	if name == "" {
		name = fmt.Sprintf("OpenAI Account %d", len(cfg.ProviderConnections)+1)
	}

	now := time.Now().UTC().Format(time.RFC3339)
	expiresIn := tokens.ExpiresIn
	if expiresIn == 0 {
		expiresIn = 3600
	}

	conn := domain.ProviderConnection{
		ID:           uuid.New().String(),
		Provider:     "openai",
		AuthType:     "oauth",
		Name:         name,
		Weight:       100,
		IsActive:     true,
		AccessToken:  tokens.AccessToken,
		RefreshToken: tokens.RefreshToken,
		ExpiresAt:    time.Now().Add(time.Duration(expiresIn) * time.Second).UTC().Format(time.RFC3339),
		ExpiresIn:    expiresIn,
		Email:        email,
		TestStatus:   "active",
		ProviderSpecificData: map[string]interface{}{
			"authMethod": "openai-oauth",
			"provider":   "OpenAI",
		},
		CreatedAt: now,
		UpdatedAt: now,
	}

	cfg.ProviderConnections = append(cfg.ProviderConnections, conn)
	if err := store.Save(cfg); err != nil {
		return fmt.Errorf("save connection: %w", err)
	}

	fmt.Printf("\nOpenAI connection saved!\n")
	fmt.Printf("  ID:       %s\n", conn.ID)
	fmt.Printf("  Name:     %s\n", conn.Name)
	fmt.Printf("  Provider: openai\n")
	if email != "" {
		fmt.Printf("  Email:    %s\n", email)
	}

	return nil
}

func runAuthTest(cmd *cobra.Command, args []string) error {
	id := args[0]

	store, err := getStore()
	if err != nil {
		return err
	}

	cfg, err := store.Load()
	if err != nil {
		return err
	}

	var conn *domain.ProviderConnection
	for i := range cfg.ProviderConnections {
		if cfg.ProviderConnections[i].ID == id || strings.HasPrefix(cfg.ProviderConnections[i].ID, id) {
			conn = &cfg.ProviderConnections[i]
			break
		}
	}

	if conn == nil {
		return fmt.Errorf("connection not found: %s", id)
	}

	fmt.Printf("Testing connection: %s (%s)\n", conn.Name, conn.ID)

	// Try token refresh
	refreshSvc := auth.NewTokenRefreshService(store)
	if refreshSvc.NeedsRefresh(conn) {
		fmt.Println("Token expiring soon, refreshing...")
		refreshed, err := refreshSvc.Refresh(conn)
		if err != nil {
			fmt.Printf("Token refresh failed: %s\n", err)
			return nil
		}
		conn = refreshed
		store.Save(cfg)
		fmt.Println("Token refreshed successfully.")
	}

	if conn.AccessToken == "" {
		fmt.Println("No access token available.")
		return nil
	}

	fmt.Println("Access token: present")
	fmt.Printf("Expires at: %s\n", conn.ExpiresAt)

	if email := auth.ExtractEmailFromJWT(conn.AccessToken); email != "" {
		fmt.Printf("Email: %s\n", email)
	}

	fmt.Println("Connection OK.")
	return nil
}
