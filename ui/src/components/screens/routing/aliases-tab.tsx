import { useMemo, useState } from "react";
import { Link2, Loader2, Plus } from "lucide-react";
import { toast } from "sonner";
import { goApi } from "@/lib/go-api";
import { Button } from "@/components/ui/button";
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
import type { AliasMap, ConnectionOption, UiModel } from "./types";
import { AliasDialog } from "./alias-dialog";
import { AliasesList } from "./aliases-list";
import { RoutingEmptyState } from "./routing-empty-state";
import { RoutingToolbar } from "./routing-toolbar";

export interface AliasesTabProps {
  aliases: AliasMap;
  models: UiModel[];
  connections: ConnectionOption[];
  loading?: boolean;
  hasLoadError?: boolean;
  onRefresh: () => Promise<void>;
  onOpenLogModal: (alias: string) => void;
}

export default function AliasesTab({
  aliases,
  models,
  connections,
  loading,
  hasLoadError,
  onRefresh,
  onOpenLogModal,
}: AliasesTabProps) {
  const [search, setSearch] = useState("");
  const [dialogOpen, setDialogOpen] = useState(false);
  const [deleteTarget, setDeleteTarget] = useState<string | null>(null);
  const [deleting, setDeleting] = useState(false);
  const q = search.trim().toLowerCase();

  const filteredAliases = useMemo(() => {
    const entries = Object.entries(aliases).sort(([a], [b]) => a.localeCompare(b));
    if (!q) return entries;
    return entries.filter(([alias, target]) => `${alias} ${target}`.toLowerCase().includes(q));
  }, [aliases, q]);

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
      <RoutingToolbar
        search={search}
        onSearchChange={setSearch}
        placeholder="Search aliases or targets..."
        summary={`${filteredAliases.length} of ${Object.keys(aliases).length} aliases`}
        action={
          <Button onClick={() => setDialogOpen(true)} className="h-9 gap-2">
            <Plus className="h-4 w-4" />
            Create alias
          </Button>
        }
      />

      {loading ? (
        <div className="flex items-center justify-center rounded-lg border bg-card p-16 text-sm text-muted-foreground">
          <Loader2 className="mr-2 h-5 w-5 animate-spin" />
          Loading aliases...
        </div>
      ) : filteredAliases.length === 0 ? (
        <RoutingEmptyState
          icon={<Link2 className="h-5 w-5 text-muted-foreground" />}
          title={q ? "No matching aliases" : hasLoadError ? "Aliases could not load" : "No aliases configured"}
          description={
            q
              ? `No aliases match "${search.trim()}".`
              : hasLoadError
                ? "Retry loading routing data from the error banner above."
                : "Aliases map custom request names to provider models."
          }
          action={!q && !hasLoadError ? <Button onClick={() => setDialogOpen(true)}>Create alias</Button> : undefined}
        />
      ) : (
        <AliasesList
          aliases={filteredAliases}
          models={models}
          connections={connections}
          onOpenLogModal={onOpenLogModal}
          onDelete={setDeleteTarget}
        />
      )}

      <AliasDialog
        open={dialogOpen}
        onOpenChange={setDialogOpen}
        aliases={aliases}
        models={models}
        onRefresh={onRefresh}
      />

      <AlertDialog open={!!deleteTarget} onOpenChange={(open) => { if (!open) setDeleteTarget(null); }}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Delete alias</AlertDialogTitle>
            <AlertDialogDescription>
              Delete "{deleteTarget}"? Requests using this alias will no longer resolve through it.
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
              {deleting ? "Deleting..." : "Delete"}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  );
}
