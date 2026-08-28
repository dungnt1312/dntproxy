import { useMemo, useState } from 'react';
import { Trash2 } from 'lucide-react';
import {
  AlertDialog,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog';
import { Button } from '@/components/ui/button';
import { Checkbox } from '@/components/ui/checkbox';
import type { Connection } from '@/types/connections';

/**
 * Confirmation for deleting *problematic* connections, grouped by reason so
 * borderline cases (e.g. still rate-limited but working accounts) can be
 * excluded before an irreversible bulk delete.
 */

export interface CleanupGroup {
  key: string;
  label: string;
  conns: Connection[];
}

interface BulkCleanupDialogProps {
  groups: CleanupGroup[];
  busy?: boolean;
  onConfirm?: (ids: string[]) => void;
  onClose: () => void;
}

const REASON_HINTS: Record<string, string> = {
  expired: 'Token expired — cannot auto-refresh',
  'rate-limited': 'Currently rate-limited — may recover later',
  error: 'Recent error recorded',
};

export function BulkCleanupDialog({ groups, onConfirm, onClose }: BulkCleanupDialogProps) {
  const nonEmpty = useMemo(() => groups.filter((g) => g.conns.length > 0), [groups]);
  const [checked, setChecked] = useState<Record<string, boolean>>(() =>
    Object.fromEntries(nonEmpty.map((g) => [g.key, g.key !== 'rate-limited'])),
  );

  const selectedIds = nonEmpty.flatMap((g) => (checked[g.key] ? g.conns.map((c) => c.id) : []));
  const total = selectedIds.length;

  const previewNames = nonEmpty
    .flatMap((g) => (checked[g.key] ? g.conns.map((c) => c.name) : []))
    .slice(0, 6);

  return (
    <AlertDialog open onOpenChange={(open) => { if (!open) onClose(); }}>
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>Clean up problem connections</AlertDialogTitle>
          <AlertDialogDescription asChild>
            <div className="space-y-3">
              <span>Pick reason groups to delete permanently. Tokens, settings, and quota data will be lost.</span>
              <div className="space-y-1.5">
                {nonEmpty.map((g) => (
                  <label
                    key={g.key}
                    className="flex cursor-pointer items-start gap-2 rounded-md border bg-background/60 p-2.5 text-sm"
                  >
                    <Checkbox
                      checked={checked[g.key] ?? false}
                      onCheckedChange={(v) => setChecked((prev) => ({ ...prev, [g.key]: Boolean(v) }))}
                      className="mt-0.5"
                    />
                    <span className="min-w-0">
                      <span className="font-medium">
                        {g.label} ({g.conns.length})
                      </span>
                      <span className="block truncate text-xs text-muted-foreground">
                        {REASON_HINTS[g.key] ?? ''} · {g.conns.map((c) => c.name).join(', ').slice(0, 90)}
                        {g.conns.map((c) => c.name).join(', ').length > 90 ? '…' : ''}
                      </span>
                    </span>
                  </label>
                ))}
              </div>
              {previewNames.length > 0 && (
                <ul className="list-disc pl-5 text-xs text-muted-foreground">
                  {previewNames.map((n) => (
                    <li key={n} className="truncate">{n}</li>
                  ))}
                </ul>
              )}
            </div>
          </AlertDialogDescription>
        </AlertDialogHeader>
        <AlertDialogFooter>
          <Button variant="outline" onClick={onClose}>
            Cancel
          </Button>
          <Button
            variant="destructive"
            disabled={total === 0}
            className="gap-2"
            onClick={() => onConfirm?.(selectedIds)}
          >
            <Trash2 size={14} /> Delete {total} permanently
          </Button>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  );
}
