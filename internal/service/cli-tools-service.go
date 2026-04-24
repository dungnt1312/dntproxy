package service

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/dungnt/dntproxy/internal/domain"
	"github.com/dungnt/dntproxy/internal/port"
)

type cliToolDefinition struct {
	ID        domain.CLIToolID
	Name      string
	ConfigRel []string
}

var cliToolDefinitions = []cliToolDefinition{
	{ID: domain.CLIToolClaudeCode, Name: "Claude Code", ConfigRel: []string{".claude", "settings.json"}},
	{ID: domain.CLIToolOpenCode, Name: "OpenCode", ConfigRel: []string{".config", "opencode", "opencode.json"}},
	{ID: domain.CLIToolCodex, Name: "Codex CLI", ConfigRel: []string{".codex", "config.toml"}},
}

// CLIToolsService detects and writes user-level coding CLI config files.
type CLIToolsService struct {
	store   port.CredentialStore
	homeDir string
	now     func() time.Time
}

func NewCLIToolsService(store port.CredentialStore) *CLIToolsService {
	home, _ := os.UserHomeDir()
	return &CLIToolsService{store: store, homeDir: home, now: time.Now}
}

func NewCLIToolsServiceForTest(store port.CredentialStore, homeDir string, now func() time.Time) *CLIToolsService {
	return &CLIToolsService{store: store, homeDir: homeDir, now: now}
}

func (s *CLIToolsService) Preview(req domain.CLIToolsConfigRequest) (*domain.CLIToolsPreviewResponse, error) {
	if err := validateCLIRequest(req); err != nil {
		return nil, err
	}
	models := req.EffectiveModels()
	resolved, aliases, err := s.resolveModels(models)
	if err != nil {
		return nil, err
	}

	resp := &domain.CLIToolsPreviewResponse{
		Endpoint: req.Endpoint,
		Models:   resolved,
		Aliases:  aliases,
	}
	for _, toolID := range normalizeTools(req.Tools) {
		def, ok := lookupTool(toolID)
		if !ok {
			return nil, fmt.Errorf("unsupported tool: %s", toolID)
		}
		path := s.configPath(def)
		content, err := renderToolPreview(toolID, req.Endpoint, req.APIKey, resolved)
		if err != nil {
			return nil, fmt.Errorf("render %s preview: %w", toolID, err)
		}
		resp.Previews = append(resp.Previews, domain.CLIToolConfigPreview{
			ToolID:     toolID,
			ConfigPath: path,
			Content:    content,
			Aliases:    aliases,
		})
	}
	return resp, nil
}

func (s *CLIToolsService) Apply(req domain.CLIToolsConfigRequest) (*domain.CLIToolsApplyResponse, error) {
	preview, err := s.Preview(req)
	if err != nil {
		return nil, err
	}
	if err := s.ensureAliases(preview.Aliases); err != nil {
		return nil, fmt.Errorf("ensure aliases: %w", err)
	}

	resp := &domain.CLIToolsApplyResponse{
		Endpoint: preview.Endpoint,
		Models:   preview.Models,
		Aliases:  preview.Aliases,
	}
	for _, item := range preview.Previews {
		result := domain.CLIToolApplyResult{
			ToolID:     item.ToolID,
			ConfigPath: item.ConfigPath,
		}
		current, err := readConfigOrEmpty(item.ConfigPath)
		if err != nil {
			result.Error = err.Error()
			resp.Results = append(resp.Results, result)
			continue
		}
		content, err := renderToolConfig(item.ToolID, current, req.Endpoint, req.APIKey, preview.Models)
		if err != nil {
			result.Error = err.Error()
			resp.Results = append(resp.Results, result)
			continue
		}
		backupPath, err := backupAndWrite(item.ConfigPath, content, s.now())
		if err != nil {
			result.Error = err.Error()
		} else {
			result.Applied = true
			result.BackupPath = backupPath
		}
		resp.Results = append(resp.Results, result)
	}
	return resp, nil
}

func (s *CLIToolsService) Restore(req domain.CLIToolsRestoreRequest) []domain.CLIToolRestoreResult {
	results := make([]domain.CLIToolRestoreResult, 0, len(normalizeTools(req.Tools)))
	for _, toolID := range normalizeTools(req.Tools) {
		def, ok := lookupTool(toolID)
		if !ok {
			results = append(results, domain.CLIToolRestoreResult{ToolID: toolID, Error: "unsupported tool"})
			continue
		}
		path := s.configPath(def)
		result := domain.CLIToolRestoreResult{ToolID: toolID, ConfigPath: path}
		backupPath := latestBackup(path)
		if backupPath == "" {
			result.Error = "no backup found"
			results = append(results, result)
			continue
		}
		if err := restoreBackup(path, backupPath); err != nil {
			result.Error = err.Error()
		} else {
			result.BackupPath = backupPath
			result.Restored = true
		}
		results = append(results, result)
	}
	return results
}

func (s *CLIToolsService) configPath(def cliToolDefinition) string {
	parts := append([]string{s.homeDir}, def.ConfigRel...)
	return filepath.Join(parts...)
}

func lookupTool(id domain.CLIToolID) (cliToolDefinition, bool) {
	for _, def := range cliToolDefinitions {
		if def.ID == id {
			return def, true
		}
	}
	return cliToolDefinition{}, false
}

func normalizeTools(tools []domain.CLIToolID) []domain.CLIToolID {
	if len(tools) == 0 {
		return []domain.CLIToolID{domain.CLIToolClaudeCode, domain.CLIToolOpenCode, domain.CLIToolCodex}
	}
	return tools
}

func validateCLIRequest(req domain.CLIToolsConfigRequest) error {
	if req.Endpoint == "" {
		return fmt.Errorf("endpoint is required")
	}
	if req.APIKey == "" {
		return fmt.Errorf("apiKey is required")
	}
	models := req.EffectiveModels()
	if len(models) == 0 {
		return fmt.Errorf("at least one model is required")
	}
	return nil
}
