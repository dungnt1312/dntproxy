import { AlertTriangle, Gauge, Link2, Lock, TimerReset } from 'lucide-react';
import { Card, CardContent } from '@/components/ui/card';
import { Progress } from '@/components/ui/progress';
import { cn } from '@/lib/utils';
import type { Connection } from '@/types/connections';

interface ConnectionHealthDashboardProps {
  connections: Connection[];
}

function isRateLimited(c: Connection) {
  return Boolean(c.rateLimitedUntil && new Date(c.rateLimitedUntil) > new Date());
}

function isExpired(c: Connection) {
  return Boolean(c.expiresAt && new Date(c.expiresAt) < new Date());
}

function hasCooldown(c: Connection) {
  return (c.backoffLevel ?? 0) > 0;
}

function hasError(c: Connection) {
  return Boolean(c.lastError);
}

function getHealthScore(connections: Connection[]) {
  if (connections.length === 0) return 100;
  const active = connections.filter((c) => c.isActive).length;
  const issueWeight = connections.reduce((sum, c) => {
    if (!c.isActive) return sum + 0.25;
    let penalty = 0;
    if (isExpired(c)) penalty += 1;
    if (isRateLimited(c)) penalty += 0.8;
    if (hasCooldown(c)) penalty += Math.min(0.6, (c.backoffLevel ?? 0) * 0.15);
    if (hasError(c)) penalty += 0.5;
    return sum + Math.min(1, penalty);
  }, 0);
  const availability = active / connections.length;
  const penalty = issueWeight / connections.length;
  return Math.max(0, Math.round((availability * 0.75 + (1 - penalty) * 0.25) * 100));
}

export function ConnectionHealthDashboard({ connections }: ConnectionHealthDashboardProps) {
  const total = connections.length;
  const active = connections.filter((c) => c.isActive).length;
  const rateLimited = connections.filter(isRateLimited).length;
  const expired = connections.filter(isExpired).length;
  const cooldown = connections.filter(hasCooldown).length;
  const errors = connections.filter(hasError).length;
  const detectedModels = connections.reduce((sum, c) => sum + (c.supportedModels?.length || 0), 0);
  const score = getHealthScore(connections);

  const scoreTone = score >= 85 ? 'text-emerald-600' : score >= 65 ? 'text-amber-600' : 'text-destructive';
  const progressTone = score >= 85 ? '[&>div]:bg-emerald-500' : score >= 65 ? '[&>div]:bg-amber-500' : '[&>div]:bg-destructive';

  return (
    <div className="grid grid-cols-1 gap-3 xl:grid-cols-[1.4fr_1fr_1fr_1fr]">
      <Card className="overflow-hidden border-primary/15 bg-gradient-to-br from-primary/5 via-card to-card">
        <CardContent className="p-4">
          <div className="flex items-start justify-between gap-4">
            <div>
              <div className="mb-1 flex items-center gap-2 text-xs font-medium text-muted-foreground">
                <Gauge className="h-3.5 w-3.5" /> Overall Health
              </div>
              <div className={cn('text-3xl font-bold tracking-tight', scoreTone)}>{score}%</div>
            </div>
            <div className="rounded-full border bg-background/70 px-2.5 py-1 text-xs text-muted-foreground">
              {active}/{total} active
            </div>
          </div>
          <Progress value={score} className={cn('mt-4 h-2', progressTone)} />
          <p className="mt-2 text-xs text-muted-foreground">
            Calculated from availability, rate limits, token expiry, cooldown/backoff, and recent errors.
          </p>
        </CardContent>
      </Card>

      <Card>
        <CardContent className="p-4">
          <div className="mb-1 flex items-center gap-2 text-xs text-muted-foreground">
            <Link2 className="h-3.5 w-3.5" /> Routing Capacity
          </div>
          <div className="text-2xl font-bold">{detectedModels}</div>
          <p className="text-xs text-muted-foreground">detected models across {total} connections</p>
        </CardContent>
      </Card>

      <Card className={rateLimited + cooldown > 0 ? 'border-amber-500/35' : ''}>
        <CardContent className="p-4">
          <div className="mb-1 flex items-center gap-2 text-xs text-muted-foreground">
            <TimerReset className="h-3.5 w-3.5" /> Throttling
          </div>
          <div className="flex items-baseline gap-3">
            <span className={cn('text-2xl font-bold', rateLimited + cooldown > 0 && 'text-amber-600')}>
              {rateLimited + cooldown}
            </span>
            <span className="text-xs text-muted-foreground">affected</span>
          </div>
          <p className="text-xs text-muted-foreground">{rateLimited} rate-limited · {cooldown} cooldown</p>
        </CardContent>
      </Card>

      <Card className={expired + errors > 0 ? 'border-destructive/35' : ''}>
        <CardContent className="p-4">
          <div className="mb-1 flex items-center gap-2 text-xs text-muted-foreground">
            {expired > 0 ? <Lock className="h-3.5 w-3.5" /> : <AlertTriangle className="h-3.5 w-3.5" />}
            Auth & Errors
          </div>
          <div className="flex items-baseline gap-3">
            <span className={cn('text-2xl font-bold', expired + errors > 0 && 'text-destructive')}>
              {expired + errors}
            </span>
            <span className="text-xs text-muted-foreground">issues</span>
          </div>
          <p className="text-xs text-muted-foreground">{expired} expired · {errors} with last error</p>
        </CardContent>
      </Card>
    </div>
  );
}
