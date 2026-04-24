import { useCallback, useEffect, useState } from "react";
import { motion } from "framer-motion";
import { RefreshCw, TerminalSquare } from "lucide-react";
import { toast } from "sonner";

import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { useServerInfo } from "@/hooks/use-server-info";
import { goApi } from "@/lib/go-api";

import { ToolCard } from "./cli-tools/tool-card";
import type { ApiKeyOption, CliToolStatus, ModelOption, PreviewItem, ToolConfig } from "./cli-tools/types";
import { TOOL_MODEL_ROLES } from "./cli-tools/types";

const containerVariants = {
  hidden: { opacity: 0 },
  visible: { opacity: 1, transition: { staggerChildren: 0.08 } },
};

const itemVariants = {
  hidden: { opacity: 0, y: 16 },
  visible: { opacity: 1, y: 0, transition: { duration: 0.3 } },
};

/** Build default models map for a tool, filling each role with firstModel. */
function defaultModelsForTool(toolId: string, firstModel: string): Record<string, string> {
  const roles = TOOL_MODEL_ROLES[toolId] || [{ key: "model", label: "Model" }];
  const models: Record<string, string> = {};
  for (const r of roles) {
    models[r.key] = firstModel;
  }
  return models;
}

export default function CliToolsScreen() {
  const { info, loading: infoLoading, refresh: refreshInfo } = useServerInfo();

  const [tools, setTools] = useState<CliToolStatus[]>([]);
  const [models, setModels] = useState<ModelOption[]>([]);
  const [keys, setKeys] = useState<ApiKeyOption[]>([]);
  const [loading, setLoading] = useState(true);

  // Per-tool state
  const [configs, setConfigs] = useState<Record<string, ToolConfig>>({});
  const [previews, setPreviews] = useState<Record<string, PreviewItem[]>>({});
  const [aliases, setAliases] = useState<Record<string, string>>({});

  const [previewingTool, setPreviewingTool] = useState("");
  const [applyingTool, setApplyingTool] = useState("");
  const [restoring, setRestoring] = useState(false);

  const load = useCallback(async () => {
    try {
      setLoading(true);
      const [toolPayload, modelPayload, keyPayload] = await Promise.all([
        goApi.getCliToolConfigs(),
        goApi.getModels(),
        goApi.getKeys(),
      ]);
      const activeKeys = (keyPayload || []).filter((k: ApiKeyOption) => k.isActive);
      const toolList: CliToolStatus[] = toolPayload.tools || [];

      setTools(toolList);
      setModels(modelPayload || []);
      setKeys(activeKeys);

      const defaultEndpoint = info?.baseUrl || "http://127.0.0.1:20199/v1";
      const firstModel = modelPayload?.[0]?.id || "";
      const firstKey = activeKeys[0]?.key || "";
      setConfigs((prev) => {
        const next = { ...prev };
        for (const t of toolList) {
          if (!next[t.id]) {
            next[t.id] = {
              endpoint: defaultEndpoint,
              apiKey: firstKey,
              models: defaultModelsForTool(t.id, firstModel),
            };
          }
        }
        return next;
      });
    } catch (error) {
      toast.error("Failed to load CLI tool config state", {
        description: error instanceof Error ? error.message : undefined,
      });
    } finally {
      setLoading(false);
    }
  }, [info?.baseUrl]);

  useEffect(() => {
    if (!infoLoading) load();
  }, [infoLoading, load]);

  const handleRefresh = async () => {
    await refreshInfo();
    await load();
  };

  const clearToolPreview = (toolId: string) => {
    setPreviews((prev) => ({ ...prev, [toolId]: [] }));
    setAliases((prev) => ({ ...prev, [toolId]: "" }));
  };

  const handleConfigChange = useCallback((toolId: string, patch: Partial<ToolConfig>) => {
    setConfigs((prev) => ({ ...prev, [toolId]: { ...prev[toolId], ...patch } }));
    setPreviews((prev) => ({ ...prev, [toolId]: [] }));
    setAliases((prev) => ({ ...prev, [toolId]: "" }));
  }, []);

  const handleModelChange = useCallback((toolId: string, role: string, value: string) => {
    setConfigs((prev) => ({
      ...prev,
      [toolId]: {
        ...prev[toolId],
        models: { ...prev[toolId]?.models, [role]: value },
      },
    }));
    setPreviews((prev) => ({ ...prev, [toolId]: [] }));
    setAliases((prev) => ({ ...prev, [toolId]: "" }));
  }, []);

  const handleModelsReplace = useCallback((toolId: string, models: Record<string, string>) => {
    setConfigs((prev) => ({
      ...prev,
      [toolId]: { ...prev[toolId], models },
    }));
    setPreviews((prev) => ({ ...prev, [toolId]: [] }));
    setAliases((prev) => ({ ...prev, [toolId]: "" }));
  }, []);

  const validateToolConfig = (toolId: string) => {
    const cfg = configs[toolId];
    if (!cfg?.endpoint?.trim()) return toast.error("Endpoint is required"), false;
    if (!cfg?.apiKey?.trim()) return toast.error("API key is required"), false;
    const hasModel = Object.values(cfg?.models || {}).some((v) => v?.trim());
    if (!hasModel) return toast.error("At least one model is required"), false;
    return true;
  };

  const handlePreview = async (toolId: string) => {
    if (!validateToolConfig(toolId)) return;
    const cfg = configs[toolId];
    setPreviewingTool(toolId);
    try {
      const result = await goApi.previewCliToolConfigs({
        endpoint: cfg.endpoint, apiKey: cfg.apiKey, models: cfg.models, tools: [toolId],
      });
      setPreviews((prev) => ({ ...prev, [toolId]: result.previews || [] }));
      setAliases((prev) => ({ ...prev, [toolId]: result.aliases ? JSON.stringify(result.aliases) : "" }));
      toast.success("Preview generated");
    } catch (error) {
      toast.error("Preview failed", { description: error instanceof Error ? error.message : undefined });
    } finally {
      setPreviewingTool("");
    }
  };

  const handleApply = async (toolId: string) => {
    if (!validateToolConfig(toolId)) return;
    const cfg = configs[toolId];
    setApplyingTool(toolId);
    try {
      const result = await goApi.applyCliToolConfigs({
        endpoint: cfg.endpoint, apiKey: cfg.apiKey, models: cfg.models, tools: [toolId],
      });
      const failures = (result.results || []).filter((r: { applied: boolean }) => !r.applied);
      toast[failures.length > 0 ? "error" : "success"](failures.length > 0 ? "Config failed" : "CLI config applied");
      clearToolPreview(toolId);
      await load();
    } catch (error) {
      toast.error("Apply failed", { description: error instanceof Error ? error.message : undefined });
    } finally {
      setApplyingTool("");
    }
  };

  const handleRestore = async (toolId: string) => {
    setRestoring(true);
    try {
      const result = await goApi.restoreCliToolConfigs([toolId]);
      const first = result.results?.[0];
      first?.restored ? toast.success("Config restored") : toast.error("Restore failed", { description: first?.error });
      await load();
    } catch (error) {
      toast.error("Restore failed", { description: error instanceof Error ? error.message : undefined });
    } finally {
      setRestoring(false);
    }
  };

  if (loading || infoLoading) {
    return (
      <div className="space-y-6">
        <Skeleton className="h-8 w-56" />
        <Skeleton className="h-40 w-full rounded-xl" />
        <Skeleton className="h-40 w-full rounded-xl" />
        <Skeleton className="h-40 w-full rounded-xl" />
      </div>
    );
  }

  const emptyConfig: ToolConfig = { endpoint: "", apiKey: "", models: {} };

  return (
    <motion.div variants={containerVariants} initial="hidden" animate="visible" className="space-y-6">
      <motion.div variants={itemVariants} className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <div>
          <h1 className="flex items-center gap-2 text-2xl font-bold">
            <TerminalSquare className="size-6" />CLI Tools
          </h1>
          <p className="mt-1 text-muted-foreground">
            Route Claude Code, OpenCode, and Codex CLI through dntproxy.
          </p>
        </div>
        <Button variant="outline" size="icon" onClick={handleRefresh} disabled={loading}>
          <RefreshCw className="size-4" />
        </Button>
      </motion.div>

      <motion.div variants={itemVariants} className="grid gap-4">
        {tools.map((tool) => (
          <ToolCard
            key={tool.id}
            tool={tool}
            config={configs[tool.id] || emptyConfig}
            localEndpoint={info?.localUrl || ""}
            tunnelUrl={info?.tunnelUrl || ""}
            models={models}
            keys={keys}
            previews={previews[tool.id] || []}
            alias={aliases[tool.id] || ""}
            previewing={previewingTool === tool.id}
            applying={applyingTool === tool.id}
            restoring={restoring}
            onConfigChange={handleConfigChange}
            onModelChange={handleModelChange}
            onModelsReplace={handleModelsReplace}
            onPreview={handlePreview}
            onApply={handleApply}
            onRestore={handleRestore}
          />
        ))}
      </motion.div>
    </motion.div>
  );
}
