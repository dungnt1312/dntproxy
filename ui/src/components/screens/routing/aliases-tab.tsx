import { useState, useMemo } from "react";
import { Search, Plus, Trash2, Link2, Zap, TerminalSquare, Loader2, ArrowRight } from "lucide-react";
import { toast } from "sonner";
import { goApi } from "@/lib/go-api";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
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
import { AliasMap } from "./types";
import { RoutingCard } from "./routing-card";

export interface AliasesTabProps {
  aliases: AliasMap;
  loading?: boolean;
  onRefresh: () => Promise<void>;
  onOpenLogModal: (alias: string) => void;
}

export default function AliasesTab({ aliases, loading, onRefresh, onOpenLogModal }: AliasesTabProps) {
  const [search, setSearch] = useState("");
  const [showAdd, setShowAdd] = useState(false);
  const [aliasInput, setAliasInput] = useState("");
  const [modelInput, setModelInput] = useState("");
  const [savingAlias, setSavingAlias] = useState(false);
  const [deleteTarget, setDeleteTarget] = useState<string | null>(null);
  const [deleting, setDeleting] = useState(false);

  const q = search.trim().toLowerCase();

  const filteredAliases = useMemo(() => {
    const entries = Object.entries(aliases);
    if (!q) return entries;
    return entries.filter(
      ([alias, target]) =>
        alias.toLowerCase().includes(q) || target.toLowerCase().includes(q)
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
      await onRefresh();
    } catch {
      toast.error("Failed to create alias");
    } finally {
      setSavingAlias(false);
    }
  }

  async function handleDeleteAlias() {
    if (!deleteTarget) return;
    
    setDeleting(true);
    try {
      await goApi.deleteAlias(deleteTarget);
      toast.success("Alias deleted");
      setDeleteTarget(null);
      await onRefresh();
    } catch {
      toast.error("Failed to delete alias");
    } finally {
      setDeleting(false);
    }
  }

  return (
    <div className="space-y-4 outline-none">
      {/* Toolbar */}
      <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <div className="relative flex-1 min-w-0 sm:max-w-md">
          <Search size={14} className="absolute left-3 top-1/2 -translate-y-1/2 text-muted-foreground pointer-events-none" />
          <Input
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            placeholder="Search aliases by name or target..."
            className="pl-9 h-9 text-sm"
          />
        </div>
        <Button
          onClick={() => setShowAdd((v) => !v)}
          className="gap-2 self-start sm:self-auto h-9 bg-emerald-600 hover:bg-emerald-700 text-white"
        >
          <Plus className="h-4 w-4" /> Create Alias
        </Button>
      </div>

      {/* Inline create form */}
      {showAdd && (
        <div className="rounded-xl border bg-muted/20 p-4 space-y-3">
          <div className="flex items-center gap-2 text-sm font-medium text-foreground">
            <Link2 className="h-4 w-4 text-blue-500" />
            New Alias
          </div>
          <div className="grid gap-3 sm:grid-cols-[1fr_auto_1fr_auto]">
            <Input
              value={aliasInput}
              onChange={(e) => setAliasInput(e.target.value)}
              placeholder="Alias name (e.g. gpt-4)"
              className="h-9 text-sm"
            />
            <div className="hidden sm:flex items-center justify-center">
              <ArrowRight className="h-4 w-4 text-muted-foreground/40" />
            </div>
            <Input
              value={modelInput}
              onChange={(e) => setModelInput(e.target.value)}
              placeholder="Target (e.g. oai/gpt-4-turbo)"
              className="h-9 text-sm"
            />
            <div className="flex gap-2">
              <Button
                onClick={handleAddAlias}
                disabled={savingAlias}
                className="h-9"
              >
                {savingAlias ? (
                  <>
                    <Loader2 className="h-3.5 w-3.5 animate-spin mr-1.5" />
                    Saving...
                  </>
                ) : "Save"}
              </Button>
              <Button
                variant="ghost"
                onClick={() => { setShowAdd(false); setAliasInput(""); setModelInput(""); }}
                className="h-9"
              >
                Cancel
              </Button>
            </div>
          </div>
        </div>
      )}

      {/* List */}
      {loading ? (
        <div className="flex items-center justify-center rounded-lg border bg-card p-16 text-sm text-muted-foreground">
          <Loader2 className="h-5 w-5 animate-spin mr-2" />
          Loading aliases...
        </div>
      ) : filteredAliases.length === 0 ? (
        <div className="flex flex-col items-center justify-center rounded-xl border border-dashed py-20 text-center">
          <div className="flex h-14 w-14 items-center justify-center rounded-xl bg-blue-500/10 mb-4">
            <Link2 className="h-6 w-6 text-blue-500" />
          </div>
          <h3 className="mb-1 text-lg font-semibold">No aliases configured</h3>
          <p className="max-w-md text-sm text-muted-foreground">
            {q ? `No aliases match "${q}".` : "Aliases map custom names to provider models. Create one to get started."}
          </p>
        </div>
      ) : (
        <div className="rounded-xl border bg-card shadow-sm overflow-hidden">
          <div className="divide-y divide-border/50">
            {filteredAliases.map(([alias, target]) => (
              <RoutingCard
                key={alias}
                title={alias}
                type="alias"
                targets={[target]}
                actions={
                  <>
                    <Button 
                      variant="ghost"
                      size="icon"
                      onClick={(e) => { e.stopPropagation(); onOpenLogModal(alias); }}
                      className="h-8 w-8 text-muted-foreground hover:text-foreground hover:bg-muted"
                      title={`View logs for alias ${alias}`}
                    >
                      <TerminalSquare className="h-4 w-4" />
                    </Button>
                    <Button
                      variant="ghost"
                      size="icon"
                      onClick={() => setDeleteTarget(alias)}
                      className="h-8 w-8 text-destructive/70 hover:text-destructive hover:bg-destructive/10"
                      title="Delete alias"
                    >
                      <Trash2 className="h-4 w-4" />
                    </Button>
                  </>
                }
              />
            ))}
          </div>
        </div>
      )}

      {/* Tip */}
      <div className="flex items-center gap-2.5 rounded-xl bg-blue-500/5 px-4 py-3 text-xs text-blue-600 dark:text-blue-400 border border-blue-500/15">
        <Zap className="h-4 w-4 shrink-0" />
        <span>
          Aliases act as custom model names mapped directly to a provider's model. Use them in your API requests like any model ID.
        </span>
      </div>

      {/* Delete confirmation */}
      <AlertDialog open={!!deleteTarget} onOpenChange={(open) => { if (!open) setDeleteTarget(null) }}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Delete Alias</AlertDialogTitle>
            <AlertDialogDescription>
              Are you sure you want to delete alias "{deleteTarget}"? 
              Any requests using this alias will fail if the backend cannot fall back correctly.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={deleting}>Cancel</AlertDialogCancel>
            <AlertDialogAction
              onClick={(event) => {
                event.preventDefault();
                handleDeleteAlias();
              }}
              disabled={deleting}
              className="bg-destructive text-white hover:bg-destructive/90"
            >
              {deleting ? 'Deleting…' : 'Delete'}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  );
}
