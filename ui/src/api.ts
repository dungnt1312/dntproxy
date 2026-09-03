import type { LogFilters } from './types/logs';
import { getStoredApiKey, notifyUnauthorized, onUnauthorized } from './lib/go-api';

const BASE = '/api';

/** @deprecated Use onUnauthorized from go-api — kept as alias for older imports. */
export const onLegacyUnauthorized = onUnauthorized;

function isAuthSessionFailure(status: number, bodyText: string): boolean {
  if (status === 401) return true;
  if (status !== 403) return false;
  const lower = bodyText.toLowerCase();
  return (
    lower.includes('dashboard access') ||
    lower.includes('tenant is disabled') ||
    lower.includes('missing api key') ||
    lower.includes('invalid api key')
  );
}

async function request(path: string, options?: RequestInit) {
  const apiKey = getStoredApiKey();
  const headers: Record<string, string> = {
    'Content-Type': 'application/json',
    ...(options?.headers as Record<string, string>),
  };
  if (apiKey) {
    headers['Authorization'] = `Bearer ${apiKey}`;
  }

  const res = await fetch(`${BASE}${path}`, {
    ...options,
    headers,
  });

  if (res.status === 401 || res.status === 403) {
    const bodyText = await res.clone().text().catch(() => '');
    if (isAuthSessionFailure(res.status, bodyText)) {
      notifyUnauthorized();
      throw new Error('Unauthorized');
    }
  }

  if (!res.ok) {
    const err = await res.json().catch(() => ({ error: res.statusText }));
    const message =
      (typeof err?.error === 'string' && err.error) ||
      err?.error?.message ||
      err?.message ||
      res.statusText;
    throw new Error(message);
  }
  return res.json();
}

function logQuery(filters: Partial<LogFilters> = {}) {
  const params = new URLSearchParams()
  Object.entries(filters).forEach(([key, value]) => {
    if (value && value !== 'all') params.set(key, value)
  })
  const query = params.toString()
  return query ? `?${query}` : ''
}

import type { CreateConnectionPayload } from './types/provider-metadata';

export type ConnectionImportResult = {
  ok: boolean;
  imported: number;
  updated: number;
  skipped: number;
  errors?: string[];
};

export const api = {
  // Provider catalog (dynamic add-connection UI)
  getProviders: () => request('/providers'),

  // Connections
  getConnections: () => request('/connections'),
  createConnection: (data: CreateConnectionPayload) =>
    request('/connections', { method: 'POST', body: JSON.stringify(data) }),
  importConnection: (data: { refreshToken: string; clientId?: string; clientSecret?: string; region?: string; authMethod?: string }) =>
    request('/connections/import', { method: 'POST', body: JSON.stringify(data) }),
  addOpenAIConnection: (data: { name?: string; apiKey: string; supportedModels?: string[] }) =>
    request('/connections/add-openai', { method: 'POST', body: JSON.stringify(data) }),
  addCustomConnection: (data: { name?: string; apiKey?: string; baseUrl: string; routePrefix?: string; modelPrefix?: string; supportedModels?: string[] }) =>
    request('/connections/add-custom', { method: 'POST', body: JSON.stringify(data) }),
  addGLMConnection: (data: { name?: string; apiKey: string; baseUrl?: string; supportedModels?: string[] }) =>
    request('/connections/add-glm', { method: 'POST', body: JSON.stringify(data) }),
  addMiniMaxConnection: (data: { name?: string; apiKey: string; baseUrl?: string; supportedModels?: string[] }) =>
    request('/connections/add-minimax', { method: 'POST', body: JSON.stringify(data) }),
  addQwenConnection: (data: { name?: string; apiKey: string; baseUrl?: string; supportedModels?: string[] }) =>
    request('/connections/add-qwen', { method: 'POST', body: JSON.stringify(data) }),
  addAnthropicConnection: (data: { name?: string; apiKey: string; baseUrl?: string; supportedModels?: string[] }) =>
    request('/connections/add-anthropic', { method: 'POST', body: JSON.stringify(data) }),
  addGeminiConnection: (data: { name?: string; apiKey: string; baseUrl?: string; supportedModels?: string[] }) =>
    request('/connections/add-gemini', { method: 'POST', body: JSON.stringify(data) }),
  addClineConnection: (data: { name?: string; apiKey: string; baseUrl?: string; supportedModels?: string[] }) =>
    request('/connections/add-cline', { method: 'POST', body: JSON.stringify(data) }),
  deleteConnection: (id: string) => request(`/connections/${id}`, { method: 'DELETE' }),
  testConnection: (id: string) => request(`/connections/${id}/test`, { method: 'POST' }),
  updateConnection: (id: string, data: Record<string, unknown>) =>
    request(`/connections/${id}`, { method: 'PUT', body: JSON.stringify(data) }),
  detectKiroToken: () => request('/connections/detect-kiro', { method: 'POST' }),
  detectCommandCodeAuth: (data?: { import?: boolean }) =>
    request('/connections/detect-commandcode', { method: 'POST', body: JSON.stringify(data ?? {}) }),
  resetCooldown: (id: string) => request(`/connections/${id}/reset-cooldown`, { method: 'POST' }),
  clearConnectionError: (id: string, model?: string) =>
    request(`/connections/${id}/clear-error`, { method: 'POST', body: JSON.stringify(model ? { model } : {}) }),
  importConnectionFile: (data: object, mode: 'add' | 'replace' | 'merge' = 'merge') =>
    request('/connections/import-file', {
      method: 'POST',
      body: JSON.stringify({ ...data, mode }),
    }) as Promise<ConnectionImportResult>,
  importConnectionsFile: (data: object, mode: 'add' | 'replace' | 'merge' = 'merge') =>
    request('/connections/import-multiple', {
      method: 'POST',
      body: JSON.stringify({ ...data, mode }),
    }) as Promise<ConnectionImportResult>,

  // Kiro Auth Flows
  startBuilderID: () => request('/auth/kiro/start-builderid', { method: 'POST' }),
  startIDC: (data: { startUrl: string; region?: string }) =>
    request('/auth/kiro/start-idc', { method: 'POST', body: JSON.stringify(data) }),
  pollAuth: (sessionId: string) =>
    request('/auth/kiro/poll', { method: 'POST', body: JSON.stringify({ sessionId }) }),
  startSocialLogin: (provider: 'google' | 'github') =>
    request('/auth/kiro/start-social', { method: 'POST', body: JSON.stringify({ provider }) }),
  exchangeSocialCode: (data: { sessionId: string; callbackUrl?: string; code?: string }) =>
    request('/auth/kiro/exchange-social', { method: 'POST', body: JSON.stringify(data) }),
  addKiroApiKey: (data: { apiKey: string; name?: string; region?: string }) =>
    request('/auth/kiro/api-key', { method: 'POST', body: JSON.stringify(data) }),

  // OpenAI OAuth (PKCE)
  startOpenAIOAuth: () => request('/auth/openai/start', { method: 'POST' }),
  pollOpenAIOAuth: (sessionId: string, callbackUrl?: string) =>
    request('/auth/openai/exchange', { method: 'POST', body: JSON.stringify({ sessionId, callbackUrl }) }),

  // OpenAI bulk auto-login (automated browser)
  startOpenAIAutoLogin: (payload: { accounts: string[]; workers: number; headless?: boolean; skipExisting?: boolean }) =>
    request('/auth/openai/auto-login/start', { method: 'POST', body: JSON.stringify(payload) }),
  getOpenAIAutoLoginStatus: () => request('/auth/openai/auto-login/status'),
  stopOpenAIAutoLogin: () => request('/auth/openai/auto-login/stop', { method: 'POST' }),

  // Qwen OAuth (Device Code)
  startQwenOAuth: () => request('/auth/qwen/start', { method: 'POST' }),
  pollQwenOAuth: (sessionId: string) =>
    request('/auth/qwen/poll', { method: 'POST', body: JSON.stringify({ sessionId }) }),

  // xAI/Grok OAuth (PKCE)
  startXAIOAuth: () => request('/auth/xai/start', { method: 'POST' }),
  exchangeXAIOAuth: (sessionId: string, callbackUrl?: string, code?: string, state?: string) =>
    request('/auth/xai/exchange', { method: 'POST', body: JSON.stringify({ sessionId, callbackUrl, code, state }) }),
  importXAIAuthFile: (data: object) =>
    request('/auth/xai/import-file', { method: 'POST', body: JSON.stringify({ data }) }),

  // Fetch models from provider API
  fetchConnectionModels: (id: string) =>
    request(`/connections/${id}/fetch-models`, { method: 'POST', body: JSON.stringify({}) }),

  // Test a specific model through a connection
  testModel: (id: string, model: string) =>
    request(`/connections/${id}/test-model`, { method: 'POST', body: JSON.stringify({ model }) }),

  // Combos
  getCombos: () => request('/combos'),
  createCombo: (data: { name: string; models?: string[]; connectionIds?: string[] }) =>
    request('/combos', { method: 'POST', body: JSON.stringify(data) }),
  updateCombo: (id: string, data: Record<string, unknown>) =>
    request(`/combos/${id}`, { method: 'PUT', body: JSON.stringify(data) }),
  deleteCombo: (id: string) => request(`/combos/${id}`, { method: 'DELETE' }),
  checkQuota: (id: string) => request(`/connections/${id}/check-quota`, { method: 'POST' }),
  getUsage: (id: string) => request(`/usage/${id}`),

  // Aliases
  getAliases: () => request('/aliases'),
  setAlias: (alias: string, model: string) =>
    request('/aliases', { method: 'POST', body: JSON.stringify({ alias, model }) }),
  deleteAlias: (name: string) => request(`/aliases/${name}`, { method: 'DELETE' }),

  // Keys
  getKeys: () => request('/keys'),
  createKey: (name: string) => request('/keys', { method: 'POST', body: JSON.stringify({ name }) }),
  deleteKey: (id: string) => request(`/keys/${id}`, { method: 'DELETE' }),

  // Settings
  getSettings: () => request('/settings'),
  updateSettings: (data: Record<string, unknown>) =>
    request('/settings', { method: 'PUT', body: JSON.stringify(data) }),

  // Models
  getModels: () => request('/models'),
  getModelRegistry: () => request('/models/registry'),

  // Logs
  getLogs: (filters?: Partial<LogFilters>) => request(`/logs${logQuery(filters)}`),
  getLogSummary: (filters?: Partial<LogFilters>) => request(`/logs/summary${logQuery(filters)}`),
  getLogConnections: (filters?: Partial<LogFilters>) => request(`/logs/connections${logQuery(filters)}`),
  getLogPrices: () => request('/logs/prices'),
  clearLogs: () => request('/logs/clear', { method: 'POST' }),

  // Backup
  exportBackup: () => request('/backup/export'),
  importBackup: (data: unknown, mode: string, sections?: string[]) =>
    request('/backup/import', { method: 'POST', body: JSON.stringify({ ...data as object, mode, sections: sections || [] }) }),

  // Tunnel
  enableTunnel: () => request('/tunnel/enable', { method: 'POST' }),
  disableTunnel: () => request('/tunnel/disable', { method: 'POST' }),
  getTunnelStatus: () => request('/tunnel/status'),
};
