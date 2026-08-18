import { useEffect, useState, useCallback, useMemo } from "react";
import {
  RotateCcw,
  Save,
  Loader2,
  Sparkles,
  FileText,
  Boxes,
  GitBranch,
  Link2,
  ChevronDown,
} from "lucide-react";
import {
  Card,
  CardContent,
  CardHeader,
  CardTitle,
  CardDescription,
} from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Button } from "@/components/ui/button";
import { Switch } from "@/components/ui/switch";
import { Skeleton } from "@/components/ui/skeleton";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { toast } from "sonner";
import { goApi } from "@/lib/go-api";
import { useProviderCatalog } from "@/lib/use-provider-catalog";

interface SettingsData {
  id: string;
  defaultRoutingStrategy: string;
  connectionStrategy: string;
  connectionStrategies: Record<string, string>;
  compressionEnabled: boolean;
  compressionMinLength: number;
  compressionLogSavings: boolean;
  logBodies: boolean;
  defaultModels: Record<string, string[]>;
  disableImageGeneration: boolean;
}

const DEFAULT_SETTINGS: SettingsData = {
  id: "",
  defaultRoutingStrategy: "fallback",
  connectionStrategy: "fill-first",
  connectionStrategies: {},
  compressionEnabled: false,
  compressionMinLength: 500,
  compressionLogSavings: true,
  logBodies: false,
  defaultModels: {},
  disableImageGeneration: false,
};

const PROVIDERS = [
  { id: "kiro", label: "Kiro" },
  { id: "openai", label: "OpenAI" },
  { id: "openai-compatible", label: "OpenAI Compatible" },
  { id: "xai", label: "xAI / Grok" },
  { id: "qwen", label: "Qwen" },
  { id: "glm", label: "GLM" },
  { id: "minimax", label: "MiniMax" },
  { id: "anthropic", label: "Anthropic" },
  { id: "cline", label: "Cline" },
  { id: "gemini", label: "Gemini" },
];

const COMBO_STRATEGIES = [
  {
    value: "fallback",
    label: "Fallback",
    hint: "Try models in list order. Next model only runs if the previous one fails.",
  },
  {
    value: "round-robin",
    label: "Round-robin",
    hint: "Rotate the starting model on each request, then fall through on failure.",
  },
];

const CONNECTION_STRATEGIES = [
  {
    value: "fill-first",
    label: "Fill first",
    hint: "Keep using the best available account until it fails. Best cache hit rate.",
  },
  {
    value: "weighted-random",
    label: "Weighted random",
    hint: "Pick at random; higher connection weight gets more traffic.",
  },
  {
    value: "priority-fallback",
    label: "Priority first",
    hint: "Always try the highest-weight account first, then the next on failure.",
  },
  {
    value: "round-robin",
    label: "Round-robin",
    hint: "Rotate evenly across accounts in the same priority group.",
  },
];

const USE_DEFAULT = "__default__";

export default function SettingsScreen() {
  const [settings, setSettings] = useState<SettingsData>(DEFAULT_SETTINGS);
  const [saved, setSaved] = useState<SettingsData>(DEFAULT_SETTINGS);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);

  const hasChanges = useMemo(
    () => JSON.stringify(settings) !== JSON.stringify(saved),
    [settings, saved],
  );

  const fetchSettings = useCallback(async () => {
    try {
      setLoading(true);
      const json = await goApi.getSettings();
      setSettings(json);
      setSaved(json);
    } catch {
      toast.error("Failed to load settings");
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    fetchSettings();
  }, [fetchSettings]);

  const updateField = <K extends keyof SettingsData>(
    key: K,
    value: SettingsData[K],
  ) => {
    setSettings((prev) => ({ ...prev, [key]: value }));
  };

  const handleSave = async () => {
    try {
      setSaving(true);
      const json = await goApi.updateSettings({
        defaultRoutingStrategy: settings.defaultRoutingStrategy,
        connectionStrategy: settings.connectionStrategy,
        connectionStrategies: settings.connectionStrategies,
        compressionEnabled: settings.compressionEnabled,
        compressionMinLength: settings.compressionMinLength,
        compressionLogSavings: settings.compressionLogSavings,
        logBodies: settings.logBodies,
        defaultModels: settings.defaultModels,
        disableImageGeneration: settings.disableImageGeneration,
      });
      setSettings(json);
      setSaved(json);
      toast.success("Settings saved");
    } catch {
      toast.error("Failed to save settings");
    } finally {
      setSaving(false);
    }
  };

  const handleDiscard = () => {
    setSettings(saved);
    toast.info("Changes discarded");
  };

  if (loading) {
    return (
      <div className="mx-auto max-w-3xl space-y-6">
        <div>
          <Skeleton className="h-8 w-40" />
          <Skeleton className="mt-2 h-4 w-72" />
        </div>
        {Array.from({ length: 3 }).map((_, i) => (
          <Card key={i}>
            <CardContent className="space-y-3 p-6">
              <Skeleton className="h-4 w-32" />
              <Skeleton className="h-10 w-full" />
            </CardContent>
          </Card>
        ))}
      </div>
    );
  }

  const comboHint =
    COMBO_STRATEGIES.find((s) => s.value === settings.defaultRoutingStrategy)
      ?.hint ?? "";
  const connHint =
    CONNECTION_STRATEGIES.find((s) => s.value === settings.connectionStrategy)
      ?.hint ?? "";

  return (
    <div className="mx-auto max-w-3xl space-y-6 pb-24">
      <div className="flex flex-col gap-3 sm:flex-row sm:items-end sm:justify-between">
        <div>
          <h1 className="text-2xl font-bold">Settings</h1>
          <p className="mt-1 text-sm text-muted-foreground">
            How this proxy picks accounts and logs traffic.
          </p>
        </div>
        <div className="flex items-center gap-2">
          {hasChanges && (
            <span className="text-xs text-amber-600">Unsaved changes</span>
          )}
          <Button
            variant="outline"
            size="sm"
            onClick={handleDiscard}
            disabled={!hasChanges || saving}
          >
            <RotateCcw className="size-4" />
            Discard
          </Button>
          <Button
            size="sm"
            onClick={handleSave}
            disabled={!hasChanges || saving}
          >
            {saving ? (
              <Loader2 className="size-4 animate-spin" />
            ) : (
              <Save className="size-4" />
            )}
            Save
          </Button>
        </div>
      </div>

      <Card>
        <CardHeader>
          <div className="flex items-center gap-2">
            <GitBranch className="size-5 text-indigo-600" />
            <CardTitle className="text-base">Routing</CardTitle>
          </div>
          <CardDescription>
            Two independent choices: which model in a combo to try next, and
            which account to use inside a provider.
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-6 p-6 pt-0">
          <div className="space-y-2">
            <Label htmlFor="comboStrategy">Combo models</Label>
            <Select
              value={settings.defaultRoutingStrategy}
              onValueChange={(value) =>
                updateField("defaultRoutingStrategy", value)
              }
            >
              <SelectTrigger id="comboStrategy" className="max-w-xs">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {COMBO_STRATEGIES.map((opt) => (
                  <SelectItem key={opt.value} value={opt.value}>
                    {opt.label}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
            <p className="text-xs text-muted-foreground">{comboHint}</p>
          </div>

          <div className="space-y-2">
            <Label htmlFor="connectionStrategy">Accounts (default)</Label>
            <Select
              value={settings.connectionStrategy}
              onValueChange={(value) =>
                updateField("connectionStrategy", value)
              }
            >
              <SelectTrigger id="connectionStrategy" className="max-w-xs">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {CONNECTION_STRATEGIES.map((opt) => (
                  <SelectItem key={opt.value} value={opt.value}>
                    {opt.label}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
            <p className="text-xs text-muted-foreground">{connHint}</p>
          </div>

          <div className="space-y-3 border-t pt-4">
            <div className="flex items-center gap-2">
              <Link2 className="size-4 text-muted-foreground" />
              <div>
                <p className="text-sm font-medium">Per-provider accounts</p>
                <p className="text-xs text-muted-foreground">
                  Override the default only when a provider should pick accounts
                  differently. “Use default” follows the setting above.
                </p>
              </div>
            </div>
            <div className="space-y-2">
              {PROVIDERS.map((p) => {
                const override = settings.connectionStrategies?.[p.id];
                const value = override ?? USE_DEFAULT;
                return (
                  <div
                    key={p.id}
                    className="flex flex-col gap-1 sm:flex-row sm:items-center sm:gap-3"
                  >
                    <span className="w-40 shrink-0 text-sm">{p.label}</span>
                    <Select
                      value={value}
                      onValueChange={(next) => {
                        const map = { ...settings.connectionStrategies };
                        if (next === USE_DEFAULT) {
                          delete map[p.id];
                        } else {
                          map[p.id] = next;
                        }
                        updateField("connectionStrategies", map);
                      }}
                    >
                      <SelectTrigger className="max-w-xs">
                        <SelectValue />
                      </SelectTrigger>
                      <SelectContent>
                        <SelectItem value={USE_DEFAULT}>
                          Use default ({settings.connectionStrategy})
                        </SelectItem>
                        {CONNECTION_STRATEGIES.map((opt) => (
                          <SelectItem key={opt.value} value={opt.value}>
                            {opt.label}
                          </SelectItem>
                        ))}
                      </SelectContent>
                    </Select>
                  </div>
                );
              })}
            </div>
          </div>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <div className="flex items-center gap-2">
            <FileText className="size-5 text-blue-600" />
            <CardTitle className="text-base">Requests & logs</CardTitle>
          </div>
          <CardDescription>
            Shrink noisy tool output before it hits the model, and optionally
            keep raw bodies for debugging.
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-5 p-6 pt-0">
          <div className="flex items-center justify-between gap-4">
            <div className="space-y-0.5">
              <Label htmlFor="compEnabled" className="flex items-center gap-1.5">
                <Sparkles className="size-3.5 text-violet-600" />
                Compress tool results
              </Label>
              <p className="text-xs text-muted-foreground">
                Detects git/test/ls/log/JSON dumps and stores a shorter
                equivalent. Fail-open: bad parse leaves the original body.
              </p>
            </div>
            <Switch
              id="compEnabled"
              checked={settings.compressionEnabled}
              onCheckedChange={(v) => updateField("compressionEnabled", v)}
            />
          </div>

          <div className="space-y-2">
            <Label htmlFor="compMin">Minimum size to compress</Label>
            <Input
              id="compMin"
              type="number"
              min={1}
              disabled={!settings.compressionEnabled}
              value={settings.compressionMinLength}
              onChange={(e) =>
                updateField(
                  "compressionMinLength",
                  Math.max(1, parseInt(e.target.value, 10) || 500),
                )
              }
              className="max-w-xs"
            />
            <p className="text-xs text-muted-foreground">
              Bytes. Smaller tool results are left unchanged.
            </p>
          </div>

          <div className="flex items-center justify-between gap-4">
            <div className="space-y-0.5">
              <Label htmlFor="compLog">Record compression savings</Label>
              <p className="text-xs text-muted-foreground">
                Writes original vs compressed size onto each request log.
              </p>
            </div>
            <Switch
              id="compLog"
              disabled={!settings.compressionEnabled}
              checked={settings.compressionLogSavings}
              onCheckedChange={(v) => updateField("compressionLogSavings", v)}
            />
          </div>

          <div className="flex items-center justify-between gap-4 border-t pt-4">
            <div className="space-y-0.5">
              <Label htmlFor="logBodies">Store request/response bodies</Label>
              <p className="text-xs text-muted-foreground">
                Persists payloads in SQLite for log detail. Grows the database
                and may include user content. Off by default.
              </p>
            </div>
            <Switch
              id="logBodies"
              checked={settings.logBodies}
              onCheckedChange={(v) => updateField("logBodies", v)}
            />
          </div>

          <div className="flex items-center justify-between gap-4 border-t pt-4">
            <div className="space-y-0.5">
              <Label htmlFor="disableImages">Disable image generation</Label>
              <p className="text-xs text-muted-foreground">
                Returns 404 on <code>/v1/images/generations</code> and edits.
                Live after save.
              </p>
            </div>
            <Switch
              id="disableImages"
              checked={settings.disableImageGeneration}
              onCheckedChange={(v) => updateField("disableImageGeneration", v)}
            />
          </div>
        </CardContent>
      </Card>

      <DefaultModelsCard settings={settings} updateField={updateField} />
    </div>
  );
}

function listsEqual(a: string[], b: string[]): boolean {
  if (a.length !== b.length) return false;
  return a.every((v, i) => v === b[i]);
}

function DefaultModelsCard({
  settings,
  updateField,
}: {
  settings: SettingsData;
  updateField: <K extends keyof SettingsData>(
    key: K,
    value: SettingsData[K],
  ) => void;
}) {
  const [openId, setOpenId] = useState<string | null>(null);
  const { providers, loading } = useProviderCatalog();
  const custom = settings.defaultModels || {};

  const rows = useMemo(() => {
    const byId = new Map(providers.map((p) => [p.id, p]));
    const ids = PROVIDERS.map((p) => p.id);
    for (const p of providers) {
      if (!ids.includes(p.id) && (p.recommendedModels?.length || custom[p.id]?.length)) {
        ids.push(p.id);
      }
    }
    return ids.map((id) => {
      const catalog = byId.get(id);
      const builtIn = catalog?.recommendedModels?.filter(Boolean) ?? [];
      const override = custom[id];
      const isCustom = Array.isArray(override) && override.length > 0;
      const models = isCustom ? override : builtIn;
      return {
        id,
        label: catalog?.name || PROVIDERS.find((p) => p.id === id)?.label || id,
        builtIn,
        models,
        isCustom,
      };
    });
  }, [providers, custom]);

  const handleModelsChange = (providerId: string, value: string, builtIn: string[]) => {
    const list = value
      .split(/[\n,]+/)
      .map((s) => s.trim())
      .filter((s) => s.length > 0);
    const next = { ...settings.defaultModels };
    if (list.length === 0 || listsEqual(list, builtIn)) {
      delete next[providerId];
    } else {
      next[providerId] = list;
    }
    updateField("defaultModels", next);
  };

  const resetProvider = (providerId: string) => {
    const next = { ...settings.defaultModels };
    delete next[providerId];
    updateField("defaultModels", next);
  };

  return (
    <Card>
      <CardHeader>
        <div className="flex items-center gap-2">
          <Boxes className="size-5 text-orange-600" />
          <CardTitle className="text-base">Default models on new connections</CardTitle>
        </div>
        <CardDescription>
          These models are pre-selected when you add a connection. Edit a
          provider to change its list. One model per line.
        </CardDescription>
      </CardHeader>
      <CardContent className="space-y-1 p-6 pt-0">
        {loading && rows.every((r) => r.models.length === 0) ? (
          <p className="text-sm text-muted-foreground">Loading built-in model lists…</p>
        ) : null}
        {rows.map((row) => {
          const open = openId === row.id;
          const preview =
            row.models.length === 0
              ? "No models — detect on the connection"
              : row.models.join(", ");
          return (
            <div key={row.id} className="rounded-md border">
              <button
                type="button"
                className="flex w-full items-start justify-between gap-3 px-3 py-2 text-left"
                onClick={() => setOpenId(open ? null : row.id)}
              >
                <span className="min-w-0">
                  <span className="block text-sm font-medium">{row.label}</span>
                  <span className="mt-0.5 block truncate text-xs text-muted-foreground">
                    {preview}
                  </span>
                </span>
                <span className="flex shrink-0 items-center gap-2 pt-0.5 text-xs text-muted-foreground">
                  {row.isCustom ? "custom" : `${row.models.length} models`}
                  <ChevronDown
                    className={`size-4 transition-transform ${open ? "rotate-180" : ""}`}
                  />
                </span>
              </button>
              {open && (
                <div className="space-y-2 border-t px-3 py-2">
                  <textarea
                    className="min-h-[72px] w-full rounded-md border bg-background px-3 py-2 font-mono text-xs"
                    rows={Math.max(3, row.models.length + 1)}
                    value={row.models.join("\n")}
                    onChange={(e) =>
                      handleModelsChange(row.id, e.target.value, row.builtIn)
                    }
                  />
                  {row.isCustom ? (
                    <button
                      type="button"
                      className="text-xs text-muted-foreground hover:text-foreground"
                      onClick={() => resetProvider(row.id)}
                    >
                      Reset to built-in list
                    </button>
                  ) : null}
                </div>
              )}
            </div>
          );
        })}
      </CardContent>
    </Card>
  );
}
