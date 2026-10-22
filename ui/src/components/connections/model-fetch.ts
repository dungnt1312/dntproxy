import { api } from '@/api';

/**
 * Shared "load the provider's model list" logic. The Edit-models dialog and the
 * post-create Verify panel use it, so loading means the same thing in both.
 * Loading only reads: nothing is stored until the user saves.
 */

/** Providers whose `fetch-models` endpoint asks the provider, not just config defaults. */
const LIVE_FETCH_PROVIDERS = new Set(['openai', 'openai-compatible', 'kiro']);

export const supportsLiveModelFetch = (provider: string): boolean => LIVE_FETCH_PROVIDERS.has(provider);

export interface FetchedModels {
  /** Model ids exactly as the provider reported them (no route prefix). */
  models: string[];
  /** False when the backend could only return its built-in defaults. */
  live: boolean;
  /** Backend explanation, e.g. why a live fetch fell back to defaults. */
  note?: string;
}

interface FetchResponse {
  models?: unknown;
  source?: string;
  note?: string;
}

/** Calls `fetch-models` and normalises the answer. Throws on transport/API errors. */
export async function fetchModelList(connectionId: string): Promise<FetchedModels> {
  const res = (await api.fetchConnectionModels(connectionId)) as FetchResponse | null;
  const models = Array.isArray(res?.models) ? res.models.filter((m): m is string => typeof m === 'string') : [];
  return { models, live: res?.source !== 'provider-config', note: res?.note };
}

export interface ModelDiff {
  added: string[];
  removed: string[];
}

/** What replacing `current` with `next` changes. */
export function diffModels(current: string[], next: string[]): ModelDiff {
  const cur = new Set(current);
  const nxt = new Set(next);
  return {
    added: next.filter((m) => !cur.has(m)),
    removed: current.filter((m) => !nxt.has(m)),
  };
}

/** "3 models" / "1 model". */
export const modelCount = (n: number): string => `${n} model${n === 1 ? '' : 's'}`;

/** One-line summary shown after loading, e.g. "Loaded 12 models · 3 new · 1 no longer offered." */
export function loadedSummary(total: number, diff: ModelDiff, hadSelection: boolean): string {
  const base = `Loaded ${modelCount(total)} from the provider`;
  if (!hadSelection) return `${base}. Review the list, then Save.`;
  if (diff.added.length === 0 && diff.removed.length === 0) return `${base} — same as your current list.`;
  const parts = [`${diff.added.length} new`, `${diff.removed.length} no longer offered`];
  return `${base} · ${parts.join(' · ')}. Review the list, then Save.`;
}
