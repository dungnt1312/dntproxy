import { useMemo, useState, type ReactNode } from 'react';
import { KeyRound, Search } from 'lucide-react';
import { Badge } from '@/components/ui/badge';
import { Input } from '@/components/ui/input';
import { cn } from '@/lib/utils';

export type ProviderPickerItem = {
    id: string;
    name: string;
    description: string;
    auth: string;
    recommended?: boolean;
    icon: ReactNode;
};

type Props = {
    providers: ProviderPickerItem[];
    onSelect: (providerId: string) => void;
};

export function ProviderPicker({ providers, onSelect }: Props) {
    const [query, setQuery] = useState('');
    const filtered = useMemo(() => {
        const needle = query.trim().toLowerCase();
        if (!needle) return providers;
        return providers.filter((provider) =>
            [provider.name, provider.description, provider.auth, provider.id].join(' ').toLowerCase().includes(needle),
        );
    }, [providers, query]);

    return (
        <div className="space-y-3 rounded-xl border bg-muted/20 p-3">
            <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
                <div>
                    <p className="text-sm font-semibold">Choose provider</p>
                    <p className="text-xs text-muted-foreground">
                        Pick the account type first. Setup options appear on the next step.
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
                        placeholder="Search providers or auth..."
                        className="h-8 pl-9 text-sm"
                        autoComplete="off"
                        data-1p-ignore
                    />
                </div>
            </div>
            <div className="grid grid-cols-1 gap-2 sm:grid-cols-2 lg:grid-cols-4">
                {filtered.map((provider) => (
                    <button
                        type="button"
                        key={provider.id}
                        onClick={() => onSelect(provider.id)}
                        className={cn(
                            'group flex min-h-28 cursor-pointer items-start gap-3 rounded-lg border border-border bg-card p-3 text-left text-muted-foreground transition',
                            'hover:border-primary/20 hover:bg-background hover:text-foreground',
                        )}
                    >
                        <div className="flex h-10 w-10 shrink-0 items-center justify-center rounded-lg bg-muted/70 transition-colors group-hover:bg-muted">
                            {provider.icon}
                        </div>
                        <div className="min-w-0 flex-1 space-y-1">
                            <div className="flex items-center gap-2">
                                <span className="text-sm font-semibold leading-snug text-foreground">{provider.name}</span>
                                {provider.recommended && (
                                    <Badge variant="secondary" className="h-5 px-1.5 text-[10px]">
                                        Recommended
                                    </Badge>
                                )}
                            </div>
                            <p className="text-xs leading-snug text-muted-foreground">{provider.description}</p>
                            <p className="flex items-center gap-1 text-[11px] text-muted-foreground">
                                <KeyRound size={11} /> {provider.auth}
                            </p>
                        </div>
                    </button>
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
