import { useState } from 'react';
import { Settings2 } from 'lucide-react';
import {
    Dialog,
    DialogContent,
    DialogDescription,
    DialogFooter,
    DialogHeader,
    DialogTitle,
} from '@/components/ui/dialog';
import { Button } from '@/components/ui/button';
import { Textarea } from '@/components/ui/textarea';
import { cn } from '@/lib/utils';
import type { Connection } from '@/types/connections';
import { withModelPrefix } from './model-prefix';

type Mode = 'replace' | 'clear';

interface BulkModelsModalProps {
    connections: Connection[];
    busy: boolean;
    /** Receives the raw "prefix/model" lines (empty array = allow all). */
    onApply: (models: string[]) => void;
    onClose: () => void;
}

/**
 * Applies one model list to many connections at once. Mixed-provider
 * selections keep each connection's own prefix handling on apply.
 */
export function BulkModelsModal({ connections, busy, onApply, onClose }: BulkModelsModalProps) {
    // Prefill with the shared models when every selected connection belongs to
    // the same provider and they already agree on a list; otherwise start empty.
    const [text, setText] = useState(() => {
        const providers = new Set(connections.map((c) => c.provider));
        if (providers.size !== 1) return '';
        const sets = connections.map((c) => new Set(c.supportedModels || []));
        if (sets.length === 0) return '';
        const shared = [...sets[0]].filter((m) => sets.every((s) => s.has(m)));
        return withModelPrefix(shared.sort(), connections[0]).join('\n');
    });
    const [mode, setMode] = useState<Mode>('replace');
    const [confirmClear, setConfirmClear] = useState(false);

    const lines = text
        .split('\n')
        .map((s) => s.trim())
        .filter(Boolean);
    const canApply = (mode === 'clear' && confirmClear) || (mode === 'replace' && lines.length > 0);

    return (
        <Dialog open onOpenChange={(open) => { if (!open && !busy) onClose(); }}>
            <DialogContent className="max-w-xl">
                <DialogHeader>
                    <div className="flex items-center gap-3">
                        <div className="flex h-10 w-10 shrink-0 items-center justify-center rounded-lg border border-primary/20 bg-primary/10">
                            <Settings2 className="h-5 w-5 text-primary" />
                        </div>
                        <div>
                            <DialogTitle className="text-lg">
                                Replace models — {connections.length} connections
                            </DialogTitle>
                            <DialogDescription className="mt-1">
                                One model per line (e.g. <code>kr/claude-sonnet-4-5</code>). Provider
                                prefixes are stripped per connection on save.
                            </DialogDescription>
                        </div>
                    </div>
                </DialogHeader>

                <div className="space-y-3">
                    <fieldset className="flex gap-2">
                        <legend className="sr-only">Apply mode</legend>
                        {(
                            [
                                ['replace', 'Replace model list'],
                                ['clear', 'Clear (allow all models)'],
                            ] as const
                        ).map(([value, label]) => (
                            <label
                                key={value}
                                className={cn(
                                    'flex flex-1 cursor-pointer items-center gap-2 rounded-lg border px-3 py-2 text-sm font-medium transition-colors',
                                    mode === value ? 'border-primary/40 bg-primary/5' : 'hover:bg-muted/50',
                                )}
                            >
                                <input
                                    type="radio"
                                    name="bulk-models-mode"
                                    value={value}
                                    checked={mode === value}
                                    onChange={() => setMode(value)}
                                    disabled={busy}
                                    className="accent-primary"
                                />
                                {label}
                            </label>
                        ))}
                    </fieldset>

                    {mode === 'replace' && (
                        <div className="space-y-1">
                            <label htmlFor="bulk-models-list" className="text-sm font-medium">
                                Supported Models{' '}
                                <span className="font-normal text-muted-foreground">
                                    (one per line, replaces each connection's list)
                                </span>
                            </label>
                            <Textarea
                                id="bulk-models-list"
                                value={text}
                                onChange={(e) => setText(e.target.value)}
                                placeholder={'kr/claude-sonnet-4-5\noai/gpt-5\n…'}
                                rows={8}
                                autoComplete="off"
                                spellCheck={false}
                                disabled={busy}
                                className="font-mono text-sm"
                            />
                        </div>
                    )}

                    {mode === 'clear' && (
                        <label className="flex cursor-pointer items-start gap-2 rounded-lg border border-warning/30 bg-warning/10 p-3 text-sm">
                            <input
                                type="checkbox"
                                checked={confirmClear}
                                onChange={(e) => setConfirmClear(e.target.checked)}
                                disabled={busy}
                                className="mt-0.5 accent-primary"
                            />
                            <span>
                                Remove the model restriction from all {connections.length} selected connections.
                                They will accept any model.
                            </span>
                        </label>
                    )}
                </div>

                <DialogFooter>
                    <Button variant="outline" onClick={onClose} disabled={busy}>
                        Cancel
                    </Button>
                    <Button
                        onClick={() => onApply(mode === 'clear' ? [] : lines)}
                        disabled={busy || !canApply}
                        className="gap-2"
                    >
                        {busy ? 'Applying…' : `Apply to ${connections.length} connections`}
                    </Button>
                </DialogFooter>
            </DialogContent>
        </Dialog>
    );
}
