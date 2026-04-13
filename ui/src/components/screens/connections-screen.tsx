import { useEffect, useState, useMemo, useCallback } from 'react';
import { useNavigate } from 'react-router-dom';
import { api } from '../../api';
import { Plus, Search, Link2, AlertTriangle, ChevronDown, Zap, RefreshCw } from 'lucide-react';
import { getProviderLabel } from '../connections/helpers';
import ConnectionCard from '../connections/ConnectionCard';
import EditModelsModal from '../connections/EditModelsModal';
import EditConnectionModal from '../connections/EditConnectionModal';
import DeleteDialog from '../connections/DeleteDialog';
import { ProviderLogoIcon } from '../connections/helpers';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Card, CardContent } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { toast } from 'sonner';
import { cn } from '@/lib/utils';

// ─── Provider metadata ────────────────────────────────────────────────────────

const PROVIDER_ORDER = [
    'kiro',
    'openai',
    'qwen',
    'glm',
    'minimax',
    'anthropic',
    'gemini',
    'openai-compatible',
] as const;

const PROVIDER_META: Record<string, { label: string; colorClass: string }> = {
    'kiro': {
        label: 'AWS / Kiro',
        colorClass: 'bg-orange-500/10 border-orange-500/20 text-orange-600',
    },
    'openai': {
        label: 'OpenAI',
        colorClass: 'bg-emerald-500/10 border-emerald-500/20 text-emerald-600',
    },
    'qwen': {
        label: 'Qwen',
        colorClass: 'bg-indigo-500/10 border-indigo-500/20 text-indigo-600',
    },
    'glm': {
        label: 'GLM (Zhipu AI)',
        colorClass: 'bg-blue-500/10 border-blue-500/20 text-blue-600',
    },
    'minimax': {
        label: 'MiniMax',
        colorClass: 'bg-orange-400/10 border-orange-400/20 text-orange-500',
    },
    'anthropic': {
        label: 'Anthropic',
        colorClass: 'bg-amber-600/10 border-amber-600/20 text-amber-600',
    },
    'gemini': {
        label: 'Gemini',
        colorClass: 'bg-blue-400/10 border-blue-400/20 text-blue-500',
    },
    'openai-compatible': {
        label: 'OpenAI Compatible',
        colorClass: 'bg-purple-500/10 border-purple-500/20 text-purple-600',
    },
};

// ─── Component ────────────────────────────────────────────────────────────────

export default function ConnectionsScreen() {
    const navigate = useNavigate();
    const [conns, setConns] = useState<any[]>([]);
    const [editModelsConn, setEditModelsConn] = useState<any | null>(null);
    const [editConn, setEditConn] = useState<any | null>(null);
    const [deleteTarget, setDeleteTarget] = useState<{
        id: string;
        name: string;
    } | null>(null);
    const [quotaResult, setQuotaResult] = useState<Record<string, any>>({});
    const [searchQuery, setSearchQuery] = useState('');
    const [autoRefreshQuota, setAutoRefreshQuota] = useState(false);
    const [collapsedGroups, setCollapsedGroups] = useState<Record<string, boolean>>({});

    // Track which provider groups have been fetched for quota + which are loading
    const [fetchedGroups, setFetchedGroups] = useState<Record<string, boolean>>({});
    const [fetchingGroups, setFetchingGroups] = useState<Record<string, boolean>>({});

    const toggleGroup = useCallback((id: string) => {
        setCollapsedGroups((prev) => ({ ...prev, [id]: !prev[id] }));
    }, []);

    // ── Stats ─────────────────────────────────────────────────────────────────
    const connectionStats = useMemo(() => {
        const total = conns.length;
        const active = conns.filter((c: any) => c.isActive).length;
        const needsAttention = conns.filter((c: any) => {
            if (!c.isActive) return false;
            const rl = c.rateLimitedUntil && new Date(c.rateLimitedUntil) > new Date();
            const exp = c.expiresAt && new Date(c.expiresAt) < new Date();
            return rl || exp || (c.backoffLevel ?? 0) > 0 || !!c.lastError;
        }).length;
        return { total, active, needsAttention };
    }, [conns]);

    // ── Filter ────────────────────────────────────────────────────────────────
    const filteredConns = useMemo(() => {
        const q = searchQuery.trim().toLowerCase();
        if (!q) return conns;
        return conns.filter((c: any) => {
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
    }, [conns, searchQuery]);

    // ── Group connections by provider (dynamic, no "other" bucket) ──────────────
    const groupedConns = useMemo(() => {
        const list = [...filteredConns].sort((a, b) =>
            a.name.localeCompare(b.name, undefined, { sensitivity: 'base' }),
        );
        const groups: Record<string, any[]> = {};
        list.forEach((c: any) => {
            const p = c.provider || 'unknown';
            if (!groups[p]) groups[p] = [];
            groups[p].push(c);
        });

        const result: any[] = [];

        // Known providers in priority order
        PROVIDER_ORDER.forEach((p) => {
            if (groups[p]) {
                const meta = PROVIDER_META[p];
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
            if (!PROVIDER_ORDER.includes(k as any)) {
                const label = getProviderLabel(k);
                result.push({
                    id: k,
                    label,
                    items: groups[k],
                    icon: <ProviderLogoIcon provider={k} size={28} className="w-full h-full object-cover" />,
                    colorClass: 'bg-gray-500/10 border-gray-500/20 text-gray-400',
                });
            }
        });

        return result;
    }, [filteredConns]);

    // ── Load connections only (no quota fetch) ─────────────────────────────────
    const load = () =>
        api
            .getConnections()
            .then((d) => setConns(d || []))
            .catch(() => {});

    useEffect(() => {
        load();
    }, []);

    // ── On-demand quota fetch per provider group ───────────────────────────────
    const handleFetchGroupQuota = useCallback(
        async (groupId: string) => {
            const group = groupedConns.find((g) => g.id === groupId);
            if (!group) return;

            setFetchingGroups((prev) => ({ ...prev, [groupId]: true }));

            const promises = group.items
                .filter((c: any) => c.isActive)
                .map(async (c: any) => {
                    try {
                        const res = await api.getUsage(c.id);
                        setQuotaResult((prev) => ({ ...prev, [c.id]: res }));
                    } catch (e: any) {
                        setQuotaResult((prev) => ({ ...prev, [c.id]: { error: e.message } }));
                    }
                });

            await Promise.allSettled(promises);
            setFetchingGroups((prev) => ({ ...prev, [groupId]: false }));
            setFetchedGroups((prev) => ({ ...prev, [groupId]: true }));
        },
        [groupedConns],
    );

    // ── Auto-refresh: only for groups that were explicitly fetched ──────────────
    useEffect(() => {
        if (!autoRefreshQuota) return;
        const fetchedGroupIds = Object.keys(fetchedGroups);
        if (fetchedGroupIds.length === 0) return;

        const t = setInterval(() => {
            fetchedGroupIds.forEach((groupId) => {
                const group = groupedConns.find((g) => g.id === groupId);
                group?.items.forEach((c: any) => {
                    if (c.isActive) {
                        api.getUsage(c.id)
                            .then((res) => setQuotaResult((prev) => ({ ...prev, [c.id]: res })))
                            .catch(() => {});
                    }
                });
            });
        }, 30000);
        return () => clearInterval(t);
    }, [autoRefreshQuota, conns, fetchedGroups, groupedConns]);

    // ── Handlers ───────────────────────────────────────────────────────────────
    const handleDeleteConfirm = async (id: string) => {
        await api.deleteConnection(id);
        load();
    };

    const handleModelsEditSave = () => {
        setEditModelsConn(null);
        load();
    };

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

            {/* Stats */}
            {conns.length > 0 && (
                <div className="grid grid-cols-1 sm:grid-cols-3 gap-4">
                    <Card>
                        <CardContent className="p-4">
                            <div className="flex items-center gap-2 text-xs text-muted-foreground mb-1">
                                <Link2 size={12} /> Total Connections
                            </div>
                            <div className="text-2xl font-bold">{connectionStats.total}</div>
                        </CardContent>
                    </Card>
                    <Card>
                        <CardContent className="p-4">
                            <div className="flex items-center gap-2 text-xs text-emerald-600 mb-1">
                                <span className="h-2 w-2 rounded-full bg-emerald-500" /> Active
                            </div>
                            <div className="text-2xl font-bold text-emerald-600">{connectionStats.active}</div>
                        </CardContent>
                    </Card>
                    <Card className={connectionStats.needsAttention > 0 ? 'border-destructive/40' : ''}>
                        <CardContent className="p-4">
                            <div
                                className={cn(
                                    'flex items-center gap-2 text-xs mb-1',
                                    connectionStats.needsAttention > 0 ? 'text-destructive' : 'text-muted-foreground',
                                )}
                            >
                                <AlertTriangle size={12} /> Issues
                            </div>
                            <div
                                className={cn(
                                    'text-2xl font-bold',
                                    connectionStats.needsAttention > 0 ? 'text-destructive' : 'text-muted-foreground',
                                )}
                            >
                                {connectionStats.needsAttention}
                            </div>
                        </CardContent>
                    </Card>
                </div>
            )}

            {/* Toolbar */}
            {conns.length > 0 && (
                <div className="flex flex-col sm:flex-row sm:items-center gap-3 rounded-lg border bg-muted/30 p-3">
                    <div className="relative flex-1 min-w-0">
                        <Search
                            size={14}
                            className="absolute left-3 top-1/2 -translate-y-1/2 text-muted-foreground pointer-events-none"
                        />
                        <Input
                            type="search"
                            value={searchQuery}
                            onChange={(e) => setSearchQuery(e.target.value)}
                            placeholder="Search by name, email, or model…"
                            className="pl-9 h-8 text-sm"
                            autoComplete="off"
                            data-1p-ignore
                        />
                    </div>
                    <label className="flex cursor-pointer items-center gap-2 text-xs text-muted-foreground hover:text-foreground transition-colors select-none shrink-0">
                        <input
                            type="checkbox"
                            checked={autoRefreshQuota}
                            onChange={(e) => setAutoRefreshQuota(e.target.checked)}
                            className="w-3.5 h-3.5 rounded cursor-pointer"
                        />
                        Auto refresh Quotas
                    </label>
                </div>
            )}

            {/* List */}
            {conns.length === 0 ? (
                <div className="flex flex-col items-center justify-center rounded-lg border border-dashed py-16 text-center">
                    <div className="flex h-14 w-14 items-center justify-center rounded-xl bg-primary/10 mb-4">
                        <Link2 className="h-6 w-6 text-primary" />
                    </div>
                    <h3 className="text-lg font-semibold mb-1">No connections yet</h3>
                    <p className="text-sm text-muted-foreground mb-4 max-w-sm">
                        Add Kiro AI or OpenAI accounts to begin routing requests.
                    </p>
                    <Button onClick={() => navigate('/connections/add')} variant="outline" className="gap-2">
                        <Plus size={16} /> Connect Now
                    </Button>
                </div>
            ) : filteredConns.length === 0 ? (
                <div className="flex flex-col items-center justify-center rounded-lg border py-12 text-center">
                    <p className="text-sm text-muted-foreground">No connections matching "{searchQuery.trim()}"</p>
                    <Button variant="link" onClick={() => setSearchQuery('')} className="mt-2 text-sm">
                        Clear filters
                    </Button>
                </div>
            ) : (
                <div className="space-y-6">
                    {groupedConns.map((group) => {
                        const isCollapsed = collapsedGroups[group.id];
                        const hasActiveItems = group.items.some((c: any) => c.isActive);
                        const isFetching = fetchingGroups[group.id];
                        const hasFetched = fetchedGroups[group.id];

                        return (
                            <div key={group.id}>
                                <div className="flex items-center gap-2 mb-3">
                                    {/* Clickable group header (toggle collapse) */}
                                    <div
                                        className="flex items-center gap-2 cursor-pointer select-none flex-1 group/col"
                                        onClick={() => toggleGroup(group.id)}
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
                                        <ChevronDown
                                            className={`h-4 w-4 text-muted-foreground transition-transform ${isCollapsed ? '-rotate-90' : ''}`}
                                        />
                                    </div>

                                    {/* Check quota button — only for groups with active connections */}
                                    {hasActiveItems && !isFetching && (
                                        <Button
                                            variant="outline"
                                            size="sm"
                                            onClick={(e) => {
                                                e.stopPropagation();
                                                handleFetchGroupQuota(group.id);
                                            }}
                                            className={cn(
                                                'text-xs h-7 gap-1 shrink-0',
                                                hasFetched
                                                    ? 'text-primary'
                                                    : 'text-amber-600 border-amber-500/30 hover:bg-amber-500/10',
                                            )}
                                            title={hasFetched ? 'Refresh quota' : 'Check quota'}
                                        >
                                            {hasFetched ? (
                                                <RefreshCw className="h-3 w-3" />
                                            ) : (
                                                <Zap className="h-3 w-3" />
                                            )}
                                            {hasFetched ? 'Refresh' : 'Check'}
                                        </Button>
                                    )}

                                    {/* Loading spinner while fetching */}
                                    {isFetching && <RefreshCw className="h-4 w-4 animate-spin text-primary shrink-0" />}
                                </div>
                                {!isCollapsed && (
                                    <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 gap-3">
                                        {group.items.map((c: any) => (
                                            <ConnectionCard
                                                key={c.id}
                                                conn={c}
                                                initialQuotaResult={quotaResult[c.id]}
                                                onReload={load}
                                                onDelete={(id, name) => setDeleteTarget({ id, name })}
                                                onEditModels={setEditModelsConn}
                                                onEditConnection={setEditConn}
                                            />
                                        ))}
                                    </div>
                                )}
                            </div>
                        );
                    })}
                </div>
            )}

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
