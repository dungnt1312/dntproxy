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
import { getModelProviderId } from '@/lib/provider-registry';
import type { Connection } from '@/types/connections';

/**
 * Strips the provider/route prefix from a "prefix/model" string so it can be
 * stored in a connection's supportedModels (same semantics as EditModelsModal).
 */
export function stripModelForConnection(model: string, conn: Connection): string {
    const trimmed = model.trim();
    const prefix =
        conn.provider === 'openai-compatible' && conn.routePrefix
            ? conn.routePrefix
            : getModelProviderId(conn.provider);
    if (trimmed.startsWith(prefix + '/')) return trimmed.slice(prefix.length + 1);
    const slash = trimmed.indexOf('/');
    return slash >= 0 ? trimmed.slice(slash + 1) : trimmed;
}

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
        const conn = connections[0];
        const prefix = getModelProviderId(conn.provider);
        const sets = connections.map((c) => new Set(c.supportedModels || []));
        if (sets.length === 0) return '';
        const shared = [...sets[0]].filter((m) => sets.every((s) => s.has(m)));
        return shared.sort().map((m) => (m.includes('/') ? m : `${prefix}/${m}`)).join('\n');
    });
    const [mode, setMode] = useState<Mode>('replace');

    const lines = text
        .split('\n')
        .map((s) => s.trim())
        .filter(Boolean);
    const canApply = mode === 'clear' || lines.length > 0;

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
                                Update Models — {connections.length} connections
                            </DialogTitle>
                            <DialogDescription className="text-xs mt-1">
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
                                className="flex flex-1 cursor-pointer items-center gap-2 rounded-lg border px-3 py-2 text-xs font-medium transition-colors"
                                style={{
                                    borderColor: mode === value ? 'hsl(var(--primary) / 0.4)' : undefined,
                                    background: mode === value ? 'hsl(var(--primary) / 0.05)' : undefined,
                                }}
                            >
                                <input
                                    type="radio"
                                    name="bulk-models-mode"
                                    value={value}
                                    checked={mode === value}
                                    onChange={() => setMode(value)}
                                    disabled={busy}
                                    className="accent-[hsl(var(--primary))]"
                                />
                                {label}
                            </label>
                        ))}
                    </fieldset>

                    {mode === 'replace' && (
                        <div className="space-y-1">
                            <label htmlFor="bulk-models-list" className="text-xs font-medium">
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
                                className="text-xs font-mono"
                            />
                        </div>
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
