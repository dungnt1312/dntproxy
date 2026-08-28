import { AlertTriangle, CalendarClock, Copy, ExternalLink, Link2, Lock, TerminalSquare } from 'lucide-react';
import { toast } from 'sonner';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { ScrollArea } from '@/components/ui/scroll-area';
import { Separator } from '@/components/ui/separator';
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
} from '@/components/ui/sheet';
import { ProviderLogoIcon } from '@/components/connections/helpers';
import { getProviderLabel } from '@/lib/provider-registry';
import { hasBackoff, isExpired, isRateLimited } from '@/lib/connection-status';
import type { Connection } from '@/types/connections';

interface ConnectionDetailSheetProps {
  connection: Connection | null;
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onEditModels: (connection: Connection) => void;
  onEditConnection: (connection: Connection) => void;
}

function copyText(text: string, label: string) {
  navigator.clipboard?.writeText(text);
  toast.success(`${label} copied`);
}

function Row({ label, value }: { label: string; value?: string | number | null }) {
  return (
    <div className="grid grid-cols-[110px_1fr] gap-3 text-sm">
      <span className="text-muted-foreground">{label}</span>
      <span className="min-w-0 break-all font-medium">{value || '—'}</span>
    </div>
  );
}

export function ConnectionDetailSheet({
  connection,
  open,
  onOpenChange,
  onEditModels,
  onEditConnection,
}: ConnectionDetailSheetProps) {
  const c = connection;
  if (!c) return null;

  const rateLimited = isRateLimited(c);
  const expired = isExpired(c);
  const cooldown = hasBackoff(c);
  const hasIssue = rateLimited || expired || cooldown || Boolean(c.lastError);

  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent className="w-full sm:max-w-xl p-0">
        <SheetHeader className="border-b p-5 pr-10">
          <div className="flex items-start gap-3">
            <div className="mt-0.5 flex h-10 w-10 shrink-0 items-center justify-center overflow-hidden rounded-lg border bg-muted">
              <ProviderLogoIcon provider={c.provider} size={36} className="h-full w-full object-cover" />
            </div>
            <div className="min-w-0">
              <SheetTitle className="truncate text-lg">{c.name}</SheetTitle>
              <SheetDescription>
                {getProviderLabel(c.provider)} · {c.email || c.baseUrl || c.authMethod || 'Connection'}
              </SheetDescription>
            </div>
          </div>
        </SheetHeader>

        <ScrollArea className="flex-1">
          <div className="space-y-5 p-5">
            <section className="space-y-3">
              <div className="flex flex-wrap items-center gap-2">
                <Badge variant="outline" className={c.isActive ? 'border-emerald-500/25 bg-emerald-500/10 text-emerald-600' : 'bg-muted text-muted-foreground'}>
                  {c.isActive ? 'Active' : 'Inactive'}
                </Badge>
                {rateLimited && <Badge variant="outline" className="border-amber-500/25 bg-amber-500/10 text-amber-600">Rate limited</Badge>}
                {expired && <Badge variant="outline" className="border-destructive/25 bg-destructive/10 text-destructive">Expired token</Badge>}
                {cooldown && <Badge variant="outline" className="border-amber-500/25 bg-amber-500/10 text-amber-600">Backoff L{c.backoffLevel}</Badge>}
                {c.supportsQuota && <Badge variant="secondary">Quota supported</Badge>}
              </div>

              {hasIssue && (
                <div className="rounded-lg border border-amber-500/25 bg-amber-500/10 p-3 text-sm">
                  <div className="mb-1 flex items-center gap-2 font-medium text-amber-700 dark:text-amber-300">
                    <AlertTriangle className="h-4 w-4" /> Attention needed
                  </div>
                  <p className="text-xs text-muted-foreground">
                    {c.lastError || 'This connection currently has rate-limit, token, or cooldown signals.'}
                  </p>
                </div>
              )}
            </section>

            <Separator />

            <section className="space-y-3">
              <h3 className="flex items-center gap-2 text-sm font-semibold"><Link2 className="h-4 w-4" /> Routing</h3>
              <Row label="Connection ID" value={c.id} />
              <Row label="Provider" value={getProviderLabel(c.provider)} />
              <Row label="Auth method" value={c.authMethod || c.authType} />
              <Row label="Route prefix" value={c.routePrefix || c.modelPrefix} />
              <div className="flex flex-wrap gap-2 pt-1">
                <Button variant="outline" size="sm" className="h-8 gap-2" onClick={() => copyText(c.id, 'Connection ID')}>
                  <Copy className="h-3.5 w-3.5" /> Copy ID
                </Button>
                <Button variant="outline" size="sm" className="h-8 gap-2" onClick={() => onEditModels(c)}>
                  <Lock className="h-3.5 w-3.5" /> Configure Models
                </Button>
                {(c.authType === 'apikey' || c.apiKey) && (
                  <Button variant="outline" size="sm" className="h-8 gap-2" onClick={() => onEditConnection(c)}>
                    <ExternalLink className="h-3.5 w-3.5" /> Edit
                  </Button>
                )}
              </div>
            </section>

            <Separator />

            <section className="space-y-3">
              <h3 className="flex items-center gap-2 text-sm font-semibold"><Lock className="h-4 w-4" /> Models</h3>
              {c.supportedModels?.length ? (
                <div className="flex max-h-48 flex-wrap gap-2 overflow-auto rounded-lg border bg-muted/25 p-3">
                  {c.supportedModels.map((model) => (
                    <code key={model} className="rounded border bg-background px-2 py-1 font-mono text-xs text-muted-foreground">
                      {model}
                    </code>
                  ))}
                </div>
              ) : (
                <p className="rounded-lg border bg-muted/25 p-3 text-sm text-muted-foreground">
                  No explicit model allow-list. This connection can serve models according to provider/default behavior.
                </p>
              )}
            </section>

            <Separator />

            <section className="space-y-3">
              <h3 className="flex items-center gap-2 text-sm font-semibold"><CalendarClock className="h-4 w-4" /> Runtime signals</h3>
              <Row label="Rate limited" value={c.rateLimitedUntil} />
              <Row label="Expires at" value={c.expiresAt} />
              <Row label="Backoff level" value={c.backoffLevel ?? 0} />
              <Row label="Model locks" value={Object.keys(c.modelLocks || {}).length} />
              {c.lastError && (
                <div className="rounded-lg border border-destructive/20 bg-destructive/10 p-3">
                  <div className="mb-1 flex items-center gap-2 text-sm font-medium text-destructive">
                    <TerminalSquare className="h-4 w-4" /> Last error
                  </div>
                  <p className="break-words text-xs text-muted-foreground">{c.lastError}</p>
                </div>
              )}
            </section>
          </div>
        </ScrollArea>
      </SheetContent>
    </Sheet>
  );
}
