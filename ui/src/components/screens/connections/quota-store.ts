import { useEffect, useRef, useState, useSyncExternalStore } from 'react';
import { api } from '@/api';
import { needsAttention } from '@/lib/connection-status';
import type { Connection, UsageData } from '@/types/connections';

/**
 * Central quota cache + rate-limited fetch governor.
 *
 * Quota data is cached with an age label so display is always cheap, while
 * provider calls stay explicit and controlled:
 *  - manual:        "Load quotas" on a provider header / inspector refresh /
 *                   bulk action are the ONLY ways quota is fetched by default
 *  - watched tier:  with Quota-auto ON, the inspector-open connection polls slowly (45s)
 *  - hot tier:      with Quota-auto ON, near-limit/attention connections poll faster (2m)
 * All requests flow through one queue capped at MAX_CONCURRENT with failure
 * backoff; automatic tiers pause while the tab is hidden.
 */

export interface QuotaEntry {
  data: UsageData | null;
  error?: string;
  fetchedAt: number;
}

const WATCH_MS = 45_000;
const HOT_MS = 2 * 60_000;
const SWEEP_MS = 15_000;
const MAX_CONCURRENT = 3;
const FAIL_BASE_MS = 30_000;
const FAIL_MAX_MS = 10 * 60_000;

/** Highest non-unlimited bucket usage in [0,100]; 0 when there is no bucket data. */
export function worstUsedPct(data: UsageData | null): number {
  let worst = 0;
  for (const b of data?.quotas ?? []) {
    if (!b.unlimited && b.pct > worst) worst = b.pct;
  }
  return Math.min(100, Math.max(0, worst));
}

/**
 * The pct a list row should display: the provider-marked primary bucket (the
 * operationally relevant window, e.g. Command Code's 5-hour), falling back to
 * the worst bucket. Null when the entry has no known quota buckets.
 */
export function primaryQuotaPct(entry: QuotaEntry | undefined): number | null {
  const buckets = entry?.data?.quotas ?? [];
  const known = buckets.filter((b) => !b.unlimited);
  if (known.length === 0) return null;
  const primary = known.find((b) => b.primary);
  return primary ? primary.pct : worstUsedPct(entry?.data ?? null);
}

/**
 * The pct aggregation should treat a connection as: a hard-capped account
 * (weekly/monthly/5-hour limit flagged by the provider) counts as fully used
 * even when its displayed window bucket still looks healthy.
 */
export function effectiveQuotaPct(entry: QuotaEntry | undefined): number | null {
  const pct = primaryQuotaPct(entry);
  if (pct === null) return null;
  return (entry?.data?.limits?.length ?? 0) > 0 ? 100 : pct;
}

class QuotaStore {
  private entries = new Map<string, QuotaEntry>();
  private listeners = new Set<() => void>();
  private version = 0;
  private hiQueue: string[] = [];
  private inflight = new Set<string>();
  private failCount = new Map<string, number>();
  private nextOkAt = new Map<string, number>();
  private inspectedId: string | null = null;
  private autoEnabled = false;
  private timer: ReturnType<typeof setInterval> | null = null;
  /** Latest connections snapshot pushed by the screen — used for tier classification. */
  private connsProvider: (() => Connection[]) | null = null;

  subscribe = (listener: () => void): (() => void) => {
    this.listeners.add(listener);
    return () => this.listeners.delete(listener);
  };

  getVersion = (): number => this.version;

  private emit() {
    this.version++;
    this.listeners.forEach((l) => l());
  }

  setConnsProvider(provider: () => Connection[]) {
    this.connsProvider = provider;
  }

  peek(id: string | null | undefined): QuotaEntry | undefined {
    return id ? this.entries.get(id) : undefined;
  }

  isFetching(id: string): boolean {
    return this.inflight.has(id);
  }

  setInspected(id: string | null) {
    this.inspectedId = id;
    // No fetch here: opening a connection must never move the viewport.
    // Quota loads on explicit refresh, or via the Quota-auto watch tier.
  }

  /** Rows entering the viewport do NOT trigger fetches — quota is load-on-request. */

  /** Manual refresh: force-bypasses staleness and failure backoff. */
  refresh(ids: string[]) {
    ids.forEach((id) => this.queueFetch(id, true));
    this.pump();
  }

  setAuto(on: boolean) {
    this.autoEnabled = on;
    if (on) {
      if (!this.timer) {
        this.timer = setInterval(() => this.tick(), SWEEP_MS);
      }
      this.tick();
    } else if (this.timer) {
      clearInterval(this.timer);
      this.timer = null;
    }
  }

  dispose() {
    if (this.timer) clearInterval(this.timer);
    this.timer = null;
  }

  private tick() {
    if (!this.autoEnabled || (typeof document !== 'undefined' && document.hidden)) return;
    this.sweepTiers();
    this.pump();
  }

  /** Enqueue watch/hot tiers that are due. */
  private sweepTiers() {
    const now = Date.now();
    const conns = this.connsProvider?.() ?? [];
    for (const c of conns) {
      if (!c.isActive || c.supportsQuota === false) continue;
      const entry = this.entries.get(c.id);
      const age = entry ? now - entry.fetchedAt : Infinity;
      if (c.id === this.inspectedId) {
        if (age >= WATCH_MS) this.queueFetch(c.id, false);
        continue;
      }
      const hot = worstUsedPct(entry?.data ?? null) >= 70 || needsAttention(c);
      if (hot && age >= HOT_MS) this.queueFetch(c.id, false);
    }
  }

  private queueFetch(id: string, force: boolean) {
    if (!force && (this.nextOkAt.get(id) ?? 0) > Date.now()) return;
    if (this.inflight.has(id)) return;
    this.pushUnique(this.hiQueue, id);
  }

  private pushUnique(queue: string[], id: string) {
    if (!queue.includes(id)) queue.push(id);
  }

  private pump() {
    while (this.inflight.size < MAX_CONCURRENT) {
      const id = this.hiQueue.shift();
      if (!id) break;
      if (this.inflight.has(id)) continue;
      // Skip only connections known to be ineligible. A connection missing from
      // the snapshot (e.g. created a moment ago, before the list reloaded) is
      // still fetched — the request was explicit.
      const conn = this.connsProvider?.().find((c) => c.id === id);
      if (conn && (conn.supportsQuota === false || !conn.isActive)) continue;
      this.inflight.add(id);
      api
        .getUsage(id)
        .then((data) => {
          this.entries.set(id, { data, fetchedAt: Date.now() });
          this.failCount.delete(id);
          this.nextOkAt.delete(id);
        })
        .catch((e: unknown) => {
          const message = e instanceof Error ? e.message : 'Failed to load quota';
          this.entries.set(id, { data: null, error: message, fetchedAt: Date.now() });
          const fails = (this.failCount.get(id) ?? 0) + 1;
          this.failCount.set(id, fails);
          const backoff = Math.min(FAIL_MAX_MS, FAIL_BASE_MS * 4 ** (fails - 1));
          this.nextOkAt.set(id, Date.now() + backoff);
        })
        .finally(() => {
          this.inflight.delete(id);
          this.emit();
          this.pump();
        });
    }
  }
}

export const quotaStore = new QuotaStore();

// ─── React bindings ───────────────────────────────────────────────────────────

/** Re-renders the component whenever any quota data changes. */
export function useQuotaVersion(): number {
  return useSyncExternalStore(quotaStore.subscribe, quotaStore.getVersion);
}

export function useQuotaEntry(id: string | null | undefined): QuotaEntry | undefined {
  useQuotaVersion();
  return quotaStore.peek(id);
}

/** Ticking clock so age labels stay honest without tight rerenders. */
export function useNow(intervalMs = 30_000): number {
  const [now, setNow] = useState(() => Date.now());
  useEffect(() => {
    const t = setInterval(() => setNow(Date.now()), intervalMs);
    return () => clearInterval(t);
  }, [intervalMs]);
  return now;
}

/** Convenience hook: keep the store's connection snapshot in sync each mount/update. */
export function useQuotaConnsSync(conns: Connection[]) {
  const latest = useRef(conns);
  useEffect(() => {
    latest.current = conns;
  });
  useEffect(() => {
    quotaStore.setConnsProvider(() => latest.current);
    return () => quotaStore.setConnsProvider(() => []);
  }, []);
}
