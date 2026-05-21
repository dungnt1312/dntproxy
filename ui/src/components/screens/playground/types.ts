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
  routePrefix?: string;
  capabilities?: string[];
};

export type Connection = {
  id: string;
  name: string;
  provider: string;
  routePrefix?: string;
  isActive: boolean;
  supportedModels?: string[];
};

export type Attachment = {
  id: string;
  type: "image" | "file";
  name: string;
  size: number;
  dataUrl: string;
  mimeType: string;
};

export type Message = {
  id: string;
  role: "user" | "assistant" | "system";
  content: string;
  attachments?: Attachment[];
  status?: "queued" | "sending" | "done" | "error";
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
