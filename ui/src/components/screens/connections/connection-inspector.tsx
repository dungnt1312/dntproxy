import { useEffect, useState } from 'react';
import {
  AlertTriangle,
  Eraser,
  ExternalLink,
  Loader2,
  RefreshCw,
  Settings2,
  TestTube,
  TerminalSquare,
} from 'lucide-react';
import { toast } from 'sonner';
import { api } from '@/api';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { ScrollArea } from '@/components/ui/scroll-area';
import { Separator } from '@/components/ui/separator';
import { Switch } from '@/components/ui/switch';
import { cn } from '@/lib/utils';
import InlineName from '@/components/connections/InlineName';
import { ProviderLogoIcon } from '@/components/connections/helpers';
import LogsViewerModal from '@/components/connections/LogsViewerModal';
import QuotaPanel from '@/components/connections/QuotaPanel';
import { getProviderLabel } from '@/lib/provider-registry';
import {
  hasBackoff,
  isExpired,
  isRateLimited,
  lockedModelCount,
  rateLimitSecondsLeft,
  secsAgoHuman,
  secsToHuman,
  tokenSecondsLeft,
} from '@/lib/connection-status';
import { quotaStore, useNow, useQuotaEntry } from './quota-store';
import type { Connection } from '@/types/connections';

interface ConnectionInspectorProps {
  conn: Connection;
  /** Stripped model ids missing from the current catalog, e.g. ["claude-3-sonnet"]. */
  legacyModels?: string[];
  onReload: () => void;
  onDelete: (id: string, name: string) => void;
  onEditModels: (conn: Connection) => void;
  onEditConnection?: (conn: Connection) => void;
}

function Row({ label, value }: { label: string; value?: string | number | null }) {
  return (
    <div className="grid grid-cols-[110px_1fr] gap-3 text-xs">
      <span className="text-muted-foreground">{label}</span>
      <span className="min-w-0 break-all font-medium">{value || '—'}</span>
    </div>
  );
}

function errMessage(e: unknown, fallback: string): string {
  return e instanceof Error && e.message ? e.message : fallback;
}

export function ConnectionInspector({
  conn: c,
  legacyModels = [],
  onReload,
  onDelete,
  onEditModels,
  onEditConnection,
}: ConnectionInspectorProps) {
  const [testResult, setTestResult] = useState<{ loading?: boolean; status?: string; message?: string } | null>(null);
  const [toggleLoading, setToggleLoading] = useState(false);
  const [isLogOpen, setIsLogOpen] = useState(false);

  // The open inspector *is* the watch trigger for the watched quota tier.
  useEffect(() => {
    quotaStore.setInspected(c.id);
    return () => quotaStore.setInspected(null);
  }, [c.id]);

  useNow(); // keeps age labels honest
  const entry = useQuotaEntry(c.id);
  const fetching = !entry && quotaStore.isFetching(c.id);

  const rl = isRateLimited(c);
  const expired = isExpired(c);
  const backoff = hasBackoff(c);
  const locks = lockedModelCount(c);

  const handleRename = async (_id: string, name: string) => {
    await api.updateConnection(c.id, { name });
    onReload();
  };

  const handleToggle = async () => {
    setToggleLoading(true);
    try {
      await api.updateConnection(c.id, { isActive: !c.isActive });
      toast.success(`${c.name} ${c.isActive ? 'disabled' : 'enabled'}`);
      await onReload();
    } catch (e: unknown) {
      toast.error(errMessage(e, 'Failed to update connection'));
    } finally {
      setToggleLoading(false);
    }
  };

  const handleTest = async () => {
    setTestResult({ loading: true });
    try {
      const res = await api.testConnection(c.id);
      setTestResult(res);
      if (res.status === 'ok') toast.success(`${c.name} test passed`);
      else toast.error(res.message || `${c.name} test failed`);
      await onReload();
    } catch (e: unknown) {
      const message = errMessage(e, 'Connection test failed');
      setTestResult({ status: 'error', message });
      toast.error(message);
      await onReload();
    }
  };

  const clearIssue = async (action: () => Promise<unknown>, successMsg: string) => {
    try {
      await action();
      toast.success(successMsg);
      await onReload();
    } catch (e: unknown) {
      toast.error(errMessage(e, 'Failed'));
    }
  };

  const secsLeft = tokenSecondsLeft(c);

  const now = useNow();

  return (
    <>
      <div className="flex h-full flex-col">
        {/* Header */}
        <div className="flex items-start gap-3 border-b p-4">
          <div className="mt-0.5 flex h-10 w-10 shrink-0 items-center justify-center overflow-hidden rounded-lg border bg-muted">
            <ProviderLogoIcon provider={c.provider} size={36} className="h-full w-full object-cover" />
          </div>
          <div className="min-w-0 flex-1">
            <InlineName conn={c} onRename={handleRename} />
            <p className="truncate text-[11px] text-muted-foreground">
              {getProviderLabel(c.provider)} · {c.email || c.baseUrl || c.authMethod || 'Connection'}
            </p>
          </div>
          <Switch
            checked={c.isActive}
            disabled={toggleLoading}
            onCheckedChange={handleToggle}
            aria-label={c.isActive ? 'Disable connection' : 'Enable connection'}
            className="data-[state=checked]:bg-emerald-500"
          />
        </div>

        <ScrollArea className="flex-1">
          <div className="space-y-4 p-4">
            {/* Status */}
            <section className="space-y-2">
              <div className="flex flex-wrap items-center gap-1.5">
                <Badge variant="outline" className={cn(c.isActive ? 'border-emerald-500/25 bg-emerald-500/10 text-emerald-600' : 'bg-muted text-muted-foreground')}>
                  {c.isActive ? 'Active' : 'Idle'}
                </Badge>
                {rl && (
                  <Badge variant="outline" className="border-amber-500/25 bg-amber-500/10 text-amber-600">
                    Rate-limited · {secsToHuman(rateLimitSecondsLeft(c))}
                  </Badge>
                )}
                {expired && (
                  <Badge variant="outline" className="border-destructive/25 bg-destructive/10 text-destructive">
                    Token expired
                  </Badge>
                )}
                {!expired && secsLeft !== null && (
                  <Badge variant="outline" className="border-border bg-muted/40 tabular-nums text-muted-foreground">
                    Token expires in {secsToHuman(secsLeft)}
                  </Badge>
                )}
                {backoff && (
                  <Badge variant="outline" className="border-amber-500/25 bg-amber-500/10 text-amber-600">
                    Backoff L{c.backoffLevel}/7
                  </Badge>
                )}
                {locks > 0 && (
                  <Badge variant="outline" className="border-amber-500/25 bg-amber-500/10 text-amber-600">
                    {locks} locked models
                  </Badge>
                )}
              </div>

              {(rl || expired || c.lastError) && (
                <div className="rounded-lg border border-amber-500/25 bg-amber-500/10 p-2.5 text-xs">
                  <p className="break-words text-muted-foreground">{c.lastError || 'Connection has active runtime error signals.'}</p>
                  <div className="mt-2 flex flex-wrap gap-1.5">
                    {c.lastError && (
                      <Button size="sm" variant="outline" className="h-6 gap-1 px-2 text-[11px]" onClick={() => clearIssue(() => api.clearConnectionError(c.id), 'Error state cleared')}>
                        <Eraser size={11} /> Clear error
                      </Button>
                    )}
                    {(rl || backoff) && (
                      <Button size="sm" variant="outline" className="h-6 gap-1 px-2 text-[11px]" onClick={() => clearIssue(() => api.resetCooldown(c.id), 'Cooldown reset')}>
                        <RefreshCw size={11} /> Reset cooldown
                      </Button>
                    )}
                    <Button size="sm" variant="ghost" className="h-6 gap-1 px-2 text-[11px]" onClick={() => setIsLogOpen(true)}>
                      <TerminalSquare size={11} /> View logs
                    </Button>
                  </div>
                </div>
              )}
            </section>

            <Separator />

            {/* Quota — auto-loaded & watched while this connection is selected */}
            <section className="space-y-2">
              <div className="flex items-center justify-between">
                <h3 className="text-sm font-semibold">Quota</h3>
                <div className="flex items-center gap-2">
                  {entry && !fetching && (
                    <span className="text-[10px] text-muted-foreground">{secsAgoHuman(Math.floor((now - entry.fetchedAt) / 1000))}</span>
                  )}
                  {fetching && <Loader2 size={12} className="animate-spin text-muted-foreground" />}
                  <button
                    type="button"
                    onClick={() => quotaStore.refresh([c.id])}
                    title="Refresh quota"
                    aria-label="Refresh quota"
                    className="rounded p-1 text-muted-foreground transition-colors hover:bg-muted hover:text-foreground"
                  >
                    <RefreshCw size={12} className={cn(fetching && 'animate-spin')} />
                  </button>
                </div>
              </div>
              {c.supportsQuota === false ? (
                <p className="rounded-lg border bg-muted/25 p-3 text-xs text-muted-foreground">
                  This provider does not support quota checks.
                </p>
              ) : entry?.error ? (
                <div className="flex items-center justify-between gap-2 rounded-lg border border-destructive/20 bg-destructive/5 p-3">
                  <p className="text-xs text-destructive">{entry.error}</p>
                  <Button size="sm" variant="outline" className="h-7 shrink-0" onClick={() => quotaStore.refresh([c.id])}>
                    Retry
                  </Button>
                </div>
              ) : entry ? (
                <QuotaPanel data={entry.data} loading={false} onRefresh={() => quotaStore.refresh([c.id])} />
              ) : (
                <p className="flex items-center gap-2 rounded-lg border bg-muted/25 p-3 text-xs text-muted-foreground">
                  <Loader2 size={12} className="animate-spin" /> Loading quota…
                </p>
              )}
            </section>

            <Separator />

            {/* Models */}
            <section className="space-y-2">
              <div className="flex items-center justify-between">
                <h3 className="text-sm font-semibold">Models</h3>
                <Badge variant="secondary" className="tabular-nums">
                  {c.supportedModels?.length ? `${c.supportedModels.length} configured` : 'allow-all'}
                </Badge>
              </div>
              {legacyModels.length > 0 && (
                <div className="rounded-lg border border-amber-500/30 bg-amber-500/10 p-2.5">
                  <p className="mb-1 flex items-center gap-1.5 text-xs font-medium text-amber-700 dark:text-amber-300">
                    <AlertTriangle size={13} /> {legacyModels.length} model(s) missing from the current catalog
                  </p>
                  <p className="font-mono text-[10px] leading-relaxed text-muted-foreground">{legacyModels.join(', ')}</p>
                </div>
              )}
              {c.supportedModels?.length ? (
                <div className="flex max-h-40 flex-wrap gap-1.5 overflow-auto rounded-lg border bg-muted/25 p-2.5">
                  {c.supportedModels.map((m) => (
                    <code key={m} className="rounded border bg-background px-1.5 py-0.5 font-mono text-[11px] text-muted-foreground">
                      {m}
                    </code>
                  ))}
                </div>
              ) : (
                <p className="text-xs text-muted-foreground">Unrestricted — the connection serves any model using provider defaults.</p>
              )}
              <div className="flex flex-wrap gap-2 pt-1">
                <Button variant="outline" size="sm" className="h-8 gap-2" onClick={() => onEditModels(c)}>
                  <Settings2 size={14} /> Edit models
                </Button>
                {(c.authType === 'apikey' || c.apiKey) && onEditConnection && (
                  <Button variant="outline" size="sm" className="h-8 gap-2" onClick={() => onEditConnection(c)}>
                    <ExternalLink size={14} /> Edit connection
                  </Button>
                )}
              </div>
            </section>

            <Separator />

            {/* Routing details */}
            <section className="space-y-2.5">
              <h3 className="text-sm font-semibold">Routing details</h3>
              <Row label="Connection ID" value={c.id} />
              <Row label="Auth method" value={c.authMethod || c.authType} />
              <Row label="Route prefix" value={c.routePrefix || c.modelPrefix} />
              <Row label="Expires at" value={c.expiresAt ? new Date(c.expiresAt).toLocaleString() : null} />
              <Row label="Priority / weight" value={[c.priority ?? '—', c.weight ?? '—'].join(' / ')} />
            </section>
          </div>
        </ScrollArea>

        {/* Footer actions */}
        <div className="flex items-center gap-2 border-t p-3">
          <Button variant="secondary" size="sm" className="gap-1.5" onClick={handleTest} disabled={testResult?.loading}>
            {testResult?.loading ? <Loader2 size={13} className="animate-spin" /> : <TestTube size={13} />} Test
          </Button>
          {testResult && !testResult.loading && (
            <span
              className={cn(
                'rounded-md px-2 py-0.5 text-[10px] font-medium',
                testResult.status === 'ok' ? 'bg-emerald-500/10 text-emerald-600' : 'bg-destructive/10 text-destructive',
              )}
              title={testResult.message}
            >
              {testResult.status === 'ok' ? '✓ OK' : '✗ Fail'}
            </span>
          )}
          <span className="flex-1" />
          <Button variant="ghost" size="sm" className="text-xs text-muted-foreground" onClick={() => onDelete(c.id, c.name)}>
            Delete…
          </Button>
        </div>
      </div>

      <LogsViewerModal isOpen={isLogOpen} onClose={() => setIsLogOpen(false)} title={`Logs: ${c.name}`} filter={{ connectionId: c.id }} />
    </>
  );
}
