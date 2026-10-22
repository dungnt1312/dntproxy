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
                    <p className="text-sm text-muted-foreground">
                        Pick a provider, or a method to jump straight into setup.
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
                        aria-label="Search providers"
                        className="h-9 pl-9 text-sm"
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
    // Card = a main button (default method) plus sibling method buttons — no nested interactive elements.
    return (
        <div
            className={cn(
                'group relative flex flex-col items-start gap-2 rounded-lg border border-border bg-card p-3 text-left transition',
                'hover:border-primary/30 hover:bg-background focus-within:border-primary/40',
                dense ? 'min-h-0' : 'min-h-28',
            )}
        >
            <button
                type="button"
                onClick={() => onSelect(provider.id)}
                className="flex w-full cursor-pointer items-center gap-2.5 rounded-md text-left focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
            >
                <span className="flex size-9 shrink-0 items-center justify-center rounded-lg bg-muted/70 transition-colors group-hover:bg-muted">
                    {provider.icon}
                </span>
                <span className="min-w-0 flex-1">
                    <span className="block truncate text-sm font-semibold leading-snug">{provider.name}</span>
                    {(provider.accountCount ?? 0) > 0 && (
                        <span className="text-xs tabular-nums text-muted-foreground">
                            {provider.accountCount} connected
                        </span>
                    )}
                </span>
                {isTopUsed && (
                    <Badge variant="secondary" className="h-5 shrink-0 px-1.5 text-xs" title="Your most-used provider">
                        Most used
                    </Badge>
                )}
            </button>

            {!dense && <p className="line-clamp-2 text-sm leading-snug text-muted-foreground">{provider.description}</p>}

            {flows.length > 1 && (
                <div className="flex flex-wrap gap-1" role="group" aria-label={`Connection methods for ${provider.name}`}>
                    {flows.map((flow) => (
                        <button
                            key={flow.id}
                            type="button"
                            title={flow.description}
                            aria-label={`${provider.name}: ${flow.label}`}
                            onClick={() => onSelect(provider.id, flow.id)}
                            className="inline-flex h-7 cursor-pointer items-center gap-1 rounded-md border bg-muted/40 px-2 text-xs font-medium text-muted-foreground transition-colors hover:border-primary/30 hover:bg-primary/5 hover:text-primary focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
                        >
                            {flow.icon}
                            {flow.short}
                        </button>
                    ))}
                </div>
            )}
        </div>
    );
}
