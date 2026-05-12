import { AlertTriangle, CheckCircle2, CircleOff, Layers3 } from 'lucide-react';
import type { ReactNode } from 'react';
import { Button } from '@/components/ui/button';
import { Badge } from '@/components/ui/badge';
import { cn } from '@/lib/utils';

export type ConnectionStatusFilter = 'all' | 'active' | 'inactive' | 'issues';

interface StatusFilterOption {
    value: ConnectionStatusFilter;
    label: string;
    count: number;
    icon: ReactNode;
}

interface ConnectionStatusFilterProps {
    value: ConnectionStatusFilter;
    total: number;
    active: number;
    inactive: number;
    issues: number;
    onChange: (value: ConnectionStatusFilter) => void;
}

export function ConnectionStatusFilter({
    value,
    total,
    active,
    inactive,
    issues,
    onChange,
}: ConnectionStatusFilterProps) {
    const options: StatusFilterOption[] = [
        { value: 'all', label: 'All', count: total, icon: <Layers3 className="h-3.5 w-3.5" /> },
        { value: 'active', label: 'Active', count: active, icon: <CheckCircle2 className="h-3.5 w-3.5" /> },
        { value: 'inactive', label: 'Inactive', count: inactive, icon: <CircleOff className="h-3.5 w-3.5" /> },
        { value: 'issues', label: 'Issues', count: issues, icon: <AlertTriangle className="h-3.5 w-3.5" /> },
    ];

    return (
        <div className="flex w-full flex-wrap gap-1 rounded-lg border bg-background p-1 sm:w-auto">
            {options.map((option) => {
                const isSelected = option.value === value;

                return (
                    <Button
                        key={option.value}
                        type="button"
                        variant={isSelected ? 'secondary' : 'ghost'}
                        size="sm"
                        className={cn(
                            'h-8 flex-1 gap-1.5 px-2 text-xs sm:flex-none',
                            isSelected && 'shadow-sm',
                        )}
                        aria-pressed={isSelected}
                        onClick={() => onChange(option.value)}
                    >
                        {option.icon}
                        <span>{option.label}</span>
                        <Badge variant="outline" className="h-4 min-w-5 px-1 text-[10px]">
                            {option.count}
                        </Badge>
                    </Button>
                );
            })}
        </div>
    );
}
