import { useState, type ReactNode } from 'react';
import { ChevronDown } from 'lucide-react';
import { Badge } from '@/components/ui/badge';
import { Collapsible, CollapsibleContent, CollapsibleTrigger } from '@/components/ui/collapsible';
import { cn } from '@/lib/utils';

export type AuthMethodOption<T extends string = string> = {
    id: T;
    label: string;
    description: string;
    icon: ReactNode;
    recommended?: boolean;
};

type Props<T extends string> = {
    value: T;
    onChange: (id: T) => void;
    primary: AuthMethodOption<T>[];
    more?: AuthMethodOption<T>[];
};

function MethodTile<T extends string>({
    option,
    selected,
    onSelect,
    compact,
}: {
    option: AuthMethodOption<T>;
    selected: boolean;
    onSelect: () => void;
    compact?: boolean;
}) {
    return (
        <button
            type="button"
            aria-pressed={selected}
            onClick={onSelect}
            className={cn(
                'flex cursor-pointer items-center gap-2 rounded-lg border text-left text-xs font-medium transition',
                compact ? 'min-h-11 px-3 py-2' : 'min-h-16 flex-col items-center justify-center p-3 text-center',
                selected
                    ? 'border-primary/30 bg-primary/5 text-primary ring-1 ring-primary/10'
                    : 'border-transparent bg-transparent text-muted-foreground hover:border-border hover:bg-muted hover:text-foreground',
            )}
        >
            {option.icon}
            <span className="flex items-center gap-1">
                {option.label}
                {option.recommended && (
                    <Badge variant="secondary" className="h-4 px-1 text-[9px]">
                        Recommended
                    </Badge>
                )}
            </span>
            {!compact && (
                <span className="text-[10px] font-normal text-muted-foreground">{option.description}</span>
            )}
        </button>
    );
}

export function AuthMethodSelector<T extends string>({
    value,
    onChange,
    primary,
    more = [],
}: Props<T>) {
    const moreSelected = more.some((option) => option.id === value);
    // Derived open state: opens automatically when a "more" method is selected,
    // with an explicit user override once they toggle it themselves.
    const [moreOpenOverride, setMoreOpen] = useState<boolean | null>(null);
    const moreOpen = moreOpenOverride ?? moreSelected;

    return (
        <div className="space-y-3">
            {primary.length <= 2 ? (
                <div className="mx-auto flex w-fit gap-1 rounded-lg bg-muted p-1">
                    {primary.map((option) => {
                        const selected = value === option.id;
                        return (
                            <button
                                key={option.id}
                                type="button"
                                aria-pressed={selected}
                                onClick={() => onChange(option.id)}
                                className={cn(
                                    'flex cursor-pointer items-center gap-1.5 rounded-md px-4 py-2 text-xs font-medium transition',
                                    selected
                                        ? 'bg-background text-foreground shadow-sm'
                                        : 'text-muted-foreground hover:text-foreground',
                                )}
                            >
                                {option.icon}
                                {option.label}
                                {option.recommended && (
                                    <Badge variant="secondary" className="h-4 px-1 text-[9px]">
                                        Recommended
                                    </Badge>
                                )}
                            </button>
                        );
                    })}
                </div>
            ) : (
                <div className="grid grid-cols-2 gap-2">
                    {primary.map((option) => (
                        <MethodTile
                            key={option.id}
                            option={option}
                            selected={value === option.id}
                            onSelect={() => onChange(option.id)}
                        />
                    ))}
                </div>
            )}

            {more.length > 0 && (
                <Collapsible open={moreOpen} onOpenChange={setMoreOpen}>
                    <CollapsibleTrigger
                        type="button"
                        className="mx-auto flex cursor-pointer items-center gap-1 text-xs text-muted-foreground hover:text-foreground"
                    >
                        More ways
                        <ChevronDown className={cn('h-3.5 w-3.5 transition-transform', moreOpen && 'rotate-180')} />
                    </CollapsibleTrigger>
                    <CollapsibleContent>
                        <div className="mt-2 grid grid-cols-2 gap-2 sm:grid-cols-4">
                            {more.map((option) => (
                                <MethodTile
                                    key={option.id}
                                    option={option}
                                    selected={value === option.id}
                                    onSelect={() => onChange(option.id)}
                                    compact
                                />
                            ))}
                        </div>
                    </CollapsibleContent>
                </Collapsible>
            )}
        </div>
    );
}
