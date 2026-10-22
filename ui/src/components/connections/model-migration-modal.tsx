import { useId, useMemo, useState } from 'react';
import { AlertTriangle, ArrowRight, CheckCircle2, Loader2, Plus, X, XCircle } from 'lucide-react';
import { toast } from 'sonner';
import { api } from '../../api';
import { applyModelOps, sameModels, type ModelOps } from './model-ops';
import { errorMessage } from './add/helpers';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import type { Connection } from '@/types/connections';

/**
 * Model migration — surgical add / remove / rename applied to many
 * connections. Each target keeps its own list; only declared ops touch it,
 * and only connections whose list actually changes are written.
 */

interface ModelMigrationModalProps {
  targets: Connection[];
  onApplied?: () => void;
  onClose: () => void;
}

type TargetResult = { ok: true } | { ok: false; error: string };

const parseList = (text: string): string[] => text.split(/[,\n]/).map((s) => s.trim()).filter(Boolean);

export function ModelMigrationModal({ targets, onApplied, onClose }: ModelMigrationModalProps) {
  const id = useId();
  const [addText, setAddText] = useState('');
  const [removeText, setRemoveText] = useState('');
  const [renameRows, setRenameRows] = useState<Array<{ from: string; to: string }>>([]);
  const [applying, setApplying] = useState(false);
  const [progress, setProgress] = useState('');
  const [results, setResults] = useState<Record<string, TargetResult>>({});

  const ops: ModelOps = useMemo(
    () => ({
      add: parseList(addText),
      remove: parseList(removeText),
      renames: renameRows.filter((r) => r.from.trim() && r.to.trim()),
    }),
    [addText, removeText, renameRows],
  );
  const hasOps = ops.add.length + ops.remove.length + ops.renames.length > 0;

  const preview = useMemo(() => {
    if (!hasOps) return [];
    const noop: ModelOps = { add: [], remove: [], renames: [] };
    return targets.map((conn) => {
      // Normalise stored ids the same way the ops do, so prefixes never fake a diff.
      const before = applyModelOps(conn.supportedModels ?? [], noop, conn).result;
      const { result, conflicts } = applyModelOps(conn.supportedModels ?? [], ops, conn);
      return { conn, before, after: result, conflicts, changed: !sameModels(before, result) };
    });
  }, [targets, ops, hasOps]);

  const changedLines = preview.filter((p) => p.changed);
  const conflictLines = preview.flatMap((p) => p.conflicts);
  const doneCount = Object.keys(results).length;
  const failCount = Object.values(results).filter((r) => !r.ok).length;
  // After a run, "Apply" only retries what is still pending or failed.
  const toApply = changedLines.filter((p) => !results[p.conn.id]?.ok);

  const handleApply = async () => {
    setApplying(true);
    let ok = 0;
    let fails = 0;
    for (let i = 0; i < toApply.length; i++) {
      const { conn, after } = toApply[i];
      setProgress(`${i + 1}/${toApply.length}`);
      try {
        await api.updateConnection(conn.id, { supportedModels: after, setModels: true });
        ok++;
        setResults((prev) => ({ ...prev, [conn.id]: { ok: true } }));
      } catch (e: unknown) {
        fails++;
        setResults((prev) => ({ ...prev, [conn.id]: { ok: false, error: errorMessage(e, 'Update failed') } }));
      }
    }
    setApplying(false);
    setProgress('');
    if (fails > 0) toast.error(`Migration: ${ok}/${toApply.length} applied, ${fails} failed`);
    else toast.success(`Migration applied to ${ok} connection${ok === 1 ? '' : 's'}`);
    onApplied?.();
  };

  return (
    <Dialog open onOpenChange={(open) => { if (!open && !applying) onClose(); }}>
      <DialogContent className="flex max-h-[90vh] max-w-2xl flex-col">
        <DialogHeader>
          <DialogTitle className="text-lg">
            Add / remove / rename models — {targets.length} connection{targets.length === 1 ? '' : 's'}
          </DialogTitle>
          <DialogDescription>Change individual models on each connection without replacing its whole list.</DialogDescription>
        </DialogHeader>

        <div className="flex-1 space-y-5 overflow-y-auto py-2">
          <section className="space-y-3">
            <h3 className="text-sm font-semibold">Operations</h3>
            <div className="grid gap-3 sm:grid-cols-2">
              <div className="space-y-1.5">
                <label htmlFor={`${id}-add`} className="text-sm font-medium">Add models</label>
                <Input id={`${id}-add`} value={addText} onChange={(e) => setAddText(e.target.value)} disabled={applying} placeholder="claude-sonnet-4-5, haiku-4-5" className="font-mono text-sm" />
              </div>
              <div className="space-y-1.5">
                <label htmlFor={`${id}-remove`} className="text-sm font-medium">Remove models</label>
                <Input id={`${id}-remove`} value={removeText} onChange={(e) => setRemoveText(e.target.value)} disabled={applying} placeholder="claude-3-sonnet" className="font-mono text-sm" />
              </div>
            </div>
            <p className="text-xs text-muted-foreground">Separate multiple models with commas. Provider prefixes are optional.</p>

            {renameRows.length > 0 && (
              <fieldset className="space-y-2">
                <legend className="mb-1 text-sm font-medium">Rename</legend>
                {renameRows.map((row, idx) => (
                  <div key={idx} className="flex items-center gap-2">
                    <Input
                      aria-label={`Rename ${idx + 1}: old model`}
                      value={row.from}
                      onChange={(e) => setRenameRows((prev) => prev.map((r, i) => (i === idx ? { ...r, from: e.target.value } : r)))}
                      disabled={applying}
                      placeholder="old model"
                      className="flex-1 font-mono text-sm"
                    />
                    <ArrowRight className="size-4 shrink-0 text-muted-foreground" aria-hidden />
                    <Input
                      aria-label={`Rename ${idx + 1}: new model`}
                      value={row.to}
                      onChange={(e) => setRenameRows((prev) => prev.map((r, i) => (i === idx ? { ...r, to: e.target.value } : r)))}
                      disabled={applying}
                      placeholder="new model"
                      className="flex-1 font-mono text-sm"
                    />
                    <Button variant="ghost" size="icon" className="size-9 shrink-0" onClick={() => setRenameRows((prev) => prev.filter((_, i) => i !== idx))} disabled={applying} aria-label={`Remove rename ${idx + 1}`}>
                      <X className="size-4" />
                    </Button>
                  </div>
                ))}
              </fieldset>
            )}
            <Button variant="outline" size="sm" className="gap-1.5" onClick={() => setRenameRows((prev) => [...prev, { from: '', to: '' }])} disabled={applying}>
              <Plus className="size-4" /> Add rename
            </Button>
          </section>

          {conflictLines.length > 0 && (
            <div role="status" className="space-y-1 rounded-lg border border-warning/30 bg-warning/10 p-3 text-sm">
              {conflictLines.slice(0, 4).map((c, i) => (
                <p key={i} className="flex items-start gap-2">
                  <AlertTriangle className="mt-0.5 size-4 shrink-0 text-warning" /> {c}
                </p>
              ))}
              {conflictLines.length > 4 && <p className="pl-6 text-muted-foreground">+{conflictLines.length - 4} more</p>}
            </div>
          )}

          {hasOps ? (
            <section className="space-y-2">
              <div className="flex items-center justify-between">
                <h3 className="text-sm font-semibold">Preview</h3>
                <span className="text-sm tabular-nums text-muted-foreground">
                  {changedLines.length}/{targets.length} will change
                </span>
              </div>
              <ul className="max-h-64 space-y-1 overflow-y-auto rounded-lg border bg-muted/20 p-2">
                {preview.map((line) => {
                  const r = results[line.conn.id];
                  const added = line.after.filter((m) => !line.before.includes(m));
                  const removed = line.before.filter((m) => !line.after.includes(m));
                  return (
                    <li key={line.conn.id} className="rounded-md bg-background px-3 py-2 text-sm">
                      <div className="flex items-center gap-2">
                        <span className="truncate font-medium">{line.conn.name}</span>
                        <span className="ml-auto shrink-0 text-xs">
                          {r?.ok && <span className="inline-flex items-center gap-1 text-success"><CheckCircle2 className="size-3.5" /> Applied</span>}
                          {r && !r.ok && <span className="inline-flex items-center gap-1 text-destructive" title={r.error}><XCircle className="size-3.5" /> Failed</span>}
                          {!r && !line.changed && <span className="text-muted-foreground">Unchanged</span>}
                        </span>
                      </div>
                      {line.changed && (
                        <div className="mt-1 flex flex-wrap gap-x-3 gap-y-0.5 font-mono text-xs">
                          {added.map((m) => <span key={`+${m}`} className="text-success">+ {m}</span>)}
                          {removed.map((m) => <span key={`-${m}`} className="text-destructive line-through">{m}</span>)}
                        </div>
                      )}
                      {r && !r.ok && <p className="mt-1 text-xs text-destructive">{r.error}</p>}
                    </li>
                  );
                })}
              </ul>
            </section>
          ) : (
            <p className="rounded-lg border border-dashed p-6 text-center text-sm text-muted-foreground">Add at least one operation to see a preview.</p>
          )}
        </div>

        <DialogFooter className="items-center sm:justify-between">
          <span className="text-sm tabular-nums text-muted-foreground" aria-live="polite">
            {applying ? (
              <span className="inline-flex items-center gap-1.5"><Loader2 className="size-4 animate-spin" /> Applying {progress}</span>
            ) : doneCount > 0 ? (
              `${doneCount - failCount} applied${failCount > 0 ? ` · ${failCount} failed` : ''}`
            ) : null}
          </span>
          <div className="flex gap-2">
            <Button variant="outline" onClick={onClose} disabled={applying}>
              {doneCount > 0 ? 'Close' : 'Cancel'}
            </Button>
            <Button onClick={handleApply} disabled={applying || toApply.length === 0} className="gap-2">
              {applying && <Loader2 className="size-4 animate-spin" />}
              {failCount > 0 ? `Retry ${toApply.length} failed` : `Apply to ${toApply.length} connection${toApply.length === 1 ? '' : 's'}`}
            </Button>
          </div>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
