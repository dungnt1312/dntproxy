import { Check, ChevronDown, RefreshCw, Zap } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Badge } from '@/components/ui/badge';
import { cn } from '@/lib/utils';
import ConnectionCard from '@/components/connections/ConnectionCard';
import type { Connection, ConnectionGroup, UsageData } from '@/types/connections';

interface ConnectionGroupProps {
    group: ConnectionGroup;
    isCollapsed: boolean;
    isFetching: boolean;
    hasFetched: boolean;
    quotaResult: Record<string, UsageData>;
    onToggle: () => void;
    onFetchQuota: () => void;
    onReload: () => void;
    onDelete: (id: string, name: string) => void;
    onEditModels: (conn: Connection) => void;
    onEditConnection: (conn: Connection) => void;
    onViewDetails?: (conn: Connection) => void;
    selectionMode?: boolean;
    selectedIds?: ReadonlySet<string>;
    onToggleSelect?: (id: string) => void;
    onToggleSelectAll?: (ids: string[], selectAll: boolean) => void;
}

/**
 * Collapsible provider group with quota fetch button and connection cards.
 */
export function ConnectionGroup({
    group,
    isCollapsed,
    isFetching,
    hasFetched,
    quotaResult,
    onToggle,
    onFetchQuota,
    onReload,
    onDelete,
    onEditModels,
    onEditConnection,
    onViewDetails,
    selectionMode = false,
    selectedIds,
    onToggleSelect,
    onToggleSelectAll,
}: ConnectionGroupProps) {
    const hasActiveItems = group.items.some((c) => c.isActive);
    const activeCount = group.items.filter((c) => c.isActive).length;
    const inactiveCount = group.items.length - activeCount;
    const groupIds = group.items.map((c) => c.id);
    const selectedInGroup = selectionMode
        ? groupIds.filter((id) => selectedIds?.has(id)).length
        : 0;
    const allInGroupSelected = selectionMode && selectedInGroup === group.items.length;

    return (
        <div>
            <div className="flex items-center gap-2 mb-3">
                {/* Group-level select-all checkbox (bulk mode only) */}
                {selectionMode && onToggleSelectAll && (
                    <button
                        type="button"
                        role="checkbox"
                        aria-checked={allInGroupSelected}
                        aria-label={`${allInGroupSelected ? 'Deselect all' : 'Select all'} ${group.label} connections`}
                        onClick={() => onToggleSelectAll(groupIds, !allInGroupSelected)}
                        className={cn(
                            'flex h-5 w-5 shrink-0 cursor-pointer items-center justify-center rounded border transition-colors',
                            allInGroupSelected
                                ? 'border-primary bg-primary text-primary-foreground'
                                : 'border-input bg-background hover:border-primary/50',
                        )}
                    >
                        {allInGroupSelected && <Check className="h-3 w-3" aria-hidden="true" />}
                    </button>
                )}

                {/* Clickable group header (toggle collapse) */}
                <button
                    type="button"
                    className="flex items-center gap-2 cursor-pointer select-none flex-1 group/col text-left"
                    onClick={onToggle}
                    aria-expanded={!isCollapsed}
                    aria-label={`${isCollapsed ? 'Expand' : 'Collapse'} ${group.label} connections`}
                >
                    <div
                        className={cn(
                            'flex h-7 w-7 items-center justify-center rounded-lg border transition-transform group-hover/col:scale-105 overflow-hidden',
                            group.colorClass,
                        )}
                    >
                        {group.icon}
                    </div>
                    <h3 className="text-sm font-semibold">{group.label}</h3>
                    <Badge variant="secondary" className="text-[10px] h-5">
                        {group.items.length}
                    </Badge>
                    {selectionMode && selectedInGroup > 0 && (
                        <Badge
                            variant="outline"
                            className="h-5 border-primary/30 bg-primary/10 text-[10px] text-primary"
                        >
                            {selectedInGroup} selected
                        </Badge>
                    )}
                    {activeCount > 0 && (
                        <Badge
                            variant="outline"
                            className="hidden h-5 border-emerald-500/20 bg-emerald-500/10 text-[10px] text-emerald-600 sm:inline-flex"
                        >
                            {activeCount} active
                        </Badge>
                    )}
                    {inactiveCount > 0 && (
                        <Badge
                            variant="outline"
                            className="hidden h-5 border-border bg-muted/40 text-[10px] text-muted-foreground sm:inline-flex"
                        >
                            {inactiveCount} inactive
                        </Badge>
                    )}
                    <ChevronDown
                        className={`h-4 w-4 text-muted-foreground transition-transform ${isCollapsed ? '-rotate-90' : ''}`}
                    />
                </button>

                {/* Check quota button — only for groups with active connections */}
                {hasActiveItems && !isFetching && (
                    <Button
                        variant="outline"
                        size="sm"
                        onClick={(e) => {
                            e.stopPropagation();
                            onFetchQuota();
                        }}
                        className={cn(
                            'text-xs h-7 gap-1 shrink-0',
                            hasFetched
                                ? 'text-primary'
                                : 'text-amber-600 border-amber-500/30 hover:bg-amber-500/10',
                        )}
                        title={hasFetched ? 'Refresh loaded quotas' : 'Load group quotas'}
                    >
                        {hasFetched ? <RefreshCw className="h-3 w-3" /> : <Zap className="h-3 w-3" />}
                        {hasFetched ? 'Refresh quotas' : 'Load quotas'}
                    </Button>
                )}

                {/* Loading spinner while fetching */}
                {isFetching && <RefreshCw className="h-4 w-4 animate-spin text-primary shrink-0" />}
            </div>
            {!isCollapsed && (
                <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 gap-3">
                    {group.items.map((c) => (
                        <ConnectionCard
                            key={c.id}
                            conn={c}
                            initialQuotaResult={quotaResult[c.id]}
                            onReload={onReload}
                            onDelete={onDelete}
                            onEditModels={onEditModels}
                            onEditConnection={onEditConnection}
                            onViewDetails={onViewDetails}
                            selectionMode={selectionMode}
                            selected={selectedIds?.has(c.id) ?? false}
                            onToggleSelect={onToggleSelect}
                        />
                    ))}
                </div>
            )}
        </div>
    );
}
