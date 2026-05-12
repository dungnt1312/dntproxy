import { useMemo, useState } from "react";
import { Search } from "lucide-react";
import { ProviderLogo } from "@/components/connections/ProviderLogo";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { getProviderLabel } from "@/lib/provider-registry";
import type { UiModel } from "./types";
import { getModelDisplayName, getModelSearchText, isRegistryModel, sortModels } from "./model-display";

interface ModelPickerProps {
  models: UiModel[];
  value: string;
  onChange: (value: string) => void;
  allowManual?: boolean;
}

export function ModelPicker({ models, value, onChange, allowManual }: ModelPickerProps) {
  const [search, setSearch] = useState("");
  const [manualValue, setManualValue] = useState(value);
  const q = search.trim().toLowerCase();

  const registryModels = useMemo(() => sortModels(models.filter(isRegistryModel)), [models]);
  const filteredModels = useMemo(() => {
    if (!q) return registryModels.slice(0, 80);
    return registryModels.filter((model) => getModelSearchText(model).includes(q)).slice(0, 80);
  }, [q, registryModels]);

  return (
    <div className="space-y-3">
      <div className="relative">
        <Search className="pointer-events-none absolute left-3 top-1/2 h-3.5 w-3.5 -translate-y-1/2 text-muted-foreground" />
        <Input
          value={search}
          onChange={(event) => setSearch(event.target.value)}
          placeholder="Search registry models..."
          className="h-9 pl-9"
        />
      </div>
      <div className="max-h-72 space-y-2 overflow-y-auto rounded-md border p-2">
        {filteredModels.length === 0 ? (
          <p className="px-2 py-6 text-center text-sm text-muted-foreground">No registry model matches.</p>
        ) : (
          filteredModels.map((model) => (
            <button
              key={model.id}
              type="button"
              onClick={() => onChange(model.id)}
              className="flex w-full items-start gap-3 rounded-md border p-3 text-left transition-colors hover:bg-muted/50 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
              aria-pressed={value === model.id}
            >
              <ProviderLogo provider={model.provider} size={20} />
              <div className="min-w-0 flex-1">
                <div className="flex flex-wrap items-center gap-2">
                  <span className="truncate text-sm font-medium">{getModelDisplayName(model)}</span>
                  <Badge variant="secondary" className="text-[10px]">
                    {getProviderLabel(model.provider)}
                  </Badge>
                </div>
                <code className="mt-1 block truncate font-mono text-xs text-muted-foreground">{model.id}</code>
              </div>
            </button>
          ))
        )}
      </div>
      {allowManual && (
        <div className="space-y-2 rounded-md border border-dashed p-3">
          <p className="text-xs font-medium text-muted-foreground">Manual target</p>
          <div className="flex gap-2">
            <Input value={manualValue} onChange={(event) => setManualValue(event.target.value)} placeholder="provider/model" />
            <Button type="button" variant="outline" onClick={() => onChange(manualValue.trim())}>
              Use
            </Button>
          </div>
        </div>
      )}
    </div>
  );
}
