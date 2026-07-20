import { ShieldCheck, Building2 } from "lucide-react";
import { cn } from "@/lib/utils";
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@/components/ui/tooltip";

interface TenantBadgeProps {
  /** Tenant id; empty string means legacy/global admin scope. */
  tenantId?: string;
  /** True when this is an admin (legacy/global) key with cross-tenant visibility. */
  isAdmin?: boolean;
  /** When true (sidebar collapsed), render as an icon-only button. */
  compact?: boolean;
  className?: string;
}

/**
 * TenantBadge shows the active tenant context resolved from the current API key.
 *
 * - Admin (legacy/global) keys render an "Admin" shield badge.
 * - Tenant-scoped keys render the tenant slug with a building icon.
 * - When no session is loaded yet, nothing is rendered.
 */
export function TenantBadge({
  tenantId,
  isAdmin,
  compact = false,
  className,
}: TenantBadgeProps) {
  // Don't render anything until we know the scope.
  if (!tenantId && !isAdmin) return null;

  const isAdminMode = isAdmin || !tenantId;
  const label = isAdminMode ? "Admin" : tenantId;

  if (compact) {
    return (
      <Tooltip delayDuration={0}>
        <TooltipTrigger asChild>
          <div
            className={cn(
              "flex h-8 w-8 items-center justify-center rounded-md border",
              isAdminMode
                ? "border-primary/30 bg-primary/10 text-primary"
                : "border-border bg-muted text-muted-foreground",
              className,
            )}
          >
            {isAdminMode ? (
              <ShieldCheck className="h-4 w-4" />
            ) : (
              <Building2 className="h-4 w-4" />
            )}
          </div>
        </TooltipTrigger>
        <TooltipContent side="right">
          {isAdminMode
            ? "Admin (all tenants)"
            : `Tenant: ${tenantId}`}
        </TooltipContent>
      </Tooltip>
    );
  }

  return (
    <div
      className={cn(
        "flex items-center gap-1.5 rounded-md border px-2 py-1 text-xs font-medium",
        isAdminMode
          ? "border-primary/30 bg-primary/10 text-primary"
          : "border-border bg-muted text-muted-foreground",
        className,
      )}
    >
      {isAdminMode ? (
        <ShieldCheck className="h-3.5 w-3.5 shrink-0" />
      ) : (
        <Building2 className="h-3.5 w-3.5 shrink-0" />
      )}
      <span className="truncate max-w-[120px]">{label}</span>
    </div>
  );
}
