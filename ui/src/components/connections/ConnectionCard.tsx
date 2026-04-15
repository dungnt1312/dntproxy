import { useState, useEffect } from "react";
import {
  TestTube,
  Settings2,
  MoreHorizontal,
  RefreshCw,
  Trash2,
  Loader2,
  Lock,
  TerminalSquare,
  Edit3,
} from "lucide-react";
import { api } from "../../api";
import InlineName from "./InlineName";
import { TokenBar, getProviderInfo } from "./helpers";
import QuotaPanel from "./QuotaPanel";
import LogsViewerModal from "./LogsViewerModal";
import {
  Card,
  CardContent,
  CardHeader,
  CardTitle,
  CardFooter,
} from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { Switch } from "@/components/ui/switch";
import { cn } from "@/lib/utils";

interface ConnectionCardProps {
  conn: any;
  initialQuotaResult?: any;
  onReload: () => void;
  onDelete: (id: string, name: string) => void;
  onEditModels: (conn: any) => void;
  onEditConnection?: (conn: any) => void;
}

export default function ConnectionCard({
  conn: c,
  initialQuotaResult,
  onReload,
  onDelete,
  onEditModels,
  onEditConnection,
}: ConnectionCardProps) {
  const [testResult, setTestResult] = useState<any>(null);
  const [quotaResult, setQuotaResult] = useState<any>(
    initialQuotaResult ?? null,
  );
  const [quotaLoading, setQuotaLoading] = useState(false);
  const [isQuotaOpen, setIsQuotaOpen] = useState(false);
  const [isLogOpen, setIsLogOpen] = useState(false);

  useEffect(() => {
    if (initialQuotaResult !== undefined) {
      setQuotaResult(initialQuotaResult);
    }
  }, [initialQuotaResult]);

  const isRL = c.rateLimitedUntil && new Date(c.rateLimitedUntil) > new Date();
  const isExpired = c.expiresAt && new Date(c.expiresAt) < new Date();
  const hasIssue = isRL || isExpired || c.backoffLevel > 0 || c.lastError;
  const providerInfo = getProviderInfo(c.provider);

  const handleTest = async () => {
    setTestResult({ loading: true });
    try {
      const res = await api.testConnection(c.id);
      setTestResult(res);
      // Reload connections to reflect updated status (e.g., token refreshed, errors cleared)
      onReload();
    } catch (e: any) {
      setTestResult({ status: "error", message: e.message });
      onReload();
    }
  };

  const handleCheckQuota = async (e?: React.MouseEvent) => {
    e?.stopPropagation();
    setQuotaLoading(true);
    try {
      const res = await api.getUsage(c.id);
      setQuotaResult(res);
    } catch (e: any) {
      setQuotaResult({ error: e.message });
    } finally {
      setQuotaLoading(false);
    }
  };

  const handleToggle = async () => {
    await api.updateConnection(c.id, { isActive: !c.isActive });
    onReload();
  };

  const handleRename = async (id: string, name: string) => {
    await api.updateConnection(id, { name });
    onReload();
  };

  const handleResetCooldown = async () => {
    try {
      await api.resetCooldown(c.id);
      onReload();
    } catch (e: any) {
      console.error(e.message);
    }
  };

  const renderBadgeStatus = () => {
    if (!c.isActive)
      return (
        <Badge
          variant="outline"
          className="bg-muted text-muted-foreground border-border"
        >
          Idle
        </Badge>
      );
    if (isRL)
      return (
        <Badge
          variant="outline"
          className="bg-amber-500/10 text-amber-600 dark:text-amber-400 border-amber-500/20"
        >
          Rate Limited
        </Badge>
      );
    if (hasIssue)
      return (
        <Badge
          variant="outline"
          className="bg-destructive/10 text-destructive dark:text-red-400 border-destructive/20 cursor-pointer hover:bg-destructive/20"
          onClick={() => setIsLogOpen(true)}
        >
          Error
        </Badge>
      );
    return (
      <Badge
        variant="outline"
        className="bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 border-emerald-500/20"
      >
        Active
      </Badge>
    );
  };

  return (
    <>
      <Card
        className={cn(
          "flex flex-col h-full transition-all duration-300 border bg-card hover:shadow-md hover:border-primary/20 p-0 gap-2",
          !c.isActive && "opacity-60 grayscale-[0.3]",
          hasIssue &&
            c.isActive &&
            "border-destructive/30 shadow-destructive/10",
        )}
      >
        <CardHeader className="p-3 pb-2 flex flex-row items-center justify-between gap-3 space-y-0 relative">
          <div className="flex items-center gap-3 min-w-0">
            <div className="shrink-0 rounded-md overflow-hidden flex shadow-sm">
              {providerInfo.icon}
            </div>
            <div className="min-w-0 flex flex-col">
              <CardTitle className="text-sm font-semibold truncate leading-tight">
                <InlineName conn={c} onRename={handleRename} />
              </CardTitle>
              <span className="text-[11px] text-muted-foreground truncate mt-0.5">
                {c.email ||
                  c.baseUrl?.replace("https://", "") ||
                  c.authMethod ||
                  "API Key"}
              </span>
            </div>
          </div>
          <div className="shrink-0">{renderBadgeStatus()}</div>
        </CardHeader>

        <CardContent className="flex-1 p-3 pt-1 flex flex-col gap-2">
          {/* Metrics / Quota Section */}
          <div className="flex flex-col gap-2">
            <div className="flex items-center gap-1.5 text-[11px] text-muted-foreground pl-0.5">
              <Lock className="h-3 w-3 shrink-0" />
              <TokenBar conn={c} />
              <span
                className="ml-auto font-medium text-muted-foreground hover:text-foreground cursor-pointer transition-colors underline decoration-dashed underline-offset-[3px]"
                onClick={(e) => {
                  e.stopPropagation();
                  onEditModels(c);
                }}
                title="Configure available models"
              >
                {c.supportedModels?.length
                  ? `${c.supportedModels.length} models`
                  : "All models"}
              </span>
            </div>

            {/* Inline Quota Display */}
            <div className="rounded-md bg-muted/40 p-2.5  flex flex-col justify-center w-full">
              {!c.isActive ? (
                <p className="text-[10px] text-muted-foreground italic">
                  Quota check unavailable when inactive.
                </p>
              ) : quotaResult || quotaLoading ? (
                <QuotaPanel
                  data={quotaResult}
                  loading={quotaLoading}
                  onRefresh={handleCheckQuota}
                />
              ) : (
                <div className="flex items-center justify-between">
                  <span className="text-[10px] text-muted-foreground italic">
                    Quota not loaded
                  </span>
                  <Button
                    variant="secondary"
                    size="sm"
                    className="h-5 text-[10px] px-2 font-medium"
                    onClick={handleCheckQuota}
                  >
                    Load
                  </Button>
                </div>
              )}
            </div>
          </div>
        </CardContent>

        {/* Thinner, cleaner footer */}
        <CardFooter className="mt-auto shrink-0 px-3 py-2.5 border-t border-border/30 bg-transparent flex items-center justify-between gap-2">
          <div className="flex items-center gap-2 min-w-0">
            <Button
              variant="secondary"
              size="sm"
              className="h-7 text-xs px-2.5 bg-secondary/50 text-secondary-foreground hover:bg-secondary border-none"
              onClick={handleTest}
              disabled={testResult?.loading}
              title="Test connection"
            >
              {testResult?.loading ? (
                <Loader2 className="h-3 w-3 animate-spin mr-1.5" />
              ) : (
                <TestTube className="h-3 w-3 mr-1.5" />
              )}
              Test
            </Button>

            {testResult && !testResult.loading && (
              <span
                className={cn(
                  "text-[10px] font-medium px-2 py-1 rounded-md truncate max-w-[80px]",
                  testResult.status === "ok"
                    ? "text-emerald-600 bg-emerald-500/10"
                    : "text-destructive bg-destructive/10",
                )}
              >
                {testResult.status === "ok" ? "✓ OK" : "✗ Fail"}
              </span>
            )}
          </div>

          <div className="flex items-center gap-1 shrink-0">
            <div
              className="flex items-center p-1.5 rounded-md hover:bg-muted/60 transition-colors cursor-pointer"
              onClick={handleToggle}
              title={c.isActive ? "Disable connection" : "Enable connection"}
            >
              <Switch
                checked={c.isActive}
                onCheckedChange={handleToggle}
                className="scale-75 data-[state=checked]:bg-emerald-500 pointer-events-none m-0"
              />
            </div>

            {/* Dropdown Action Menu */}
            <DropdownMenu>
              <DropdownMenuTrigger asChild>
                <Button
                  variant="ghost"
                  size="icon"
                  className="h-6 w-6 text-muted-foreground hover:text-foreground"
                >
                  <MoreHorizontal className="h-4 w-4" />
                </Button>
              </DropdownMenuTrigger>
              <DropdownMenuContent align="end" className="w-48 shadow-lg">
                {(c.authType === "apikey" || c.hasApiKey) &&
                  onEditConnection && (
                    <DropdownMenuItem
                      onClick={() => onEditConnection(c)}
                      className="gap-2 cursor-pointer text-xs py-2"
                    >
                      <Edit3 size={15} /> Edit Connection
                    </DropdownMenuItem>
                  )}
                <DropdownMenuItem
                  onClick={() => onEditModels(c)}
                  className="gap-2 cursor-pointer text-xs py-2"
                >
                  <Settings2 size={15} /> Configure Models
                </DropdownMenuItem>
                <DropdownMenuItem
                  onClick={() => setIsLogOpen(true)}
                  className="gap-2 cursor-pointer text-xs py-2 text-foreground"
                >
                  <TerminalSquare size={15} /> View System Logs
                </DropdownMenuItem>
                {(isRL || c.backoffLevel > 0) && (
                  <DropdownMenuItem
                    onClick={handleResetCooldown}
                    className="gap-2 cursor-pointer text-xs py-2 text-amber-600 focus:text-amber-700"
                  >
                    <RefreshCw size={15} /> Reset Cooldown
                  </DropdownMenuItem>
                )}
                <DropdownMenuSeparator />
                <DropdownMenuItem
                  onClick={() => onDelete(c.id, c.name)}
                  className="gap-2 text-xs py-2 text-destructive focus:text-destructive focus:bg-destructive/10 cursor-pointer"
                >
                  <Trash2 size={15} /> Delete Connection
                </DropdownMenuItem>
              </DropdownMenuContent>
            </DropdownMenu>
          </div>
        </CardFooter>
      </Card>

      {/* Reusable Log Modal Instance */}
      <LogsViewerModal
        isOpen={isLogOpen}
        onClose={() => setIsLogOpen(false)}
        title={`Logs: ${c.name || "Connection"}`}
        filter={{ connectionId: c.id }}
      />
    </>
  );
}
