import { useMemo, useState } from 'react';
import { ArrowRight, Loader2, Plus, X } from 'lucide-react';
import { toast } from 'sonner';
import { api } from '../../api';
import { applyModelOps, type ModelOps } from './model-ops';
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
import { cn } from '@/lib/utils';
import type { Connection } from '@/types/connections';

/**
 * Model migration wizard — surgical updates (add / remove / rename) applied to
 * many connections at once. Unlike "apply exact list", each target keeps its
 * own list; only the declared operations touch it.
 */

interface MigrationPreviewLine {
  conn: Connection;
  before: string[];
  after: string[];
  conflicts: string[];
}

interface ModelMigrationModalProps {
  targets: Connection[];
  /** Pre-seed the "remove" step, e.g. legacy ids found by the detector. */
  prefillRemove?: string[];
  busy?: boolean;
  onApplied?: () => void;
  onClose: () => void;
}

type TargetState = 'pending' | 'ok' | 'fail';

export function ModelMigrationModal({ targets, prefillRemove = [], onApplied, onClose }: ModelMigrationModalProps) {
  const [addText, setAddText] = useState('');
  const [removeText, setRemoveText] = useState(() => prefillRemove.join(', '));
  const [renameRows, setRenameRows] = useState<Array<{ from: string; to: string }>>([]);
  const [applying, setApplying] = useState(false);
  const [progress, setProgress] = useState('');
  const [results, setResults] = useState<Record<string, TargetState>>({});
  const [doneIds, setDoneIds] = useState<Set<string>>(new Set());

  const parseList = (text: string): string[] =>
    text.split(/[,\n]/).map((s) => s.trim()).filter(Boolean);

  const ops: ModelOps = useMemo(
    () => ({
      add: parseList(addText),
      remove: parseList(removeText),
      renames: renameRows.filter((r) => r.from.trim() && r.to.trim()),
    }),
    [addText, removeText, renameRows],
  );

  const hasOps = ops.add.length + ops.remove.length + ops.renames.length > 0;

  const preview = useMemo<MigrationPreviewLine[]>(() => {
    if (!hasOps) return [];
    return targets.map((conn) => {
      const before = conn.supportedModels ?? [];
      const { result, conflicts } = applyModelOps(before, ops, conn);
      return { conn, before, after: result, conflicts };
    });
  }, [targets, ops, hasOps]);

  const affectedCount = preview.filter((p) => JSON.stringify(p.before.slice().sort()) !== JSON.stringify(p.after.slice().sort())).length;
  const conflictLines = preview.flatMap((p) => p.conflicts);

  const handleApply = async () => {
    setApplying(true);
    try {
      let ok = 0;
      let fails = 0;
      for (let i = 0; i < targets.length; i++) {
        const target = targets[i];
        setProgress(`${i + 1}/${targets.length}`);
        try {
          const { result } = applyModelOps(target.supportedModels ?? [], ops, target);
          await api.updateConnection(target.id, { supportedModels: result, setModels: true });
          ok++;
          setResults((prev) => ({ ...prev, [target.id]: 'ok' }));
          setDoneIds((prev) => new Set(prev).add(target.id));
        } catch {
          fails++;
          setResults((prev) => ({ ...prev, [target.id]: 'fail' }));
        }
      }
      if (fails > 0) toast.error(`Migration partially applied: ${ok}/${targets.length}. Review the list and retry.`);
      else toast.success(`Migration applied to ${ok} connection(s)`);
      onApplied?.();
    } finally {
      setApplying(false);
    }
  };

  const failedTargets = targets.filter((t) => results[t.id] === 'fail');
  const pendingOk = targets.filter((t) => results[t.id] === 'ok').length;

  return (
    <Dialog open onOpenChange={(open) => { if (!open && !applying) onClose(); }}>
      <DialogContent className="max-w-2xl max-h-[90vh] flex flex-col">
        <DialogHeader>
          <div>
            <DialogTitle className="text-lg">Migrate models — {targets.length} connection</DialogTitle>
            <DialogDescription className="text-xs mt-1">
              Add / remove / rename models on each connection without overwriting its whole list.
            </DialogDescription>
          </div>
        </DialogHeader>

        <div className="flex-1 space-y-4 overflow-y-auto py-2">
          {/* Step 1 — operations */}
          <section className="space-y-2">
            <h3 className="text-[10px] font-bold uppercase tracking-wider text-muted-foreground">Operations</h3>

            <div className="grid gap-2 sm:grid-cols-2">
              <label className="space-y-1">
                <span className="text-xs font-medium text-emerald-600">Add models</span>
                <Input
                  value={addText}
                  onChange={(e) => setAddText(e.target.value)}
                  disabled={applying}
                  placeholder="claude-sonnet-4-5, haiku-4-5"
                  className="h-8 font-mono text-xs"
                />
              </label>
              <label className="space-y-1">
                <span className="text-xs font-medium text-destructive">Remove models</span>
                <Input
                  value={removeText}
                  onChange={(e) => setRemoveText(e.target.value)}
                  disabled={applying}
                  placeholder="claude-3-sonnet"
                  className="h-8 font-mono text-xs"
                />
              </label>
            </div>

            {/* Rename rows */}
            <div className="space-y-1.5">
              {renameRows.map((row, idx) => (
                <div key={idx} className="flex items-center gap-1.5">
                  <Input
                    value={row.from}
                    onChange={(e) => setRenameRows((prev) => prev.map((r, i) => (i === idx ? { ...r, from: e.target.value } : r)))}
                    disabled={applying}
                    placeholder="old model"
                    className="h-8 flex-1 font-mono text-xs"
                  />
                  <ArrowRight size={13} className="shrink-0 text-muted-foreground" />
                  <Input
                    value={row.to}
                    onChange={(e) => setRenameRows((prev) => prev.map((r, i) => (i === idx ? { ...r, to: e.target.value } : r)))}
                    disabled={applying}
                    placeholder="new model"
                    className="h-8 flex-1 font-mono text-xs"
                  />
                  <Button
                    variant="ghost"
                    size="icon"
                    className="h-7 w-7 shrink-0"
                    onClick={() => setRenameRows((prev) => prev.filter((_, i) => i !== idx))}
                    aria-label="Remove rename row"
                  >
                    <X size={13} />
                  </Button>
                </div>
              ))}
              <button
                type="button"
                onClick={() => setRenameRows((prev) => [...prev, { from: '', to: '' }])}
                disabled={applying}
                className="flex items-center gap-1 text-xs font-medium text-muted-foreground transition-colors hover:text-foreground"
              >
                <Plus size={12} /> Add rename pair
              </button>
            </div>
          </section>

          {/* Conflicts */}
          {conflictLines.length > 0 && (
            <div className="rounded-lg border border-amber-500/30 bg-amber-500/10 p-2.5 text-xs text-amber-700 dark:text-amber-300">
              {conflictLines.slice(0, 3).map((c, i) => (
                <p key={i}>⚠ {c}</p>
              ))}
            </div>
          )}

          {/* Step 2 — dry-run preview */}
          {hasOps && (
            <section className="space-y-2">
              <div className="flex items-center justify-between">
                <h3 className="text-[10px] font-bold uppercase tracking-wider text-muted-foreground">Preview (dry-run)</h3>
                <span className="text-xs tabular-nums text-muted-foreground">
                  {affectedCount}/{targets.length} will change
                </span>
              </div>
              <div className="max-h-56 space-y-1 overflow-y-auto rounded-lg border bg-muted/20 p-2">
                {preview.slice(0, 50).map((line) => {
                  const changed =
                    JSON.stringify(line.before.slice().sort()) !== JSON.stringify(line.after.slice().sort());
                  const state = results[line.conn.id];
                  return (
                    <div key={line.conn.id} className="rounded-md bg-background px-2 py-1.5 text-xs">
                      <div className="flex items-center gap-2">
                        <span className="truncate font-medium">{line.conn.name}</span>
                        {state === 'ok' && <span className="ml-auto shrink-0 text-[10px] text-emerald-600">✓ applied</span>}
                        {state === 'fail' && <span className="ml-auto shrink-0 text-[10px] text-destructive">✗ failed</span>}
                        {!changed && state !== 'ok' && (
                          <span className="ml-auto shrink-0 text-[10px] text-muted-foreground/50">unchanged</span>
                        )}
                      </div>
                      {changed && (
                        <div className="mt-0.5 truncate font-mono text-[10px] leading-relaxed">
                          {line.after
                            .filter((m) => !line.before.includes(m))
                            .slice(0, 4)
                            .map((m) => (
                              <span key={m} className="mr-2 text-emerald-600">+{m}</span>
                            ))}
                          {line.before
                            .filter((m) => !line.after.includes(m))
                            .slice(0, 4)
                            .map((m) => (
                              <span key={m} className="mr-2 line-through text-destructive/70">{m}</span>
                            ))}
                        </div>
                      )}
                    </div>
                  );
                })}
                {preview.length > 50 && (
                  <p className="px-2 pt-1 text-center text-[11px] text-muted-foreground">…and {preview.length - 50} more connection(s)</p>
                )}
              </div>
            </section>
          )}

          {!hasOps && (
            <p className="rounded-lg border border-dashed p-6 text-center text-sm text-muted-foreground">
              Declare at least one operation to preview results.
            </p>
          )}
        </div>

        <DialogFooter className="items-center justify-between sm:justify-between">
          <span className={cn('text-xs tabular-nums text-muted-foreground', applying && 'inline-flex items-center gap-1.5')}>
            {applying ? (
              <>
                <Loader2 size={12} className="animate-spin" /> Applying {progress}
              </>
            ) : doneIds.size > 0 ? (
              `${pendingOk}/${targets.length} succeeded${failedTargets.length > 0 ? ` · ${failedTargets.length} failed` : ''}`
            ) : (
              ''
            )}
          </span>
          <div className="flex gap-2">
            <Button variant="outline" onClick={onClose} disabled={applying}>
              {doneIds.size > 0 ? 'Close' : 'Cancel'}
            </Button>
            <Button onClick={handleApply} disabled={applying || !hasOps || targets.length === 0} className="gap-2">
              {applying && <Loader2 size={14} className="animate-spin" />}
              Apply to {affectedCount} connection(s)
            </Button>
          </div>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
