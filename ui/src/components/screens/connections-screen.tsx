import { useEffect, useState, useMemo, useCallback } from 'react';
import { useNavigate } from 'react-router-dom';
import { api } from '../../api';
import { Plus, Search, Link2, AlertTriangle, RefreshCw, Loader2 } from 'lucide-react';
import EditModelsModal from '../connections/EditModelsModal';
import EditConnectionModal from '../connections/EditConnectionModal';
import DeleteDialog from '../connections/DeleteDialog';
import { ProviderLogoIcon } from '../connections/helpers';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Switch } from '@/components/ui/switch';
import { toast } from 'sonner';
import { getProviderLabel, getProviderMeta, PROVIDER_ORDER } from '@/lib/provider-registry';
import { ConnectionHealthDashboard } from './connections/connection-health-dashboard';
import { ConnectionDetailSheet } from './connections/connection-detail-sheet';
import { ConnectionGroup } from './connections/connection-group';
import {
    ConnectionStatusFilter,
    type ConnectionStatusFilter as ConnectionStatusFilterValue,
} from './connections/connection-status-filter';
import { useQuotaFetch } from './connections/use-quota-fetch';
import type { Connection, ConnectionGroup as ConnectionGroupType } from '@/types/connections';

function connectionNeedsAttention(c: Connection) {
    if (!c.isActive) return false;
    const rateLimited = c.rateLimitedUntil && new Date(c.rateLimitedUntil) > new Date();
    const expired = c.expiresAt && new Date(c.expiresAt) < new Date();

    return Boolean(rateLimited || expired || (c.backoffLevel ?? 0) > 0 || c.lastError);
}

// ─── Component ────────────────────────────────────────────────────────────────

export default function ConnectionsScreen() {
    const navigate = useNavigate();
    const [conns, setConns] = useState<Connection[]>([]);
    const [isLoading, setIsLoading] = useState(true);
    const [loadError, setLoadError] = useState('');
    const [editModelsConn, setEditModelsConn] = useState<Connection | null>(null);
    const [editConn, setEditConn] = useState<Connection | null>(null);
    const [detailConn, setDetailConn] = useState<Connection | null>(null);
    const [deleteTarget, setDeleteTarget] = useState<{
        id: string;
        name: string;
    } | null>(null);
    const [searchQuery, setSearchQuery] = useState('');
    const [statusFilter, setStatusFilter] = useState<ConnectionStatusFilterValue>('active');
    const [autoRefreshQuota, setAutoRefreshQuota] = useState(false);
    const [collapsedGroups, setCollapsedGroups] = useState<Record<string, boolean>>({});

    const toggleGroup = useCallback((id: string) => {
        setCollapsedGroups((prev) => ({ ...prev, [id]: !prev[id] }));
    }, []);

    // ── Stats ─────────────────────────────────────────────────────────────────
    const connectionStats = useMemo(() => {
        const total = conns.length;
        const active = conns.filter((c) => c.isActive).length;
        const inactive = total - active;
        const needsAttention = conns.filter(connectionNeedsAttention).length;
        return { total, active, inactive, needsAttention };
    }, [conns]);

    // ── Filter ────────────────────────────────────────────────────────────────
    const filteredConns = useMemo(() => {
        const q = searchQuery.trim().toLowerCase();
        return conns.filter((c) => {
            if (statusFilter === 'active' && !c.isActive) return false;
            if (statusFilter === 'inactive' && c.isActive) return false;
            if (statusFilter === 'issues' && !connectionNeedsAttention(c)) return false;
            if (!q) return true;

            const providerLabel = getProviderLabel(c.provider).toLowerCase();
            const hay = [
                c.name,
                c.email,
                c.baseUrl,
                c.providerName,
                c.authMethod,
                providerLabel,
                ...(c.supportedModels || []),
            ]
                .filter(Boolean)
                .join(' ')
                .toLowerCase();
            return hay.includes(q);
        });
    }, [conns, searchQuery, statusFilter]);

    const hasActiveFilters = statusFilter !== 'all' || searchQuery.trim().length > 0;

    const clearFilters = useCallback(() => {
        setSearchQuery('');
        setStatusFilter('all');
    }, []);

    // ── Group connections by provider (dynamic, no "other" bucket) ──────────────
    const groupedConns = useMemo(() => {
        const list = [...filteredConns].sort((a, b) =>
            a.name.localeCompare(b.name, undefined, { sensitivity: 'base' }),
        );
        const groups: Record<string, Connection[]> = {};
        list.forEach((c) => {
            const p = c.provider || 'unknown';
            if (!groups[p]) groups[p] = [];
            groups[p].push(c);
        });

        const result: ConnectionGroupType[] = [];

        // Known providers in priority order
        PROVIDER_ORDER.forEach((p) => {
            if (groups[p]) {
                const meta = getProviderMeta(p);
                result.push({
                    id: p,
                    label: meta.label,
                    items: groups[p],
                    icon: <ProviderLogoIcon provider={p} size={28} className="w-full h-full object-cover" />,
                    colorClass: meta.colorClass,
                });
            }
        });

        // Unknown providers → own group
        Object.keys(groups).forEach((k) => {
            if (!PROVIDER_ORDER.includes(k as (typeof PROVIDER_ORDER)[number])) {
                const meta = getProviderMeta(k);
                result.push({
                    id: k,
                    label: getProviderLabel(k),
                    items: groups[k],
                    icon: <ProviderLogoIcon provider={k} size={28} className="w-full h-full object-cover" />,
                    colorClass: meta.colorClass,
                });
            }
        });

        return result;
    }, [filteredConns]);

    // ── Quota fetch hook ───────────────────────────────────────────────────────
    const { quotaResult, fetchedGroups, fetchingGroups, handleFetchGroupQuota } = useQuotaFetch({
        groupedConns,
        autoRefreshQuota,
        onConnectionsUpdate: setConns,
    });

    // ── Load connections only (no quota fetch) ─────────────────────────────────
    const load = useCallback(async () => {
        setLoadError('');
        try {
            const data = await api.getConnections();
            setConns(data || []);
        } catch (e: any) {
            const message = e.message || 'Failed to load connections';
            setLoadError(message);
            toast.error(message);
        } finally {
            setIsLoading(false);
        }
    }, []);

    useEffect(() => {
        load();
    }, [load]);

    // ── Handlers ───────────────────────────────────────────────────────────────
    const handleDeleteConfirm = useCallback(async (id: string) => {
        await api.deleteConnection(id);
        await load();
    }, [load]);

    const handleModelsEditSave = useCallback(() => {
        setEditModelsConn(null);
        load();
    }, [load]);

    return (
        <div className="space-y-6">
            {/* Header */}
            <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
                <div className="flex items-center gap-3">
                    <div className="flex h-9 w-9 items-center justify-center rounded-lg bg-primary/10">
                        <Link2 className="h-5 w-5 text-primary" />
                    </div>
                    <div>
                        <h1 className="text-2xl font-bold tracking-tight">Connections</h1>
                        <p className="text-sm text-muted-foreground">
                            Manage your AI provider accounts. Connect Kiro, OpenAI or any compatible endpoints.
                        </p>
                    </div>
                </div>
                <Button onClick={() => navigate('/connections/add')} className="gap-2 self-start sm:self-auto">
                    <Plus className="h-4 w-4" /> Add Connection
                </Button>
            </div>

            {/* Health Dashboard */}
            {conns.length > 0 && <ConnectionHealthDashboard connections={conns} />}

            {/* Toolbar */}
            {conns.length > 0 && (
                <div className="flex flex-col gap-3 rounded-lg border bg-muted/30 p-3">
                    <div className="flex flex-col gap-3 lg:flex-row lg:items-center">
                        <div className="relative flex-1 min-w-0">
                            <Search
                                size={14}
                                className="absolute left-3 top-1/2 -translate-y-1/2 text-muted-foreground pointer-events-none"
                            />
                            <Input
                                type="search"
                                value={searchQuery}
                                onChange={(e) => setSearchQuery(e.target.value)}
                                placeholder="Search by name, email, provider, or model..."
                                className="h-8 pl-9 text-sm"
                                autoComplete="off"
                                data-1p-ignore
                            />
                        </div>
                        <ConnectionStatusFilter
                            value={statusFilter}
                            total={connectionStats.total}
                            active={connectionStats.active}
                            inactive={connectionStats.inactive}
                            issues={connectionStats.needsAttention}
                            onChange={setStatusFilter}
                        />
                    </div>
                    <div className="flex flex-col gap-2 text-xs text-muted-foreground sm:flex-row sm:items-center sm:justify-between">
                        <span>
                            Showing {filteredConns.length} of {conns.length} connections
                            {statusFilter !== 'all' ? ` (${statusFilter})` : ''}
                        </span>
                        <div className="flex flex-wrap items-center gap-3">
                            {hasActiveFilters && (
                                <Button variant="link" onClick={clearFilters} className="h-auto p-0 text-xs">
                                    Clear filters
                                </Button>
                            )}
                            <label className="flex cursor-pointer items-center gap-2 hover:text-foreground transition-colors select-none">
                                <Switch
                                    checked={autoRefreshQuota}
                                    onCheckedChange={setAutoRefreshQuota}
                                />
                                Auto-refresh loaded quotas
                            </label>
                        </div>
                    </div>
                </div>
            )}

            {/* List */}
            {isLoading ? (
                <div className="flex flex-col items-center justify-center rounded-lg border py-16 text-center">
                    <Loader2 className="h-6 w-6 animate-spin text-primary mb-3" />
                    <p className="text-sm text-muted-foreground">Loading connections…</p>
                </div>
            ) : loadError ? (
                <div className="flex flex-col items-center justify-center rounded-lg border border-destructive/30 py-12 text-center">
                    <AlertTriangle className="h-6 w-6 text-destructive mb-3" />
                    <h3 className="text-base font-semibold mb-1">Failed to load connections</h3>
                    <p className="text-sm text-muted-foreground mb-4 max-w-md">{loadError}</p>
                    <Button onClick={load} variant="outline" className="gap-2">
                        <RefreshCw size={14} /> Retry
                    </Button>
                </div>
            ) : conns.length === 0 ? (
                <div className="flex flex-col items-center justify-center rounded-lg border border-dashed py-16 text-center">
                    <div className="flex h-14 w-14 items-center justify-center rounded-xl bg-primary/10 mb-4">
                        <Link2 className="h-6 w-6 text-primary" />
                    </div>
                    <h3 className="text-lg font-semibold mb-1">No connections yet</h3>
                    <p className="text-sm text-muted-foreground mb-4 max-w-sm">
                        Add Kiro, OpenAI, Anthropic, Gemini, or any compatible endpoint to begin routing requests.
                    </p>
                    <Button onClick={() => navigate('/connections/add')} variant="outline" className="gap-2">
                        <Plus size={16} /> Connect Now
                    </Button>
                </div>
            ) : filteredConns.length === 0 ? (
                <div className="flex flex-col items-center justify-center rounded-lg border py-12 text-center">
                    <p className="text-sm text-muted-foreground">
                        No connections match the current search and status filter.
                    </p>
                    <Button variant="link" onClick={clearFilters} className="mt-2 text-sm">
                        Clear filters
                    </Button>
                </div>
            ) : (
                <div className="space-y-6">
                    {groupedConns.map((group) => (
                        <ConnectionGroup
                            key={group.id}
                            group={group}
                            isCollapsed={collapsedGroups[group.id]}
                            isFetching={fetchingGroups[group.id]}
                            hasFetched={fetchedGroups[group.id]}
                            quotaResult={quotaResult}
                            onToggle={() => toggleGroup(group.id)}
                            onFetchQuota={() => handleFetchGroupQuota(group.id)}
                            onReload={load}
                            onDelete={(id, name) => setDeleteTarget({ id, name })}
                            onEditModels={setEditModelsConn}
                            onEditConnection={setEditConn}
                            onViewDetails={setDetailConn}
                        />
                    ))}
                </div>
            )}

            <ConnectionDetailSheet
                connection={detailConn}
                open={!!detailConn}
                onOpenChange={(open) => {
                    if (!open) setDetailConn(null);
                }}
                onEditModels={(conn) => {
                    setDetailConn(null);
                    setEditModelsConn(conn);
                }}
                onEditConnection={(conn) => {
                    setDetailConn(null);
                    setEditConn(conn);
                }}
            />

            {editModelsConn && (
                <EditModelsModal
                    conn={editModelsConn}
                    onSave={handleModelsEditSave}
                    onClose={() => setEditModelsConn(null)}
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
                    onConfirm={handleDeleteConfirm}
                    onClose={() => setDeleteTarget(null)}
                />
            )}
        </div>
    );
}
