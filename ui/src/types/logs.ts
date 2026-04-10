export interface LogEntry {
  id: string
  timestamp: string
  timestampMs: number
  level: string
  provider: string
  direction: string
  method?: string
  path?: string
  statusCode?: number
  durationMs?: number
  connectionId?: string
  connectionName?: string
  model?: string
  requestId?: string
  message: string
  error?: string
  bodySize?: number
  requestBody?: string
  responseBody?: string
  inputTokens?: number
  outputTokens?: number
  totalTokens?: number
  usageSource?: string
  costTotal?: number
  currency?: string
  metadataJson?: string
}

export interface LogSummary {
  requests: number
  errors: number
  inputTokens: number
  outputTokens: number
  totalTokens: number
  costTotal: number
  currency: string
}

export interface LogConnectionSummary {
  connectionId: string
  connectionName: string
  provider: string
  requests: number
  errors: number
  totalTokens: number
  costTotal: number
  currency: string
}

export interface LogFilters {
  range: string
  connectionId: string
  provider: string
  level: string
  q: string
}
