import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { useSearchParams } from 'react-router-dom';
import { AlertTriangle, ArrowLeft, Link2, ListChecks, Loader2, MousePointerClick, Plus, RefreshCw, Search, Upload, X } from 'lucide-react';
import { toast } from 'sonner';
import { api } from '@/api';
import EditModelsModal from '../connections/EditModelsModal';
import EditConnectionModal from '../connections/EditConnectionModal';
import DeleteDialog from '../connections/DeleteDialog';
import { BulkActionBar, type BulkAction } from '../connections/BulkActionBar';
import { BulkModelsModal } from '../connections/BulkModelsModal';
import { stripModelForConnection } from '../connections/model-prefix';
import { BulkDeleteDialog } from '../connections/BulkDeleteDialog';
import { ModelMigrationModal } from '../connections/model-migration-modal';
import { AddConnectionModal } from '../connections/add-connection-modal';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Switch } from '@/components/ui/switch';
import { Skeleton } from '@/components/ui/skeleton';
import { cn } from '@/lib/utils';
import { getProviderLabel, getProviderMeta, PROVIDER_ORDER } from '@/lib/provider-registry';
import { compareByAttention, hasBackoff, isRateLimited, needsAttention } from '@/lib/connection-status';
import { ConnectionStatusFilter, type IssueQuickAction } from './connections/connection-status-filter';
import { ConnectionList, type RailGroup } from './connections/connection-list';
import { ConnectionInspector } from './connections/connection-inspector';
import { effectiveQuotaPct, quotaStore, useQuotaConnsSync, useQuotaVersion } from './connections/quota-store';
import { downloadConnectionsExport } from './connections/export';
import { errMessage } from './connections/status-tone';
import type { Connection } from '@/types/connections';

const LS_COLLAPSED = 'dntproxy.connections.collapsedGroups';
const LS_QUOTA_AUTO = 'dntproxy.connections.quotaAuto';

type StatusParam = 'all' | 'active' | 'inactive' | 'issues';
const STATUS_VALUES: StatusParam[] = ['all', 'active', 'inactive', 'issues'];

export default function ConnectionsScreen() {
  const [searchParams, setSearchParams] = useSearchParams();

  // ── URL-driven state ────────────────────────────────────────────────────────
  const q = searchParams.get('q') ?? '';
  const rawStatus = searchParams.get('status') as StatusParam | null;
  const status: StatusParam = rawStatus && STATUS_VALUES.includes(rawStatus) ? rawStatus : 'all';
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

  useQuotaConnsSync(conns);

  const load = useCallback(async () => {
    try {
      const data = await api.getConnections();
      setConns(Array.isArray(data) ? data : []);
      setLoadError('');
    } catch (e: unknown) {
      setLoadError(errMessage(e, 'Failed to load connections'));
    } finally {
      setIsLoading(false);
    }
  }, []);

  useEffect(() => {
    // Bootstrap fetch — state settles asynchronously after the await inside load().
    // eslint-disable-next-line react-hooks/set-state-in-effect
    void load();
    // Show the last known quotas right away from the server cache (no provider calls).
    void quotaStore.loadCached();
  }, [load]);

  // ── Counts & filtering ──────────────────────────────────────────────────────
  const counts = useMemo(
    () => ({
      all: conns.length,
      active: conns.filter((c) => c.isActive).length,
      inactive: conns.filter((c) => !c.isActive).length,
      issues: conns.filter(needsAttention).length,
    }),
    [conns],
  );

  const filteredConns = useMemo(() => {
    const needle = q.trim().toLowerCase();
    return conns.filter((c) => {
      if (status === 'active' && !c.isActive) return false;
      if (status === 'inactive' && c.isActive) return false;
      if (status === 'issues' && !needsAttention(c)) return false;
      if (!needle) return true;
      return [c.name, c.email, c.baseUrl, c.providerName, c.authMethod, getProviderLabel(c.provider), ...(c.supportedModels || [])]
        .filter(Boolean)
        .join(' ')
        .toLowerCase()
        .includes(needle);
    });
  }, [conns, q, status]);

  const hasActiveFilters = status !== 'all' || q.trim().length > 0;

  // ── Quota-auto (persisted; default OFF so provider APIs aren't spammed) ─────
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
  const openDetail = useCallback((id: string) => patchParams({ conn: id }), [patchParams]);
  const closeDetail = useCallback(() => patchParams({ conn: null }), [patchParams]);

  // ── Selection ───────────────────────────────────────────────────────────────
  const [selectionMode, setSelectionMode] = useState(false);
  const [selectedIds, setSelectedIds] = useState<Set<string>>(new Set());
  const [bulkBusy, setBulkBusy] = useState<string | null>(null);

  const toggleSelect = useCallback((id: string) => {
    setSelectedIds((prev) => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });
  }, []);
  const toggleSelectAll = useCallback((ids: string[], selectAll: boolean) => {
    setSelectedIds((prev) => {
      const next = new Set(prev);
      ids.forEach((id) => (selectAll ? next.add(id) : next.delete(id)));
      return next;
    });
  }, []);
  const exitSelectionMode = useCallback(() => {
    setSelectionMode(false);
    setSelectedIds(new Set());
  }, []);

  // Only visible + existing connections count as selected; filtering never hides an action target.
  const selectedConns = useMemo(() => filteredConns.filter((c) => selectedIds.has(c.id)), [filteredConns, selectedIds]);
  const allVisibleSelected = filteredConns.length > 0 && selectedConns.length === filteredConns.length;

  // ── Overlays ────────────────────────────────────────────────────────────────
  const [editModelsConn, setEditModelsConn] = useState<Connection | null>(null);
  const [editConn, setEditConn] = useState<Connection | null>(null);
  const [deleteTarget, setDeleteTarget] = useState<Connection | null>(null);
  const [bulkModelsOpen, setBulkModelsOpen] = useState(false);
  const [bulkMigrateOpen, setBulkMigrateOpen] = useState(false);
  const [bulkDeleteOpen, setBulkDeleteOpen] = useState(false);
  const [addOpen, setAddOpen] = useState(false);
  const [addProviderPreset, setAddProviderPreset] = useState<string | null>(null);
  const openAddModal = useCallback((providerId?: string) => {
    setAddProviderPreset(providerId ?? null);
    setAddOpen(true);
  }, []);

  /** Runs an API call per target sequentially and reports one summary toast. */
  const runOverTargets = useCallback(
    async (targets: Connection[], label: string, fn: (conn: Connection) => Promise<unknown>, opts?: { clearAfter?: boolean }) => {
      if (targets.length === 0) return;
      setBulkBusy(label);
      let done = 0;
      const errors: string[] = [];
      for (const conn of targets) {
        try {
          await fn(conn);
          done++;
        } catch (e) {
          errors.push(`${conn.name}: ${errMessage(e, 'failed')}`);
        }
      }
      setBulkBusy(null);
      if (errors.length > 0) {
        toast.error(`${label}: ${done}/${targets.length} succeeded`, {
          description: errors.slice(0, 3).join('\n') + (errors.length > 3 ? `\n+${errors.length - 3} more` : ''),
        });
      } else {
        toast.success(`${label}: ${done}/${targets.length} succeeded`);
      }
      if (opts?.clearAfter) setSelectedIds(new Set());
      await load();
    },
    [load],
  );

  // ── Issue quick actions ─────────────────────────────────────────────────────
  const quickTargets = useMemo(
    () => ({
      errors: filteredConns.filter((c) => Boolean(c.lastError)),
      cooldowns: filteredConns.filter((c) => isRateLimited(c) || hasBackoff(c)),
    }),
    [filteredConns],
  );
  const handleQuickAction = useCallback(
    (action: IssueQuickAction) => {
      if (action === 'clear-errors') void runOverTargets(quickTargets.errors, 'Clear errors', (c) => api.clearConnectionError(c.id));
      else void runOverTargets(quickTargets.cooldowns, 'Reset cooldowns', (c) => api.resetCooldown(c.id));
    },
    [quickTargets, runOverTargets],
  );

  // ── Bulk actions ────────────────────────────────────────────────────────────
  const handleBulkAction = useCallback(
    async (action: BulkAction) => {
      if (action === 'models') setBulkModelsOpen(true);
      else if (action === 'migrate') setBulkMigrateOpen(true);
      else if (action === 'delete') setBulkDeleteOpen(true);
      else if (action === 'quotas') {
        const ids = selectedConns.filter((c) => c.isActive && c.supportsQuota !== false).map((c) => c.id);
        if (ids.length === 0) toast.info('None of the selected connections report quota.');
        else quotaStore.refresh(ids);
      } else if (action === 'enable') {
        void runOverTargets(selectedConns, 'Enable', (c) => api.updateConnection(c.id, { isActive: true }), { clearAfter: true });
      } else if (action === 'disable') {
        void runOverTargets(selectedConns, 'Disable', (c) => api.updateConnection(c.id, { isActive: false }), { clearAfter: true });
      } else if (action === 'export') {
        setBulkBusy('Exporting');
        try {
          await downloadConnectionsExport(selectedConns.map((c) => c.id));
          toast.success(`Exported ${selectedConns.length} connection(s)`);
        } catch (e) {
          toast.error(errMessage(e, 'Export failed'));
        } finally {
          setBulkBusy(null);
        }
      }
    },
    [selectedConns, runOverTargets],
  );

  // ── Groups ──────────────────────────────────────────────────────────────────
  const quotaVersion = useQuotaVersion();
  const groups = useMemo<RailGroup[]>(() => {
    void quotaVersion; // group averages follow quota updates
    const sorted = [...filteredConns].sort(compareByAttention);
    if (layout === 'status') {
      return [{ id: '__flat__', label: 'All connections', items: sorted, issueCount: sorted.filter(needsAttention).length, avgUsagePct: avgKnownUsage(sorted) }];
    }
    const byProvider = new Map<string, Connection[]>();
    for (const c of sorted) {
      const key = c.provider || 'unknown';
      if (!byProvider.has(key)) byProvider.set(key, []);
      byProvider.get(key)!.push(c);
    }
    const order = [...PROVIDER_ORDER.filter((p) => byProvider.has(p)), ...[...byProvider.keys()].filter((k) => !(PROVIDER_ORDER as readonly string[]).includes(k))];
    return order.map((key) => {
      const items = byProvider.get(key)!;
      return { id: key, label: getProviderMeta(key).label, items, issueCount: items.filter(needsAttention).length, avgUsagePct: avgKnownUsage(items) };
    });
  }, [filteredConns, layout, quotaVersion]);

  const handleLoadGroupQuota = useCallback(
    (groupId: string) => {
      const group = groups.find((g) => g.id === groupId);
      if (group) quotaStore.refresh(group.items.filter((c) => c.isActive && c.supportsQuota !== false).map((c) => c.id));
    },
    [groups],
  );

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
        if (!data || typeof data !== 'object' || Array.isArray(data)) throw new Error('Choose a dntproxy connection export JSON file.');
        const payload = data as Record<string, unknown>;
        let result;
        if (Object.prototype.hasOwnProperty.call(payload, 'connection')) result = await api.importConnectionFile(payload);
        else if (Array.isArray(payload.providerConnections)) result = await api.importConnectionsFile(payload);
        else throw new Error('This JSON is not a dntproxy connection export.');

        const details = [
          result.imported > 0 && `${result.imported} imported`,
          result.updated > 0 && `${result.updated} updated`,
          result.skipped > 0 && `${result.skipped} skipped`,
        ]
          .filter(Boolean)
          .join(', ') || 'no changes';
        const errors = result.errors ?? [];
        if (errors.length > 0) toast.warning(`Import: ${details}`, { description: errors.slice(0, 3).join('\n') });
        else toast.success(`Import: ${details}`);
        await load();
      } catch (error) {
        toast.error(errMessage(error, 'Could not import.'));
      } finally {
        setIsImporting(false);
      }
    },
    [load],
  );

  // ─── Render ─────────────────────────────────────────────────────────────────
  const showInspectorOnMobile = Boolean(detailConn);

  return (
    <div className="space-y-4">
      {/* Header */}
      <header className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <div className="min-w-0">
          <h1 className="text-2xl font-bold tracking-tight">Connections</h1>
          <p className="text-sm text-muted-foreground">
            {counts.all} account{counts.all === 1 ? '' : 's'} · {counts.active} enabled
            {counts.issues > 0 && <span className="text-destructive"> · {counts.issues} need attention</span>}
          </p>
        </div>
        <div className="flex flex-wrap gap-2">
          <input ref={importInputRef} type="file" accept="application/json,.json" className="sr-only" tabIndex={-1} aria-hidden onChange={handleImportFile} />
          <Button variant="outline" onClick={() => importInputRef.current?.click()} disabled={isImporting} className="gap-2">
            {isImporting ? <Loader2 className="size-4 animate-spin" /> : <Upload className="size-4" />}
            {isImporting ? 'Importing…' : 'Import'}
          </Button>
          <Button onClick={() => openAddModal()} className="gap-2">
            <Plus className="size-4" /> Add connection
          </Button>
        </div>
      </header>

      {/* Toolbar */}
      <div className="flex flex-col gap-3 lg:flex-row lg:items-center">
        <div className="relative min-w-0 flex-1">
          <Search className="pointer-events-none absolute left-3 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" />
          <Input
            type="search"
            value={q}
            onChange={(e) => patchParams({ q: e.target.value })}
            placeholder="Search name, email, provider, model…"
            aria-label="Search connections"
            className="h-9 pl-9"
            autoComplete="off"
            data-1p-ignore
          />
        </div>
        <div className="flex flex-wrap items-center gap-2">
          <ConnectionStatusFilter
            value={status}
            counts={counts}
            onChange={(v) => patchParams({ status: v === 'all' ? null : v })}
            quickActions={{ clearErrors: quickTargets.errors.length, resetCooldowns: quickTargets.cooldowns.length, onRun: handleQuickAction }}
          />
          <div role="radiogroup" aria-label="Group by" className="inline-flex rounded-lg border bg-background p-0.5">
            {([['provider', 'By provider'], ['status', 'By status']] as const).map(([v, label]) => (
              <button
                key={v}
                type="button"
                role="radio"
                aria-checked={layout === v}
                onClick={() => patchParams({ layout: v === 'provider' ? null : v })}
                className={cn(
                  'h-8 cursor-pointer rounded-md px-3 text-sm transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring',
                  layout === v ? 'bg-secondary font-medium shadow-sm' : 'text-muted-foreground hover:text-foreground',
                )}
              >
                {label}
              </button>
            ))}
          </div>
          <label className="flex h-9 cursor-pointer items-center gap-2 rounded-lg border bg-background px-3 text-sm" title="Periodically refresh quota for the open connection and near-limit accounts">
            <Switch checked={quotaAuto} onCheckedChange={setQuotaAuto} />
            Auto quota
          </label>
          <Button
            variant={selectionMode ? 'secondary' : 'outline'}
            className="gap-1.5"
            aria-pressed={selectionMode}
            onClick={() => (selectionMode ? exitSelectionMode() : setSelectionMode(true))}
            disabled={conns.length === 0}
          >
            <ListChecks className="size-4" />
            {selectionMode ? 'Done' : 'Select'}
          </Button>
        </div>
      </div>

      {/* Body */}
      <div className="grid gap-4 lg:grid-cols-[minmax(0,1fr)_400px]">
        <div
          className={cn(
            'min-h-[420px] overflow-hidden rounded-lg border bg-card lg:h-[calc(100dvh-220px)]',
            showInspectorOnMobile && 'hidden lg:block',
          )}
        >
          {renderListBody()}
        </div>

        <aside
          aria-label="Connection details"
          className={cn(
            'overflow-hidden rounded-lg border bg-card lg:block lg:h-[calc(100dvh-220px)]',
            showInspectorOnMobile ? 'block' : 'hidden',
          )}
        >
          {detailConn ? (
            <div className="flex h-full flex-col">
              <div className="border-b p-2 lg:hidden">
                <Button variant="ghost" size="sm" className="gap-1.5" onClick={closeDetail}>
                  <ArrowLeft className="size-4" /> All connections
                </Button>
              </div>
              <div className="min-h-0 flex-1">
                <ConnectionInspector
                  key={detailConn.id}
                  conn={detailConn}
                  onReload={load}
                  onDelete={setDeleteTarget}
                  onEditModels={setEditModelsConn}
                  onEditConnection={setEditConn}
                />
              </div>
            </div>
          ) : (
            <div className="flex h-full flex-col items-center justify-center gap-2 p-8 text-center">
              <MousePointerClick className="size-6 text-muted-foreground" />
              <p className="text-sm font-medium">Select a connection</p>
              <p className="max-w-[260px] text-sm text-muted-foreground">Status, quota, models and actions appear here.</p>
            </div>
          )}
        </aside>
      </div>

      {selectionMode && conns.length > 0 && (
        <BulkActionBar
          selectedCount={selectedConns.length}
          visibleCount={filteredConns.length}
          allVisibleSelected={allVisibleSelected}
          busy={bulkBusy}
          onToggleSelectAll={() => toggleSelectAll(filteredConns.map((c) => c.id), !allVisibleSelected)}
          onExit={exitSelectionMode}
          onAction={(a) => void handleBulkAction(a)}
        />
      )}

      {/* Overlays */}
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
            void load();
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
              'Set models',
              (c) => api.updateConnection(c.id, { supportedModels: models.map((m) => stripModelForConnection(m, c)), setModels: true }),
              { clearAfter: true },
            );
          }}
          onClose={() => setBulkModelsOpen(false)}
        />
      )}

      {bulkMigrateOpen && (
        <ModelMigrationModal targets={[...selectedConns].sort(compareByAttention)} onApplied={load} onClose={() => setBulkMigrateOpen(false)} />
      )}

      {editConn && (
        <EditConnectionModal
          conn={editConn}
          onSuccess={(msg) => {
            toast.success(msg);
            setEditConn(null);
            void load();
          }}
          onClose={() => setEditConn(null)}
        />
      )}

      {deleteTarget && (
        <DeleteDialog
          target={{ id: deleteTarget.id, name: deleteTarget.name }}
          onConfirm={async (id) => {
            await api.deleteConnection(id);
            if (detailId === id) closeDetail();
            toast.success(`Deleted ${deleteTarget.name}`);
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
            const deleted = new Set<string>();
            await runOverTargets(
              selectedConns,
              'Delete',
              async (c) => {
                await api.deleteConnection(c.id);
                deleted.add(c.id);
              },
              { clearAfter: true },
            );
            // Close the detail pane only if its connection was actually removed.
            if (detailId && deleted.has(detailId)) closeDetail();
            setBulkDeleteOpen(false);
          }}
          onClose={() => setBulkDeleteOpen(false)}
        />
      )}
    </div>
  );

  function renderListBody() {
    if (isLoading) {
      return (
        <div className="space-y-2 p-3" aria-busy="true" aria-label="Loading connections">
          {Array.from({ length: 6 }).map((_, i) => (
            <Skeleton key={i} className="h-12 w-full" />
          ))}
        </div>
      );
    }
    if (loadError) {
      return (
        <Centered>
          <AlertTriangle className="size-6 text-destructive" />
          <h2 className="text-base font-semibold">Couldn't load connections</h2>
          <p role="alert" className="max-w-md text-sm text-muted-foreground">{loadError}</p>
          <Button onClick={() => { setIsLoading(true); void load(); }} variant="outline" size="sm" className="mt-2 gap-2">
            <RefreshCw className="size-4" /> Retry
          </Button>
        </Centered>
      );
    }
    if (conns.length === 0) {
      return (
        <Centered>
          <div className="mb-2 flex size-12 items-center justify-center rounded-xl bg-muted">
            <Link2 className="size-6 text-muted-foreground" />
          </div>
          <h2 className="text-lg font-semibold">No connections yet</h2>
          <p className="mb-2 max-w-sm text-sm text-muted-foreground">
            Add a Kiro, OpenAI, Anthropic, Gemini or any OpenAI-compatible account to start routing requests.
          </p>
          <div className="flex gap-2">
            <Button onClick={() => openAddModal()} className="gap-2">
              <Plus className="size-4" /> Add connection
            </Button>
            <Button variant="outline" onClick={() => importInputRef.current?.click()} className="gap-2">
              <Upload className="size-4" /> Import
            </Button>
          </div>
        </Centered>
      );
    }
    if (filteredConns.length === 0) {
      return (
        <Centered>
          <Search className="size-6 text-muted-foreground" />
          <h2 className="text-base font-semibold">No matching connections</h2>
          <p className="text-sm text-muted-foreground">Try a different search or status filter.</p>
          {hasActiveFilters && (
            <Button variant="outline" size="sm" className="mt-2 gap-1.5" onClick={() => patchParams({ q: null, status: null })}>
              <X className="size-4" /> Clear filters
            </Button>
          )}
        </Centered>
      );
    }
    return (
      <ConnectionList
        groups={groups}
        collapsed={collapsedGroups}
        flat={layout === 'status'}
        detailId={detailId}
        selectionMode={selectionMode}
        selectedIds={selectedIds}
        onToggleCollapse={toggleCollapse}
        onOpen={openDetail}
        onToggleSelect={toggleSelect}
        onToggleSelectGroup={toggleSelectAll}
        onLoadGroupQuota={handleLoadGroupQuota}
        onAddConnectionForProvider={(pid) => openAddModal(pid)}
      />
    );
  }
}

function Centered({ children }: { children: React.ReactNode }) {
  return <div className="flex h-full min-h-[320px] flex-col items-center justify-center gap-2 p-8 text-center">{children}</div>;
}

/** Average effective quota across connections with known quota (hard caps count as 100%). */
function avgKnownUsage(items: Connection[]): number | null {
  const pcts: number[] = [];
  for (const c of items) {
    const pct = effectiveQuotaPct(quotaStore.peek(c.id));
    if (pct !== null) pcts.push(pct);
  }
  if (pcts.length === 0) return null;
  return Math.round(pcts.reduce((a, b) => a + b, 0) / pcts.length);
}
