import { useEffect } from 'react';
import {
  CommandDialog,
  CommandEmpty,
  CommandGroup,
  CommandInput,
  CommandItem,
  CommandList,
} from '@/components/ui/command';
import { ProviderLogoIcon } from '@/components/connections/helpers';
import { getProviderLabel } from '@/lib/provider-registry';
import { needsAttention } from '@/lib/connection-status';
import type { Connection } from '@/types/connections';

interface ConnectionPaletteProps {
  open: boolean;
  /** All connections regardless of current list filters — palette is the primary jump tool. */
  connections: Connection[];
  onPick: (id: string) => void;
  onOpenChange: (open: boolean) => void;
}

/**
 * Ctrl/Cmd+K quick jump over every connection, grouped by provider.
 * Search matches name, email, and model ids.
 */
export function ConnectionPalette({ open, connections, onPick, onOpenChange }: ConnectionPaletteProps) {
  // Register the global shortcut while mounted on the connections screen.
  useEffect(() => {
    const down = (e: KeyboardEvent) => {
      if (e.key === 'k' && (e.metaKey || e.ctrlKey)) {
        e.preventDefault();
        onOpenChange(!open);
      }
    };
    document.addEventListener('keydown', down);
    return () => document.removeEventListener('keydown', down);
  }, [open, onOpenChange]);

  const groups = new Map<string, Connection[]>();
  for (const c of connections) {
    const key = getProviderLabel(c.provider);
    if (!groups.has(key)) groups.set(key, []);
    groups.get(key)!.push(c);
  }

  return (
    <CommandDialog open={open} onOpenChange={onOpenChange}>
      <CommandInput placeholder="Jump to connection — name, email, model…" />
      <CommandList>
        <CommandEmpty>No connections found.</CommandEmpty>
        {[...groups.entries()].map(([label, items]) => (
          <CommandGroup key={label} heading={`${label} · ${items.length}`}>
            {(items.length > 120 ? items.slice(0, 120) : items).map((c) => (
              <CommandItem
                key={c.id}
                value={[c.name, c.email ?? '', c.provider, ...(c.supportedModels ?? [])].join(' ')}
                onSelect={() => {
                  onPick(c.id);
                  onOpenChange(false);
                }}
                className="gap-2"
              >
                <span className="flex h-5 w-5 shrink-0 items-center justify-center overflow-hidden rounded border bg-background">
                  <ProviderLogoIcon provider={c.provider} size={14} className="w-full object-cover" />
                </span>
                <span className="truncate font-medium">{c.name}</span>
                <span className="truncate text-xs text-muted-foreground">{c.email || c.baseUrl || ''}</span>
                {!c.isActive && <span className="ml-auto text-[10px] text-muted-foreground">idle</span>}
                {needsAttention(c) && (
                  <span className="ml-auto rounded bg-destructive/10 px-1 text-[10px] text-destructive">⚠</span>
                )}
              </CommandItem>
            ))}
          </CommandGroup>
        ))}
        {/* isAdmin intentionally unused today — reserved for tenant-scoped commands */}
      </CommandList>
    </CommandDialog>
  );
}
