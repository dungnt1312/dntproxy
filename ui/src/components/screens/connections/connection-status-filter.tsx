import { ChevronDown, Eraser, RefreshCw } from 'lucide-react';
import { Button } from '@/components/ui/button';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu';
import { cn } from '@/lib/utils';

export type ConnectionStatusFilter = 'all' | 'active' | 'inactive' | 'issues';

export type IssueQuickAction = 'clear-errors' | 'reset-cooldowns';

interface ConnectionStatusFilterProps {
  value: ConnectionStatusFilter;
  counts: Record<ConnectionStatusFilter, number>;
  quickActions: { clearErrors: number; resetCooldowns: number; onRun: (action: IssueQuickAction) => void };
  onChange: (value: ConnectionStatusFilter) => void;
}

const OPTIONS: Array<{ value: ConnectionStatusFilter; label: string }> = [
  { value: 'all', label: 'All' },
  { value: 'active', label: 'Enabled' },
  { value: 'inactive', label: 'Disabled' },
  { value: 'issues', label: 'Issues' },
];

export function ConnectionStatusFilter({ value, counts, quickActions, onChange }: ConnectionStatusFilterProps) {
  const hasQuickActions = quickActions.clearErrors > 0 || quickActions.resetCooldowns > 0;

  return (
    <div className="flex flex-wrap items-center gap-2">
      <div role="radiogroup" aria-label="Filter by status" className="inline-flex rounded-lg border bg-background p-0.5">
        {OPTIONS.map((option) => {
          const selected = option.value === value;
          const isIssues = option.value === 'issues';
          return (
            <button
              key={option.value}
              type="button"
              role="radio"
              aria-checked={selected}
              onClick={() => onChange(option.value)}
              className={cn(
                'inline-flex h-8 cursor-pointer items-center gap-1.5 rounded-md px-3 text-sm transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring',
                selected ? 'bg-secondary font-medium text-foreground shadow-sm' : 'text-muted-foreground hover:text-foreground',
              )}
            >
              {option.label}
              <span
                className={cn(
                  'min-w-5 rounded px-1 text-xs tabular-nums',
                  isIssues && counts.issues > 0 ? 'bg-destructive/10 text-destructive' : 'bg-muted text-muted-foreground',
                )}
              >
                {counts[option.value]}
              </span>
            </button>
          );
        })}
      </div>

      {hasQuickActions && (
        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <Button variant="outline" size="sm" className="h-9 gap-1.5">
              Fix issues <ChevronDown className="size-3.5" />
            </Button>
          </DropdownMenuTrigger>
          <DropdownMenuContent align="start" className="w-60">
            <DropdownMenuLabel className="text-xs font-normal text-muted-foreground">Applies to the current filter</DropdownMenuLabel>
            {quickActions.clearErrors > 0 && (
              <DropdownMenuItem onClick={() => quickActions.onRun('clear-errors')} className="gap-2">
                <Eraser className="size-4" /> Clear errors ({quickActions.clearErrors})
              </DropdownMenuItem>
            )}
            {quickActions.resetCooldowns > 0 && (
              <DropdownMenuItem onClick={() => quickActions.onRun('reset-cooldowns')} className="gap-2">
                <RefreshCw className="size-4" /> Reset cooldowns ({quickActions.resetCooldowns})
              </DropdownMenuItem>
            )}
          </DropdownMenuContent>
        </DropdownMenu>
      )}
    </div>
  );
}
