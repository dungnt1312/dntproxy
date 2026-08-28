import type { Connection } from '@/types/connections';

/**
 * Single source of truth for connection runtime status.
 * Every badge, filter, sort, and score must derive from these helpers so the
 * list, chips, inspector, and health dashboard can never disagree.
 */

export type ConnectionStatusInput = Pick<
  Connection,
  'isActive' | 'rateLimitedUntil' | 'expiresAt' | 'backoffLevel' | 'lastError' | 'modelLocks'
>;

export function isRateLimited(c: ConnectionStatusInput): boolean {
  return Boolean(c.rateLimitedUntil && new Date(c.rateLimitedUntil) > new Date());
}

export function rateLimitSecondsLeft(c: ConnectionStatusInput): number {
  if (!c.rateLimitedUntil) return 0;
  return Math.ceil((new Date(c.rateLimitedUntil).getTime() - Date.now()) / 1000);
}

export function isExpired(c: ConnectionStatusInput): boolean {
  return Boolean(c.expiresAt && new Date(c.expiresAt) < new Date());
}

export function tokenSecondsLeft(c: ConnectionStatusInput): number | null {
  if (!c.expiresAt) return null;
  return Math.ceil((new Date(c.expiresAt).getTime() - Date.now()) / 1000);
}

export function hasBackoff(c: ConnectionStatusInput): boolean {
  return (c.backoffLevel ?? 0) > 0;
}

export function hasError(c: ConnectionStatusInput): boolean {
  return Boolean(c.lastError);
}

/** Active connection showing at least one problem signal. */
export function needsAttention(c: ConnectionStatusInput): boolean {
  if (!c.isActive) return false;
  return (
    isExpired(c) || isRateLimited(c) || hasBackoff(c) || hasError(c) || lockedModelCount(c) > 0
  );
}

export function lockedModelCount(c: ConnectionStatusInput): number {
  const locks = c.modelLocks ?? {};
  const now = Date.now();
  return Object.values(locks).filter((e) => new Date(e).getTime() > now).length;
}

/**
 * Severity order used for sorting everywhere: most urgent first.
 * expired(0) > rate-limited(1) > error(2) > backoff(3) > model locks(4)
 * > healthy(5) > inactive(6).
 */
export function attentionRank(c: ConnectionStatusInput): number {
  if (!c.isActive) return 6;
  if (isExpired(c)) return 0;
  if (isRateLimited(c)) return 1;
  if (hasError(c)) return 2;
  if (hasBackoff(c)) return 3;
  if (lockedModelCount(c) > 0) return 4;
  return 5;
}

const RANK_LABELS: Record<number, string> = {
  0: 'expired',
  1: 'rate-limited',
  2: 'error',
  3: 'backoff',
  4: 'model-locks',
};

/** Machine-readable reason keys behind a connection's "needs attention" state. */
export function attentionReasons(c: ConnectionStatusInput): string[] {
  const reasons: string[] = [];
  if (!c.isActive) return reasons;
  if (isExpired(c)) reasons.push('expired');
  if (isRateLimited(c)) reasons.push('rate-limited');
  if (hasError(c)) reasons.push('error');
  if (hasBackoff(c)) reasons.push('backoff');
  if (lockedModelCount(c) > 0) reasons.push(RANK_LABELS[4]);
  return reasons;
}

export type HealthBucket = 'attention' | 'healthy' | 'inactive';

export function healthBucket(c: ConnectionStatusInput): HealthBucket {
  if (!c.isActive) return 'inactive';
  return needsAttention(c) ? 'attention' : 'healthy';
}

/** Compare two connections by severity then name — pass straight to sort(). */
export function compareByAttention(a: Connection, b: Connection): number {
  const rank = attentionRank(a) - attentionRank(b);
  return rank !== 0 ? rank : a.name.localeCompare(b.name, undefined, { sensitivity: 'base' });
}

/**
 * Fleet health score in [0,100]: availability weighted 75%, issue penalty 25%.
 * Moved here from connection-health-dashboard so chips/inspector can reuse it.
 */
export function computeHealthScore(connections: Array<ConnectionStatusInput>): number {
  if (connections.length === 0) return 100;
  const active = connections.filter((c) => c.isActive).length;
  const issueWeight = connections.reduce((sum, c) => {
    if (!c.isActive) return sum + 0.25;
    let penalty = 0;
    if (isExpired(c)) penalty += 1;
    if (isRateLimited(c)) penalty += 0.8;
    if (hasBackoff(c)) penalty += Math.min(0.6, (c.backoffLevel ?? 0) * 0.15);
    if (hasError(c)) penalty += 0.5;
    return sum + Math.min(1, penalty);
  }, 0);
  const availability = active / connections.length;
  const penalty = issueWeight / connections.length;
  return Math.max(0, Math.round((availability * 0.75 + (1 - penalty) * 0.25) * 100));
}

// ─── Time formatting ──────────────────────────────────────────────────────────

export function secsToHuman(secs: number): string {
  if (secs <= 0) return '0s';
  if (secs < 60) return `${secs}s`;
  if (secs < 3600) return `${Math.floor(secs / 60)}m`;
  if (secs < 3600 * 24) return `${Math.floor(secs / 3600)}h ${Math.floor((secs % 3600) / 60)}m`;
  return `${Math.floor(secs / (3600 * 24))}d ${Math.floor((secs % (3600 * 24)) / 3600)}h`;
}

/** Compact "fetched N ago" label for quota freshness badges. */
export function secsAgoHuman(secs: number): string {
  if (secs < 5) return 'just now';
  if (secs < 60) return `${Math.floor(secs)}s ago`;
  if (secs < 3600) return `${Math.floor(secs / 60)}m ago`;
  if (secs < 3600 * 24) return `${Math.floor(secs / 3600)}h ago`;
  return `${Math.floor(secs / (3600 * 24))}d ago`;
}
