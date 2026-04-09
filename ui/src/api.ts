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

export const api = {
  // Connections
  getConnections: () => request('/connections'),
  importConnection: (data: { refreshToken: string; clientId?: string; clientSecret?: string; region?: string; authMethod?: string }) =>
    request('/connections/import', { method: 'POST', body: JSON.stringify(data) }),
  addOpenAIConnection: (data: { name?: string; apiKey: string; supportedModels?: string[] }) =>
    request('/connections/add-openai', { method: 'POST', body: JSON.stringify(data) }),
  addCustomConnection: (data: { name?: string; apiKey?: string; baseUrl: string; supportedModels?: string[] }) =>
    request('/connections/add-custom', { method: 'POST', body: JSON.stringify(data) }),
  deleteConnection: (id: string) => request(`/connections/${id}`, { method: 'DELETE' }),
  testConnection: (id: string) => request(`/connections/${id}/test`, { method: 'POST' }),
  updateConnection: (id: string, data: Record<string, unknown>) =>
    request(`/connections/${id}`, { method: 'PUT', body: JSON.stringify(data) }),
  detectKiroToken: () => request('/connections/detect-kiro', { method: 'POST' }),

  // Combos
  getCombos: () => request('/combos'),
  createCombo: (data: { name: string; models: string[] }) =>
    request('/combos', { method: 'POST', body: JSON.stringify(data) }),
  deleteCombo: (id: string) => request(`/combos/${id}`, { method: 'DELETE' }),

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
};
