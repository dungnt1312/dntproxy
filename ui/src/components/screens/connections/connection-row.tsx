import { memo } from 'react';
import { AlertTriangle, ChevronRight, Clock, Infinity as InfinityIcon, KeyRound, Loader2, Lock, RefreshCw, Timer } from 'lucide-react';
import { cn } from '@/lib/utils';
import { Checkbox } from '@/components/ui/checkbox';
import { ProviderLogo } from '@/components/connections/ProviderLogo';
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
import { getProviderLabel } from '@/lib/provider-registry';
import { primaryQuotaPct, type QuotaEntry } from './quota-store';
import { QuotaBar, StatusBadge } from './status-ui';
import type { Connection } from '@/types/connections';

export const ROW_HEIGHT = 56;

interface ConnectionRowProps {
  conn: Connection;
  detail?: boolean;
  selectionMode?: boolean;
  selected?: boolean;
  /** In flat view each row shows its provider tag; grouped view doesn't need it. */
  showProvider?: boolean;
  quotaEntry?: QuotaEntry;
  quotaFetching?: boolean;
  /** Wall-clock snapshot from the container, used for freshness titles. */
  nowMs: number;
  onOpen: (id: string) => void;
  onToggleSelect?: (id: string) => void;
}

/** Human line per hard cap for the Limit badge tooltip. */
function limitTitle(limit: string, entry?: QuotaEntry): string {
  const bucket = entry?.data?.quotas?.find((b) => b.key === limit);
  const reset = bucket?.resetAt ? ` · resets ${new Date(bucket.resetAt).toLocaleString()}` : '';
  if (bucket) return `${bucket.label} limit reached${reset}`;
  if (limit === 'monthly') return `Monthly credits exhausted${reset}`;
  if (limit === '5-hour') return `5-hour limit reached${reset}`;
  return `${limit} limit reached`;
}

/** The single most severe signal; everything else lives in the inspector. */
function SignalChip({ conn }: { conn: Connection }) {
  if (!conn.isActive) return <StatusBadge tone="neutral">Disabled</StatusBadge>;
  if (isExpired(conn)) {
    return <StatusBadge tone="danger" icon={<AlertTriangle className="size-3" />}>Expired</StatusBadge>;
  }
  if (isRateLimited(conn)) {
    return (
      <StatusBadge tone="warning" icon={<Clock className="size-3" />} title="Rate-limited">
        {secsToHuman(rateLimitSecondsLeft(conn))}
      </StatusBadge>
    );
  }
  if (conn.lastError) {
    return (
      <StatusBadge tone="danger" icon={<AlertTriangle className="size-3" />} title={conn.lastError} className="max-w-[160px]">
        <span className="truncate">Error</span>
      </StatusBadge>
    );
  }
  if (hasBackoff(conn)) {
    return (
      <StatusBadge tone="warning" icon={<RefreshCw className="size-3" />} title="Backoff level">
        Backoff {conn.backoffLevel}/7
      </StatusBadge>
    );
  }
  const locks = lockedModelCount(conn);
  if (locks > 0) {
    return (
      <StatusBadge tone="warning" icon={<Lock className="size-3" />}>
        {locks} locked
      </StatusBadge>
    );
  }
  return null;
}

function AuthHint({ conn }: { conn: Connection }) {
  if (conn.apiKey || conn.authType === 'apikey') {
    return (
      <span className="inline-flex items-center gap-1" title="API key auth">
        <KeyRound className="size-3" /> API key
      </span>
    );
  }
  const secs = tokenSecondsLeft(conn);
  if (secs === null || secs <= 0) return null;
  return (
    <span className="inline-flex items-center gap-1 tabular-nums" title={`Token expires in ${secsToHuman(secs)}`}>
      <Timer className="size-3" /> {secsToHuman(secs)}
    </span>
  );
}

function QuotaDisplay({ conn, entry, fetching, nowMs }: { conn: Connection; entry?: QuotaEntry; fetching: boolean; nowMs: number }) {
  let inner: React.ReactNode;
  let title = '';

  if (conn.supportsQuota === false) {
    inner = <span className="text-xs text-muted-foreground">N/A</span>;
    title = 'Provider does not report quota';
  } else if (fetching && !entry) {
    inner = <Loader2 className="size-3.5 animate-spin text-muted-foreground" aria-label="Loading quota" />;
  } else if (!entry) {
    inner = <span className="text-xs text-muted-foreground">—</span>;
    title = 'Quota not loaded';
  } else if (entry.error) {
    inner = (
      <span className="inline-flex items-center gap-1 text-xs text-destructive">
        <AlertTriangle className="size-3.5" /> Failed
      </span>
    );
    title = entry.error;
  } else {
    const pct = primaryQuotaPct(entry);
    const limits = entry.data?.limits ?? [];
    if (pct === null) {
      const unlimited = (entry.data?.quotas?.length ?? 0) > 0 && entry.data!.quotas!.every((b) => b.unlimited);
      inner = unlimited ? <InfinityIcon className="size-4 text-muted-foreground" aria-label="Unlimited" /> : <span className="text-xs text-muted-foreground">—</span>;
      title = entry.data?.message || (unlimited ? 'Unlimited' : 'No quota data');
    } else {
      inner = (
        <>
          <QuotaBar pct={pct} className="w-16" />
          <span className="w-9 text-right font-mono text-xs tabular-nums">{pct}%</span>
          {limits.length > 0 && (
            <StatusBadge tone="danger" title={limits.map((l) => limitTitle(l, entry)).join('\n')} className="h-5 px-1.5">
              Cap
            </StatusBadge>
          )}
        </>
      );
      const parts = (entry.data?.quotas ?? []).map((b) =>
        b.unlimited
          ? `${b.label}: unlimited`
          : `${b.label}: ${b.used}/${b.total} (${Math.round(b.pct)}%)${b.resetAt ? ` · reset ${new Date(b.resetAt).toLocaleString()}` : ''}`,
      );
      parts.push(`Updated ${secsAgoHuman(Math.floor((nowMs - entry.fetchedAt) / 1000))}`);
      title = parts.join('\n');
    }
  }

  return (
    <span className="hidden w-[150px] shrink-0 items-center justify-end gap-2 sm:flex" title={title}>
      {inner}
    </span>
  );
}

export const ConnectionRow = memo(function ConnectionRow({
  conn,
  detail,
  selectionMode,
  selected,
  showProvider,
  quotaEntry,
  quotaFetching,
  nowMs,
  onOpen,
  onToggleSelect,
}: ConnectionRowProps) {
  const subtitle = conn.email || conn.baseUrl?.replace(/^https?:\/\//, '') || conn.authMethod || '';

  return (
    <div
      className={cn(
        'group flex h-full items-center gap-3 rounded-md px-3 transition-colors',
        detail ? 'bg-accent' : 'hover:bg-muted/60',
        selectionMode && selected && 'bg-primary/10',
      )}
    >
      {selectionMode && onToggleSelect && (
        <Checkbox
          checked={Boolean(selected)}
          onCheckedChange={() => onToggleSelect(conn.id)}
          aria-label={`Select ${conn.name}`}
        />
      )}
      <button
        type="button"
        onClick={() => (selectionMode && onToggleSelect ? onToggleSelect(conn.id) : onOpen(conn.id))}
        aria-current={detail ? 'true' : undefined}
        className="flex h-full min-w-0 flex-1 cursor-pointer items-center gap-3 rounded-md text-left focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
      >
        <span
          className={cn(
            'flex size-8 shrink-0 items-center justify-center overflow-hidden rounded-md border bg-background',
            !conn.isActive && 'opacity-50 grayscale',
          )}
        >
          <ProviderLogo provider={conn.provider} size={22} />
        </span>

        <span className="min-w-0 flex-1">
          <span className="flex items-center gap-2">
            <span className={cn('truncate text-sm font-medium', !conn.isActive && 'text-muted-foreground')}>{conn.name}</span>
            {showProvider && (
              <span className="shrink-0 rounded bg-muted px-1.5 py-0.5 text-xs text-muted-foreground">
                {getProviderLabel(conn.provider)}
              </span>
            )}
          </span>
          <span className="flex items-center gap-2 truncate text-xs text-muted-foreground">
            {subtitle && <span className="truncate">{subtitle}</span>}
            <AuthHint conn={conn} />
          </span>
        </span>

        <SignalChip conn={conn} />
        <QuotaDisplay conn={conn} entry={quotaEntry} fetching={Boolean(quotaFetching)} nowMs={nowMs} />
        <ChevronRight className="size-4 shrink-0 text-muted-foreground/60 transition-transform group-hover:translate-x-0.5 motion-reduce:transition-none" />
      </button>
    </div>
  );
});
