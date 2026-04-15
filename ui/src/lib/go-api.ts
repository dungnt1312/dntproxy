const GO_API_BASE = import.meta.env.VITE_GO_API_URL || "/api";

type AliasMap = Record<string, string>;

async function goRequest<T = unknown>(
  path: string,
  options?: RequestInit,
): Promise<T> {
  const res = await fetch(`${GO_API_BASE}${path}`, {
    headers: { "Content-Type": "application/json" },
    ...options,
  });

  if (!res.ok) {
    const err = await res.json().catch(() => ({ error: res.statusText }));
    throw new Error(
      (err as { error?: string; message?: string }).error ||
        (err as { message?: string }).message ||
        res.statusText,
    );
  }

  return res.json();
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
  const rawModelId = String(go.modelId || go.model || go.id || "");

  return {
    ...go,
    id: canonicalId || rawModelId,
    modelId: rawModelId || canonicalId,
    displayName: String(go.displayName || go.name || rawModelId || canonicalId),
    provider: String(go.provider || "unknown"),
    connectionId: String(go.connectionId || ""),
    connectionName: String(go.connectionName || ""),
    isActive: typeof go.isActive === "boolean" ? go.isActive : true,
  };
}

function mapSettings(go: any) {
  return {
    ...go,
    id: 'default',
    serverPort: Number(go.port ?? 20199),
    apiKeyAuthEnabled: Boolean(go.requireApiKey),
    defaultRoutingStrategy: String(go.comboStrategy || "fallback"),
  };
}

export const goApi: any = {
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
        keyMasked: key.keyMasked || "",
        updatedAt: key.updatedAt || key.createdAt || new Date().toISOString(),
      })),
    ),

  createKey: (name: string) =>
    goRequest("/keys", { method: "POST", body: JSON.stringify({ name }) }),

  deleteKey: (id: string) => goRequest(`/keys/${id}`, { method: "DELETE" }),

  // Settings
  getSettings: () => goRequest("/settings").then(mapSettings),

  updateSettings: (data: Record<string, unknown>) => {
    const payload = {
      port: data.serverPort ?? data.port ?? 20199,
      requireApiKey: data.apiKeyAuthEnabled ?? data.requireApiKey ?? false,
      comboStrategy:
        data.defaultRoutingStrategy ?? data.comboStrategy ?? "fallback",
      stickyRoundRobinLimit: data.stickyRoundRobinLimit ?? 3,
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
};
