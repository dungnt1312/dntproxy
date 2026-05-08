import { useState, useCallback, useEffect, useRef } from 'react';
import { api } from '@/api';
import type { Connection, ConnectionGroup, UsageData } from '@/types/connections';

interface UseQuotaFetchOptions {
    groupedConns: ConnectionGroup[];
    autoRefreshQuota: boolean;
    onConnectionsUpdate?: (updater: (prev: Connection[]) => Connection[]) => void;
}

interface UseQuotaFetchReturn {
    quotaResult: Record<string, UsageData>;
    fetchedGroups: Record<string, boolean>;
    fetchingGroups: Record<string, boolean>;
    handleFetchGroupQuota: (groupId: string) => Promise<void>;
}

/**
 * Custom hook for managing quota fetching logic per provider group.
 * Handles on-demand fetching and auto-refresh for explicitly loaded groups.
 */
export function useQuotaFetch({
    groupedConns,
    autoRefreshQuota,
    onConnectionsUpdate,
}: UseQuotaFetchOptions): UseQuotaFetchReturn {
    const [quotaResult, setQuotaResult] = useState<Record<string, UsageData>>({});
    const [fetchedGroups, setFetchedGroups] = useState<Record<string, boolean>>({});
    const [fetchingGroups, setFetchingGroups] = useState<Record<string, boolean>>({});
    const quotaRefreshInFlightRef = useRef(false);

    // ── On-demand quota fetch per provider group ───────────────────────────────
    const handleFetchGroupQuota = useCallback(
        async (groupId: string) => {
            const group = groupedConns.find((g) => g.id === groupId);
            if (!group) return;

            setFetchingGroups((prev) => ({ ...prev, [groupId]: true }));

            const promises = group.items
                .filter((c) => c.isActive)
                .map(async (c) => {
                    try {
                        const res = await api.getUsage(c.id);
                        return [c.id, res] as const;
                    } catch (e: any) {
                        return [c.id, { error: e.message }] as const;
                    }
                });

            const settled = await Promise.allSettled(promises);
            const quotaUpdates: Record<string, UsageData> = {};
            settled.forEach((result) => {
                if (result.status === 'fulfilled') {
                    const [connId, value] = result.value;
                    quotaUpdates[connId] = value;
                }
            });

            if (Object.keys(quotaUpdates).length > 0) {
                setQuotaResult((prev) => ({ ...prev, ...quotaUpdates }));
                onConnectionsUpdate?.((prev) =>
                    prev.map((conn) => {
                        const update = quotaUpdates[conn.id];
                        if (update && !update.error && !update.limitReached) {
                            return {
                                ...conn,
                                lastError: undefined,
                                rateLimitedUntil: undefined,
                                backoffLevel: undefined,
                            };
                        }
                        return conn;
                    }),
                );
            }
            setFetchingGroups((prev) => ({ ...prev, [groupId]: false }));
            setFetchedGroups((prev) => ({ ...prev, [groupId]: true }));
        },
        [groupedConns, onConnectionsUpdate],
    );

    // ── Auto-refresh: only for groups that were explicitly fetched ──────────────
    useEffect(() => {
        if (!autoRefreshQuota) return;
        const fetchedGroupIds = Object.keys(fetchedGroups);
        if (fetchedGroupIds.length === 0) return;

        const refreshFetchedGroupsQuota = async () => {
            if (quotaRefreshInFlightRef.current) return;
            quotaRefreshInFlightRef.current = true;

            try {
                const activeConnections = fetchedGroupIds.flatMap((groupId) => {
                    const group = groupedConns.find((g) => g.id === groupId);
                    if (!group) return [] as Connection[];
                    return group.items.filter((c) => c.isActive);
                });

                if (activeConnections.length === 0) return;

                const uniqueConnections = Array.from(new Map(activeConnections.map((c) => [c.id, c])).values());
                const results = await Promise.allSettled(
                    uniqueConnections.map(async (c) => {
                        const usage = await api.getUsage(c.id);
                        return [c.id, usage] as const;
                    }),
                );

                const quotaUpdates: Record<string, UsageData> = {};
                results.forEach((result) => {
                    if (result.status === 'fulfilled') {
                        const [id, usage] = result.value;
                        quotaUpdates[id] = usage;
                    }
                });

                if (Object.keys(quotaUpdates).length > 0) {
                    setQuotaResult((prev) => ({ ...prev, ...quotaUpdates }));
                    onConnectionsUpdate?.((prev) =>
                        prev.map((conn) => {
                            const update = quotaUpdates[conn.id];
                            if (update && !update.error && !update.limitReached) {
                                return {
                                    ...conn,
                                    lastError: undefined,
                                    rateLimitedUntil: undefined,
                                    backoffLevel: undefined,
                                };
                            }
                            return conn;
                        }),
                    );
                }
            } finally {
                quotaRefreshInFlightRef.current = false;
            }
        };

        refreshFetchedGroupsQuota();
        const t = setInterval(refreshFetchedGroupsQuota, 30000);
        return () => clearInterval(t);
    }, [autoRefreshQuota, fetchedGroups, groupedConns, onConnectionsUpdate]);

    return {
        quotaResult,
        fetchedGroups,
        fetchingGroups,
        handleFetchGroupQuota,
    };
}
