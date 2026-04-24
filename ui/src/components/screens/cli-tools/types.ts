export interface CliToolStatus {
  id: string;
  name: string;
  configPath: string;
  exists: boolean;
  writable: boolean;
  configured: boolean;
  lastBackup?: string;
  error?: string;
}

/** Per-tool config state. models is a role→modelId map. */
export interface ToolConfig {
  endpoint: string;
  apiKey: string;
  models: Record<string, string>;
}

/** Model role definitions per tool type. */
export const TOOL_MODEL_ROLES: Record<string, { key: string; label: string }[]> = {
  claude: [
    { key: "sonnet", label: "Sonnet" },
    { key: "opus", label: "Opus" },
    { key: "haiku", label: "Haiku" },
  ],
  opencode: [{ key: "model", label: "Model" }],
  codex: [{ key: "model", label: "Model" }],
};

export interface PreviewItem {
  toolId: string;
  configPath: string;
  content: string;
}

export interface ModelOption {
  id: string;
  name?: string;
  displayName?: string;
  provider: string;
}

export interface ApiKeyOption {
  id: string;
  name: string;
  key: string;
  isActive: boolean;
}
