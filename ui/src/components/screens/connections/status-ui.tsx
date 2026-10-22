import type { ReactNode } from 'react';
import { cn } from '@/lib/utils';
import { quotaTone, type Tone } from './status-tone';

/**
 * Shared status visuals for the connections screen. Every badge and quota bar
 * uses these semantic tones so list rows, group headers and the inspector
 * never drift apart (and stay AA-contrast in both themes).
 */

const BADGE_TONE: Record<Tone, string> = {
  neutral: 'border-border bg-muted/50 text-muted-foreground',
  success: 'border-success/30 bg-success/10 text-success',
  warning: 'border-warning/30 bg-warning/10 text-warning',
  danger: 'border-destructive/30 bg-destructive/10 text-destructive',
};

export function StatusBadge({
  tone,
  icon,
  children,
  title,
  className,
}: {
  tone: Tone;
  icon?: ReactNode;
  children: ReactNode;
  title?: string;
  className?: string;
}) {
  return (
    <span
      title={title}
      className={cn(
        'inline-flex h-6 shrink-0 items-center gap-1 rounded-md border px-2 text-xs font-medium whitespace-nowrap',
        BADGE_TONE[tone],
        className,
      )}
    >
      {icon}
      {children}
    </span>
  );
}

const BAR_TONE: Record<Tone, string> = {
  neutral: 'bg-muted-foreground/40',
  success: 'bg-success',
  warning: 'bg-warning',
  danger: 'bg-destructive',
};

export function QuotaBar({ pct, className }: { pct: number; className?: string }) {
  const clamped = Math.max(0, Math.min(100, Math.round(pct)));
  return (
    <span
      role="meter"
      aria-valuemin={0}
      aria-valuemax={100}
      aria-valuenow={clamped}
      aria-label="Quota used"
      className={cn('block h-1.5 overflow-hidden rounded-full bg-muted', className)}
    >
      <span className={cn('block h-full rounded-full', BAR_TONE[quotaTone(clamped)])} style={{ width: `${clamped}%` }} />
    </span>
  );
}
