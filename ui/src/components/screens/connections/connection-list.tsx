import { useMemo } from 'react';
import { ChevronDown, Loader2, Plus, RefreshCw, Zap } from 'lucide-react';
import { cn } from '@/lib/utils';
import { Button } from '@/components/ui/button';
import { Checkbox } from '@/components/ui/checkbox';
import { ProviderLogoIcon } from '@/components/connections/helpers';
import { ConnectionRow } from './connection-row';
import { quotaStore, useNow, useQuotaVersion } from './quota-store';
import { QuotaBar, StatusBadge } from './status-ui';
import type { Connection } from '@/types/connections';

export interface RailGroup {
  id: string;
  label: string;
  items: Connection[]; // already sorted (attention-first)
  issueCount: number;
  /** Average effective quota across connections whose quota is known, else null. */
  avgUsagePct: number | null;
}

interface ConnectionListProps {
  groups: RailGroup[];
  collapsed: Record<string, boolean>;
  /** Flat (status-ranked) layout shows the provider tag on each row and no group header. */
  flat: boolean;
  detailId: string | null;
  selectionMode: boolean;
  selectedIds: ReadonlySet<string>;
  onToggleCollapse: (id: string) => void;
  onOpen: (id: string) => void;
  onToggleSelect: (id: string) => void;
  onToggleSelectGroup: (ids: string[], selectAll: boolean) => void;
  onLoadGroupQuota: (groupId: string) => void;
  onAddConnectionForProvider: (providerId: string) => void;
}

export function ConnectionList({
  groups,
  collapsed,
  flat,
  detailId,
  selectionMode,
  selectedIds,
  onToggleCollapse,
  onOpen,
  onToggleSelect,
  onToggleSelectGroup,
  onLoadGroupQuota,
  onAddConnectionForProvider,
}: ConnectionListProps) {
  // Rerender rows whenever any quota entry lands; keep a clock for freshness titles.
  useQuotaVersion();
  const nowMs = useNow();

  return (
    <div className="h-full space-y-3 overflow-y-auto p-2">
      {groups.map((group) => {
        const isCollapsed = !flat && Boolean(collapsed[group.id]);
        const listId = `conn-group-${group.id}`;
        return (
          <section key={group.id} aria-label={group.label}>
            {!flat && (
              <GroupHeader
                group={group}
                listId={listId}
                collapsed={isCollapsed}
                selectionMode={selectionMode}
                selectedCount={group.items.filter((c) => selectedIds.has(c.id)).length}
                onSelectAll={(selectAll) => onToggleSelectGroup(group.items.map((c) => c.id), selectAll)}
                onToggle={() => onToggleCollapse(group.id)}
                onLoadQuota={() => onLoadGroupQuota(group.id)}
                onAdd={() => onAddConnectionForProvider(group.id)}
              />
            )}
            {!isCollapsed && (
              <ul id={listId} className={cn('space-y-0.5', !flat && 'mt-1')}>
                {group.items.map((conn) => (
                  <li key={conn.id} className="h-14">
                    <ConnectionRow
                      conn={conn}
                      detail={detailId === conn.id}
                      selectionMode={selectionMode}
                      selected={selectedIds.has(conn.id)}
                      showProvider={flat}
                      quotaEntry={quotaStore.peek(conn.id)}
                      quotaFetching={quotaStore.isFetching(conn.id)}
                      nowMs={nowMs}
                      onOpen={onOpen}
                      onToggleSelect={onToggleSelect}
                    />
                  </li>
                ))}
              </ul>
            )}
          </section>
        );
      })}
    </div>
  );
}

// ─── Group header ─────────────────────────────────────────────────────────────

function GroupHeader({
  group,
  listId,
  collapsed,
  selectionMode,
  selectedCount,
  onSelectAll,
  onToggle,
  onLoadQuota,
  onAdd,
}: {
  group: RailGroup;
  listId: string;
  collapsed: boolean;
  selectionMode: boolean;
  selectedCount: number;
  onSelectAll: (selectAll: boolean) => void;
  onToggle: () => void;
  onLoadQuota: () => void;
  onAdd: () => void;
}) {
  const allSelected = group.items.length > 0 && selectedCount === group.items.length;
  const quotaIds = useMemo(
    () => group.items.filter((c) => c.isActive && c.supportsQuota !== false).map((c) => c.id),
    [group.items],
  );
  const fetching = quotaIds.some((id) => quotaStore.isFetching(id));
  const anyLoaded = quotaIds.some((id) => quotaStore.peek(id) !== undefined);

  return (
    <div className="flex h-11 items-center gap-2 rounded-lg border bg-muted/40 px-3">
      {selectionMode && (
        <Checkbox
          checked={allSelected ? true : selectedCount > 0 ? 'indeterminate' : false}
          onCheckedChange={() => onSelectAll(!allSelected)}
          aria-label={`${allSelected ? 'Deselect' : 'Select'} all ${group.label} connections`}
        />
      )}
      <button
        type="button"
        onClick={onToggle}
        aria-expanded={!collapsed}
        aria-controls={listId}
        className="flex h-full min-w-0 flex-1 cursor-pointer items-center gap-2 rounded text-left focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
      >
        <ChevronDown
          className={cn('size-4 shrink-0 text-muted-foreground transition-transform motion-reduce:transition-none', collapsed && '-rotate-90')}
        />
        <span className="flex size-6 shrink-0 items-center justify-center overflow-hidden rounded-md border bg-background">
          <ProviderLogoIcon provider={group.id} size={18} />
        </span>
        <span className="truncate text-sm font-semibold">{group.label}</span>
        <span className="text-xs tabular-nums text-muted-foreground">{group.items.length}</span>
        {group.issueCount > 0 && (
          <StatusBadge tone="danger" className="h-5 px-1.5" title={`${group.issueCount} need attention`}>
            {group.issueCount} issue{group.issueCount > 1 ? 's' : ''}
          </StatusBadge>
        )}
        {group.avgUsagePct !== null && (
          <span className="ml-auto hidden items-center gap-2 pr-1 sm:flex" title="Average quota used (hard-capped accounts count as 100%)">
            <QuotaBar pct={group.avgUsagePct} className="w-16" />
            <span className="font-mono text-xs tabular-nums text-muted-foreground">{group.avgUsagePct}%</span>
          </span>
        )}
      </button>

      {quotaIds.length > 0 && (
        <Button
          variant="ghost"
          size="sm"
          className="h-8 shrink-0 gap-1.5 text-xs"
          onClick={onLoadQuota}
          disabled={fetching}
          title="Load quota for active connections in this group"
        >
          {fetching ? <Loader2 className="size-3.5 animate-spin" /> : anyLoaded ? <RefreshCw className="size-3.5" /> : <Zap className="size-3.5" />}
          <span className="hidden md:inline">{anyLoaded ? 'Refresh quota' : 'Load quota'}</span>
        </Button>
      )}
      <Button
        variant="ghost"
        size="icon"
        className="size-8 shrink-0"
        onClick={onAdd}
        title={`Add ${group.label} connection`}
        aria-label={`Add ${group.label} connection`}
      >
        <Plus className="size-4" />
      </Button>
    </div>
  );
}
