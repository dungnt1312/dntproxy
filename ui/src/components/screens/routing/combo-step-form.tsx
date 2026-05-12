import { Pin, Plus, Shuffle } from "lucide-react";
import { ProviderLogo } from "@/components/connections/ProviderLogo";
import { Button } from "@/components/ui/button";
import { Label } from "@/components/ui/label";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { getProviderLabel } from "@/lib/provider-registry";
import type { ConnectionOption, UiModel } from "./types";
import type { ComboStep } from "./combo-step-builder";
import { getModelDisplayName, isRegistryModel } from "./model-display";

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
}: ComboStepFormProps) {
  const providers = Array.from(new Set(models.filter(isRegistryModel).map((model) => model.provider))).sort();
  const availableModels = models.filter((model) => isRegistryModel(model) && model.provider === selectedProvider);
  const availableAccounts = connections.filter((connection) => connection.provider === selectedProvider);
  const canAdd = selectedProvider && selectedModel && (accountMode === "auto" || selectedAccount);

  return (
    <div className="space-y-4 rounded-lg border bg-muted/20 p-4">
      <div className="grid gap-3 md:grid-cols-3">
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
          <Label className="text-xs">Model</Label>
          <Select value={selectedModel} onValueChange={onModelChange} disabled={!selectedProvider}>
            <SelectTrigger className="h-9 w-full">
              <SelectValue placeholder="Select model" />
            </SelectTrigger>
            <SelectContent>
              {availableModels.map((model) => (
                <SelectItem key={model.id} value={model.modelId || model.id}>
                  {getModelDisplayName(model)}
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
      {selectedProvider && selectedModel && (
        <code className="block rounded-md bg-background px-3 py-2 font-mono text-xs">
          {selectedProvider}/{selectedModel}
          {accountMode === "pinned" && selectedAccount ? `@${selectedAccount}` : ""}
        </code>
      )}
      <Button type="button" onClick={onAddStep} disabled={!canAdd} className="h-9 w-full gap-2">
        <Plus className="h-4 w-4" />
        Add step
      </Button>
    </div>
  );
}

export function makeComboStep(step: Omit<ComboStep, "id" | "order">, order: number): ComboStep {
  return { ...step, id: `step-${Date.now()}-${order}`, order };
}
