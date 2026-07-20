const GO_API_BASE = import.meta.env.VITE_GO_API_URL || "/api";

type AliasMap = Record<string, string>;

const AUTH_KEY = "dntproxy_api_key";

export function getStoredApiKey(): string {
  return localStorage.getItem(AUTH_KEY) || "";
}

export function setStoredApiKey(key: string) {
  if (key) {
    localStorage.setItem(AUTH_KEY, key);
  } else {
    localStorage.removeItem(AUTH_KEY);
  }
}

export function clearStoredApiKey() {
  localStorage.removeItem(AUTH_KEY);
}

let on401Callback: (() => void) | null = null;

export function onUnauthorized(cb: () => void) {
  on401Callback = cb;
}

/** Clear stored key and notify the App auth gate (login screen). */
export function notifyUnauthorized() {
  clearStoredApiKey();
  on401Callback?.();
}

/**
 * True only for dntproxy auth/session failures — NOT upstream provider 403s
 * (e.g. xAI spending-limit on /v1/images/generations).
 */
function isAuthSessionFailure(res: Response): boolean {
  return (
    (res.status === 401 || res.status === 403) &&
    res.headers.get("X-DNTProxy-Auth-Error") === "true"
  );
}

function extractErrorMessage(err: unknown, fallback: string): string {
  if (!err || typeof err !== "object") return fallback;
  const e = err as {
    error?: string | { message?: string };
    message?: string;
  };
  if (typeof e.error === "string" && e.error) return e.error;
  if (e.error && typeof e.error === "object" && e.error.message) {
    return e.error.message;
  }
  if (typeof e.message === "string" && e.message) return e.message;
  return fallback;
}

/** If the response is a session auth failure, clear key + show login and throw. */
async function rejectIfAuthFailure(res: Response): Promise<void> {
  if (isAuthSessionFailure(res)) {
    notifyUnauthorized();
    throw new Error("Unauthorized");
  }
}

async function goRequest<T = unknown>(
  path: string,
  options?: RequestInit,
): Promise<T> {
  const apiKey = getStoredApiKey();
  const headers: Record<string, string> = {
    "Content-Type": "application/json",
    ...(options?.headers as Record<string, string>),
  };
  if (apiKey) {
    headers["Authorization"] = `Bearer ${apiKey}`;
  }

  const res = await fetch(`${GO_API_BASE}${path}`, {
    ...options,
    headers,
  });

  await rejectIfAuthFailure(res);

  if (!res.ok) {
    const err = await res.json().catch(() => ({ error: res.statusText }));
    throw new Error(extractErrorMessage(err, res.statusText));
  }

  return res.json();
}

// Raw fetch with auth + 401 handling (for non-JSON or streaming responses)
export async function goFetch(
  path: string,
  options?: RequestInit,
): Promise<Response> {
  const apiKey = getStoredApiKey();
  const headers: Record<string, string> = {
    ...(options?.headers as Record<string, string>),
  };
  if (apiKey) {
    headers["Authorization"] = `Bearer ${apiKey}`;
  }

  const res = await fetch(`${GO_API_BASE}${path}`, {
    ...options,
    headers,
  });

  await rejectIfAuthFailure(res);
  return res;
}

// Streaming fetch for SSE endpoints (e.g. /v1/chat/completions)
export async function goStreamFetch(
  path: string,
  options?: RequestInit,
): Promise<Response> {
  const apiKey = getStoredApiKey();
  const headers: Record<string, string> = {
    "Content-Type": "application/json",
    ...(options?.headers as Record<string, string>),
  };
  if (apiKey) {
    headers["Authorization"] = `Bearer ${apiKey}`;
  }

  // Stream endpoints are under root, not /api
  const base = GO_API_BASE.replace(/\/api$/, "");
  const res = await fetch(`${base}${path}`, {
    ...options,
    headers,
  });

  await rejectIfAuthFailure(res);
  return res;
}

function toSearchParams(filters?: Record<string, unknown>) {
  const params = new URLSearchParams();
  if (!filters) return params;

  Object.entries(filters).forEach(([key, value]) => {
    if (
      value === undefined ||
      value === null ||
      value === "" ||
      value === "all"
    )
      return;
    params.set(key, String(value));
  });

  return params;
}

function mapConnection(go: any) {
  const status =
    go.testStatus === "active"
      ? "active"
      : go.rateLimitedUntil
        ? "rate_limited"
        : go.lastError
          ? "error"
          : "idle";

  return {
    ...go,
    id: go.id,
    name: go.name,
    provider: go.provider,
    authType: go.authType,
    authConfig: JSON.stringify({}),
    isActive: Boolean(go.isActive),
    status,
    lastError: go.lastError || null,
    backoffUntil: go.rateLimitedUntil || null,
    backoffCount: go.backoffLevel || 0,
    modelCount: go.supportedModels?.length || 0,
    createdAt: go.createdAt || new Date().toISOString(),
    updatedAt: go.updatedAt || new Date().toISOString(),
  };
}

function mapCombo(go: any) {
  return {
    ...go,
    id: go.id,
    name: go.name,
    models: Array.isArray(go.models) ? go.models : [],
    connectionIds: Array.isArray(go.connectionIds) ? go.connectionIds : [],
    createdAt: go.createdAt || new Date().toISOString(),
    updatedAt: go.updatedAt || go.createdAt || new Date().toISOString(),
  };
}

function toComboPayload(data: Record<string, any>) {
  if (Array.isArray(data.models)) {
    return {
      name: data.name,
      models: data.models,
      connectionIds: Array.isArray(data.connectionIds)
        ? data.connectionIds
        : [],
      setModels: true,
      setConnections: true,
    };
  }

  return data;
}

function mapModel(go: any) {
  const canonicalId = String(go.fullModel || go.id || "");
  const rawModelId = String(
    go.modelId || go.model || stripProviderPrefix(canonicalId),
  );
  const displayName = String(
    go.displayName || go.name || rawModelId || canonicalId || "Unnamed model",
  );

  return {
    ...go,
    id: canonicalId || rawModelId,
    modelId: rawModelId || canonicalId,
    name: displayName,
    displayName,
    provider: String(go.provider || "unknown"),
    connectionId: String(go.connectionId || ""),
    connectionName: String(go.connectionName || ""),
    connections: Array.isArray(go.connections)
      ? go.connections.filter((conn: any) => conn?.isActive !== false)
      : [],
    isActive: typeof go.isActive === "boolean" ? go.isActive : true,
  };
}

function stripProviderPrefix(modelId: string) {
  const slashIndex = modelId.indexOf("/");
  return slashIndex === -1 ? modelId : modelId.slice(slashIndex + 1);
}

function mapSettings(go: any) {
  return {
    ...go,
    id: 'default',
    serverPort: Number(go.port ?? 20199),
    apiKeyAuthEnabled: Boolean(go.requireApiKey),
    defaultRoutingStrategy: String(go.comboStrategy || "fallback"),
    connectionStrategy: String(go.connectionStrategy || "weighted-random"),
    compressionEnabled: Boolean(go?.compression?.enabled),
    compressionMinLength: Number(go?.compression?.minContentLength ?? 500),
    compressionLogSavings: go?.compression?.logSavings !== false,
    logBodies: Boolean(go.logBodies),
    telegramEnabled: Boolean(go?.telegram?.enabled),
    telegramBotToken: String(go?.telegram?.botToken || ""),
    telegramOwnerID: Number(go?.telegram?.ownerId || 0),
    defaultModels: go.defaultModels || {},
  };
}

export const goApi: any = {
  // Auth helpers
  getAuthHeaders: () => {
    const apiKey = getStoredApiKey();
    const headers: Record<string, string> = {};
    if (apiKey) {
      headers['Authorization'] = `Bearer ${apiKey}`;
    }
    return headers;
  },

  // Connections
  getConnections: () =>
    goRequest<any[]>("/connections").then((conns) =>
      (Array.isArray(conns) ? conns : []).map(mapConnection),
    ),

  createConnection: (data: {
    name: string;
    provider: string;
    authType: string;
    authConfig?: string;
  }) => {
    if (data.provider === "openai") {
      return goRequest("/connections/add-openai", {
        method: "POST",
        body: JSON.stringify({ name: data.name, apiKey: data.authConfig }),
      });
    }

    if (data.provider === "openai-compatible") {
      const body = JSON.parse(data.authConfig || "{}");
      body.name = data.name;
      return goRequest("/connections/add-custom", {
        method: "POST",
        body: JSON.stringify(body),
      });
    }

    return goRequest("/connections/import", {
      method: "POST",
      body: JSON.stringify({ refreshToken: data.authConfig }),
    });
  },

  updateConnection: (id: string, data: Record<string, unknown>) =>
    goRequest(`/connections/${id}`, {
      method: "PUT",
      body: JSON.stringify(data),
    }),

  deleteConnection: (id: string) =>
    goRequest(`/connections/${id}`, { method: "DELETE" }),

  testConnection: (id: string) =>
    goRequest(`/connections/${id}/test`, { method: "POST" }),

  resetCooldown: (id: string) =>
    goRequest(`/connections/${id}/reset-cooldown`, { method: "POST" }),

  detectKiroToken: () =>
    goRequest("/connections/detect-kiro", { method: "POST" }),

  checkQuota: (id: string) =>
    goRequest(`/connections/${id}/check-quota`, { method: "POST" }),

  testModel: (id: string, model: string) =>
    goRequest(`/connections/${id}/test-model`, {
      method: "POST",
      body: JSON.stringify({ model }),
    }),

  // Combos
  getCombos: () =>
    goRequest<any[]>("/combos").then((combos) =>
      (Array.isArray(combos) ? combos : []).map(mapCombo),
    ),

  createCombo: (data: {
    name: string;
    models?: string[];
    connectionIds?: string[];
  }) =>
    goRequest("/combos", {
      method: "POST",
      body: JSON.stringify(toComboPayload(data as any)),
    }),

  updateCombo: (id: string, data: Record<string, unknown>) =>
    goRequest(`/combos/${id}`, {
      method: "PUT",
      body: JSON.stringify(toComboPayload(data as any)),
    }),

  deleteCombo: (id: string) => goRequest(`/combos/${id}`, { method: "DELETE" }),

  // Aliases
  getAliases: () =>
    goRequest<AliasMap>("/aliases").then((aliases) => aliases || {}),

  setAlias: (alias: string, model: string) =>
    goRequest("/aliases", {
      method: "POST",
      body: JSON.stringify({ alias, model }),
    }),

  deleteAlias: (name: string) =>
    goRequest(`/aliases/${name}`, { method: "DELETE" }),

  // Keys
  getKeys: () =>
    goRequest<any[]>("/keys").then((keys) =>
      (Array.isArray(keys) ? keys : []).map((key) => ({
        ...key,
        updatedAt: key.updatedAt || key.createdAt || new Date().toISOString(),
        allowedConnectionIds: Array.isArray(key.allowedConnectionIds)
          ? key.allowedConnectionIds
          : [],
        allowedModels: Array.isArray(key.allowedModels) ? key.allowedModels : [],
      })),
    ),

  createKey: (payload: string | Record<string, unknown>) => {
    const body = typeof payload === "string" ? { name: payload } : payload;
    return goRequest("/keys", { method: "POST", body: JSON.stringify(body) });
  },

  updateKey: (id: string, payload: Record<string, unknown>) =>
    goRequest(`/keys/${id}`, {
      method: "PUT",
      body: JSON.stringify(payload),
    }),

  deleteKey: (id: string) => goRequest(`/keys/${id}`, { method: "DELETE" }),

  // Tenants (admin-only; backend enforces requireAdmin)
  getTenants: () =>
    goRequest<any[]>("/tenants").then((tenants) =>
      (Array.isArray(tenants) ? tenants : []).map((t) => ({
        ...t,
        status: t.status || "active",
      })),
    ),
  createTenant: (payload: { slug: string; name?: string; notes?: string }) =>
    goRequest("/tenants", { method: "POST", body: JSON.stringify(payload) }),
  updateTenant: (
    id: string,
    payload: { name?: string; notes?: string; status?: "active" | "disabled" },
  ) =>
    goRequest(`/tenants/${id}`, { method: "PUT", body: JSON.stringify(payload) }),
  deleteTenant: (id: string, cascade = false) =>
    goRequest(`/tenants/${id}?cascade=${cascade}`, { method: "DELETE" }),
  generateTenantKey: (
    id: string,
    payload: {
      name: string;
      dashboardAccess?: boolean;
      allowedConnectionIds?: string[];
      allowedModels?: string[];
    },
  ) =>
    goRequest<{ id: string; name: string; key: string; tenantId: string }>(
      `/tenants/${id}/keys`,
      { method: "POST", body: JSON.stringify(payload) },
    ),

  // Settings
  getSettings: () => goRequest("/settings").then(mapSettings),

  updateSettings: (data: Record<string, unknown>) => {
    const payload: Record<string, unknown> = {
      port: data.serverPort ?? data.port ?? 20199,
      // Auth is always enforced server-side; only include requireApiKey when
      // the caller explicitly sets it (legacy callers). Do not default to false.
      ...(data.apiKeyAuthEnabled !== undefined || data.requireApiKey !== undefined
        ? {
            requireApiKey:
              data.apiKeyAuthEnabled ?? data.requireApiKey ?? true,
          }
        : {}),
      comboStrategy:
        data.defaultRoutingStrategy ?? data.comboStrategy ?? "fallback",
      connectionStrategy:
        data.connectionStrategy ?? "weighted-random",
      compression: {
        enabled: Boolean(data.compressionEnabled ?? false),
        minContentLength: Number(data.compressionMinLength ?? 500),
        logSavings: data.compressionLogSavings !== false,
      },
      logBodies: Boolean(data.logBodies ?? false),
      defaultModels: data.defaultModels || {},
      telegram: {
        enabled: Boolean(data.telegramEnabled ?? false),
        botToken: String(data.telegramBotToken || ""),
        ownerId: Number(data.telegramOwnerID || 0),
      },
    };
    return goRequest("/settings", {
      method: "PUT",
      body: JSON.stringify(payload),
    }).then(mapSettings);
  },

  // Models
  getModels: () =>
    goRequest<any>("/models").then((payload) => {
      const models = Array.isArray(payload)
        ? payload
        : Array.isArray(payload?.models)
          ? payload.models
          : [];

      return models.map(mapModel);
    }),

  // Logs
  getLogs: (filters?: Record<string, unknown>) => {
    const params = toSearchParams(filters);
    const query = params.toString() ? `?${params.toString()}` : "";
    return goRequest(`/logs${query}`);
  },

  getLogDetail: (id: string) => {
    return goRequest<import("../types/logs").LogEntry>(`/logs/detail/${encodeURIComponent(id)}`);
  },

  getLogSummary: (filters?: Record<string, unknown>) => {
    const params = toSearchParams(filters);
    const query = params.toString() ? `?${params.toString()}` : "";
    return goRequest<any>(`/logs/summary${query}`).then((summary) => ({
      ...summary,
      totalRequests: Number(summary?.totalRequests ?? summary?.requests ?? 0),
      successRequests: Number(
        summary?.successRequests ??
          (summary?.requests ?? 0) - (summary?.errors ?? 0),
      ),
      errorRequests: Number(summary?.errorRequests ?? summary?.errors ?? 0),
      avgLatencyMs: Number(summary?.avgLatencyMs ?? 0),
      requestsByHour: Array.isArray(summary?.requestsByHour)
        ? summary.requestsByHour
        : [],
      requestsByProvider: Array.isArray(summary?.requestsByProvider)
        ? summary.requestsByProvider
        : [],
      requestsByStatus: Array.isArray(summary?.requestsByStatus)
        ? summary.requestsByStatus
        : [],
      recentErrors: Array.isArray(summary?.recentErrors)
        ? summary.recentErrors
        : [],
    }));
  },

  getLogConnections: (filters?: Record<string, unknown>) => {
    const params = toSearchParams(filters);
    const query = params.toString() ? `?${params.toString()}` : "";
    return goRequest(`/logs/connections${query}`);
  },

  getLogDaily: (range?: string) => {
    const query = range ? `?range=${encodeURIComponent(range)}` : "";
    return goRequest<import("../types/logs").DailyUsageStat[]>(`/logs/daily${query}`);
  },

  clearLogs: () => goRequest("/logs/clear", { method: "POST" }),

  // Backup
  exportBackup: (mask?: boolean) => {
    const url = mask ? "/backup/export?mask=true" : "/backup/export";
    return goRequest(url);
  },

  importBackup: (data: unknown, mode: string, sections?: string[]) =>
    goRequest("/backup/import", {
      method: "POST",
      body: JSON.stringify({
        ...(data as object),
        mode,
        sections: sections || [],
      }),
    }),

  // Auth flows
  startBuilderID: () =>
    goRequest("/auth/kiro/start-builderid", { method: "POST" }),

  startIDC: (data: { startUrl: string; region?: string }) =>
    goRequest("/auth/kiro/start-idc", {
      method: "POST",
      body: JSON.stringify(data),
    }),

  pollAuth: (sessionId: string) =>
    goRequest("/auth/kiro/poll", {
      method: "POST",
      body: JSON.stringify({ sessionId }),
    }),

  startSocialLogin: (provider: "google" | "github") =>
    goRequest("/auth/kiro/start-social", {
      method: "POST",
      body: JSON.stringify({ provider }),
    }),

  exchangeSocialCode: (data: {
    sessionId: string;
    callbackUrl?: string;
    code?: string;
  }) =>
    goRequest("/auth/kiro/exchange-social", {
      method: "POST",
      body: JSON.stringify(data),
    }),

  startOpenAIOAuth: () => goRequest("/auth/openai/start", { method: "POST" }),

  pollOpenAIOAuth: (sessionId: string, callbackUrl?: string) =>
    goRequest("/auth/openai/exchange", {
      method: "POST",
      body: JSON.stringify({ sessionId, callbackUrl }),
    }),

  // Usage
  getUsage: (connectionId: string) => goRequest(`/usage/${connectionId}`),
  getUsageStats: (params?: Record<string, string>) => {
    const qs = params ? '?' + new URLSearchParams(params).toString() : '';
    return goRequest(`/usage/stats${qs}`);
  },
  getUsageRequestDetails: (params?: Record<string, string>) => {
    const qs = params ? '?' + new URLSearchParams(params).toString() : '';
    return goRequest(`/usage/request-details${qs}`);
  },

  // Tunnel
  enableTunnel: () => goRequest("/tunnel/enable", { method: "POST" }),
  disableTunnel: () => goRequest("/tunnel/disable", { method: "POST" }),
  getTunnelStatus: () => goRequest("/tunnel/status"),

  // Profiles
  getProfiles: () => goRequest<any>("/profiles"),
  getProfilePresets: () => goRequest<any[]>("/profiles/presets"),
  createProfile: (data: { name: string; description?: string; aliases: Record<string, string> }) =>
    goRequest("/profiles", { method: "POST", body: JSON.stringify(data) }),
  createProfileFromPreset: (preset: string) =>
    goRequest("/profiles/from-preset", {
      method: "POST",
      body: JSON.stringify({ preset }),
    }),
  updateProfile: (
    name: string,
    data: { addAliases?: Record<string, string>; removeAliases?: string[] },
  ) => goRequest(`/profiles/${name}`, { method: "PUT", body: JSON.stringify(data) }),
  deleteProfile: (name: string) =>
    goRequest(`/profiles/${name}`, { method: "DELETE" }),
  activateProfile: (name: string) =>
    goRequest(`/profiles/${name}/activate`, { method: "POST" }),
  deactivateProfile: () =>
    goRequest("/profiles/deactivate", { method: "POST" }),
  getActiveProfile: () => goRequest("/profiles/active"),

  // Auth
  validateKey: (key: string) =>
    goRequest<{ valid: boolean; dashboardAccess?: boolean; tenantId?: string; isAdmin?: boolean }>("/auth/validate-key", {
      method: "POST",
      body: JSON.stringify({ key }),
    }),

  // Session info for currently-stored key (used on page reload)
  getSession: () =>
    goRequest<{
      authenticated: boolean;
      dashboardAccess?: boolean;
      tenantId?: string;
      isAdmin?: boolean;
      keyId?: string;
      keyName?: string;
    }>("/auth/session"),

  // Tools
  getTools: () => goRequest<any[]>("/tools"),
  getToolsStatus: () => goRequest<any>("/tools/status"),
  configureTool: (id: string) =>
    goRequest(`/tools/${id}/configure`, { method: "POST" }),
  resetTool: (id: string) =>
    goRequest(`/tools/${id}/reset`, { method: "POST" }),
  configureAllTools: () =>
    goRequest("/tools/configure-all", { method: "POST" }),
  resetAllTools: () =>
    goRequest("/tools/reset-all", { method: "POST" }),

  // Telegram
  getTelegramStatus: () => goRequest<any>("/telegram/status"),
  startTelegram: () => goRequest("/telegram/start", { method: "POST" }),
  stopTelegram: () => goRequest("/telegram/stop", { method: "POST" }),
	testTelegram: () => goRequest("/telegram/test", { method: "POST" }),

  // Image generation
  getImageModels: async () => {
    const apiKey = getStoredApiKey();
    const headers: Record<string, string> = {};
    if (apiKey) headers["Authorization"] = `Bearer ${apiKey}`;

    const base = GO_API_BASE.replace(/\/api$/, "");
    const res = await fetch(`${base}/v1/models?type=image`, { headers });
    await rejectIfAuthFailure(res);
    if (!res.ok) {
      const err = await res.json().catch(() => ({ error: res.statusText }));
      throw new Error(extractErrorMessage(err, res.statusText));
    }
    const payload = await res.json();
    const list = Array.isArray(payload?.data) ? payload.data : [];
    return list.map((item: unknown) => {
      const model = item as {
        id?: string;
        owned_by?: string;
        capabilities?: string[];
        image_capabilities?: Record<string, unknown>;
      };
      return {
        id: model.id || "",
        displayName: model.id || "",
        provider: model.owned_by || "unknown",
        capabilities: model.capabilities || [],
        imageCapabilities: model.image_capabilities || null,
      };
    });
  },

  generateImage: async (params: {
    model: string;
    prompt: string;
    n?: number;
    size?: string;
    quality?: string;
    response_format?: string;
  }) => {
    const apiKey = getStoredApiKey();
    const headers: Record<string, string> = { "Content-Type": "application/json" };
    if (apiKey) headers["Authorization"] = `Bearer ${apiKey}`;

    const base = GO_API_BASE.replace(/\/api$/, "");
    const res = await fetch(`${base}/v1/images/generations`, {
      method: "POST",
      headers,
      body: JSON.stringify(params),
    });
    await rejectIfAuthFailure(res);
    if (!res.ok) {
      const err = await res.json().catch(() => ({ error: res.statusText }));
      throw new Error(extractErrorMessage(err, res.statusText));
    }
    return res.json();
  },

  editImage: async (params: {
    model: string;
    prompt: string;
    image?: string;
    images?: { image_url: string }[];
    mask?: string;
    n?: number;
    size?: string;
    response_format?: string;
  }) => {
    const apiKey = getStoredApiKey();
    const headers: Record<string, string> = { "Content-Type": "application/json" };
    if (apiKey) headers["Authorization"] = `Bearer ${apiKey}`;

    const base = GO_API_BASE.replace(/\/api$/, "");
    const res = await fetch(`${base}/v1/images/edits`, {
      method: "POST",
      headers,
      body: JSON.stringify(params),
    });
    await rejectIfAuthFailure(res);
    if (!res.ok) {
      const err = await res.json().catch(() => ({ error: res.statusText }));
      throw new Error(extractErrorMessage(err, res.statusText));
    }
    return res.json();
  },
};
