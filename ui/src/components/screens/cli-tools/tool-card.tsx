import { useState } from "react";
import { ArrowRight, CheckCircle2, ChevronDown, FileCode2, FileClock, Loader2, Plus, RotateCcw, Save, X } from "lucide-react";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Textarea } from "@/components/ui/textarea";
import { cn } from "@/lib/utils";

import { ModelCombobox } from "./model-combobox";
import type { ApiKeyOption, CliToolStatus, ModelOption, PreviewItem, ToolConfig } from "./types";
import { TOOL_MODEL_ROLES } from "./types";

interface ToolCardProps {
  tool: CliToolStatus;
  config: ToolConfig;
  localEndpoint: string;
  tunnelUrl: string;
  models: ModelOption[];
  keys: ApiKeyOption[];
  previews: PreviewItem[];
  alias: string;
  previewing: boolean;
  applying: boolean;
  restoring: boolean;
  onConfigChange: (toolId: string, patch: Partial<ToolConfig>) => void;
  onModelChange: (toolId: string, role: string, value: string) => void;
  onModelsReplace: (toolId: string, models: Record<string, string>) => void;
  onPreview: (toolId: string) => void;
  onApply: (toolId: string) => void;
  onRestore: (toolId: string) => void;
}

/** Claude Code: from→to model mapping rows */
function ClaudeModelSection({ config, models, toolId, onModelChange }: {
  config: ToolConfig; models: ModelOption[]; toolId: string;
  onModelChange: (toolId: string, role: string, value: string) => void;
}) {
  const roles = TOOL_MODEL_ROLES.claude;
  return (
    <div className="space-y-2">
      <Label className="text-xs">Model Mapping</Label>
      {roles.map((role) => (
        <div key={role.key} className="flex items-center gap-2">
          <span className="w-16 shrink-0 text-right text-xs font-medium text-muted-foreground">
            {role.label}
          </span>
          <ArrowRight className="size-3 shrink-0 text-muted-foreground" />
          <div className="flex-1">
            <ModelCombobox
              value={config.models[role.key] || ""}
              models={models}
              placeholder={`Select ${role.label.toLowerCase()}...`}
              onChange={(v) => onModelChange(toolId, role.key, v)}
            />
          </div>
        </div>
      ))}
    </div>
  );
}

/** OpenCode: dynamic model list with add/remove */
function OpenCodeModelSection({ config, models, toolId, onModelChange, onModelsReplace }: {
  config: ToolConfig; models: ModelOption[]; toolId: string;
  onModelChange: (toolId: string, role: string, value: string) => void;
  onModelsReplace: (toolId: string, models: Record<string, string>) => void;
}) {
  // Main model (required)
  const mainModel = config.models.model || "";
  // Extra models: all keys except "model"
  const extraKeys = Object.keys(config.models).filter((k) => k !== "model" && k.startsWith("extra_"));

  const addModel = () => {
    const idx = extraKeys.length;
    onModelChange(toolId, `extra_${idx}`, "");
  };

  const removeModel = (key: string) => {
    const next = { ...config.models };
    delete next[key];
    onModelsReplace(toolId, next);
  };

  return (
    <div className="space-y-2">
      <Label className="text-xs">Main Model</Label>
      <ModelCombobox
        value={mainModel}
        models={models}
        placeholder="Select main model..."
        onChange={(v) => onModelChange(toolId, "model", v)}
      />

      <div className="flex items-center justify-between pt-1">
        <Label className="text-xs">Additional Models</Label>
        <Button type="button" variant="ghost" size="sm" className="h-6 gap-1 px-2 text-xs" onClick={addModel}>
          <Plus className="size-3" />Add
        </Button>
      </div>
      {extraKeys.map((key) => (
        <div key={key} className="flex items-center gap-2">
          <div className="flex-1">
            <ModelCombobox
              value={config.models[key] || ""}
              models={models}
              placeholder="Select model..."
              onChange={(v) => onModelChange(toolId, key, v)}
            />
          </div>
          <Button
            type="button" variant="ghost" size="icon"
            className="size-7 shrink-0 text-muted-foreground hover:text-destructive"
            onClick={() => removeModel(key)}
          >
            <X className="size-3.5" />
          </Button>
        </div>
      ))}
    </div>
  );
}

/** Codex / default: single model selector */
function DefaultModelSection({ config, models, toolId, onModelChange }: {
  config: ToolConfig; models: ModelOption[]; toolId: string;
  onModelChange: (toolId: string, role: string, value: string) => void;
}) {
  return (
    <div className="space-y-1.5">
      <Label className="text-xs">Model</Label>
      <ModelCombobox
        value={config.models.model || ""}
        models={models}
        placeholder="Select model..."
        onChange={(v) => onModelChange(toolId, "model", v)}
      />
    </div>
  );
}

export function ToolCard({
  tool, config, localEndpoint, tunnelUrl, models, keys, previews,
  alias, previewing, applying, restoring,
  onConfigChange, onModelChange, onModelsReplace, onPreview, onApply, onRestore,
}: ToolCardProps) {
  const [expanded, setExpanded] = useState(false);
  const toolPreview = previews.find((p) => p.toolId === tool.id);

  return (
    <Card className={cn(tool.configured && "border-emerald-500/30")}>
      <CardHeader
        className="cursor-pointer select-none pb-3"
        onClick={() => setExpanded((v) => !v)}
      >
        <div className="flex items-start justify-between gap-3">
          <div className="min-w-0 flex-1">
            <CardTitle className="text-base">{tool.name}</CardTitle>
            <p className="mt-1 break-all font-mono text-xs text-muted-foreground">
              {tool.configPath}
            </p>
          </div>
          <ChevronDown
            className={cn("size-4 shrink-0 text-muted-foreground transition-transform", expanded && "rotate-180")}
          />
        </div>

        <div className="flex flex-wrap gap-2 pt-2">
          <Badge variant={tool.exists ? "secondary" : "outline"}>
            {tool.exists ? "Exists" : "Missing"}
          </Badge>
          <Badge variant={tool.writable ? "secondary" : "outline"}>
            {tool.writable ? "Writable" : "Dir missing/read-only"}
          </Badge>
          {tool.configured && (
            <Badge className="gap-1 bg-emerald-600">
              <CheckCircle2 className="size-3" />Configured
            </Badge>
          )}
        </div>
      </CardHeader>

      {expanded && (
        <CardContent className="space-y-4 border-t pt-4">
          {/* Endpoint */}
          <div className="space-y-1.5">
            <Label className="text-xs">Endpoint</Label>
            <div className="flex gap-2">
              <Input
                className="text-xs"
                value={config.endpoint}
                onChange={(e) => onConfigChange(tool.id, { endpoint: e.target.value })}
              />
              <Button
                type="button" variant="outline" size="sm"
                onClick={() => onConfigChange(tool.id, { endpoint: localEndpoint })}
              >
                Local
              </Button>
              {tunnelUrl && (
                <Button
                  type="button" variant="outline" size="sm"
                  onClick={() => onConfigChange(tool.id, { endpoint: tunnelUrl })}
                >
                  Tunnel
                </Button>
              )}
            </div>
          </div>

          {/* Model section — per tool type */}
          {tool.id === "claude" && (
            <ClaudeModelSection config={config} models={models} toolId={tool.id} onModelChange={onModelChange} />
          )}
          {tool.id === "opencode" && (
            <OpenCodeModelSection
              config={config} models={models} toolId={tool.id}
              onModelChange={onModelChange} onModelsReplace={onModelsReplace}
            />
          )}
          {tool.id !== "claude" && tool.id !== "opencode" && (
            <DefaultModelSection config={config} models={models} toolId={tool.id} onModelChange={onModelChange} />
          )}

          {/* API Key */}
          <div className="space-y-1.5">
            <Label className="text-xs">API Key</Label>
            {keys.length > 0 && (
              <Select value={config.apiKey} onValueChange={(v) => onConfigChange(tool.id, { apiKey: v })}>
                <SelectTrigger className="text-xs">
                  <SelectValue placeholder="Select API key" />
                </SelectTrigger>
                <SelectContent>
                  {keys.map((k) => (
                    <SelectItem key={k.id} value={k.key}>{k.name}</SelectItem>
                  ))}
                </SelectContent>
              </Select>
            )}
            <Input
              className="font-mono text-xs"
              value={config.apiKey}
              onChange={(e) => onConfigChange(tool.id, { apiKey: e.target.value })}
              placeholder="Paste dntproxy API key"
            />
          </div>

          {/* Alias notice */}
          {alias && (
            <p className="text-xs text-muted-foreground">
              Models with slashes will have aliases created in dntproxy.
            </p>
          )}

          {/* Backup info */}
          {tool.lastBackup && (
            <div className="flex items-center gap-2 text-xs text-muted-foreground">
              <FileClock className="size-3.5" />
              <span className="truncate">{tool.lastBackup}</span>
            </div>
          )}

          {/* Actions */}
          <div className="flex flex-wrap gap-2">
            <Button variant="outline" size="sm" disabled={previewing || applying} onClick={() => onPreview(tool.id)}>
              {previewing ? <Loader2 className="size-3.5 animate-spin" /> : <FileCode2 className="size-3.5" />}
              Preview
            </Button>
            <Button size="sm" disabled={applying || previewing} onClick={() => onApply(tool.id)}>
              {applying ? <Loader2 className="size-3.5 animate-spin" /> : <Save className="size-3.5" />}
              Apply
            </Button>
            {tool.lastBackup && (
              <Button variant="outline" size="sm" disabled={restoring} onClick={() => onRestore(tool.id)}>
                <RotateCcw className="size-3.5" />Restore
              </Button>
            )}
          </div>

          {/* Inline preview */}
          {toolPreview && (
            <div className="space-y-1.5">
              <Label className="text-xs text-muted-foreground">Preview</Label>
              <Textarea
                readOnly
                value={toolPreview.content}
                className="min-h-[180px] font-mono text-xs"
              />
            </div>
          )}
        </CardContent>
      )}
    </Card>
  );
}
