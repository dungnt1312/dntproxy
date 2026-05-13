import { useState } from "react";
import { Label } from "@/components/ui/label";
import type { ConnectionOption, UiModel } from "./types";
import { ComboStepForm, makeComboStep } from "./combo-step-form";
import { ComboStepList } from "./combo-step-list";

export interface ComboStep {
  id: string;
  provider: string;
  model: string;
  accountMode: "auto" | "pinned";
  accountId?: string;
  order: number;
}

interface ComboStepBuilderProps {
  steps: ComboStep[];
  connections: ConnectionOption[];
  models: UiModel[];
  onChange: (steps: ComboStep[]) => void;
  serializeStep: (step: ComboStep) => string;
}

export function ComboStepBuilder({ steps, connections, models, onChange, serializeStep }: ComboStepBuilderProps) {
  const [selectedProvider, setSelectedProvider] = useState("");
  const [selectedModel, setSelectedModel] = useState("");
  const [accountMode, setAccountMode] = useState<"auto" | "pinned">("auto");
  const [selectedAccount, setSelectedAccount] = useState("");

  function handleProviderChange(provider: string) {
    setSelectedProvider(provider);
    setSelectedModel("");
    setAccountMode("auto");
    setSelectedAccount("");
  }

  function handleAccountChange(mode: "auto" | "pinned", accountId?: string) {
    setAccountMode(mode);
    setSelectedAccount(accountId || "");
  }

  function handleAddStep() {
    if (!selectedProvider || !selectedModel) return;
    onChange([
      ...steps,
      makeComboStep(
        {
          provider: selectedProvider,
          model: selectedModel,
          accountMode,
          accountId: accountMode === "pinned" ? selectedAccount : undefined,
        },
        steps.length,
      ),
    ]);
    setAccountMode("auto");
    setSelectedAccount("");
  }

  function handleDeleteStep(stepId: string) {
    onChange(steps.filter((step) => step.id !== stepId).map((step, index) => ({ ...step, order: index })));
  }

  function handleMoveStep(stepId: string, direction: "up" | "down") {
    const index = steps.findIndex((step) => step.id === stepId);
    const targetIndex = direction === "up" ? index - 1 : index + 1;
    if (index < 0 || targetIndex < 0 || targetIndex >= steps.length) return;
    const next = [...steps];
    [next[index], next[targetIndex]] = [next[targetIndex], next[index]];
    onChange(next.map((step, order) => ({ ...step, order })));
  }

  function handleSwitchStepToAuto(stepId: string) {
    onChange(
      steps.map((step) =>
        step.id === stepId
          ? { ...step, accountMode: "auto", accountId: undefined }
          : step,
      ),
    );
  }

  return (
    <div className="space-y-4">
      <ComboStepForm
        connections={connections}
        models={models}
        selectedProvider={selectedProvider}
        selectedModel={selectedModel}
        accountMode={accountMode}
        selectedAccount={selectedAccount}
        onProviderChange={handleProviderChange}
        onModelChange={setSelectedModel}
        onAccountChange={handleAccountChange}
        onAddStep={handleAddStep}
        serializeStep={serializeStep}
      />
      <section className="space-y-2" aria-label="Combo steps">
        <div className="flex flex-col gap-1 sm:flex-row sm:items-end sm:justify-between">
          <Label className="text-xs">Combo Steps ({steps.length})</Label>
          <p className="text-xs text-muted-foreground">Order matters. Routing tries these targets using the active combo strategy.</p>
        </div>
        <ComboStepList
          steps={steps}
          connections={connections}
          models={models}
          onMove={handleMoveStep}
          onDelete={handleDeleteStep}
          onSwitchToAuto={handleSwitchStepToAuto}
        />
      </section>
    </div>
  );
}
