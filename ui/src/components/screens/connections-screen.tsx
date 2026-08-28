import { useEffect, useState, useMemo, useCallback, useRef } from 'react';
import { useSearchParams } from 'react-router-dom';
import { api } from '../../api';
import {
  AlertTriangle,
  Command as CommandIcon,
  Link2,
  ListChecks,
  Loader2,
  MousePointerClick,
  RefreshCw,
  Search,
  Upload,
} from 'lucide-react';
import EditModelsModal from '../connections/EditModelsModal';
import EditConnectionModal from '../connections/EditConnectionModal';
import DeleteDialog from '../connections/DeleteDialog';
import { BulkActionBar, type BulkAction } from '../connections/BulkActionBar';
import { BulkModelsModal, stripModelForConnection } from '../connections/BulkModelsModal';
import { BulkDeleteDialog } from '../connections/BulkDeleteDialog';
import { BulkCleanupDialog, type CleanupGroup } from '../connections/bulk-cleanup-dialog';
import { ModelMigrationModal } from '../connections/model-migration-modal';
import { AddConnectionModal } from '../connections/add-connection-modal';
import { ProviderLogoIcon } from '../connections/helpers';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Switch } from '@/components/ui/switch';
import { cn } from '@/lib/utils';
import { toast } from 'sonner';
import { getProviderLabel, getProviderMeta, PROVIDER_ORDER } from '@/lib/provider-registry';
import { compareByAttention, computeHealthScore, hasBackoff, isExpired, isRateLimited, needsAttention } from '@/lib/connection-status';
import { ConnectionHealthDashboard } from './connections/connection-health-dashboard';
import { ConnectionDetailSheet } from './connections/connection-detail-sheet';
import { ConnectionGroup } from './connections/connection-group';
import { ConnectionStatusFilter, type IssueQuickAction } from './connections/connection-status-filter';
import { ConnectionList, type RailGroup } from './connections/connection-list';
import { ConnectionPalette } from './connections/connection-palette';
import { ConnectionInspector } from './connections/connection-inspector';
import { quotaStore, useQuotaConnsSync, useQuotaVersion } from './connections/quota-store';
import { findLegacyModels, loadKnownModelIndex, type KnownModelIndex } from './connections/legacy-models';
import { useQuotaFetch } from './connections/use-quota-fetch';
import type { Connection, ConnectionGroup as LegacyConnectionGroup } from '@/types/connections';

// ─── Persisted prefs ──────────────────────────────────────────────────────────

const LS_COLLAPSED = 'dntproxy.connections.collapsedGroups';
const LS_QUOTA_AUTO = 'dntproxy.connections.quotaAuto';

type StatusParam = 'all' | 'active' | 'inactive' | 'issues' | 'legacy';

// ─── Component ────────────────────────────────────────────────────────────────

export default function ConnectionsScreen() {
  const [searchParams, setSearchParams] = useSearchParams();

  // ── URL-driven state ────────────────────────────────────────────────────────
  const q = searchParams.get('q') ?? '';
  const status = (searchParams.get('status') ?? 'all') as StatusParam;
  const view = searchParams.get('view') === 'cards' ? 'cards' : 'list';
  const layout = searchParams.get('layout') === 'status' ? 'status' : 'provider';
  const detailId = searchParams.get('conn');

  const patchParams = useCallback(
    (patch: Record<string, string | null>) => {
      setSearchParams(
        (prev) => {
          const next = new URLSearchParams(prev);
          for (const [k, v] of Object.entries(patch)) {
            if (v === null || v === '') next.delete(k);
            else next.set(k, v);
          }
          return next;
        },
        { replace: true },
      );
    },
    [setSearchParams],
  );

  // ── Core data ───────────────────────────────────────────────────────────────
  const [conns, setConns] = useState<Connection[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [loadError, setLoadError] = useState('');
  const [knownIndex, setKnownIndex] = useState<KnownModelIndex>({});

  useQuotaConnsSync(conns);

  const legacyByConn = useMemo(() => {
    const map: Record<string, string[]> = {};
    for (const c of conns) {
      const legacy = findLegacyModels(c, knownIndex);
      if (legacy.length > 0) map[c.id] = legacy;
    }
    return map;
  }, [conns, knownIndex]);

  const legacyCounts = useMemo(() => {
    const out: Record<string, number> = {};
    for (const [id, models] of Object.entries(legacyByConn)) out[id] = models.length;
    return out;
  }, [legacyByConn]);

  const load = useCallback(async () => {
    try {
      const data = await api.getConnections();
      setConns(data || []);
      setLoadError('');
    } catch (e: unknown) {
      const message = e instanceof Error ? e.message : 'Failed to load connections';
      setLoadError(message);
      toast.error(message);
    } finally {
      setIsLoading(false);
    }
  }, []);

  useEffect(() => {
    // Bootstrap fetch — states settle asynchronously after the awaits inside load().
    // eslint-disable-next-line react-hooks/set-state-in-effect
    void load();
    void loadKnownModelIndex().then(setKnownIndex);
  }, [load]);

  // Healthy quota snapshots clear stale runtime errors.
  useEffect(() => {
    quotaStore.onSuccess = (id, data) => {
      if (data.error || data.limitReached) return;
      setConns((prev) =>
        prev.map((c) =>
          c.id === id ? { ...c, lastError: undefined, rateLimitedUntil: undefined, backoffLevel: undefined } : c,
        ),
      );
    };
    return () => {
      quotaStore.onSuccess = undefined;
    };
  }, []);

  // ── Stats ───────────────────────────────────────────────────────────────────
  const stats = useMemo(
    () => ({
      total: conns.length,
      active: conns.filter((c) => c.isActive).length,
      inactive: conns.filter((c) => !c.isActive).length,
      issues: conns.filter(needsAttention).length,
      legacy: Object.keys(legacyCounts).length,
    }),
    [conns, legacyCounts],
  );

  // ── Filtering ───────────────────────────────────────────────────────────────
  const filteredConns = useMemo(() => {
    const needle = q.trim().toLowerCase();
    return conns.filter((c) => {
      if (status === 'active' && !c.isActive) return false;
      if (status === 'inactive' && c.isActive) return false;
      if (status === 'issues' && !needsAttention(c)) return false;
      if (status === 'legacy' && !legacyByConn[c.id]?.length) return false;
      if (!needle) return true;
      const hay = [
        c.name,
        c.email,
        c.baseUrl,
        c.providerName,
        c.authMethod,
        getProviderLabel(c.provider),
        ...(c.supportedModels || []),
      ]
        .filter(Boolean)
        .join(' ')
        .toLowerCase();
      return hay.includes(needle);
    });
  }, [conns, q, status, legacyByConn]);

  const hasActiveFilters = status !== 'all' || q.trim().length > 0;

  // ── Quota-auto (persisted; default OFF to honor the no-spam guarantee) ──────
  const [quotaAuto, setQuotaAutoState] = useState(() => localStorage.getItem(LS_QUOTA_AUTO) === '1');
  const setQuotaAuto = useCallback((on: boolean) => {
    setQuotaAutoState(on);
    localStorage.setItem(LS_QUOTA_AUTO, on ? '1' : '0');
  }, []);
  useEffect(() => {
    quotaStore.setAuto(quotaAuto);
    return () => quotaStore.dispose();
  }, [quotaAuto]);

  // ── Collapsed groups (persisted locally) ────────────────────────────────────
  const [collapsedGroups, setCollapsedGroups] = useState<Record<string, boolean>>(() => {
    try {
      return JSON.parse(localStorage.getItem(LS_COLLAPSED) ?? '{}');
    } catch {
      return {};
    }
  });
  useEffect(() => {
    localStorage.setItem(LS_COLLAPSED, JSON.stringify(collapsedGroups));
  }, [collapsedGroups]);

  const toggleCollapse = useCallback((id: string) => {
    setCollapsedGroups((prev) => ({ ...prev, [id]: !prev[id] }));
  }, []);

  // ── Detail selection (?conn=) ───────────────────────────────────────────────
  const detailConn = useMemo(() => conns.find((c) => c.id === detailId) ?? null, [conns, detailId]);
  const [scrollToId, setScrollToId] = useState<string | null>(null);

  const openDetail = useCallback(
    (id: string) => {
      patchParams({ conn: id });
      setScrollToId(id);
      const provider = conns.find((c) => c.id === id)?.provider ?? '';
      setCollapsedGroups((prev) => (prev[provider] ? { ...prev, [provider]: false } : prev));
    },
    [patchParams, conns],
  );

  // ── Bulk selection ──────────────────────────────────────────────────────────
  const [selectionMode, setSelectionMode] = useState(false);
  const [selectedIds, setSelectedIds] = useState<Set<string>>(new Set());
  const [bulkBusy, setBulkBusy] = useState<string | null>(null);
  // ── Modals / overlays ───────────────────────────────────────────────────────
  const [editModelsConn, setEditModelsConn] = useState<Connection | null>(null);
  const [editConn, setEditConn] = useState<Connection | null>(null);
  const [deleteTarget, setDeleteTarget] = useState<{ id: string; name: string } | null>(null);
  const [bulkModelsOpen, setBulkModelsOpen] = useState(false);
  const [bulkMigrateOpen, setBulkMigrateOpen] = useState(false);
  const [bulkDeleteOpen, setBulkDeleteOpen] = useState(false);
  const [cleanupOpen, setCleanupOpen] = useState(false);
  const [paletteOpen, setPaletteOpen] = useState(false);

  const toggleSelect = useCallback(
    (id: string) => {
      setSelectedIds((prev) => {
        const next = new Set(prev);
        if (next.has(id)) next.delete(id);
        else next.add(id);
        return next;
      });
    },
    [setSelectedIds],
  );

  const toggleSelectAll = useCallback(
    (ids: string[], selectAll: boolean) => {
      setSelectedIds((prev) => {
        const next = new Set(prev);
        ids.forEach((id) => (selectAll ? next.add(id) : next.delete(id)));
        return next;
      });
    },
    [setSelectedIds],
  );

  const exitSelectionMode = useCallback(() => {
    setSelectionMode(false);
    setSelectedIds(new Set());
  }, [setSelectionMode, setSelectedIds]);

  // Selections for connections deleted elsewhere are filtered out at render time.
  const selectedConns = useMemo(
    () => conns.filter((c) => selectedIds.has(c.id)),
    [conns, selectedIds],
  );
  const allVisibleSelected =
    filteredConns.length > 0 && filteredConns.every((c) => selectedIds.has(c.id));

  /** Runs an API call per target sequentially, reporting a summary toast. */
  const runOverTargets = useCallback(
    async (
      targets: Connection[],
      label: string,
      fn: (conn: Connection) => Promise<unknown>,
      opts?: { clearAfter?: boolean },
    ) => {
      if (targets.length === 0) return;
      setBulkBusy(label);
      let done = 0;
      const errors: string[] = [];
      for (const conn of targets) {
        try {
          await fn(conn);
          done++;
        } catch (e) {
          errors.push(`${conn.name}: ${e instanceof Error ? e.message : 'failed'}`);
        }
      }
      setBulkBusy(null);
      if (errors.length > 0) {
        toast.error(`${label}: ${done}/${targets.length}. First — ${errors[0]}${errors.length > 1 ? ` (+${errors.length - 1})` : ''}`);
      } else {
        toast.success(`${label}: ${done}/${targets.length}`);
      }
      if (opts?.clearAfter) setSelectedIds(new Set());
      await load();
    },
    [load],
  );

  // ── Issue quick actions (Issues chip menu) ──────────────────────────────────
  const quickActionTargets = useMemo(
    () => ({
      errors: filteredConns.filter((c) => Boolean(c.lastError)),
      cooldowns: filteredConns.filter((c) => isRateLimited(c) || hasBackoff(c)),
      expiring: filteredConns.filter(needsAttention),
    }),
    [filteredConns],
  );

  const handleQuickAction = useCallback(
    (action: IssueQuickAction) => {
      if (action === 'clear-errors') {
        void runOverTargets(quickActionTargets.errors, 'Clear errors', (c) => api.clearConnectionError(c.id));
      } else if (action === 'reset-cooldowns') {
        void runOverTargets(quickActionTargets.cooldowns, 'Reset cooldowns', (c) => api.resetCooldown(c.id));
      } else {
        setCleanupOpen(true);
      }
    },
    [quickActionTargets, runOverTargets],
  );

  const handleCleanupConfirm = useCallback(async () => {
    setCleanupOpen(false);
    const ids = buildCleanupGroups(filteredConns).flatMap((g) =>
      g.conns.map((c) => ({ id: c.id, name: c.name })),
    );
    if (ids.length === 0) return;
    setBulkBusy('Cleaning up');
    const res = await deleteConnectionsSequentially(ids);
    setBulkBusy(null);
    if (res.fails > 0) toast.error(`Deleted ${res.ok}, ${res.fails} failed`);
    else toast.success(`Deleted ${res.ok} connection(s)`);
    setSelectedIds(new Set());
    await load();
  }, [filteredConns, load]);

  // ── Bulk action router ──────────────────────────────────────────────────────
  const handleBulkAction = useCallback(
    (action: BulkAction) => {
      if (action === 'models') setBulkModelsOpen(true);
      else if (action === 'migrate') setBulkMigrateOpen(true);
      else if (action === 'delete') setBulkDeleteOpen(true);
      else if (action === 'quotas')
        quotaStore.refresh(conns.filter((c) => selectedIds.has(c.id)).map((c) => c.id));
      else if (action === 'enable')
        void runOverTargets(selectedConns, 'Enable', (c) => api.updateConnection(c.id, { isActive: true }), { clearAfter: true });
      else if (action === 'disable')
        void runOverTargets(selectedConns, 'Disable', (c) => api.updateConnection(c.id, { isActive: false }), { clearAfter: true });
    },
    [selectedConns, conns, selectedIds, runOverTargets],
  );

  // ── List building (attention-ranked; provider or status layout) ─────────────
  // Subscribing to quota versions keeps group averages honest as data lands.
  const quotaVersion = useQuotaVersion();
  const railGroups = useMemo<RailGroup[]>(() => {
    void quotaVersion;
    if (layout === 'status') {
      const flat = [...filteredConns].sort(compareByAttention);
      return [
        {
          id: '__flat__',
          label: `${flat.length} results · severity-ranked`,
          colorClass: '',
          items: flat,
          issueCount: flat.filter(needsAttention).length,
          avgUsagePct: avgKnownUsage(flat),
        },
      ];
    }

    const byProvider = new Map<string, Connection[]>();
    for (const c of [...filteredConns].sort(compareByAttention)) {
      const key = c.provider || 'unknown';
      if (!byProvider.has(key)) byProvider.set(key, []);
      byProvider.get(key)!.push(c);
    }
    const result: RailGroup[] = [];
    const push = (key: string) => {
      const items = byProvider.get(key)!;
      result.push({
        id: key,
        label: getProviderMeta(key).label,
        colorClass: getProviderMeta(key).colorClass,
        items,
        issueCount: items.filter(needsAttention).length,
        avgUsagePct: avgKnownUsage(items),
      });
      byProvider.delete(key);
    };
    PROVIDER_ORDER.forEach((p) => {
      if (byProvider.has(p)) push(p);
    });
    for (const key of [...byProvider.keys()]) push(key);
    return result;
  }, [filteredConns, layout, quotaVersion]);

  // ── Group-header actions (quota is load-on-request, per provider) ───────────
  const handleLoadGroupQuota = useCallback(
    (groupId: string) => {
      const group = railGroups.find((g) => g.id === groupId);
      if (!group) return;
      const ids = group.items
        .filter((c) => c.isActive && c.supportsQuota !== false)
        .map((c) => c.id);
      quotaStore.refresh(ids);
    },
    [railGroups],
  );

  // ── Add-connection modal ────────────────────────────────────────────────────
  const [addOpen, setAddOpen] = useState(false);
  const [addProviderPreset, setAddProviderPreset] = useState<string | null>(null);
  const openAddModal = useCallback((providerId?: string) => {
    setAddProviderPreset(providerId ?? null);
    setAddOpen(true);
  }, []);

  const healthScore = useMemo(() => computeHealthScore(conns), [conns]);
  const scoreTone =
    healthScore >= 85 ? 'text-emerald-600' : healthScore >= 65 ? 'text-amber-600' : 'text-destructive';

  // ── Import JSON ─────────────────────────────────────────────────────────────
  const [isImporting, setIsImporting] = useState(false);
  const importInputRef = useRef<HTMLInputElement>(null);
  const handleImportFile = useCallback(
    async (event: React.ChangeEvent<HTMLInputElement>) => {
      const file = event.target.files?.[0];
      event.target.value = '';
      if (!file) return;

      setIsImporting(true);
      try {
        let data: unknown;
        try {
          data = JSON.parse(await file.text());
        } catch {
          throw new Error('The selected file is not valid JSON.');
        }
        if (!data || typeof data !== 'object' || Array.isArray(data)) {
          throw new Error('Choose a dntproxy connection export JSON file.');
        }
        const payload = data as Record<string, unknown>;
        const result = Object.prototype.hasOwnProperty.call(payload, 'connection')
          ? await api.importConnectionFile(payload)
          : Array.isArray(payload.providerConnections)
            ? await api.importConnectionsFile(payload)
            : (() => {
                throw new Error('This JSON is not a dntproxy connection export.');
              })();

        const details = [
          result.imported > 0 && `${result.imported} imported`,
          result.updated > 0 && `${result.updated} updated`,
          result.skipped > 0 && `${result.skipped} skipped`,
        ]
          .filter(Boolean)
          .join(', ');
        const firstError = result.errors?.[0];
        if (firstError) toast.warning(`Import: ${details || 'no changes'}. ${firstError}`);
        else toast.success(`Import: ${details || 'no changes'}.`);
        await load();
      } catch (error) {
        toast.error(error instanceof Error ? error.message : 'Could not import.');
      } finally {
        setIsImporting(false);
      }
    },
    [load],
  );

  // ─── Render ─────────────────────────────────────────────────────────────────

  return (
    <div className="space-y-4">
      {/* Header */}
      <div className="flex flex-col gap-3 lg:flex-row lg:items-center lg:justify-between">
        <div className="flex min-w-0 items-center gap-3">
          <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg bg-primary/10">
            <Link2 className="h-5 w-5 text-primary" />
          </div>
          <div className="min-w-0">
            <h1 className="text-2xl font-bold tracking-tight">Connections</h1>
            <p className="truncate text-sm text-muted-foreground">Manage provider accounts and routing health.</p>
          </div>
          <span className="ml-2 shrink-0 rounded-full border bg-background px-2 py-1 text-xs tabular-nums text-muted-foreground">
            Health <span className={cn('font-semibold', scoreTone)}>{healthScore}%</span>
          </span>
        </div>

        <div className="flex flex-wrap gap-2 self-start lg:self-auto">
          <input ref={importInputRef} type="file" accept="application/json,.json" className="sr-only" onChange={handleImportFile} />
          <Button variant="outline" onClick={() => importInputRef.current?.click()} disabled={isImporting} className="gap-2">
            {isImporting ? <Loader2 className="h-4 w-4 animate-spin" /> : <Upload className="h-4 w-4" />}
            {isImporting ? 'Importing…' : 'Import JSON'}
          </Button>
          <Button onClick={() => openAddModal()} className="gap-2">
            Add connection
          </Button>
        </div>
      </div>

      {/* Toolbar (single row) */}
      <div className="flex flex-col gap-3 rounded-lg border bg-muted/30 p-3 xl:flex-row xl:items-center">
        <div className="relative min-w-0 flex-1">
          <Search size={14} className="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-muted-foreground" />
          <Input
            type="search"
            value={q}
            onChange={(e) => patchParams({ q: e.target.value })}
            placeholder="Search by name, email, provider, model…"
            className="h-8 pl-9 pr-14 text-sm"
            autoComplete="off"
            data-1p-ignore
          />
          <kbd className="pointer-events-none absolute right-3 top-1/2 flex -translate-y-1/2 items-center rounded border bg-muted px-1.5 py-px text-[10px] text-muted-foreground">
            <CommandIcon size={10} className="mr-0.5" />K
          </kbd>
        </div>

        <div className="flex flex-wrap items-center gap-2">
          <ConnectionStatusFilter
            value={status}
            total={stats.total}
            active={stats.active}
            inactive={stats.inactive}
            issues={stats.issues}
            legacy={stats.legacy}
            onChange={(v) => patchParams({ status: v === 'all' ? null : v })}
            issueQuickActions={{
              clearErrors: quickActionTargets.errors.length,
              resetCooldowns: quickActionTargets.cooldowns.length,
              cleanupCandidates: quickActionTargets.expiring.length,
              onRun: handleQuickAction,
            }}
          />

          <SegmentToggle
            options={[['list', 'List'], ['cards', 'Cards']]}
            value={view}
            onChange={(v) => patchParams({ view: v === 'list' ? null : v })}
          />
          {view === 'list' && (
            <SegmentToggle
              options={[['provider', 'Group'], ['status', 'Status']]}
              value={layout}
              onChange={(v) => patchParams({ layout: v === 'provider' ? null : v })}
            />
          )}

          <label className="flex cursor-pointer select-none items-center gap-2 rounded-lg border bg-background px-2 text-xs transition-colors hover:text-foreground">
            <Switch checked={quotaAuto} onCheckedChange={setQuotaAuto} className="scale-90" />
            Quota-auto
          </label>

          <Button
            variant={selectionMode ? 'secondary' : 'outline'}
            size="sm"
            className="h-8 shrink-0 gap-1.5 text-xs"
            aria-pressed={selectionMode}
            onClick={() => (selectionMode ? exitSelectionMode() : setSelectionMode(true))}
          >
            <ListChecks className="h-3.5 w-3.5" />
            {selectionMode ? 'Exit select' : 'Select'}
          </Button>

          {hasActiveFilters && (
            <Button variant="link" onClick={() => patchParams({ q: null, status: null })} className="h-8 px-1 text-xs">
              Clear filters
            </Button>
          )}
        </div>
      </div>

      {/* Body — inspector column is permanent so clicking a row never resizes the list */}
      {view === 'list' ? (
        <div className="grid gap-3 lg:grid-cols-[minmax(0,1fr)_380px]">
          {/* Rail */}
          <div className="max-h-[70vh] overflow-hidden rounded-lg border bg-card lg:max-h-none lg:h-[calc(100vh-236px)] lg:min-h-[460px]">
            {renderListBody()}
          </div>

          {/* Inspector */}
          <aside className="hidden max-h-[70vh] overflow-hidden rounded-lg border bg-card lg:block lg:max-h-none lg:h-[calc(100vh-236px)]">
            {detailConn ? (
              <ConnectionInspector
                conn={detailConn}
                legacyModels={legacyByConn[detailConn.id] ?? []}
                onReload={load}
                onDelete={(id, name) => setDeleteTarget({ id, name })}
                onEditModels={setEditModelsConn}
                onEditConnection={setEditConn}
              />
            ) : (
              <EmptyInspector onOpenPalette={() => setPaletteOpen(true)} />
            )}
          </aside>

          {/* Mobile/tablet: detail appears under the list once a connection is picked */}
          {detailConn && (
            <div className="max-h-[70vh] overflow-hidden rounded-lg border bg-card lg:hidden">
              <ConnectionInspector
                conn={detailConn}
                legacyModels={legacyByConn[detailConn.id] ?? []}
                onReload={load}
                onDelete={(id, name) => setDeleteTarget({ id, name })}
                onEditModels={setEditModelsConn}
                onEditConnection={setEditConn}
              />
            </div>
          )}
        </div>
      ) : (
        <CardsWorkspace
          conns={conns}
          isLoading={isLoading}
          loadError={loadError}
          collapsedGroups={collapsedGroups}
          onToggleCollapse={toggleCollapse}
          onReload={load}
          onOpenAdd={() => openAddModal()}
          selectionMode={selectionMode}
          selectedIds={selectedIds}
          onToggleSelect={toggleSelect}
          onToggleSelectAll={toggleSelectAll}
          onViewDetails={(c) => patchParams({ conn: c.id })}
          onEditModels={setEditModelsConn}
          onEditConnection={setEditConn}
          onDeleteTarget={setDeleteTarget}
          retryLoad={load}
          quotaAuto={quotaAuto}
          onConnsUpdate={setConns}
        />
      )}

      {/* Bulk bar */}
      {selectionMode && conns.length > 0 && (
        <BulkActionBar
          selectedCount={selectedIds.size}
          visibleCount={filteredConns.length}
          allVisibleSelected={allVisibleSelected}
          busy={bulkBusy}
          onToggleSelectAll={() => toggleSelectAll(filteredConns.map((c) => c.id), !allVisibleSelected)}
          onClearSelection={() => setSelectedIds(new Set())}
          onExit={exitSelectionMode}
          onAction={handleBulkAction}
        />
      )}

      {/* Overlays */}
      <ConnectionPalette open={paletteOpen} onOpenChange={setPaletteOpen} connections={conns} onPick={openDetail} />

      <AddConnectionModal
        open={addOpen}
        onOpenChange={(next) => {
          setAddOpen(next);
          if (!next) void load();
        }}
        initialProvider={addProviderPreset}
        onCreated={() => void load()}
      />

      {editModelsConn && (
        <EditModelsModal
          conn={editModelsConn}
          connections={conns}
          onSave={() => {
            setEditModelsConn(null);
            load();
          }}
          onBulkApplied={load}
          onClose={() => setEditModelsConn(null)}
        />
      )}

      {bulkModelsOpen && (
        <BulkModelsModal
          connections={selectedConns}
          busy={bulkBusy !== null}
          onApply={async (models) => {
            setBulkModelsOpen(false);
            await runOverTargets(
              selectedConns,
              'Copy models',
              (c) =>
                api.updateConnection(c.id, {
                  supportedModels: models.map((m) => stripModelForConnection(m, c)),
                  setModels: true,
                }),
              { clearAfter: true },
            );
          }}
          onClose={() => setBulkModelsOpen(false)}
        />
      )}

      {bulkMigrateOpen && (
        <ModelMigrationModal
          targets={[...selectedConns].sort(compareByAttention)}
          onApplied={load}
          onClose={() => setBulkMigrateOpen(false)}
        />
      )}

      {cleanupOpen && (
        <BulkCleanupDialog
          groups={buildCleanupGroups(filteredConns)}
          onConfirm={handleCleanupConfirm}
          onClose={() => setCleanupOpen(false)}
        />
      )}

      {editConn && (
        <EditConnectionModal
          conn={editConn}
          onSuccess={(msg) => {
            toast.success(msg);
            setEditConn(null);
            load();
          }}
          onClose={() => setEditConn(null)}
        />
      )}
      {deleteTarget && (
        <DeleteDialog
          target={deleteTarget}
          onConfirm={async (id) => {
            await api.deleteConnection(id);
            setDeleteTarget(null);
            await load();
          }}
          onClose={() => setDeleteTarget(null)}
        />
      )}
      {bulkDeleteOpen && (
        <BulkDeleteDialog
          names={selectedConns.map((c) => c.name)}
          busy={bulkBusy !== null}
          onConfirm={async () => {
            await runOverTargets(selectedConns, 'Delete', (c) => api.deleteConnection(c.id), { clearAfter: true });
            setBulkDeleteOpen(false);
          }}
          onClose={() => setBulkDeleteOpen(false)}
        />
      )}
    </div>
  );

  // ── list body inline ─────────────────────────────────────────────────────────

  function renderListBody() {
    if (isLoading) {
      return (
        <Centered>
          <Loader2 className="h-6 w-6 animate-spin text-primary" />
          <p className="text-sm text-muted-foreground">Loading connections…</p>
        </Centered>
      );
    }
    if (loadError) {
      return (
        <Centered>
          <AlertTriangle className="h-6 w-6 text-destructive" />
          <h3 className="text-base font-semibold">Failed to load connections</h3>
          <p className="max-w-md text-sm text-muted-foreground">{loadError}</p>
          <Button onClick={load} variant="outline" size="sm" className="mt-2 gap-2">
            <RefreshCw size={14} /> Retry
          </Button>
        </Centered>
      );
    }
    if (conns.length === 0) {
      return (
        <div className="flex h-full flex-col items-center justify-center rounded-lg border border-dashed border-transparent py-16 text-center">
          <div className="mb-4 flex h-14 w-14 items-center justify-center rounded-xl bg-primary/10">
            <Link2 className="h-6 w-6 text-primary" />
          </div>
          <h3 className="mb-1 text-lg font-semibold">No connections yet</h3>
          <p className="mb-4 max-w-sm text-sm text-muted-foreground">
            Add Kiro, OpenAI, Anthropic, Gemini, or any compatible endpoint to start routing.
          </p>
          <Button onClick={() => openAddModal()} variant="outline" className="gap-2">
            Connect now
          </Button>
        </div>
      );
    }
    return (
      <ConnectionList
        groups={railGroups}
        collapsed={collapsedGroups}
        layout={layout}
        detailId={detailId}
        selectionMode={selectionMode}
        selectedIds={selectedIds}
        legacyCounts={legacyCounts}
        onToggleCollapse={toggleCollapse}
        onOpen={openDetail}
        onToggleSelect={toggleSelect}
        onToggleSelectGroup={toggleSelectAll}
        onLoadGroupQuota={handleLoadGroupQuota}
        onAddConnectionForProvider={(pid) => openAddModal(pid)}
        scrollToId={scrollToId}
      />
    );
  }
}

// ─── Small shared pieces ──────────────────────────────────────────────────────

function Centered({ children }: { children: React.ReactNode }) {
  return (
    <div className="flex h-full min-h-[300px] flex-col items-center justify-center gap-2 p-10 text-center">{children}</div>
  );
}

/** Permanent inspector placeholder — keeps the two-column layout stable. */
function EmptyInspector({ onOpenPalette }: { onOpenPalette: () => void }) {
  return (
    <div className="flex h-full flex-col items-center justify-center gap-3 p-8 text-center">
      <div className="flex h-12 w-12 items-center justify-center rounded-xl bg-muted">
        <MousePointerClick className="h-5 w-5 text-muted-foreground" />
      </div>
      <p className="text-sm font-medium">Select a connection</p>
      <p className="max-w-[240px] text-xs text-muted-foreground">
        Status details, quota, models, and quick actions will appear here.
      </p>
      <Button variant="outline" size="sm" className="gap-1.5 text-xs" onClick={onOpenPalette}>
        <CommandIcon size={11} />K — quick jump
      </Button>
    </div>
  );
}

function SegmentToggle<T extends string>({
  options,
  value,
  onChange,
}: {
  options: Array<[T, string]>;
  value: T;
  onChange: (v: T) => void;
}) {
  return (
    <div className="flex items-center gap-1 rounded-lg border bg-background p-1">
      {options.map(([v, label]) => (
        <Button
          key={v}
          type="button"
          variant={value === v ? 'secondary' : 'ghost'}
          size="sm"
          className="h-7 px-2 text-xs"
          aria-pressed={value === v}
          onClick={() => onChange(v)}
        >
          {label}
        </Button>
      ))}
    </div>
  );
}

function avgKnownUsage(items: Connection[]): number | null {
  const pcts: number[] = [];
  for (const c of items) {
    for (const b of quotaStore.peek(c.id)?.data?.quotas ?? []) {
      if (!b.unlimited) pcts.push(b.pct);
    }
  }
  if (pcts.length === 0) return null;
  return Math.round(pcts.reduce((a, b) => a + b, 0) / pcts.length);
}

/** Reason buckets over an issue-filtered pool, used by the cleanup dialog. */
function buildCleanupGroups(pool: Connection[]): CleanupGroup[] {
  const expired: Connection[] = [];
  const rl: Connection[] = [];
  const errored: Connection[] = [];
  for (const c of pool) {
    if (!needsAttention(c)) continue;
    if (isExpired(c)) expired.push(c);
    else if (isRateLimited(c)) rl.push(c);
    else errored.push(c);
  }
  return [
    { key: 'expired', label: 'Expired tokens', conns: expired },
    { key: 'rate-limited', label: 'Rate-limited', conns: rl },
    { key: 'error', label: 'Errors / backoff', conns: errored },
  ];
}

/** Sequential delete with a summary report; first error surfaces in its own toast. */
async function deleteConnectionsSequentially(
  ids: Array<{ id: string; name: string }>,
): Promise<{ ok: number; fails: number }> {
  let ok = 0;
  let fails = 0;
  for (const t of ids) {
    try {
      await api.deleteConnection(t.id);
      ok++;
    } catch (e) {
      fails++;
      if (fails === 1) toast.error(`${t.name}: ${e instanceof Error ? e.message : 'delete failed'}`);
    }
  }
  return { ok, fails };
}

// ─── Legacy cards workspace (provider grid view) ─────────────────────────────

interface CardsWorkspaceProps {
  conns: Connection[];
  isLoading: boolean;
  loadError: string;
  collapsedGroups: Record<string, boolean>;
  onToggleCollapse: (id: string) => void;
  onReload: () => void;
  onOpenAdd: () => void;
  selectionMode: boolean;
  selectedIds: ReadonlySet<string>;
  onToggleSelect: (id: string) => void;
  onToggleSelectAll: (ids: string[], selectAll: boolean) => void;
  onViewDetails: (conn: Connection) => void;
  onEditModels: (conn: Connection) => void;
  onEditConnection: (conn: Connection) => void;
  onDeleteTarget: (t: { id: string; name: string }) => void;
  retryLoad: () => void;
  quotaAuto: boolean;
  onConnsUpdate: React.Dispatch<React.SetStateAction<Connection[]>>;
}

function CardsWorkspace({
  conns,
  isLoading,
  loadError,
  collapsedGroups,
  onToggleCollapse,
  onReload,
  onOpenAdd,
  selectionMode,
  selectedIds,
  onToggleSelect,
  onToggleSelectAll,
  onViewDetails,
  onEditModels,
  onEditConnection,
  onDeleteTarget,
  retryLoad,
  quotaAuto,
  onConnsUpdate,
}: CardsWorkspaceProps) {
  // Sheet-style detail when opened from a card's kebab menu.
  const [sheetId, setSheetId] = useState<string | null>(null);
  const sheetConn = useMemo(() => conns.find((c) => c.id === sheetId) ?? null, [conns, sheetId]);

  const groups = useMemo<LegacyConnectionGroup[]>(() => {
    const map = new Map<string, Connection[]>();
    for (const c of [...conns].sort(compareByAttention)) {
      const k = c.provider || 'unknown';
      if (!map.has(k)) map.set(k, []);
      map.get(k)!.push(c);
    }
    const result: LegacyConnectionGroup[] = [];
    const push = (key: string) => {
      const meta = getProviderMeta(key);
      result.push({
        id: key,
        label: meta.label,
        items: map.get(key)!,
        icon: <ProviderLogoIcon provider={key} size={28} className="w-full h-full object-cover" />,
        colorClass: meta.colorClass,
      });
      map.delete(key);
    };
    PROVIDER_ORDER.forEach((p) => {
      if (map.has(p)) push(p);
    });
    for (const key of [...map.keys()]) push(key);
    return result;
  }, [conns]);

  // Legacy per-group quota flow, scoped to the cards view only.
  const { quotaResult, fetchedGroups, fetchingGroups, handleFetchGroupQuota } = useQuotaFetch({
    groupedConns: groups,
    autoRefreshQuota: quotaAuto,
    onConnectionsUpdate: onConnsUpdate,
  });

  if (isLoading) {
    return (
      <Centered>
        <Loader2 className="h-6 w-6 animate-spin text-primary" />
        <p className="text-sm text-muted-foreground">Loading connections…</p>
      </Centered>
    );
  }
  if (loadError) {
    return (
      <Centered>
        <AlertTriangle className="h-6 w-6 text-destructive" />
        <h3 className="text-base font-semibold">Failed to load connections</h3>
        <p className="max-w-md text-sm text-muted-foreground">{loadError}</p>
        <Button onClick={retryLoad} variant="outline" size="sm" className="mt-2 gap-2">
          <RefreshCw size={14} /> Retry
        </Button>
      </Centered>
    );
  }

  return (
    <>
      <ConnectionHealthDashboard connections={conns} />
      <div className="space-y-6">
        {groups.map((group) => (
          <ConnectionGroup
            key={group.id}
            group={group}
            isCollapsed={Boolean(collapsedGroups[group.id])}
            isFetching={Boolean(fetchingGroups[group.id])}
            hasFetched={Boolean(fetchedGroups[group.id])}
            quotaResult={quotaResult}
            onToggle={() => onToggleCollapse(group.id)}
            onFetchQuota={() => handleFetchGroupQuota(group.id)}
            onAddConnection={() => onOpenAdd()}
            onReload={onReload}
            onDelete={(id, name) => onDeleteTarget({ id, name })}
            onEditModels={onEditModels}
            onEditConnection={onEditConnection}
            onViewDetails={(c) => {
              setSheetId(c.id);
              onViewDetails(c);
            }}
            selectionMode={selectionMode}
            selectedIds={selectedIds}
            onToggleSelect={onToggleSelect}
            onToggleSelectAll={onToggleSelectAll}
          />
        ))}
      </div>

      <ConnectionDetailSheet
        connection={sheetConn}
        open={!!sheetConn}
        onOpenChange={(open) => {
          if (!open) setSheetId(null);
        }}
        onEditModels={(c) => {
          setSheetId(null);
          onEditModels(c);
        }}
        onEditConnection={(c) => {
          setSheetId(null);
          onEditConnection(c);
        }}
      />
    </>
  );
}
