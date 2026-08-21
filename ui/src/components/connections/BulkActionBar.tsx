import { CheckCheck, Loader2, Power, PowerOff, Settings2, Trash2, X } from 'lucide-react';
import { Button } from '@/components/ui/button';

export type BulkAction = 'models' | 'enable' | 'disable' | 'delete';

interface BulkActionBarProps {
    selectedCount: number;
    visibleCount: number;
    allVisibleSelected: boolean;
    /** Label of the currently running bulk action, or null when idle. */
    busy: string | null;
    onToggleSelectAll: () => void;
    onClearSelection: () => void;
    onExit: () => void;
    onAction: (action: BulkAction) => void;
}

/**
 * Toolbar shown while the connections screen is in selection mode.
 * Reports how many cards are selected and hosts the bulk actions.
 */
export function BulkActionBar({
    selectedCount,
    visibleCount,
    allVisibleSelected,
    busy,
    onToggleSelectAll,
    onClearSelection,
    onExit,
    onAction,
}: BulkActionBarProps) {
    const disabled = busy !== null;

    return (
        <div
            role="toolbar"
            aria-label="Bulk connection actions"
            className="flex flex-wrap items-center gap-2 rounded-lg border border-primary/30 bg-primary/5 px-3 py-2"
        >
            <span className="text-sm font-medium tabular-nums" aria-live="polite">
                {busy ? (
                    <span className="inline-flex items-center gap-2">
                        <Loader2 className="h-3.5 w-3.5 animate-spin" /> {busy}…
                    </span>
                ) : (
                    `${selectedCount} selected`
                )}
            </span>

            <Button
                variant="outline"
                size="sm"
                className="h-7 gap-1.5 text-xs"
                onClick={onToggleSelectAll}
                disabled={disabled || visibleCount === 0}
            >
                <CheckCheck className="h-3.5 w-3.5" />
                {allVisibleSelected ? 'Deselect All' : `Select All (${visibleCount})`}
            </Button>
            <Button
                variant="outline"
                size="sm"
                className="h-7 gap-1.5 text-xs"
                onClick={onClearSelection}
                disabled={disabled || selectedCount === 0}
            >
                <X className="h-3.5 w-3.5" /> Clear
            </Button>

            <div className="ml-auto flex flex-wrap items-center gap-2">
                <Button
                    variant="outline"
                    size="sm"
                    className="h-7 gap-1.5 text-xs"
                    onClick={() => onAction('models')}
                    disabled={disabled || selectedCount === 0}
                >
                    <Settings2 className="h-3.5 w-3.5" /> Models…
                </Button>
                <Button
                    variant="outline"
                    size="sm"
                    className="h-7 gap-1.5 text-xs"
                    onClick={() => onAction('enable')}
                    disabled={disabled || selectedCount === 0}
                >
                    <Power className="h-3.5 w-3.5" /> Enable
                </Button>
                <Button
                    variant="outline"
                    size="sm"
                    className="h-7 gap-1.5 text-xs"
                    onClick={() => onAction('disable')}
                    disabled={disabled || selectedCount === 0}
                >
                    <PowerOff className="h-3.5 w-3.5" /> Disable
                </Button>
                <Button
                    variant="destructive"
                    size="sm"
                    className="h-7 gap-1.5 text-xs"
                    onClick={() => onAction('delete')}
                    disabled={disabled || selectedCount === 0}
                >
                    <Trash2 className="h-3.5 w-3.5" /> Delete…
                </Button>
                <Button
                    variant="ghost"
                    size="sm"
                    className="h-7 text-xs"
                    onClick={onExit}
                    disabled={disabled}
                >
                    Done
                </Button>
            </div>
        </div>
    );
}
