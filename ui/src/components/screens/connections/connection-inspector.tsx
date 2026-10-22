import { useEffect, useState } from 'react';
import {
  CheckCircle2,
  Download,
  Eraser,
  Loader2,
  Pencil,
  RefreshCw,
  Settings2,
  TerminalSquare,
  TestTube,
  Trash2,
  XCircle,
} from 'lucide-react';
import { toast } from 'sonner';
import { api } from '@/api';
import { Button } from '@/components/ui/button';
import { Switch } from '@/components/ui/switch';
import { modelCount } from '@/components/connections/model-fetch';
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
import { StatusBadge } from './status-ui';
import { errMessage } from './status-tone';
import { downloadConnectionsExport } from './export';
import type { Connection } from '@/types/connections';

interface ConnectionInspectorProps {
  conn: Connection;
  onReload: () => Promise<void> | void;
  onDelete: (conn: Connection) => void;
  onEditModels: (conn: Connection) => void;
  onEditConnection: (conn: Connection) => void;
}

function Row({ label, value, mono }: { label: string; value?: string | number | null; mono?: boolean }) {
  return (
    <div className="grid grid-cols-[120px_1fr] gap-3 text-sm">
      <dt className="text-muted-foreground">{label}</dt>
      <dd className={mono ? 'min-w-0 break-all font-mono text-xs leading-5' : 'min-w-0 break-words'}>{value || '—'}</dd>
    </div>
  );
}

function Section({ title, action, children }: { title: string; action?: React.ReactNode; children: React.ReactNode }) {
  return (
    <section className="space-y-3 border-b p-4 last:border-b-0">
      <div className="flex min-h-8 items-center justify-between gap-2">
        <h3 className="text-sm font-semibold">{title}</h3>
        {action}
      </div>
      {children}
    </section>
  );
}

/** Render with `key={conn.id}` so per-connection state (test result, busy flags) resets on switch. */
export function ConnectionInspector({ conn: c, onReload, onDelete, onEditModels, onEditConnection }: ConnectionInspectorProps) {
  const [testState, setTestState] = useState<{ loading: boolean; ok?: boolean; message?: string }>({ loading: false });
  const [toggleLoading, setToggleLoading] = useState(false);
  const [busyAction, setBusyAction] = useState<string | null>(null);
  const [isLogOpen, setIsLogOpen] = useState(false);
  const now = useNow();

  // The open inspector is the watch trigger for the Quota-auto watched tier.
  useEffect(() => {
    quotaStore.setInspected(c.id);
    return () => quotaStore.setInspected(null);
  }, [c.id]);

  const entry = useQuotaEntry(c.id);
  const fetching = quotaStore.isFetching(c.id);

  const rl = isRateLimited(c);
  const expired = isExpired(c);
  const backoff = hasBackoff(c);
  const locks = lockedModelCount(c);
  const secsLeft = tokenSecondsLeft(c);
  const hasIssue = rl || expired || backoff || locks > 0 || Boolean(c.lastError);

  const handleToggle = async (next: boolean) => {
    setToggleLoading(true);
    try {
      await api.updateConnection(c.id, { isActive: next });
      toast.success(`${c.name} ${next ? 'enabled' : 'disabled'}`);
      await onReload();
    } catch (e: unknown) {
      toast.error(errMessage(e, 'Failed to update connection'));
    } finally {
      setToggleLoading(false);
    }
  };

  const handleTest = async () => {
    setTestState({ loading: true });
    try {
      const res = await api.testConnection(c.id);
      const ok = res.status === 'ok';
      setTestState({ loading: false, ok, message: res.message });
      if (ok) toast.success(`${c.name}: test passed`);
      else toast.error(res.message || `${c.name}: test failed`);
    } catch (e: unknown) {
      const message = errMessage(e, 'Connection test failed');
      setTestState({ loading: false, ok: false, message });
      toast.error(message);
    }
    await onReload();
  };

  const runAction = async (key: string, action: () => Promise<unknown>, successMsg: string) => {
    setBusyAction(key);
    try {
      await action();
      toast.success(successMsg);
      await onReload();
    } catch (e: unknown) {
      toast.error(errMessage(e, 'Action failed'));
    } finally {
      setBusyAction(null);
    }
  };

  const handleExport = () =>
    runAction('export', () => downloadConnectionsExport([c.id]), `Exported ${c.name}`);

  return (
    <>
      <div className="flex h-full flex-col">
        {/* Header */}
        <div className="flex items-start gap-3 border-b p-4">
          <div className="flex size-10 shrink-0 items-center justify-center overflow-hidden rounded-lg border bg-background">
            <ProviderLogoIcon provider={c.provider} size={28} />
          </div>
          <div className="min-w-0 flex-1">
            <InlineName
              conn={c}
              onRename={async (name) => {
                await api.updateConnection(c.id, { name });
                await onReload();
              }}
            />
            <p className="truncate text-sm text-muted-foreground">
              {getProviderLabel(c.provider)}
              {(c.email || c.baseUrl) && ` · ${c.email || c.baseUrl}`}
            </p>
          </div>
          <label className="flex shrink-0 items-center gap-2 pt-1 text-sm text-muted-foreground">
            {c.isActive ? 'Enabled' : 'Disabled'}
            <Switch checked={c.isActive} disabled={toggleLoading} onCheckedChange={handleToggle} aria-label="Enable connection" />
          </label>
        </div>

        <div className="min-h-0 flex-1 overflow-y-auto">
          {/* Status */}
          <Section title="Status">
            <div className="flex flex-wrap gap-1.5">
              {!c.isActive && <StatusBadge tone="neutral">Disabled</StatusBadge>}
              {c.isActive && !hasIssue && <StatusBadge tone="success" icon={<CheckCircle2 className="size-3" />}>Healthy</StatusBadge>}
              {expired && <StatusBadge tone="danger">Token expired</StatusBadge>}
              {!expired && secsLeft !== null && <StatusBadge tone="neutral">Token expires in {secsToHuman(secsLeft)}</StatusBadge>}
              {rl && <StatusBadge tone="warning">Rate-limited · {secsToHuman(rateLimitSecondsLeft(c))}</StatusBadge>}
              {backoff && <StatusBadge tone="warning">Backoff {c.backoffLevel}/7</StatusBadge>}
              {locks > 0 && <StatusBadge tone="warning">{locks} locked model{locks > 1 ? 's' : ''}</StatusBadge>}
            </div>

            {c.lastError && (
              <p role="status" className="rounded-md border border-destructive/30 bg-destructive/5 p-3 text-sm break-words text-destructive">
                {c.lastError}
              </p>
            )}

            {(c.lastError || rl || backoff) && (
              <div className="flex flex-wrap gap-2">
                {c.lastError && (
                  <Button size="sm" variant="outline" className="gap-1.5" disabled={busyAction !== null}
                    onClick={() => runAction('clear', () => api.clearConnectionError(c.id), 'Error cleared')}>
                    {busyAction === 'clear' ? <Loader2 className="size-3.5 animate-spin" /> : <Eraser className="size-3.5" />} Clear error
                  </Button>
                )}
                {(rl || backoff) && (
                  <Button size="sm" variant="outline" className="gap-1.5" disabled={busyAction !== null}
                    onClick={() => runAction('cooldown', () => api.resetCooldown(c.id), 'Cooldown reset')}>
                    {busyAction === 'cooldown' ? <Loader2 className="size-3.5 animate-spin" /> : <RefreshCw className="size-3.5" />} Reset cooldown
                  </Button>
                )}
              </div>
            )}

            <div className="flex flex-wrap items-center gap-2">
              <Button size="sm" variant="secondary" className="gap-1.5" onClick={handleTest} disabled={testState.loading}>
                {testState.loading ? <Loader2 className="size-3.5 animate-spin" /> : <TestTube className="size-3.5" />} Test connection
              </Button>
              <Button size="sm" variant="ghost" className="gap-1.5" onClick={() => setIsLogOpen(true)}>
                <TerminalSquare className="size-3.5" /> Logs
              </Button>
              {!testState.loading && testState.ok !== undefined && (
                <StatusBadge
                  tone={testState.ok ? 'success' : 'danger'}
                  icon={testState.ok ? <CheckCircle2 className="size-3" /> : <XCircle className="size-3" />}
                  title={testState.message}
                >
                  {testState.ok ? 'Passed' : 'Failed'}
                </StatusBadge>
              )}
            </div>
          </Section>

          {/* Quota */}
          <Section
            title="Quota"
            action={
              c.supportsQuota !== false && (
                <div className="flex items-center gap-2">
                  {entry && !fetching && (
                    <span className="text-xs text-muted-foreground">{secsAgoHuman(Math.floor((now - entry.fetchedAt) / 1000))}</span>
                  )}
                  <Button
                    variant="ghost"
                    size="sm"
                    className="h-8 gap-1.5"
                    onClick={() => quotaStore.refresh([c.id])}
                    disabled={fetching || !c.isActive}
                    title={c.isActive ? 'Refresh quota' : 'Enable the connection to load quota'}
                  >
                    <RefreshCw className={fetching ? 'size-3.5 animate-spin' : 'size-3.5'} />
                    {entry ? 'Refresh' : 'Load'}
                  </Button>
                </div>
              )
            }
          >
            {c.supportsQuota === false ? (
              <p className="text-sm text-muted-foreground">This provider does not report quota.</p>
            ) : entry?.error ? (
              <p role="alert" className="text-sm text-destructive">{entry.error}</p>
            ) : entry?.data ? (
              <QuotaPanel data={entry.data} />
            ) : fetching ? (
              <p className="flex items-center gap-2 text-sm text-muted-foreground">
                <Loader2 className="size-3.5 animate-spin" /> Loading quota…
              </p>
            ) : (
              <p className="text-sm text-muted-foreground">
                {c.isActive ? 'Quota not loaded yet.' : 'Connection is disabled.'}
              </p>
            )}
          </Section>

          {/* Models */}
          <Section
            title="Models"
            action={
              <Button variant="outline" size="sm" className="h-8 gap-1.5" onClick={() => onEditModels(c)}>
                <Settings2 className="size-3.5" /> Edit models
              </Button>
            }
          >
            {c.supportedModels?.length ? (
              <>
                <p className="text-sm text-muted-foreground">{modelCount(c.supportedModels.length)} allowed</p>
                <div className="flex max-h-48 flex-wrap gap-1.5 overflow-y-auto">
                  {c.supportedModels.map((m) => (
                    <code key={m} className="rounded border bg-muted/40 px-1.5 py-0.5 font-mono text-xs">
                      {m}
                    </code>
                  ))}
                </div>
              </>
            ) : (
              <p className="text-sm text-muted-foreground">All models allowed (no restriction).</p>
            )}
          </Section>

          {/* Configuration */}
          <Section
            title="Configuration"
            action={
              <Button variant="outline" size="sm" className="h-8 gap-1.5" onClick={() => onEditConnection(c)}>
                <Pencil className="size-3.5" /> Edit
              </Button>
            }
          >
            <dl className="space-y-2">
              <Row label="Auth" value={c.authMethod || c.authType} />
              <Row label="Priority / weight" value={`${c.priority ?? 0} / ${c.weight || 100}`} />
              {(c.routePrefix || c.modelPrefix) && <Row label="Route prefix" value={c.routePrefix || c.modelPrefix} mono />}
              {c.baseUrl && <Row label="Base URL" value={c.baseUrl} mono />}
              <Row label="Expires" value={c.expiresAt ? new Date(c.expiresAt).toLocaleString() : null} />
              <Row label="ID" value={c.id} mono />
            </dl>
          </Section>
        </div>

        {/* Footer actions */}
        <div className="flex items-center gap-2 border-t p-3">
          <Button variant="outline" size="sm" className="gap-1.5" onClick={handleExport} disabled={busyAction !== null}>
            {busyAction === 'export' ? <Loader2 className="size-3.5 animate-spin" /> : <Download className="size-3.5" />} Export
          </Button>
          <span className="flex-1" />
          <Button variant="ghost" size="sm" className="gap-1.5 text-destructive hover:bg-destructive/10 hover:text-destructive" onClick={() => onDelete(c)}>
            <Trash2 className="size-3.5" /> Delete
          </Button>
        </div>
      </div>

      <LogsViewerModal isOpen={isLogOpen} onClose={() => setIsLogOpen(false)} title={`Logs: ${c.name}`} filter={{ connectionId: c.id }} />
    </>
  );
}
