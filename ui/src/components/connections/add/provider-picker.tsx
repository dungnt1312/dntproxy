import { useMemo, useState, type ReactNode } from 'react';
import { Search } from 'lucide-react';
import { Badge } from '@/components/ui/badge';
import { Input } from '@/components/ui/input';
import { cn } from '@/lib/utils';
import { authFlowOptions, type AuthFlowInfo } from '@/lib/provider-auth-flows';

export type ProviderPickerItem = {
    id: string;
    name: string;
    description: string;
    icon: ReactNode;
    /** Number of existing connections on this provider (drives the "most used" hint). */
    accountCount?: number;
    /** Raw auth flow ids from provider metadata; rendered as chips. */
    authFlows: readonly string[];
};

type Props = {
    providers: ProviderPickerItem[];
    /** Selecting a chip jumps straight into setup for that method. */
    onSelect: (providerId: string, method?: string) => void;
    /** Tighter two-column layout for use inside dialogs. */
    dense?: boolean;
};

export function ProviderPicker({ providers, onSelect, dense = false }: Props) {
    const [query, setQuery] = useState('');
    const filtered = useMemo(() => {
        const needle = query.trim().toLowerCase();
        if (!needle) return providers;
        return providers.filter((provider) =>
            [provider.name, provider.description, ...provider.authFlows, provider.id].join(' ').toLowerCase().includes(needle),
        );
    }, [providers, query]);

    const maxCount = Math.max(0, ...providers.map((p) => p.accountCount ?? 0));

    return (
        <div className="space-y-3 rounded-xl border bg-muted/20 p-3">
            <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
                <div>
                    <p className="text-sm font-semibold">Choose account type</p>
                    <p className="text-xs text-muted-foreground">
                        Click a method to jump straight into setup.
                    </p>
                </div>
                <div className="relative w-full sm:w-72">
                    <Search
                        size={14}
                        className="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-muted-foreground"
                    />
                    <Input
                        type="search"
                        name="provider-picker-filter"
                        value={query}
                        onChange={(event) => setQuery(event.target.value)}
                        placeholder="Search providers or methods…"
                        className="h-8 pl-9 text-sm"
                        autoComplete="off"
                        data-1p-ignore
                    />
                </div>
            </div>
            <div className={cn('grid grid-cols-1 gap-2', dense ? 'sm:grid-cols-2' : 'sm:grid-cols-2 lg:grid-cols-4')}>
                {filtered.map((provider) => (
                    <ProviderCard
                        key={provider.id}
                        provider={provider}
                        isTopUsed={!dense && maxCount > 0 && provider.accountCount === maxCount && maxCount >= 3}
                        flows={authFlowOptions(provider.authFlows)}
                        onSelect={onSelect}
                        dense={dense}
                    />
                ))}
            </div>
            {filtered.length === 0 && (
                <div className="rounded-lg border border-dashed bg-background py-8 text-center text-sm text-muted-foreground">
                    No providers match “{query.trim()}”.
                </div>
            )}
        </div>
    );
}

function ProviderCard({
    provider,
    flows,
    isTopUsed,
    onSelect,
    dense,
}: {
    provider: ProviderPickerItem;
    flows: AuthFlowInfo[];
    isTopUsed: boolean;
    onSelect: (providerId: string, method?: string) => void;
    dense?: boolean;
}) {
    // Single-flow providers: the whole card is one target.
    const defaultMethod = flows[0];

    return (
        <button
            type="button"
            onClick={() => onSelect(provider.id)}
            className={cn(
                'group flex cursor-pointer flex-col items-start gap-2 rounded-lg border border-border bg-card p-3 text-left transition',
                'hover:border-primary/20 hover:bg-background',
                dense ? 'min-h-0' : 'min-h-28',
            )}
        >
            <div className="flex w-full items-center gap-2.5">
                <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg bg-muted/70 transition-colors group-hover:bg-muted">
                    {provider.icon}
                </div>
                <div className="min-w-0 flex-1">
                    <span className="block truncate text-sm font-semibold leading-snug">{provider.name}</span>
                    {(provider.accountCount ?? 0) > 0 && (
                        <span className="text-[10px] tabular-nums text-muted-foreground">
                            {provider.accountCount} accounts
                        </span>
                    )}
                </div>
                {isTopUsed && (
                    <Badge variant="secondary" className="h-5 shrink-0 px-1.5 text-[9px]" title="Your most-used provider">
                        ⭐ Most used
                    </Badge>
                )}
            </div>

            {!dense && (
                <p className="line-clamp-2 text-xs leading-snug text-muted-foreground">{provider.description}</p>
            )}

            {defaultMethod ? (
                <div
                    className="flex flex-wrap gap-1"
                    role="group"
                    aria-label={`Connection methods for ${provider.name}`}
                >
                    {flows.map((flow) => (
                        <span
                            key={flow.id}
                            role="button"
                            tabIndex={0}
                            title={flow.description}
                            aria-label={`${provider.name} — ${flow.label}`}
                            onKeyDown={(e) => {
                                if (e.key === 'Enter' || e.key === ' ') {
                                    e.preventDefault();
                                    e.stopPropagation();
                                    onSelect(provider.id, flow.id);
                                }
                            }}
                            onClick={(e) => {
                                e.stopPropagation();
                                onSelect(provider.id, flow.id);
                            }}
                            className="inline-flex cursor-pointer items-center gap-1 rounded-md border bg-muted/40 px-1.5 py-0.5 text-[10px] font-medium text-muted-foreground transition-colors hover:border-primary/30 hover:bg-primary/5 hover:text-primary"
                        >
                            {flow.icon}
                            {flow.short}
                        </span>
                    ))}
                </div>
            ) : null}
        </button>
    );
}
