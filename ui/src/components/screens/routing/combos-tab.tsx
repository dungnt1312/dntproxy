import { useMemo, useState } from "react";
import { GitBranch, Loader2, Pencil, Plus, TerminalSquare, Trash2 } from "lucide-react";
import { toast } from "sonner";
import { goApi } from "@/lib/go-api";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog";
import type { ComboData, ConnectionOption, UiModel } from "./types";
import { ComboStepBuilder, type ComboStep } from "./combo-step-builder";
import { RoutingEmptyState } from "./routing-empty-state";
import { RoutingToolbar } from "./routing-toolbar";
import { getTargetDisplay, parseRoutingTarget, providerPrefixToProvider } from "./routing-format";

function inferProvidersFromModels(models: string[]): string[] {
  return Array.from(new Set(models.map((model) => parseRoutingTarget(model).provider.toUpperCase()).filter(Boolean)));
}

function parseModelString(modelStr: string, order: number): ComboStep {
  const parsed = parseRoutingTarget(modelStr);
  return {
    id: `step-${order}`,
    provider: providerPrefixToProvider(parsed.provider),
    model: parsed.model,
    accountMode: parsed.connectionId ? "pinned" : "auto",
    accountId: parsed.connectionId || undefined,
    order,
  };
}

function serializeStep(step: ComboStep): string {
  const prefix = step.provider === "kiro" ? "kr" : step.provider === "openai" ? "oai" : step.provider;
  const base = `${prefix}/${step.model}`;
  return step.accountMode === "pinned" && step.accountId ? `${base}@${step.accountId}` : base;
}

export interface CombosTabProps {
  combos: ComboData[];
  connections: ConnectionOption[];
  models: UiModel[];
  loading: boolean;
  hasLoadError?: boolean;
  onRefresh: () => Promise<void>;
  onOpenLogModal: (comboName: string, allowedProviders: string[]) => void;
}

export default function CombosTab({
  combos,
  connections,
  models,
  loading,
  hasLoadError,
  onRefresh,
  onOpenLogModal,
}: CombosTabProps) {
  const [dialogOpen, setDialogOpen] = useState(false);
  const [deleteTarget, setDeleteTarget] = useState<ComboData | null>(null);
  const [selectedCombo, setSelectedCombo] = useState<ComboData | null>(null);
  const [formName, setFormName] = useState("");
  const [formSteps, setFormSteps] = useState<ComboStep[]>([]);
  const [saving, setSaving] = useState(false);
  const [deleting, setDeleting] = useState(false);
  const [search, setSearch] = useState("");
  const q = search.trim().toLowerCase();

  const filteredCombos = useMemo(() => {
    const sorted = [...combos].sort((a, b) => a.name.localeCompare(b.name));
    if (!q) return sorted;
    return sorted.filter((combo) => `${combo.name} ${combo.models.join(" ")}`.toLowerCase().includes(q));
  }, [combos, q]);

  function openCreateDialog() {
    setSelectedCombo(null);
    setFormName("");
    setFormSteps([]);
    setDialogOpen(true);
  }

  function openEditDialog(combo: ComboData) {
    setSelectedCombo(combo);
    setFormName(combo.name);
    setFormSteps(combo.models.map(parseModelString));
    setDialogOpen(true);
  }

  async function handleSave() {
    const name = formName.trim();
    if (!name) return toast.error("Combo name is required");
    if (formSteps.length === 0) return toast.error("At least one step is required");

    setSaving(true);
    try {
      const payload = {
        name,
        models: [...formSteps].sort((a, b) => a.order - b.order).map(serializeStep),
        setModels: true,
      };
      if (selectedCombo) {
        await goApi.updateCombo(selectedCombo.id, payload);
        toast.success("Combo updated");
      } else {
        await goApi.createCombo(payload);
        toast.success("Combo created");
      }
      setDialogOpen(false);
      await onRefresh();
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "Failed to save combo");
    } finally {
      setSaving(false);
    }
  }

  async function handleDelete() {
    if (!deleteTarget) return;
    setDeleting(true);
    try {
      await goApi.deleteCombo(deleteTarget.id);
      toast.success("Combo deleted");
      setDeleteTarget(null);
      await onRefresh();
    } catch {
      toast.error("Failed to delete combo");
    } finally {
      setDeleting(false);
    }
  }

  return (
    <div className="space-y-4 pt-0">
      <RoutingToolbar
        search={search}
        onSearchChange={setSearch}
        placeholder="Search combos or model targets..."
        summary={`${filteredCombos.length} of ${combos.length} combos`}
        action={
          <Button onClick={openCreateDialog} className="h-9 gap-2">
            <Plus className="h-4 w-4" />
            Create combo
          </Button>
        }
      />

      {loading ? (
        <div className="flex items-center justify-center rounded-lg border bg-card p-16 text-sm text-muted-foreground">
          <Loader2 className="mr-2 h-5 w-5 animate-spin" />
          Loading combos...
        </div>
      ) : filteredCombos.length === 0 ? (
        <RoutingEmptyState
          icon={<GitBranch className="h-5 w-5 text-muted-foreground" />}
          title={q ? "No matching combos" : hasLoadError ? "Combos could not load" : "No combos configured"}
          description={
            q
              ? `No combos match "${search.trim()}".`
              : hasLoadError
                ? "Retry loading routing data from the error banner above."
                : "Create a named fallback chain of model targets."
          }
          action={!q && !hasLoadError ? <Button onClick={openCreateDialog}>Create combo</Button> : undefined}
        />
      ) : (
        <div className="overflow-hidden rounded-lg border bg-card">
          <div className="divide-y">
            {filteredCombos.map((combo) => (
              <div key={combo.id} className="flex flex-col gap-3 p-4 lg:flex-row lg:items-center lg:justify-between">
                <div className="min-w-0">
                  <div className="flex items-center gap-2">
                    <GitBranch className="h-4 w-4 text-muted-foreground" />
                    <p className="truncate text-sm font-semibold">{combo.name}</p>
                  </div>
                  <div className="mt-2 flex flex-wrap gap-1.5">
                    {combo.models.map((model, index) => (
                      <code key={`${model}-${index}`} className="max-w-full truncate rounded bg-muted px-2 py-1 font-mono text-xs text-muted-foreground" title={model}>
                        {index + 1}. {getTargetDisplay(model, models, connections)}
                      </code>
                    ))}
                  </div>
                </div>
                <div className="flex shrink-0 gap-1">
                  <Button variant="ghost" size="icon" onClick={() => onOpenLogModal(combo.name, inferProvidersFromModels(combo.models))} aria-label={`View logs for combo ${combo.name}`}>
                    <TerminalSquare className="h-4 w-4" />
                  </Button>
                  <Button variant="ghost" size="icon" onClick={() => openEditDialog(combo)} aria-label={`Edit combo ${combo.name}`}>
                    <Pencil className="h-4 w-4" />
                  </Button>
                  <Button variant="ghost" size="icon" onClick={() => setDeleteTarget(combo)} aria-label={`Delete combo ${combo.name}`}>
                    <Trash2 className="h-4 w-4 text-destructive" />
                  </Button>
                </div>
              </div>
            ))}
          </div>
        </div>
      )}

      <Dialog open={dialogOpen} onOpenChange={setDialogOpen}>
        <DialogContent className="max-h-[90vh] overflow-y-auto sm:max-w-2xl">
          <DialogHeader>
            <DialogTitle>{selectedCombo ? "Edit combo" : "Create combo"}</DialogTitle>
            <DialogDescription>Build each combo step in order. Pinned accounts stay tied to that connection.</DialogDescription>
          </DialogHeader>
          <div className="space-y-4">
            <div className="space-y-2">
              <Label htmlFor="combo-name">Combo name</Label>
              <Input id="combo-name" value={formName} onChange={(event) => setFormName(event.target.value)} placeholder="e.g. primary-backup" />
            </div>
            <ComboStepBuilder steps={formSteps} connections={connections} models={models} onChange={setFormSteps} />
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setDialogOpen(false)}>Cancel</Button>
            <Button onClick={handleSave} disabled={saving || formSteps.length === 0}>
              {saving ? <Loader2 className="mr-2 h-3.5 w-3.5 animate-spin" /> : null}
              {selectedCombo ? "Update combo" : "Create combo"}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <AlertDialog open={!!deleteTarget} onOpenChange={(open) => { if (!open) setDeleteTarget(null); }}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Delete combo</AlertDialogTitle>
            <AlertDialogDescription>
              Delete "{deleteTarget?.name}"? This removes the combo name and its model list.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={deleting}>Cancel</AlertDialogCancel>
            <AlertDialogAction
              onClick={(event) => {
                event.preventDefault();
                handleDelete();
              }}
              disabled={deleting}
              className="bg-destructive text-white hover:bg-destructive/90"
            >
              {deleting ? "Deleting..." : "Delete"}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  );
}
