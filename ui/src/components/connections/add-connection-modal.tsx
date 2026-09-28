import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { AlertTriangle, ArrowLeft, Check, Loader2 } from 'lucide-react';
import { cn } from '@/lib/utils';
import { api } from '@/api';
import { ProviderLogoIcon } from './helpers';
import { ApiKeyConnectionForm } from './ApiKeyConnectionForm';
import { CommandCodeConnectionForm } from './CommandCodeConnectionForm';
import { KiroConnectionForm } from './add/kiro-connection-form';
import { OpenAIConnectionForm } from './add/openai-connection-form';
import { ProviderPicker } from './add/provider-picker';
import { QwenConnectionForm } from './add/qwen-connection-form';
import {
    errorMessage,
    type CreateResult,
    type OnSuccess,
} from './add/helpers';
import { XaiConnectionForm } from './add/xai-connection-form';
import { ConnectionResultPanel } from './add/connection-result-panel';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import {
    Dialog,
    DialogContent,
    DialogDescription,
    DialogTitle,
} from '@/components/ui/dialog';
import { authFlowOptions } from '@/lib/provider-auth-flows';
import { getModelProviderId, usesGenericApiKeyForm } from '@/lib/provider-registry';
import { useProviderCatalog } from '@/lib/use-provider-catalog';
import type { Connection } from '@/types/connections';
import type { CreateConnectionPayload, ProviderConfigMeta } from '@/types/provider-metadata';

type Props = {
    open: boolean;
    onOpenChange: (open: boolean) => void;
    /** Preselect a provider when opening from a per-provider "+" action. */
    initialProvider?: string | null;
    /** Fired after a connection was created so the host can reload its data. */
    onCreated?: () => void;
};

const STEPS = ['Provider', 'Connect', 'Verify'] as const;

export function AddConnectionModal({ open, onOpenChange, initialProvider, onCreated }: Props) {
    const { providers, loading: catalogLoading, error: catalogError, reload } = useProviderCatalog();
    const [selectedId, setSelectedId] = useState<string | null>(null);
    const [method, setMethod] = useState('');
    const [result, setResult] = useState<CreateResult | null>(null);
    const [error, setError] = useState('');
    const [loading, setLoading] = useState(false);
    const [providerCounts, setProviderCounts] = useState<Record<string, number>>({});
    const accountNameRef = useRef('');

    // Reset state when the dialog opens — React's canonical "adjust state on
    // prop change during render" pattern instead of an effect.
    const [prevOpen, setPrevOpen] = useState(open);
    if (open !== prevOpen) {
        setPrevOpen(open);
        if (open) {
            setSelectedId(initialProvider ?? null);
            setMethod('');
            setResult(null);
            setError('');
        }
    }

    // Existing connections per provider power usage hints + name suggestion.
    useEffect(() => {
        if (!open) return;
        let alive = true;
        void api
            .getConnections()
            .then((conns: Connection[]) => {
                if (!alive) return;
                const counts: Record<string, number> = {};
                for (const c of conns) counts[c.provider] = (counts[c.provider] ?? 0) + 1;
                setProviderCounts(counts);
            })
            .catch(() => {});
        return () => {
            alive = false;
        };
    }, [open]);

    const selected = useMemo(
        () => providers.find((p) => p.id === (selectedId ?? '')) ?? null,
        [providers, selectedId],
    );
    const flows = useMemo(
        () => (selected ? authFlowOptions(selected.ui.authFlows) : []),
        [selected],
    );

    // Default method: prefer oauth when offered (the zero-key path), then the
    // backend's preference, then catalog order.
    const effectiveMethod =
        method ||
        (flows.some((f) => f.id === 'oauth') ? 'oauth' : '') ||
        selected?.ui.preferredAuthMethod ||
        selected?.ui.authFlows[0] ||
        '';

    const chooseProvider = useCallback((id: string, chosenMethod?: string) => {
        setError('');
        setMethod(chosenMethod ?? '');
        setSelectedId(id);
    }, []);

    const handleBack = () => {
        if (result) {
            setResult(null);
            return;
        }
        if (selectedId) {
            setSelectedId(null);
            setMethod('');
            setError('');
        }
    };

    /** Apply the user's chosen name after server-created flows (OAuth/import). */
    const finalizeResult = useCallback(async (incoming?: CreateResult): Promise<CreateResult | undefined> => {
        const wanted = accountNameRef.current.trim();
        if (!incoming?.id || !wanted || incoming.name === wanted) return incoming;
        try {
            await api.updateConnection(incoming.id, { name: wanted });
            return { ...incoming, name: wanted };
        } catch {
            // Renaming is cosmetic — never block onboarding for it.
            return incoming;
        }
    }, []);

    const handleSuccess = useCallback(
        (_message: string, incoming?: CreateResult) => {
            void finalizeResult(incoming).then((finalResult) => {
                setError('');
                setResult(finalResult ?? {});
                onCreated?.();
            });
        },
        [finalizeResult, onCreated],
    );

    const handleCreateConnection = useCallback(
        async (payload: CreateConnectionPayload) => {
            setLoading(true);
            setError('');
            const wanted = accountNameRef.current.trim();
            const enriched: CreateConnectionPayload = wanted ? { ...payload, name: wanted } : payload;
            try {
                const response = (await api.createConnection(enriched)) as CreateResult;
                handleSuccess('Connected!', response);
            } catch (err: unknown) {
                setError(errorMessage(err, 'Failed to add connection'));
            } finally {
                setLoading(false);
            }
        },
        [handleSuccess],
    );

    const step: 1 | 2 | 3 = result ? 3 : selected ? 2 : 1;

    return (
        <Dialog open={open} onOpenChange={onOpenChange}>
            <DialogContent className="flex h-[min(640px,85vh)] w-[min(560px,94vw)] flex-col gap-0 overflow-hidden p-0 sm:max-w-[560px]">
                {/* ── Header (fixed) ────────────────────────────────────────── */}
                <div className="flex shrink-0 items-center gap-2.5 border-b bg-muted/30 py-3 pl-3 pr-12">
                    <button
                        type="button"
                        onClick={handleBack}
                        aria-label={step === 1 ? 'Close' : 'Back'}
                        disabled={loading}
                        className={cn(
                            'flex h-7 w-7 shrink-0 cursor-pointer items-center justify-center rounded-md text-muted-foreground transition-colors',
                            'hover:bg-muted hover:text-foreground',
                            step === 1 && 'invisible', // keeps the logo aligned across steps
                        )}
                    >
                        <ArrowLeft size={15} />
                    </button>
                    {selected ? (
                        <>
                            <span className="flex h-7 w-7 shrink-0 items-center justify-center overflow-hidden rounded-md border bg-background">
                                <ProviderLogoIcon provider={selected.id} size={18} />
                            </span>
                            <div className="min-w-0">
                                <DialogTitle className="truncate text-sm font-semibold leading-tight">
                                    Connect {selected.name}
                                </DialogTitle>
                                <DialogDescription className="truncate text-[11px] leading-tight">
                                    {authFlowOptions([effectiveMethod])[0]?.label ?? effectiveMethod}
                                </DialogDescription>
                            </div>
                        </>
                    ) : (
                        <div className="min-w-0">
                            <DialogTitle className="truncate text-sm font-semibold leading-tight">Add connection</DialogTitle>
                            <DialogDescription className="truncate text-[11px] leading-tight">
                                Pick an account type and how to connect it.
                            </DialogDescription>
                        </div>
                    )}
                </div>

                {/* ── Stepper row (centered) ────────────────────────────────── */}
                <ol className="flex shrink-0 items-center justify-center gap-2 border-b bg-muted/10 py-2" aria-label="Steps">
                    {STEPS.map((label, i) => {
                        const n = (i + 1) as 1 | 2 | 3;
                        const state = n < step ? 'done' : n === step ? 'active' : 'todo';
                        return (
                            <li key={label} className="flex items-center gap-2">
                                {i > 0 && <span aria-hidden className="h-px w-8 bg-border" />}
                                <span
                                    aria-current={state === 'active' ? 'step' : undefined}
                                    className={cn(
                                        'flex items-center gap-1.5 text-[11px] font-medium',
                                        state === 'done' && 'text-emerald-600',
                                        state === 'active' && 'text-foreground',
                                        state === 'todo' && 'text-muted-foreground/50',
                                    )}
                                >
                                    <span
                                        className={cn(
                                            'flex h-4 w-4 items-center justify-center rounded-full border text-[9px] tabular-nums',
                                            state === 'done' && 'border-emerald-500/40 bg-emerald-500/10 text-emerald-600',
                                            state === 'active' && 'border-primary bg-primary text-primary-foreground',
                                            state === 'todo' && 'border-border',
                                        )}
                                    >
                                        {state === 'done' ? <Check size={9} /> : n}
                                    </span>
                                    {label}
                                </span>
                            </li>
                        );
                    })}
                </ol>

                {/* ── Body (scrollable, centered column) ────────────────────── */}
                <div className="min-h-0 flex-1 overflow-y-auto px-5 py-5">
                    {catalogError && !selected && (
                        <div role="status" className="mx-auto mb-4 flex max-w-sm items-center gap-2 rounded-lg border border-amber-500/30 bg-amber-500/5 px-3 py-2 text-xs text-muted-foreground">
                            <AlertTriangle size={13} className="shrink-0 text-amber-500" />
                            <span className="flex-1">Couldn't refresh the provider catalog — showing saved providers.</span>
                            <Button size="sm" variant="outline" className="h-6" onClick={() => void reload()}>
                                Retry
                            </Button>
                        </div>
                    )}

                    {catalogLoading && !providers.length ? (
                        <Centered>
                            <Loader2 className="h-5 w-5 animate-spin text-primary" />
                            Loading providers…
                        </Centered>
                    ) : !selected ? (
                        <ProviderPicker
                            dense
                            providers={providers.map((p) => ({
                                id: p.id,
                                name: p.name,
                                description: p.ui.description,
                                icon: <ProviderLogoIcon provider={p.id} size={22} />,
                                authFlows: p.ui.authFlows,
                                accountCount: providerCounts[p.id] ?? 0,
                            }))}
                            onSelect={chooseProvider}
                        />
                    ) : (
                        <div className="mx-auto flex w-full max-w-sm flex-col items-center gap-5">
                            {/* Visible method chips — never a dropdown */}
                            {flows.length > 1 && (
                                <section aria-label="Connection method" className="w-full space-y-2 text-center">
                                    <SectionLabel>Method</SectionLabel>
                                    <div className="flex flex-wrap justify-center gap-1.5">
                                        {flows.map((flow) => {
                                            const active = flow.id === effectiveMethod;
                                            return (
                                                <button
                                                    key={flow.id}
                                                    type="button"
                                                    onClick={() => {
                                                        setError('');
                                                        setMethod(flow.id);
                                                    }}
                                                    title={flow.description}
                                                    aria-pressed={active}
                                                    className={cn(
                                                        'inline-flex h-8 cursor-pointer select-none items-center gap-1.5 rounded-lg border px-3 text-xs font-medium transition-colors',
                                                        active
                                                            ? 'border-primary/40 bg-primary/10 text-primary ring-1 ring-primary/15'
                                                            : 'border-border bg-background text-muted-foreground hover:border-primary/25 hover:text-foreground',
                                                    )}
                                                >
                                                    {flow.icon}
                                                    {flow.label}
                                                </button>
                                            );
                                        })}
                                    </div>
                                </section>
                            )}

                            {/* Common account details */}
                            <AccountNameSection provider={selected} nameRef={accountNameRef} />

                            {/* Per-provider form bodies */}
                            <div className="w-full border-t pt-5">
                                <SetupForm
                                    provider={selected}
                                    method={effectiveMethod}
                                    loading={loading}
                                    onCreate={handleCreateConnection}
                                    onError={setError}
                                    onSuccess={handleSuccess}
                                    onImported={() => onCreated?.()}
                                />
                            </div>
                        </div>
                    )}

                    {result && selected && (
                        <div className="mx-auto w-full max-w-sm">
                            <ConnectionResultPanel
                                providerName={selected.name}
                                providerId={selected.id}
                                result={result}
                                onAddAnother={() => setResult(null)}
                                onDone={() => onOpenChange(false)}
                            />
                        </div>
                    )}
                </div>

                {/* ── Error strip ──────────────────────────────────────────── */}
                {error && (
                    <div role="alert" className="flex shrink-0 items-start gap-2 border-t border-destructive/20 bg-destructive/5 px-4 py-2.5 text-xs text-destructive">
                        <AlertTriangle size={14} className="mt-px shrink-0" />
                        <span className="min-w-0 break-words">{error}</span>
                        <button
                            type="button"
                            aria-label="Dismiss error"
                            onClick={() => setError('')}
                            className="ml-auto shrink-0 cursor-pointer font-medium hover:underline"
                        >
                            Dismiss
                        </button>
                    </div>
                )}
            </DialogContent>
        </Dialog>
    );
}

// ─── Shared bits ──────────────────────────────────────────────────────────────

function Centered({ children }: { children: React.ReactNode }) {
    return <div className="flex h-full flex-col items-center justify-center gap-3 py-14 text-center text-sm text-muted-foreground">{children}</div>;
}

function SectionLabel({ children }: { children: React.ReactNode }) {
    return <span className="block text-center text-[10px] font-bold uppercase tracking-wider text-muted-foreground">{children}</span>;
}

async function suggestAccountName(provider: ProviderConfigMeta, counts: Record<string, number>): Promise<string> {
    let prefix = provider.id;
    try {
        prefix = getModelProviderId(provider.id) || provider.id;
    } catch {
        // fall back to raw id
    }
    return `${prefix}-${String((counts[provider.id] ?? 0) + 1).padStart(2, '0')}`;
}

/** Owns its own input state; syncs the latest value up through nameRef. */
function AccountNameSection({
    provider,
    nameRef,
}: {
    provider: ProviderConfigMeta;
    nameRef: React.RefObject<string>;
}) {
    const [value, setValue] = useState('');
    const [suggestion, setSuggestion] = useState('');

    useEffect(() => {
        let alive = true;
        void api
            .getConnections()
            .then((conns: Connection[]) => {
                if (!alive) return;
                const counts: Record<string, number> = {};
                for (const c of conns) counts[c.provider] = (counts[c.provider] ?? 0) + 1;
                void suggestAccountName(provider, counts).then((s) => alive && setSuggestion(s));
            })
            .catch(() => {});
        return () => {
            alive = false;
        };
    }, [provider]);

    useEffect(() => {
        nameRef.current = value;
    }, [value, nameRef]);

    return (
        <section className="w-full space-y-1.5">
            <SectionLabel>Connection name</SectionLabel>
            <Input
                autoComplete="off"
                data-1p-ignore
                value={value}
                onChange={(event) => setValue(event.target.value)}
                placeholder={`${suggestion || `${provider.id}-01`} (optional)`}
                className="mx-auto block h-8 max-w-[260px] text-center text-sm"
            />
        </section>
    );
}

// ─── Per-provider form dispatch ───────────────────────────────────────────────

function SetupForm({
    provider,
    method,
    loading,
    onCreate,
    onError,
    onSuccess,
    onImported,
}: {
    provider: ProviderConfigMeta;
    method: string;
    loading: boolean;
    onCreate: (payload: CreateConnectionPayload) => Promise<void>;
    onError: (message: string) => void;
    onSuccess: OnSuccess;
    onImported: () => void;
}) {
    // onBusyChange guards navigation in the old page world; inside a modal there
    // is no route to guard, so a no-op keeps the form contracts intact.
    const noopBusy = useCallback(() => {}, []);
    return (
        <>
            {provider.id === 'kiro' && (
                <KiroConnectionForm initialMethod={method} onSuccess={onSuccess} onError={onError} onBusyChange={noopBusy} />
            )}
            {provider.id === 'openai' && (
                <OpenAIConnectionForm provider={provider} mode={method} loading={loading} onCreate={onCreate} onSuccess={onSuccess} onError={onError} onBusyChange={noopBusy} onImported={onImported} />
            )}
            {provider.id === 'xai' && (
                <XaiConnectionForm mode={method} onSuccess={onSuccess} onError={onError} onBusyChange={noopBusy} />
            )}
            {provider.id === 'qwen' && (
                <QwenConnectionForm provider={provider} mode={method} loading={loading} onCreate={onCreate} onSuccess={onSuccess} onError={onError} onBusyChange={noopBusy} />
            )}
            {provider.id === 'commandcode' && (
                <CommandCodeConnectionForm loading={loading} onCreate={onCreate} onSuccess={(message) => onSuccess(message)} onError={onError} onBusyChange={noopBusy} />
            )}
            {usesGenericApiKeyForm(provider) && (
                <ApiKeyConnectionForm key={provider.id} provider={provider} loading={loading} onSubmit={onCreate} />
            )}
        </>
    );
}
