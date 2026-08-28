import { memo } from 'react';
import { AlertTriangle, Check, ChevronRight, Clock, Infinity as InfinityIcon, Loader2, RefreshCw } from 'lucide-react';
import { cn } from '@/lib/utils';
import { Badge } from '@/components/ui/badge';
import { ProviderLogo } from '@/components/connections/ProviderLogo';
import {
  isExpired,
  isRateLimited,
  rateLimitSecondsLeft,
  hasBackoff,
  lockedModelCount,
  secsToHuman,
  tokenSecondsLeft,
} from '@/lib/connection-status';
import { getProviderLabel } from '@/lib/provider-registry';
import type { QuotaEntry } from './quota-store';
import type { Connection } from '@/types/connections';

export const ROW_HEIGHT = 44;

interface ConnectionRowProps {
  conn: Connection;
  detail?: boolean;
  selectionMode?: boolean;
  selected?: boolean;
  /** In flat view each row shows its provider tag; grouped view doesn't need it. */
  showProvider?: boolean;
  legacyCount?: number;
  quotaEntry?: QuotaEntry;
  quotaFetching?: boolean;
  /** Wall-clock snapshot from the container, used for freshness titles. */
  nowMs: number;
  onOpen: (id: string) => void;
  onToggleSelect?: (id: string) => void;
}

function worstPct(entry?: QuotaEntry): number | null {
  let worst = 0;
  let any = false;
  for (const b of entry?.data?.quotas ?? []) {
    any = true;
    if (!b.unlimited && b.pct > worst) worst = b.pct;
  }
  return any ? Math.min(100, Math.max(0, worst)) : null;
}

function quotaBarColor(pct: number): string {
  return pct >= 90 ? 'bg-red-500' : pct >= 70 ? 'bg-amber-500' : 'bg-emerald-500';
}

function StatusDot({ conn }: { conn: Connection }) {
  const tone = !conn.isActive
    ? 'bg-muted-foreground/40'
    : isExpired(conn)
      ? 'bg-red-500'
      : isRateLimited(conn)
        ? 'bg-amber-500 animate-pulse'
        : conn.lastError || hasBackoff(conn) || lockedModelCount(conn) > 0
          ? 'bg-amber-400'
          : 'bg-emerald-500';
  return <span className={cn('inline-block h-2 w-2 shrink-0 rounded-full', tone)} aria-hidden="true" />;
}

/** The single most severe signal, everything else lives in the inspector/tooltip. */
function SignalChip({ conn }: { conn: Connection }) {
  if (!conn.isActive) {
    return (
      <Badge variant="outline" className="h-5 gap-1 border-border bg-muted/40 px-1.5 text-[10px] text-muted-foreground">
        Idle
      </Badge>
    );
  }
  if (isExpired(conn)) {
    return (
      <Badge variant="outline" className="h-5 gap-1 border-destructive/30 bg-destructive/10 px-1.5 text-[10px] text-destructive">
        Expired
      </Badge>
    );
  }
  if (isRateLimited(conn)) {
    const secs = rateLimitSecondsLeft(conn);
    return (
      <Badge variant="outline" className="h-5 gap-1 border-amber-500/30 bg-amber-500/10 px-1.5 text-[10px] text-amber-600">
        <Clock size={9} /> RL {secsToHuman(secs)}
      </Badge>
    );
  }
  if (conn.lastError) {
    return (
      <Badge variant="outline" className="h-5 max-w-[130px] gap-1 border-destructive/30 bg-destructive/10 px-1.5 text-[10px] text-destructive">
        <AlertTriangle size={9} /> <span className="truncate">{conn.lastError}</span>
      </Badge>
    );
  }
  if (hasBackoff(conn)) {
    return (
      <Badge variant="outline" className="h-5 gap-1 border-amber-500/30 bg-amber-500/10 px-1.5 text-[10px] text-amber-600">
        <RefreshCw size={9} /> L{conn.backoffLevel}/7
      </Badge>
    );
  }
  const locks = lockedModelCount(conn);
  if (locks > 0) {
    return (
      <Badge variant="outline" className="h-5 gap-1 border-amber-500/30 bg-amber-500/10 px-1.5 text-[10px] text-amber-600">
        {locks} locked
      </Badge>
    );
  }
  return null;
}

function TokenChip({ conn }: { conn: Connection }) {
  if (conn.apiKey) {
    return (
      <span title="API key auth" className="shrink-0">
        <Badge variant="outline" className="h-5 gap-1 border-border bg-muted/40 px-1.5 text-[10px] text-muted-foreground">
          Key
        </Badge>
      </span>
    );
  }
  const secs = tokenSecondsLeft(conn);
  if (secs === null) return null;
  if (secs <= 0) return null; // already covered by the SignalChip expired badge
  return (
    <span title={`Token expires in ${secsToHuman(secs)}`} className="shrink-0">
      <Badge variant="outline" className="h-5 gap-1 border-border bg-muted/40 px-1.5 text-[10px] text-muted-foreground tabular-nums">
        ⏳ {secsToHuman(secs)}
      </Badge>
    </span>
  );
}

function QuotaDisplay({ conn, entry, fetching, nowMs }: { conn: Connection; entry?: QuotaEntry; fetching: boolean; nowMs: number }) {
  const supports = conn.supportsQuota !== false;
  if (!supports) {
    return <span className="w-[110px] shrink-0 text-right text-[11px] text-muted-foreground/40">—</span>;
  }

  let inner: React.ReactNode;
  if (fetching && !entry) {
    inner = <Loader2 size={12} className="animate-spin text-muted-foreground" />;
  } else if (!entry) {
    inner = <span className="text-[11px] text-muted-foreground/50">—</span>;
  } else if (entry.error) {
    inner = (
      <span title={entry.error} className="flex items-center gap-1 text-[11px] text-destructive">
        <AlertTriangle size={11} />
      </span>
    );
  } else if ((entry.data?.quotas?.length ?? 0) === 0) {
    const allUnlimited = entry.data && (entry.data.message ?? '').length > 0;
    inner = (
      <span
        title={entry.data?.message || 'No quota data'}
        className="text-[11px] text-muted-foreground/60"
      >
        {allUnlimited || (entry.data?.quotas?.every((b) => b.unlimited) ?? false) ? <InfinityIcon size={12} /> : '—'}
      </span>
    );
  } else {
    const pct = worstPct(entry);
    inner =
      pct === null ? (
        <span className="text-[11px] text-muted-foreground/50">—</span>
      ) : (
        <>
          <div className="h-1.5 w-14 overflow-hidden rounded-full bg-muted">
            <div className={cn('h-full rounded-full transition-all', quotaBarColor(pct))} style={{ width: `${pct}%` }} />
          </div>
          <span className="font-mono text-[11px] tabular-nums text-foreground/80">{pct}%</span>
        </>
      );
  }

  const ageSecs = entry ? Math.floor((nowMs - entry.fetchedAt) / 1000) : null;
  const titleParts: string[] = [];
  if (entry?.data) {
    for (const b of entry.data.quotas ?? []) {
      titleParts.push(
        b.unlimited
          ? `${b.label}: unlimited`
          : `${b.label}: ${b.used}/${b.total} (${Math.round(b.pct)}%)${b.resetAt ? ` · reset ${new Date(b.resetAt).toLocaleString()}` : ''}`,
      );
    }
  }
  if (entry && ageSecs !== null) titleParts.push(`Updated ${ageSecs}s ago`);
  if (entry?.error) titleParts.unshift(entry.error);

  return (
    <span className="flex h-full w-[132px] shrink-0 items-center justify-end gap-1.5" title={titleParts.join('\n')}>
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
  legacyCount,
  quotaEntry,
  quotaFetching,
  nowMs,
  onOpen,
  onToggleSelect,
}: ConnectionRowProps) {
  const handleOpen = () => {
    if (selectionMode) onToggleSelect?.(conn.id);
    else onOpen(conn.id);
  };

  return (
    <button
      type="button"
      onClick={handleOpen}
      aria-current={detail ? 'true' : undefined}
      className={cn(
        'group flex h-full w-full cursor-pointer select-none items-center gap-2 px-3 pr-2 text-left transition-colors',
        detail ? 'bg-accent' : 'hover:bg-muted/60',
        selectionMode && selected && 'bg-primary/10',
        !conn.isActive && 'opacity-70',
      )}
    >
      {selectionMode && onToggleSelect && (
        <span
          role="checkbox"
          aria-checked={selected}
          aria-label={`Select ${conn.name}`}
          onClick={(e) => {
            e.stopPropagation();
            onToggleSelect(conn.id);
          }}
          onKeyDown={(e) => {
            if (e.key === ' ' || e.key === 'Enter') {
              e.preventDefault();
              e.stopPropagation();
              onToggleSelect(conn.id);
            }
          }}
          tabIndex={0}
          className={cn(
            'flex h-4 w-4 shrink-0 items-center justify-center rounded border transition-colors',
            selected ? 'border-primary bg-primary text-primary-foreground' : 'border-input bg-background hover:border-primary/50',
          )}
        >
          {selected && <Check className="h-3 w-3" />}
        </span>
      )}

      <StatusDot conn={conn} />

      <span className="flex h-6 w-6 shrink-0 items-center justify-center overflow-hidden rounded-md border bg-background">
        <ProviderLogo provider={conn.provider} size={18} />
      </span>

      <span className="min-w-0 flex-1">
        <span className="flex items-center gap-1.5">
          <span className={cn('truncate text-sm font-medium leading-tight', detail && 'font-semibold')}>
            {conn.name}
          </span>
          {showProvider && (
            <span className="shrink-0 rounded bg-muted px-1 py-px text-[9px] uppercase tracking-wide text-muted-foreground">
              {getProviderLabel(conn.provider)}
            </span>
          )}
          {!!legacyCount && legacyCount > 0 && (
            <span
              title={`${legacyCount} model(s) missing from the current catalog (possibly outdated)`}
              className="flex shrink-0 items-center gap-0.5 rounded bg-amber-500/15 px-1 text-[9px] font-medium text-amber-600"
            >
              <AlertTriangle size={8} /> legacy ×{legacyCount}
            </span>
          )}
        </span>
        <span className="block truncate text-[11px] leading-tight text-muted-foreground">
          {conn.email || conn.baseUrl?.replace(/^https?:\/\//, '') || conn.authMethod || '—'}
        </span>
      </span>

      <TokenChip conn={conn} />
      <SignalChip conn={conn} />
      <QuotaDisplay conn={conn} entry={quotaEntry} fetching={Boolean(quotaFetching)} nowMs={nowMs} />
      <ChevronRight size={13} className="shrink-0 text-muted-foreground/40 transition-transform group-hover:translate-x-0.5" />
    </button>
  );
});
