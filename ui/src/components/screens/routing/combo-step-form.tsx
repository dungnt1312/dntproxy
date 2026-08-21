import { useState } from "react";
import { Check, Pin, Plus, Search, Shuffle } from "lucide-react";
import { ProviderLogo } from "@/components/connections/ProviderLogo";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { getProviderLabel, providersMatch } from "@/lib/provider-registry";
import type { ConnectionOption, UiModel } from "./types";
import type { ComboStep } from "./combo-step-builder";
import { getModelDisplayName, getModelSearchText, isRegistryModel } from "./model-display";

interface ComboStepFormProps {
  connections: ConnectionOption[];
  models: UiModel[];
  selectedProvider: string;
  selectedModel: string;
  accountMode: "auto" | "pinned";
  selectedAccount: string;
  onProviderChange: (value: string) => void;
  onModelChange: (value: string) => void;
  onAccountChange: (mode: "auto" | "pinned", accountId?: string) => void;
  onAddStep: () => void;
  serializeStep: (step: ComboStep) => string;
}

export function ComboStepForm({
  connections,
  models,
  selectedProvider,
  selectedModel,
  accountMode,
  selectedAccount,
  onProviderChange,
  onModelChange,
  onAccountChange,
  onAddStep,
  serializeStep,
}: ComboStepFormProps) {
  const [modelSearch, setModelSearch] = useState("");
  const providers = Array.from(new Set(models.filter(isRegistryModel).map((model) => model.provider))).sort();
  const q = modelSearch.trim().toLowerCase();
  const availableModels = models
    .filter((model) => isRegistryModel(model) && model.provider === selectedProvider)
    .filter((model) => !q || getModelSearchText(model).includes(q))
    .slice(0, 60);
  const availableAccounts = connections.filter(
    (connection) =>
      connection.isActive !== false &&
      (providersMatch(connection.provider, selectedProvider) ||
        providersMatch((connection as { publicPrefix?: string }).publicPrefix, selectedProvider)),
  );
  const previewStep: ComboStep | null = selectedProvider && selectedModel
    ? {
        id: "preview",
        provider: selectedProvider,
        model: selectedModel,
        accountMode,
        accountId: accountMode === "pinned" ? selectedAccount : undefined,
        order: 0,
      }
    : null;
  const canAdd = selectedProvider && selectedModel && (accountMode === "auto" || selectedAccount);

  return (
    <section className="space-y-4 rounded-lg border bg-muted/20 p-4" aria-label="Add combo step">
      <div>
        <h3 className="text-sm font-semibold">Add Step</h3>
        <p className="text-xs text-muted-foreground">Choose provider, model, and optional pinned account.</p>
      </div>
      <div className="grid gap-3 md:grid-cols-2">
        <div className="space-y-1.5">
          <Label className="text-xs">Provider</Label>
          <Select value={selectedProvider} onValueChange={onProviderChange}>
            <SelectTrigger className="h-9 w-full">
              <SelectValue placeholder="Select provider" />
            </SelectTrigger>
            <SelectContent>
              {providers.map((provider) => (
                <SelectItem key={provider} value={provider}>
                  <ProviderLogo provider={provider} size={16} />
                  {getProviderLabel(provider)}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
        <div className="space-y-1.5">
          <Label className="text-xs">Account</Label>
          <Select
            value={accountMode === "auto" ? "auto" : selectedAccount}
            onValueChange={(value) => onAccountChange(value === "auto" ? "auto" : "pinned", value === "auto" ? undefined : value)}
            disabled={!selectedProvider}
          >
            <SelectTrigger className="h-9 w-full">
              <SelectValue placeholder="Auto-select" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="auto">
                <Shuffle className="h-3.5 w-3.5" />
                Auto-select
              </SelectItem>
              {availableAccounts.map((account) => (
                <SelectItem key={account.id} value={account.id}>
                  <Pin className="h-3.5 w-3.5" />
                  {account.name}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
      </div>

      <div className="space-y-2">
        <Label className="text-xs">Model</Label>
        <div className="relative">
          <Search className="pointer-events-none absolute left-3 top-1/2 h-3.5 w-3.5 -translate-y-1/2 text-muted-foreground" />
          <Input
            value={modelSearch}
            onChange={(event) => setModelSearch(event.target.value)}
            disabled={!selectedProvider}
            placeholder={selectedProvider ? "Search models…" : "Select provider first…"}
            aria-label="Search combo models"
            autoComplete="off"
            className="h-9 pl-9"
          />
        </div>
        <div className="max-h-48 space-y-1 overflow-y-auto rounded-md border bg-background p-1">
          {!selectedProvider ? (
            <p className="px-2 py-6 text-center text-sm text-muted-foreground">Select provider first.</p>
          ) : availableModels.length === 0 ? (
            <p className="px-2 py-6 text-center text-sm text-muted-foreground">No matching models.</p>
          ) : (
            availableModels.map((model) => {
              const value = model.modelId || model.id;
              const isSelected = selectedModel === value;
              return (
                <button
                  key={model.id}
                  type="button"
                  onClick={() => onModelChange(value)}
                  className="flex w-full min-w-0 items-center gap-2 rounded px-2 py-2 text-left text-sm hover:bg-muted/60 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
                  aria-pressed={isSelected}
                >
                  <span className="min-w-0 flex-1 truncate">{getModelDisplayName(model)}</span>
                  <code className="hidden max-w-[220px] truncate font-mono text-xs text-muted-foreground sm:block">
                    {model.id}
                  </code>
                  {isSelected && <Check className="h-4 w-4 shrink-0 text-primary" aria-hidden="true" />}
                </button>
              );
            })
          )}
        </div>
      </div>

      {previewStep && (
        <div className="rounded-md border bg-background px-3 py-2">
          <p className="mb-1 text-xs font-medium text-muted-foreground">Target payload</p>
          <code className="block truncate font-mono text-xs" title={serializeStep(previewStep)}>
            {serializeStep(previewStep)}
          </code>
        </div>
      )}
      <div className="flex justify-end">
        <Button type="button" onClick={onAddStep} disabled={!canAdd} className="h-9 gap-2">
          <Plus className="h-4 w-4" />
          Add Step
        </Button>
      </div>
    </section>
  );
}

export function makeComboStep(step: Omit<ComboStep, "id" | "order">, order: number): ComboStep {
  return { ...step, id: `step-${Date.now()}-${order}`, order };
}
