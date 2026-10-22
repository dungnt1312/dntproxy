import { ArrowRightLeft, Download, Layers, Loader2, Power, PowerOff, RefreshCw, Trash2, X } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Checkbox } from '@/components/ui/checkbox';

export type BulkAction = 'models' | 'migrate' | 'quotas' | 'enable' | 'disable' | 'export' | 'delete';

interface BulkActionBarProps {
  selectedCount: number;
  visibleCount: number;
  allVisibleSelected: boolean;
  /** Label of the currently running bulk action, or null when idle. */
  busy: string | null;
  onToggleSelectAll: () => void;
  onExit: () => void;
  onAction: (action: BulkAction) => void;
}

/** Sticky toolbar shown while the connections screen is in selection mode. */
export function BulkActionBar({
  selectedCount,
  visibleCount,
  allVisibleSelected,
  busy,
  onToggleSelectAll,
  onExit,
  onAction,
}: BulkActionBarProps) {
  const disabled = busy !== null || selectedCount === 0;
  const someSelected = selectedCount > 0 && !allVisibleSelected;

  const action = (key: BulkAction, label: string, Icon: typeof Layers) => (
    <Button variant="outline" size="sm" className="gap-1.5" onClick={() => onAction(key)} disabled={disabled}>
      <Icon className="size-4" /> {label}
    </Button>
  );

  return (
    <div
      role="toolbar"
      aria-label="Bulk connection actions"
      className="sticky bottom-3 z-20 flex flex-wrap items-center gap-2 rounded-lg border bg-background/95 p-2 shadow-lg backdrop-blur"
    >
      <label className="flex h-9 cursor-pointer items-center gap-2 rounded-md px-2 text-sm hover:bg-muted">
        <Checkbox
          checked={allVisibleSelected ? true : someSelected ? 'indeterminate' : false}
          onCheckedChange={onToggleSelectAll}
          disabled={busy !== null || visibleCount === 0}
          aria-label={allVisibleSelected ? 'Deselect all' : 'Select all visible'}
        />
        <span className="tabular-nums" aria-live="polite">
          {busy ? (
            <span className="inline-flex items-center gap-2">
              <Loader2 className="size-4 animate-spin" /> {busy}…
            </span>
          ) : (
            <>
              <span className="font-semibold">{selectedCount}</span>
              <span className="text-muted-foreground"> / {visibleCount} selected</span>
            </>
          )}
        </span>
      </label>

      <div className="ml-auto flex flex-wrap items-center gap-2">
        {action('enable', 'Enable', Power)}
        {action('disable', 'Disable', PowerOff)}
        {action('quotas', 'Quota', RefreshCw)}
        {action('models', 'Replace models', Layers)}
        {action('migrate', 'Add / remove models', ArrowRightLeft)}
        {action('export', 'Export', Download)}
        <Button variant="destructive" size="sm" className="gap-1.5" onClick={() => onAction('delete')} disabled={disabled}>
          <Trash2 className="size-4" /> Delete
        </Button>
        <Button variant="ghost" size="icon" className="size-9" onClick={onExit} disabled={busy !== null} aria-label="Exit selection mode" title="Exit selection">
          <X className="size-4" />
        </Button>
      </div>
    </div>
  );
}
