import type { LogFilters } from './types/logs';

const BASE = '/api';

async function request(path: string, options?: RequestInit) {
  const res = await fetch(`${BASE}${path}`, {
    headers: { 'Content-Type': 'application/json' },
    ...options,
  });
  if (!res.ok) {
    const err = await res.json().catch(() => ({ error: res.statusText }));
    throw new Error(err.error || err.message || res.statusText);
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

export const api = {
  // Connections
  getConnections: () => request('/connections'),
  importConnection: (data: { refreshToken: string; clientId?: string; clientSecret?: string; region?: string; authMethod?: string }) =>
    request('/connections/import', { method: 'POST', body: JSON.stringify(data) }),
  addOpenAIConnection: (data: { name?: string; apiKey: string; supportedModels?: string[] }) =>
    request('/connections/add-openai', { method: 'POST', body: JSON.stringify(data) }),
  addCustomConnection: (data: { name?: string; apiKey?: string; baseUrl: string; supportedModels?: string[] }) =>
    request('/connections/add-custom', { method: 'POST', body: JSON.stringify(data) }),
  addGLMConnection: (data: { name?: string; apiKey: string; baseUrl?: string; supportedModels?: string[] }) =>
    request('/connections/add-glm', { method: 'POST', body: JSON.stringify(data) }),
  addMiniMaxConnection: (data: { name?: string; apiKey: string; baseUrl?: string; supportedModels?: string[] }) =>
    request('/connections/add-minimax', { method: 'POST', body: JSON.stringify(data) }),
  addQwenConnection: (data: { name?: string; apiKey: string; baseUrl?: string; supportedModels?: string[] }) =>
    request('/connections/add-qwen', { method: 'POST', body: JSON.stringify(data) }),
  deleteConnection: (id: string) => request(`/connections/${id}`, { method: 'DELETE' }),
  testConnection: (id: string) => request(`/connections/${id}/test`, { method: 'POST' }),
  updateConnection: (id: string, data: Record<string, unknown>) =>
    request(`/connections/${id}`, { method: 'PUT', body: JSON.stringify(data) }),
  detectKiroToken: () => request('/connections/detect-kiro', { method: 'POST' }),
  resetCooldown: (id: string) => request(`/connections/${id}/reset-cooldown`, { method: 'POST' }),

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

  // OpenAI OAuth (PKCE)
  startOpenAIOAuth: () => request('/auth/openai/start', { method: 'POST' }),
  pollOpenAIOAuth: (sessionId: string, callbackUrl?: string) =>
    request('/auth/openai/exchange', { method: 'POST', body: JSON.stringify({ sessionId, callbackUrl }) }),

  // Qwen OAuth (Device Code)
  startQwenOAuth: () => request('/auth/qwen/start', { method: 'POST' }),
  pollQwenOAuth: (sessionId: string) =>
    request('/auth/qwen/poll', { method: 'POST', body: JSON.stringify({ sessionId }) }),

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

  // Logs
  getLogs: (filters?: Partial<LogFilters>) => fetch(`/api/logs${logQuery(filters)}`).then(r => r.json()),
  getLogSummary: (filters?: Partial<LogFilters>) => fetch(`/api/logs/summary${logQuery(filters)}`).then(r => r.json()),
  getLogConnections: (filters?: Partial<LogFilters>) => fetch(`/api/logs/connections${logQuery(filters)}`).then(r => r.json()),
  getLogPrices: () => fetch('/api/logs/prices').then(r => r.json()),
  clearLogs: () => fetch('/api/logs/clear', { method: 'POST' }),

  // Backup
  exportBackup: (mask?: boolean) => {
    const url = mask ? '/api/backup/export?mask=true' : '/api/backup/export'
    return fetch(url).then(async r => {
      if (!r.ok) throw new Error('Export failed')
      return r.json()
    })
  },
  importBackup: (data: unknown, mode: string) =>
    request('/backup/import', { method: 'POST', body: JSON.stringify({ ...data as object, mode }) }),

  // Tunnel
  enableTunnel: () => request('/tunnel/enable', { method: 'POST' }),
  disableTunnel: () => request('/tunnel/disable', { method: 'POST' }),
  getTunnelStatus: () => request('/tunnel/status'),
};
