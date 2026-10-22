export type Tone = 'neutral' | 'success' | 'warning' | 'danger';

/** Quota usage → tone. One threshold table for rows, headers and inspector. */
export function quotaTone(pct: number): Tone {
  return pct >= 90 ? 'danger' : pct >= 70 ? 'warning' : 'success';
}

export function errMessage(e: unknown, fallback: string): string {
  return e instanceof Error && e.message ? e.message : fallback;
}
