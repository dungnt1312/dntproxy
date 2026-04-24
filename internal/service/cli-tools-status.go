package service

import "github.com/dungnt/dntproxy/internal/domain"

func (s *CLIToolsService) Statuses() []domain.CLIToolStatus {
	statuses := make([]domain.CLIToolStatus, 0, len(cliToolDefinitions))
	for _, def := range cliToolDefinitions {
		path := s.configPath(def)
		status := domain.CLIToolStatus{
			ID:         def.ID,
			Name:       def.Name,
			ConfigPath: path,
			Exists:     fileExists(path),
			Writable:   isConfigWritable(path),
			LastBackup: latestBackup(path),
		}
		configured, err := isToolConfigured(def.ID, path)
		status.Configured = configured
		if err != nil {
			status.Error = err.Error()
		}
		statuses = append(statuses, status)
	}
	return statuses
}
