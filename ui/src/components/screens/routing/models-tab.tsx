import { useMemo, useState } from "react";
import { Search, Layers, ChevronDown, TestTube, Loader2, CheckCircle2, XCircle, TerminalSquare, Box } from "lucide-react";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { goApi } from "@/lib/go-api";
import { ProviderLogo } from "@/components/connections/ProviderLogo";
import { UiModel } from "./types";
import { RoutingCard } from "./routing-card";
import { cn } from "@/lib/utils";

interface ModelTestResult {
  status: "ok" | "error" | "loading";
  message?: string;
}

export interface ModelsTabProps {
  models: UiModel[];
  loading: boolean;
  onOpenLogModal: (modelId: string) => void;
}

function providerLabel(provider: string) {
  const labels: Record<string, string> = {
    kiro: "Kiro",
    openai: "OpenAI",
    glm: "GLM",
    minimax: "MiniMax",
    qwen: "Qwen",
    anthropic: "Anthropic",
    gemini: "Gemini",
    "openai-compatible": "Custom",
  };
  return labels[provider] || provider || "Other";
}

export default function ModelsTab({ models, loading, onOpenLogModal }: ModelsTabProps) {
  const [search, setSearch] = useState("");
  const [collapsedProviders, setCollapsedProviders] = useState<Set<string>>(new Set());
  const [testResults, setTestResults] = useState<Record<string, ModelTestResult>>({});

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

  const q = search.trim().toLowerCase();

  const filteredModels = useMemo(() => {
    // Exclude 'alias' and 'combo' from the base registry
    const baseModels = models.filter(m => m.provider !== 'alias' && m.provider !== 'combo');
    if (!q) return baseModels;
    return baseModels.filter(m => m.id.toLowerCase().includes(q) || m.name.toLowerCase().includes(q));
  }, [models, q]);

  const sections = useMemo(() => {
    const byProvider: Record<string, UiModel[]> = {};
    filteredModels.forEach((model) => {
      if (!byProvider[model.provider]) byProvider[model.provider] = [];
      byProvider[model.provider].push(model);
    });
    return Object.entries(byProvider);
  }, [filteredModels]);

  const totalModels = models.filter(m => m.provider !== 'alias' && m.provider !== 'combo').length;

  return (
    <div className="space-y-4 outline-none">
      {/* Toolbar */}
      <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <div className="relative flex-1 min-w-0 sm:max-w-md">
          <Search size={14} className="absolute left-3 top-1/2 -translate-y-1/2 text-muted-foreground pointer-events-none" />
          <Input
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            placeholder="Filter models by ID or name..."
            className="pl-9 h-9 text-sm"
          />
        </div>
        <div className="flex items-center gap-2 text-xs text-muted-foreground shrink-0">
          <Box className="h-3.5 w-3.5" />
          <span>{totalModels} models across {sections.length} providers</span>
        </div>
      </div>

      {/* Content */}
      {loading ? (
        <div className="flex items-center justify-center rounded-lg border bg-card p-16 text-sm text-muted-foreground">
          <Loader2 className="h-5 w-5 animate-spin mr-2" />
          Loading models...
        </div>
      ) : sections.length === 0 ? (
        <div className="flex flex-col items-center justify-center rounded-xl border border-dashed py-20 text-center">
          <div className="flex h-14 w-14 items-center justify-center rounded-xl bg-emerald-500/10 mb-4">
            <Layers className="h-6 w-6 text-emerald-500" />
          </div>
          <h3 className="mb-1 text-lg font-semibold">No models found</h3>
          <p className="max-w-md text-sm text-muted-foreground">
            {q ? `No models match "${q}". Try a different search term.` : "No models currently in the registry. Connect a provider first."}
          </p>
        </div>
      ) : (
        <div className="space-y-3">
          {sections.map(([provider, list]) => {
            const isCollapsed = collapsedProviders.has(provider);
            return (
              <div key={provider} className="rounded-xl border bg-card shadow-sm overflow-hidden">
                {/* Provider header */}
                <button
                  onClick={() => toggleProvider(provider)}
                  className="flex w-full items-center gap-3 px-4 py-3 bg-muted/20 transition-colors hover:bg-muted/40"
                >
                  <ChevronDown
                    className={cn(
                      "h-4 w-4 text-muted-foreground transition-transform duration-200",
                      isCollapsed && "-rotate-90"
                    )}
                  />
                  <div className="flex h-6 w-6 items-center justify-center rounded-md overflow-hidden shrink-0">
                    <ProviderLogo provider={provider} size={20} />
                  </div>
                  <span className="text-sm font-semibold">{providerLabel(provider)}</span>
                  <Badge variant="secondary" className="ml-auto rounded-full text-[11px] h-5 px-2 font-medium bg-muted">
                    {list.length}
                  </Badge>
                </button>

                {/* Model cards */}
                {!isCollapsed && (
                  <div className="divide-y divide-border/50">
                    {list.map((item) => (
                      <RoutingCard
                        key={item.id}
                        title={item.name}
                        type="model"
                        targets={[]}
                        badges={
                          item.isActive === false && (
                            <Badge variant="secondary" className="bg-destructive/10 text-destructive text-[10px] h-5 px-1.5 font-normal shrink-0">Inactive</Badge>
                          )
                        }
                        actions={
                          <Button 
                            variant="ghost"
                            size="icon"
                            onClick={(e) => { e.stopPropagation(); onOpenLogModal(item.id); }}
                            className="h-8 w-8 text-muted-foreground hover:text-foreground hover:bg-muted"
                            title={`View logs for model ${item.id}`}
                          >
                            <TerminalSquare className="h-4 w-4" />
                          </Button>
                        }
                      >
                        <div className="flex flex-col gap-2 mt-0.5">
                          {/* Model ID */}
                          <div className="flex items-center gap-2">
                            <code className="truncate text-xs text-muted-foreground bg-muted/60 px-2 py-0.5 rounded-md border border-border/40 font-mono">
                              {item.id}
                            </code>
                          </div>

                          {/* Connections with test buttons */}
                          {item.connections && item.connections.length > 0 && (
                            <div className="flex flex-wrap gap-1.5">
                              {item.connections.map((conn) => {
                                const testKey = `${conn.id}::${item.id}`;
                                const testResult = testResults[testKey];
                                return (
                                  <div 
                                    key={conn.id}
                                    className={cn(
                                      "inline-flex items-center gap-1.5 rounded-lg border px-2.5 py-1 text-xs transition-all duration-200",
                                      testResult?.status === "ok" 
                                        ? "border-emerald-500/30 bg-emerald-50/50 dark:bg-emerald-950/20"
                                        : testResult?.status === "error"
                                          ? "border-destructive/30 bg-red-50/50 dark:bg-red-950/20"
                                          : "bg-muted/30 hover:bg-muted/50"
                                    )}
                                  >
                                    <span className="font-medium text-muted-foreground">
                                      {conn.name}
                                    </span>
                                    <Button
                                      variant="ghost"
                                      size="icon"
                                      onClick={(e) => {
                                        e.stopPropagation();
                                        handleTestModel(conn.id, item.id);
                                      }}
                                      disabled={testResult?.status === "loading"}
                                      className="h-5 w-5 p-0 transition-colors hover:bg-transparent disabled:opacity-50"
                                      title={`Test ${item.id} on ${conn.name}`}
                                    >
                                      {testResult?.status === "loading" ? (
                                        <Loader2 className="h-3 w-3 animate-spin text-muted-foreground" />
                                      ) : testResult?.status === "ok" ? (
                                        <CheckCircle2 className="h-3.5 w-3.5 text-emerald-600" />
                                      ) : testResult?.status === "error" ? (
                                        <XCircle className="h-3.5 w-3.5 text-destructive" />
                                      ) : (
                                        <TestTube className="h-3 w-3 text-muted-foreground/60" />
                                      )}
                                    </Button>
                                  </div>
                                );
                              })}
                            </div>
                          )}
                        </div>
                      </RoutingCard>
                    ))}
                  </div>
                )}
              </div>
            );
          })}
        </div>
      )}
    </div>
  );
}
