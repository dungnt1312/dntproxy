import { useCallback, useEffect, useLayoutEffect, useMemo, useRef, useState } from 'react';
import { ChevronDown, Plus, RefreshCw, Zap } from 'lucide-react';
import { cn } from '@/lib/utils';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { ProviderLogoIcon } from '@/components/connections/helpers';
import { ConnectionRow, ROW_HEIGHT } from './connection-row';
import { quotaStore, useNow, useQuotaVersion } from './quota-store';
import type { Connection } from '@/types/connections';

const HEADER_HEIGHT = 42;
/** Breathing room between provider groups — groups are dense content blocks. */
const GROUP_GAP = 18;
const OVERSCAN = 6;

export interface RailGroup {
  id: string;
  label: string;
  colorClass: string;
  items: Connection[]; // already sorted (attention-first)
  issueCount: number;
  /** Average worst-bucket usage across connections whose quota is known, else null. */
  avgUsagePct: number | null;
}

type FlatItem =
  | { kind: 'group'; top: number; height: number; group: RailGroup }
  | {
      kind: 'conn';
      top: number;
      height: number;
      conn: Connection;
      groupId: string;
      showProvider: boolean;
    };

interface ConnectionListProps {
  groups: RailGroup[];
  collapsed: Record<string, boolean>;
  layout: 'provider' | 'status';
  detailId: string | null;
  selectionMode: boolean;
  selectedIds: ReadonlySet<string>;
  legacyCounts: Record<string, number>;
  onToggleCollapse: (id: string) => void;
  onOpen: (id: string) => void;
  onToggleSelect: (id: string) => void;
  onToggleSelectGroup: (ids: string[], selectAll: boolean) => void;
  /** Explicit "Load quotas" click on a provider header — quota never loads by itself. */
  onLoadGroupQuota: (groupId: string) => void;
  onAddConnectionForProvider: (providerId: string) => void;
  /** Scroll this connection into view when the value changes. */
  scrollToId?: string | null;
}

export function ConnectionList({
  groups,
  collapsed,
  layout,
  detailId,
  selectionMode,
  selectedIds,
  legacyCounts,
  onToggleCollapse,
  onOpen,
  onToggleSelect,
  onToggleSelectGroup,
  onLoadGroupQuota,
  onAddConnectionForProvider,
  scrollToId,
}: ConnectionListProps) {
  const containerRef = useRef<HTMLDivElement>(null);
  const [scrollTop, setScrollTop] = useState(0);
  const [viewportH, setViewportH] = useState(600);
  const scrollRaf = useRef<number | null>(null);

  // Rerender rows whenever any quota entry lands / ages; keep a clock for freshness titles.
  useQuotaVersion();
  const nowMs = useNow();

  const flat = useMemo(() => {
    const items: FlatItem[] = [];
    let top = 0;
    const showProvider = layout === 'status';
    for (const group of groups) {
      if (items.length > 0) top += GROUP_GAP; // margin between provider blocks
      items.push({ kind: 'group', top, height: HEADER_HEIGHT, group });
      top += HEADER_HEIGHT;
      if (!collapsed[group.id]) {
        for (const conn of group.items) {
          items.push({ kind: 'conn', top, height: ROW_HEIGHT, conn, groupId: group.id, showProvider });
          top += ROW_HEIGHT;
        }
      }
    }
    return items;
  }, [groups, collapsed, layout]);

  const totalHeight = flat.length > 0 ? flat[flat.length - 1].top + flat[flat.length - 1].height : 0;

  const range = useMemo(() => {
    if (flat.length === 0) return [] as FlatItem[];
    let start = 0;
    while (
      start < flat.length &&
      flat[start].top + flat[start].height <= scrollTop
    ) {
      start++;
    }
    const end = Math.min(
      flat.length,
      start + Math.ceil((viewportH + OVERSCAN * ROW_HEIGHT) / ROW_HEIGHT) + OVERSCAN,
    );
    return flat.slice(start, end);
  }, [flat, scrollTop, viewportH]);

  // Clamp scroll position when filters/group changes shrink the virtual height.
  useEffect(() => {
    const el = containerRef.current;
    if (!el) return;
    const max = Math.max(0, totalHeight - el.clientHeight);
    if (el.scrollTop > max) el.scrollTop = max;
  }, [totalHeight]);

  // Measure + keep viewport height fresh.
  useLayoutEffect(() => {
    const el = containerRef.current;
    if (!el) return;
    const observer = new ResizeObserver(() => setViewportH(el.clientHeight));
    observer.observe(el);
    setViewportH(el.clientHeight);
    return () => observer.disconnect();
  }, []);

  const handleScroll = useCallback(() => {
    if (scrollRaf.current !== null) return;
    scrollRaf.current = requestAnimationFrame(() => {
      scrollRaf.current = null;
      setScrollTop(containerRef.current?.scrollTop ?? 0);
    });
  }, []);

  // Jump to a specific connection (palette navigation).
  useEffect(() => {
    if (!scrollToId) return;
    const index = flat.findIndex((item) => item.kind === 'conn' && item.conn.id === scrollToId);
    if (index < 0) return;
    const el = containerRef.current;
    if (!el) return;
    el.scrollTop = Math.max(0, flat[index].top - el.clientHeight / 2 + flat[index].height / 2);
  }, [scrollToId, flat]);

  return (
    <div ref={containerRef} onScroll={handleScroll} className="h-full overflow-y-auto overflow-x-hidden py-2">
      <div className="relative w-full" style={{ height: totalHeight }}>
        {range.map((item) =>
          item.kind === 'group' ? (
            <GroupHeader
              key={`g-${item.group.id}`}
              group={item.group}
              style={{ position: 'absolute', top: item.top, left: 0, right: 0, height: item.height }}
              collapsed={Boolean(collapsed[item.group.id])}
              selectionMode={selectionMode}
              selectedCount={item.group.items.filter((c) => selectedIds.has(c.id)).length}
              onSelectAll={(selectAll) =>
                onToggleSelectGroup(item.group.items.map((c) => c.id), selectAll)
              }
              onToggle={() => onToggleCollapse(item.group.id)}
              onLoadQuota={() => onLoadGroupQuota(item.group.id)}
              onAdd={() => onAddConnectionForProvider(item.group.id)}
            />
          ) : (
            <div
              key={item.conn.id}
              style={{ position: 'absolute', top: item.top, left: 0, right: 0, height: item.height }}
            >
              <ConnectionRow
                conn={item.conn}
                detail={detailId === item.conn.id}
                selectionMode={selectionMode}
                selected={selectedIds.has(item.conn.id)}
                showProvider={item.showProvider}
                legacyCount={legacyCounts[item.conn.id]}
                quotaEntry={quotaStore.peek(item.conn.id)}
                quotaFetching={quotaStore.isFetching(item.conn.id)}
                nowMs={nowMs}
                onOpen={onOpen}
                onToggleSelect={onToggleSelect}
              />
            </div>
          ),
        )}
      </div>
    </div>
  );
}

// ─── Group header ─────────────────────────────────────────────────────────────

function GroupHeader({
  group,
  style,
  collapsed,
  selectionMode,
  selectedCount,
  onSelectAll,
  onToggle,
  onLoadQuota,
  onAdd,
}: {
  group: RailGroup;
  style: React.CSSProperties;
  collapsed: boolean;
  selectionMode: boolean;
  selectedCount: number;
  onSelectAll: (selectAll: boolean) => void;
  onToggle: () => void;
  onLoadQuota: () => void;
  onAdd: () => void;
}) {
  const allSelected = selectionMode && group.items.length > 0 && selectedCount === group.items.length;

  // Quota only loads when the user asks; the button reflects that state.
  const activeIds = useMemo(
    () => group.items.filter((c) => c.isActive && c.supportsQuota !== false).map((c) => c.id),
    [group.items],
  );
  const fetching = activeIds.some((id) => quotaStore.isFetching(id));
  const anyLoaded = activeIds.some((id) => quotaStore.peek(id) !== undefined);

  return (
    <div style={style} className="flex items-center gap-2 rounded-lg border bg-muted/40 px-3">
      {selectionMode && (
        <button
          type="button"
          role="checkbox"
          aria-checked={allSelected}
          aria-label={`${allSelected ? 'Deselect all' : 'Select all'} ${group.label}`}
          onClick={() => onSelectAll(!allSelected)}
          onKeyDown={(e) => {
            if (e.key === ' ' || e.key === 'Enter') {
              e.preventDefault();
              onSelectAll(!allSelected);
            }
          }}
          tabIndex={0}
          className={cn(
            'flex h-4 w-4 shrink-0 cursor-pointer items-center justify-center rounded border transition-colors',
            allSelected ? 'border-primary bg-primary text-primary-foreground' : 'border-input bg-background hover:border-primary/50',
          )}
        />
      )}
      <button
        type="button"
        onClick={onToggle}
        aria-expanded={!collapsed}
        aria-label={`${collapsed ? 'Expand' : 'Collapse'} ${group.label} group`}
        className="flex min-w-0 flex-1 cursor-pointer items-center gap-2 text-left"
      >
        <ChevronDown size={14} className={cn('shrink-0 text-muted-foreground transition-transform', collapsed && '-rotate-90')} />
        <span className={cn('flex h-5 w-5 shrink-0 items-center justify-center overflow-hidden rounded-md border', group.colorClass)}>
          <ProviderLogoIcon provider={group.id} size={16} className="w-full object-cover" />
        </span>
        <span className="truncate text-sm font-semibold">{group.label}</span>
        <Badge variant="secondary" className="h-5 px-1.5 text-[10px] tabular-nums">
          {group.items.length} acc
        </Badge>
        {group.issueCount > 0 && (
          <Badge variant="outline" className="h-5 border-destructive/25 bg-destructive/10 px-1.5 text-[10px] text-destructive tabular-nums">
            ⚠{group.issueCount}
          </Badge>
        )}
        {group.avgUsagePct !== null && (
          <span className="hidden shrink-0 items-center gap-1.5 pl-2 sm:flex" title="Average usage across connections with known quota">
            <span className="h-1.5 w-16 overflow-hidden rounded-full bg-muted">
              <span
                className={cn(
                  'block h-full rounded-full',
                  group.avgUsagePct >= 90 ? 'bg-red-500' : group.avgUsagePct >= 70 ? 'bg-amber-500' : 'bg-emerald-500/70',
                )}
                style={{ width: `${Math.round(group.avgUsagePct)}%` }}
              />
            </span>
            <span className="font-mono text-[10px] tabular-nums text-muted-foreground">{Math.round(group.avgUsagePct)}%</span>
          </span>
        )}
      </button>

      {/* Explicit per-provider actions — quota never auto-loads */}
      {activeIds.length > 0 && !fetching && (
        <Button
          variant="outline"
          size="sm"
          className={cn('h-7 shrink-0 gap-1 text-xs', anyLoaded ? 'text-muted-foreground' : 'text-primary')}
          onClick={(e) => {
            e.stopPropagation();
            onLoadQuota();
          }}
          title="Load quotas for active connections in this group"
        >
          {anyLoaded ? <RefreshCw size={12} /> : <Zap size={12} />}
          {anyLoaded ? 'Refresh' : 'Load quotas'}
        </Button>
      )}
      {fetching && <RefreshCw size={13} className="shrink-0 animate-spin text-primary" />}
      <Button
        variant="ghost"
        size="icon"
        className="h-7 w-7 shrink-0"
        onClick={(e) => {
          e.stopPropagation();
          onAdd();
        }}
        title={`Add ${group.label} connection`}
        aria-label={`Add ${group.label} connection`}
      >
        <Plus size={15} />
      </Button>
    </div>
  );
}
