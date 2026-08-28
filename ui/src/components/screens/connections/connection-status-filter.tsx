import { AlertTriangle, CheckCircle2, CircleOff, Eraser, Layers3, MoreHorizontal, RefreshCw, Trash2 } from 'lucide-react';
import type { ReactNode } from 'react';
import { Button } from '@/components/ui/button';
import { Badge } from '@/components/ui/badge';
import {
    DropdownMenu,
    DropdownMenuContent,
    DropdownMenuItem,
    DropdownMenuLabel,
    DropdownMenuSeparator,
    DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu';
import { cn } from '@/lib/utils';

export type ConnectionStatusFilter = 'all' | 'active' | 'inactive' | 'issues' | 'legacy';

export type IssueQuickAction = 'clear-errors' | 'reset-cooldowns' | 'cleanup';

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
    /** Connections whose models are missing from the current catalog (0 hides the chip). */
    legacy?: number;
    /** Counts powering the Issues-chip quick actions; actions hidden when omitted/zero-able. */
    issueQuickActions?: {
        clearErrors: number;
        resetCooldowns: number;
        cleanupCandidates: number;
        onRun: (action: IssueQuickAction) => void;
    };
    onChange: (value: ConnectionStatusFilter) => void;
}

export function ConnectionStatusFilter({
    value,
    total,
    active,
    inactive,
    issues,
    legacy = 0,
    issueQuickActions,
    onChange,
}: ConnectionStatusFilterProps) {
    const options: StatusFilterOption[] = [
        { value: 'all', label: 'All', count: total, icon: <Layers3 className="h-3.5 w-3.5" /> },
        { value: 'active', label: 'Active', count: active, icon: <CheckCircle2 className="h-3.5 w-3.5" /> },
        { value: 'inactive', label: 'Idle', count: inactive, icon: <CircleOff className="h-3.5 w-3.5" /> },
        { value: 'issues', label: 'Issues', count: issues, icon: <AlertTriangle className="h-3.5 w-3.5" /> },
    ];
    if (legacy > 0) {
        options.push({ value: 'legacy', label: 'Legacy', count: legacy, icon: <AlertTriangle className="h-3.5 w-3.5 text-amber-500" /> });
    }

    const showIssueMenu =
        issueQuickActions !== undefined &&
        (issueQuickActions.clearErrors > 0 || issueQuickActions.resetCooldowns > 0 || issueQuickActions.cleanupCandidates > 0);

    return (
        <div className="flex w-full flex-wrap gap-1 rounded-lg border bg-background p-1 sm:w-auto">
            {options.map((option) => {
                const isSelected = option.value === value;

                return (
                    <div key={option.value} className={cn('relative flex-1 sm:flex-none', isSelected && 'rounded-md shadow-sm')}>
                        <Button
                            type="button"
                            variant={isSelected ? 'secondary' : 'ghost'}
                            size="sm"
                            className={cn(
                                'h-8 w-full gap-1.5 px-2 text-xs',
                                option.value === 'issues' && showIssueMenu && 'pr-6',
                            )}
                            aria-pressed={isSelected}
                            onClick={() => onChange(option.value)}
                        >
                            {option.icon}
                            <span>{option.label}</span>
                            <Badge variant="outline" className="h-4 min-w-5 px-1 text-[10px] tabular-nums">
                                {option.count}
                            </Badge>
                        </Button>

                        {/* Split-button caret exposing preset ops over the issue set */}
                        {option.value === 'issues' && showIssueMenu && (
                            <DropdownMenu>
                                <DropdownMenuTrigger asChild>
                                    <button
                                        type="button"
                                        aria-label="Quick actions for problem connections"
                                        title="Quick actions"
                                        className="absolute right-0.5 top-1/2 flex h-7 -translate-y-1/2 cursor-pointer items-center rounded-md px-1 text-muted-foreground hover:bg-muted hover:text-foreground"
                                    >
                                        <MoreHorizontal size={12} />
                                    </button>
                                </DropdownMenuTrigger>
                                <DropdownMenuContent align="end" className="w-60">
                                    <DropdownMenuLabel className="text-[10px] uppercase tracking-wider text-muted-foreground">
                                        Apply to {issues} results
                                    </DropdownMenuLabel>
                                    {issueQuickActions.clearErrors > 0 && (
                                        <DropdownMenuItem
                                            onClick={() => issueQuickActions.onRun('clear-errors')}
                                            className="cursor-pointer gap-2 text-xs"
                                        >
                                            <Eraser size={13} /> Clear errors ({issueQuickActions.clearErrors})
                                        </DropdownMenuItem>
                                    )}
                                    {issueQuickActions.resetCooldowns > 0 && (
                                        <DropdownMenuItem
                                            onClick={() => issueQuickActions.onRun('reset-cooldowns')}
                                            className="cursor-pointer gap-2 text-xs"
                                        >
                                            <RefreshCw size={13} /> Reset cooldowns ({issueQuickActions.resetCooldowns})
                                        </DropdownMenuItem>
                                    )}
                                    {issueQuickActions.cleanupCandidates > 0 && (
                                        <>
                                            <DropdownMenuSeparator />
                                            <DropdownMenuItem
                                                onClick={() => issueQuickActions.onRun('cleanup')}
                                                className="cursor-pointer gap-2 text-xs text-destructive focus:text-destructive focus:bg-destructive/10"
                                            >
                                                <Trash2 size={13} /> Clean up broken… ({issueQuickActions.cleanupCandidates})
                                            </DropdownMenuItem>
                                        </>
                                    )}
                                </DropdownMenuContent>
                            </DropdownMenu>
                        )}
                    </div>
                );
            })}
        </div>
    );
}
