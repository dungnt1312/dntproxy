import { useCallback, useEffect, useMemo, useState } from "react";
import {
  Search,
  Layers,
  Plus,
  Trash2,
  ChevronRight,
  Link2,
  Zap,
  ChevronDown,
  TestTube,
  Loader2,
  CheckCircle2,
  XCircle,
} from "lucide-react";
import { toast } from "sonner";

import { goApi } from "@/lib/go-api";

interface UiModel {
  id: string;
  name: string;
  provider: string;
  models?: string[];
  connections?: Array<{ id: string; name: string; provider: string }>;
}

type AliasMap = Record<string, string>;

interface ModelTestResult {
  status: "ok" | "error" | "loading";
  message?: string;
}

function providerLabel(provider: string) {
  if (provider === "kiro") return "Kiro";
  if (provider === "openai") return "OpenAI";
  if (provider === "combo") return "Combo";
  if (provider === "alias") return "Alias";
  return provider || "Other";
}

export default function ModelsScreen() {
  const [models, setModels] = useState<UiModel[]>([]);
  const [aliases, setAliases] = useState<AliasMap>({});
  const [loading, setLoading] = useState(true);
  const [search, setSearch] = useState("");

  const [showAdd, setShowAdd] = useState(false);
  const [aliasInput, setAliasInput] = useState("");
  const [modelInput, setModelInput] = useState("");
  const [savingAlias, setSavingAlias] = useState(false);

  const [collapsedProviders, setCollapsedProviders] = useState<Set<string>>(
    new Set(),
  );
  const [testResults, setTestResults] = useState<
    Record<string, ModelTestResult>
  >({});

  const toggleProvider = (provider: string) => {
    setCollapsedProviders((prev) => {
      const next = new Set(prev);
      if (next.has(provider)) {
        next.delete(provider);
      } else {
        next.add(provider);
      }
      return next;
    });
  };

  const handleTestModel = async (connectionId: string, modelId: string) => {
    const key = `${connectionId}::${modelId}`;
    setTestResults((prev) => ({ ...prev, [key]: { status: "loading" } }));
    try {
      const res = await goApi.testModel(connectionId, modelId);
      setTestResults((prev) => ({
        ...prev,
        [key]: {
          status: res.status === "ok" ? "ok" : "error",
          message: res.message,
        },
      }));
    } catch (e: any) {
      setTestResults((prev) => ({
        ...prev,
        [key]: { status: "error", message: e.message },
      }));
    }
  };

  const fetchAll = useCallback(async () => {
    setLoading(true);
    try {
      const [modelsData, aliasesData] = await Promise.all([
        goApi.getModels().catch(() => []),
        goApi.getAliases().catch(() => ({})),
      ]);

      setModels(Array.isArray(modelsData) ? modelsData : []);
      setAliases(aliasesData || {});
    } catch {
      toast.error("Failed to load models");
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    fetchAll();
  }, [fetchAll]);

  const q = search.trim().toLowerCase();

  const filteredModels = useMemo(
    () =>
      !q
        ? models
        : models.filter(
            (m) =>
              m.id.toLowerCase().includes(q) ||
              m.name.toLowerCase().includes(q),
          ),
    [models, q],
  );

  const sections = useMemo(() => {
    const byProvider: Record<string, UiModel[]> = {};
    filteredModels.forEach((model) => {
      if (!byProvider[model.provider]) byProvider[model.provider] = [];
      byProvider[model.provider].push(model);
    });
    return Object.entries(byProvider);
  }, [filteredModels]);

  const filteredAliases = useMemo(() => {
    const entries = Object.entries(aliases);
    if (!q) return entries;
    return entries.filter(
      ([alias, target]) =>
        alias.toLowerCase().includes(q) || target.toLowerCase().includes(q),
    );
  }, [aliases, q]);

  async function handleAddAlias() {
    if (!aliasInput.trim() || !modelInput.trim()) {
      toast.error("Alias and target model are required");
      return;
    }

    setSavingAlias(true);
    try {
      await goApi.setAlias(aliasInput.trim(), modelInput.trim());
      setAliasInput("");
      setModelInput("");
      setShowAdd(false);
      toast.success("Alias created");
      await fetchAll();
    } catch {
      toast.error("Failed to create alias");
    } finally {
      setSavingAlias(false);
    }
  }

  async function handleDeleteAlias(alias: string) {
    try {
      await goApi.deleteAlias(alias);
      toast.success("Alias deleted");
      await fetchAll();
    } catch {
      toast.error("Failed to delete alias");
    }
  }

  return (
    <div className="space-y-6">
      <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <div>
          <h1 className="text-2xl font-bold tracking-tight">
            Models & Aliases
          </h1>
          <p className="text-sm text-muted-foreground">
            Models are sourced from active provider connections, aliases, and
            backend combos.
          </p>
        </div>
      </div>

      <div className="rounded-lg border bg-card p-3">
        <div className="relative">
          <Search className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
          <input
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            placeholder="Filter models by id or name"
            className="h-9 w-full rounded-md border bg-background pl-9 pr-3 text-sm"
          />
        </div>
      </div>

      {loading ? (
        <div className="flex items-center justify-center rounded-lg border bg-card p-12 text-sm text-muted-foreground">
          Loading models...
        </div>
      ) : (
        <div className="space-y-4">
          {sections.length === 0 ? (
            <div className="rounded-lg border border-dashed bg-card p-8 text-center text-sm text-muted-foreground">
              No models found.
            </div>
          ) : (
            sections.map(([provider, list]) => {
              const isCollapsed = collapsedProviders.has(provider);
              return (
                <div
                  key={provider}
                  className="rounded-lg border bg-card shadow-sm"
                >
                  <button
                    onClick={() => toggleProvider(provider)}
                    className="flex w-full items-center justify-between border-b px-4 py-3 bg-muted/50 transition-colors hover:bg-muted"
                  >
                    <div className="flex items-center gap-2 text-sm font-medium">
                      <ChevronDown
                        className={`h-4 w-4 transition-transform ${isCollapsed ? "-rotate-90" : ""}`}
                      />
                      <Layers className="h-4 w-4 text-muted-foreground" />
                      {providerLabel(provider)}
                    </div>
                    <span className="rounded-full bg-muted px-2 py-0.5 text-xs font-medium">
                      {list.length}
                    </span>
                  </button>

                  {!isCollapsed && (
                    <div className="divide-y">
                      {list.map((item) => (
                        <div
                          key={item.id}
                          className="px-4 py-3 text-sm transition-colors hover:bg-muted/30"
                        >
                          <div className="flex flex-col gap-2 sm:flex-row sm:items-start sm:justify-between">
                            <div className="min-w-0 flex-1">
                              <div className="flex items-center gap-2">
                                <div className="truncate font-medium">
                                  {item.name}
                                </div>
                              </div>
                              <div className="truncate font-mono text-xs text-muted-foreground">
                                {item.id}
                              </div>
                              {item.models && item.models.length > 0 && (
                                <span className="mt-1 inline-block rounded-full bg-purple-100 px-2 py-0.5 text-xs font-medium text-purple-700 dark:bg-purple-900/30 dark:text-purple-300">
                                  {item.models.length} items
                                </span>
                              )}
                              {item.connections &&
                                item.connections.length > 0 && (
                                  <div className="mt-2 flex flex-wrap gap-1.5">
                                    {item.connections.map((conn) => {
                                      const testKey = `${conn.id}::${item.id}`;
                                      const testResult = testResults[testKey];
                                      return (
                                        <div
                                          key={conn.id}
                                          className="inline-flex items-center gap-1 rounded bg-muted px-2 py-1 text-xs"
                                        >
                                          <span className="font-medium text-muted-foreground">
                                            {conn.name}
                                          </span>
                                          <button
                                            onClick={(e) => {
                                              e.stopPropagation();
                                              handleTestModel(conn.id, item.id);
                                            }}
                                            disabled={
                                              testResult?.status === "loading"
                                            }
                                            className="inline-flex items-center justify-center rounded p-0.5 transition-colors hover:bg-background disabled:opacity-50"
                                            title={`Test ${item.id} on ${conn.name}`}
                                          >
                                            {testResult?.status ===
                                            "loading" ? (
                                              <Loader2 className="h-3 w-3 animate-spin" />
                                            ) : testResult?.status === "ok" ? (
                                              <CheckCircle2 className="h-3 w-3 text-green-600" />
                                            ) : testResult?.status ===
                                              "error" ? (
                                              <XCircle className="h-3 w-3 text-red-600" />
                                            ) : (
                                              <TestTube className="h-3 w-3 text-muted-foreground hover:text-foreground" />
                                            )}
                                          </button>
                                          {testResult?.message &&
                                            testResult.status !== "loading" && (
                                              <span
                                                className={`text-[10px] ${
                                                  testResult.status === "ok"
                                                    ? "text-green-600"
                                                    : "text-red-600"
                                                }`}
                                                title={testResult.message}
                                              >
                                                {testResult.status === "ok"
                                                  ? "✓"
                                                  : "✗"}
                                              </span>
                                            )}
                                        </div>
                                      );
                                    })}
                                  </div>
                                )}
                            </div>
                          </div>
                        </div>
                      ))}
                    </div>
                  )}
                </div>
              );
            })
          )}
        </div>
      )}

      <div
        id="aliases"
        className="space-y-3 rounded-lg border bg-card p-4 shadow-sm"
      >
        <div className="flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
          <h2 className="text-sm font-semibold">Aliases</h2>
          <button
            onClick={() => setShowAdd((v) => !v)}
            className="inline-flex h-8 items-center gap-1 self-start rounded-md border bg-background px-2 py-1 text-xs transition-colors hover:bg-accent"
          >
            <Plus className="h-3 w-3" /> Add Alias
          </button>
        </div>

        {showAdd && (
          <div className="grid gap-2 rounded-lg border bg-muted/30 p-3 sm:grid-cols-[1fr_1fr_auto]">
            <input
              value={aliasInput}
              onChange={(e) => setAliasInput(e.target.value)}
              placeholder="alias"
              className="h-9 rounded-md border bg-background px-3 text-sm"
            />
            <input
              value={modelInput}
              onChange={(e) => setModelInput(e.target.value)}
              placeholder="provider/model"
              className="h-9 rounded-md border bg-background px-3 text-sm"
            />
            <button
              onClick={handleAddAlias}
              disabled={savingAlias}
              className="inline-flex h-9 items-center justify-center rounded-md bg-primary px-3 text-sm text-primary-foreground transition-colors hover:bg-primary/90 disabled:opacity-50"
            >
              {savingAlias ? "Saving..." : "Save"}
            </button>
          </div>
        )}

        {filteredAliases.length === 0 ? (
          <div className="rounded-lg border border-dashed bg-muted/20 p-6 text-center text-sm text-muted-foreground">
            No aliases configured.
          </div>
        ) : (
          <div className="space-y-2">
            {filteredAliases.map(([alias, target]) => (
              <div
                key={alias}
                className="flex flex-col gap-2 rounded-md border bg-background px-3 py-2.5 transition-colors hover:bg-muted/30 sm:flex-row sm:items-center sm:justify-between"
              >
                <div className="min-w-0 text-sm">
                  <div className="rounded bg-muted px-1.5 py-0.5 font-mono text-sm font-medium">
                    {alias}
                  </div>
                  <div className="mt-1 font-mono text-xs text-muted-foreground">
                    {target}
                  </div>
                </div>
                <button
                  onClick={() => handleDeleteAlias(alias)}
                  className="inline-flex h-8 w-8 items-center justify-center self-start rounded-md text-destructive transition-colors hover:bg-destructive/10 sm:self-auto"
                >
                  <Trash2 className="h-4 w-4" />
                </button>
              </div>
            ))}
          </div>
        )}

        <div className="flex items-center gap-2 rounded-md bg-muted/30 px-3 py-2 text-xs text-muted-foreground">
          <Link2 className="h-3 w-3" />
          <span>
            Use aliases as model IDs in the playground and client requests.
          </span>
          <Zap className="h-3 w-3" />
        </div>
      </div>
    </div>
  );
}
