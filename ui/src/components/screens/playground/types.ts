export type ChatParams = {
  temperature: number;
  topP: number;
  maxTokens: number;
  systemPrompt: string;
};

export type Model = {
  id: string;
  displayName: string;
  provider: string;
};

export type Connection = {
  id: string;
  name: string;
  provider: string;
  isActive: boolean;
  supportedModels?: string[];
};

export type Message = {
  id: string;
  role: "user" | "assistant" | "system";
  content: string;
};

export type RequestLog = {
  id: string;
  timestamp: string;
  model: string;
  connectionId?: string;
  connectionName?: string;
  durationMs?: number;
  status: "success" | "error";
  statusCode?: number;
  requestBody: string;
  responseBody?: string;
  error?: string;
  inputTokens?: number;
  outputTokens?: number;
  totalTokens?: number;
  costTotal?: number;
};
