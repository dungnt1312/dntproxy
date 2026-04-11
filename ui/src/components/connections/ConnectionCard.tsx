import { useState, useEffect } from 'react'
import {
  TestTube, Settings2, MoreHorizontal, RefreshCw,
  Trash2, BarChart2, Loader2, Play, ChevronRight, AlertTriangle, Clock, Lock
} from 'lucide-react'
import { api } from '../../api'
import { getModelName } from '../../models-config'
import InlineName from './InlineName'
import { TokenBar, QuotaPanel, getProviderInfo, secsToHuman } from './helpers'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { Switch } from '@/components/ui/switch'
import { cn } from '@/lib/utils'

interface ConnectionCardProps {
  conn: any
  initialQuotaResult?: any
  onReload: () => void
  onDelete: (id: string, name: string) => void
  onEditModels: (conn: any) => void
}

export default function ConnectionCard({ conn: c, initialQuotaResult, onReload, onDelete, onEditModels }: ConnectionCardProps) {
  const [testResult, setTestResult] = useState<any>(null)
  const [quotaResult, setQuotaResult] = useState<any>(initialQuotaResult ?? null)
  const [quotaLoading, setQuotaLoading] = useState(false)
  const [modelTestResults, setModelTestResults] = useState<Record<string, { status: 'ok' | 'error' | 'loading'; message?: string }>>({})

  useEffect(() => {
    let mounted = true;
    if (initialQuotaResult !== undefined) {
      setQuotaResult(initialQuotaResult)
    } else if (c.isActive) {
      // Auto-load quota on mount if active
      const check = async () => {
        setQuotaLoading(true)
        try {
          const res = await api.getUsage(c.id)
          if (mounted) setQuotaResult(res)
        } catch (e: any) { 
          if (mounted) setQuotaResult({ error: e.message }) 
        } finally { 
          if (mounted) setQuotaLoading(false) 
        }
      }
      check()
    }
    return () => { mounted = false; }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  const isRL = c.rateLimitedUntil && new Date(c.rateLimitedUntil) > new Date()
  const isExpired = c.expiresAt && new Date(c.expiresAt) < new Date()
  const hasIssue = isRL || isExpired || c.backoffLevel > 0 || c.lastError
  const lockCount = c.modelLocks
    ? Object.values(c.modelLocks).filter((e: any) => new Date(e) > new Date()).length
    : 0
  const rlSecs = isRL ? Math.ceil((new Date(c.rateLimitedUntil).getTime() - Date.now()) / 1000) : 0
  const providerInfo = getProviderInfo(c.provider)

  const handleTest = async () => {
    setTestResult({ loading: true })
    try {
      const res = await api.testConnection(c.id)
      setTestResult(res)
    } catch (e: any) { setTestResult({ status: 'error', message: e.message }) }
  }

  const handleTestModel = async (model: string) => {
    setModelTestResults(prev => ({ ...prev, [model]: { status: 'loading' } }))
    try {
      const res = await api.testModel(c.id, model)
      setModelTestResults(prev => ({ ...prev, [model]: { status: res.status === 'ok' ? 'ok' : 'error', message: res.message } }))
    } catch (e: any) {
      setModelTestResults(prev => ({ ...prev, [model]: { status: 'error', message: e.message } }))
    }
  }

  const handleCheckQuota = async () => {
    setQuotaLoading(true)
    try {
      const res = await api.getUsage(c.id)
      setQuotaResult(res)
    } catch (e: any) { setQuotaResult({ error: e.message }) }
    finally { setQuotaLoading(false) }
  }

  const handleToggle = async () => {
    await api.updateConnection(c.id, { isActive: !c.isActive })
    onReload()
  }

  const handleRename = async (id: string, name: string) => {
    await api.updateConnection(id, { name })
    onReload()
  }

  const handleResetCooldown = async () => {
    try { await api.resetCooldown(c.id); onReload() } catch (e: any) { console.error(e.message) }
  }

  return (
    <Card className={cn('transition-all hover:shadow-md', !c.isActive && 'opacity-60', hasIssue && c.isActive && 'border-amber-500/40')}>
      <CardHeader className="p-3 pb-2">
        <div className="flex items-start justify-between gap-2">
          <div className="flex items-center gap-2.5 min-w-0">
            <div className={cn('flex h-9 w-9 shrink-0 items-center justify-center rounded-lg border', providerInfo.colorClass)}>
              {providerInfo.icon}
            </div>
            <div className="min-w-0 space-y-0.5">
              <CardTitle className="text-sm font-semibold truncate leading-none mt-0.5">
                <InlineName conn={c} onRename={handleRename} />
              </CardTitle>
              <span className="text-xs text-muted-foreground truncate block">
                {c.email || c.baseUrl?.replace('https://', '') || c.authMethod || 'API Key'}
              </span>
            </div>
          </div>
          <Badge 
            variant="outline" 
            className={cn('shrink-0', 
              !c.isActive ? 'bg-muted text-muted-foreground border-border' : 
              isRL ? 'bg-amber-500/15 text-amber-700 dark:text-amber-400 border-amber-500/30' : 
              hasIssue ? 'bg-destructive/15 text-destructive dark:text-red-400 border-destructive/30' : 
              'bg-emerald-500/15 text-emerald-700 dark:text-emerald-400 border-emerald-500/30'
            )}
          >
            {!c.isActive ? 'Idle' : isRL ? 'Rate Limited' : hasIssue ? 'Error' : 'Active'}
          </Badge>
        </div>
      </CardHeader>

      <CardContent className="p-3 pt-0 space-y-3">
        {/* Models summary row */}
        <div className="flex items-center gap-2 text-xs text-muted-foreground">
          <Lock className="h-3.5 w-3.5" />
          <TokenBar conn={c} />
          {c.supportedModels?.length ? (
            <Badge variant="secondary" className="ml-auto text-xs px-1.5 cursor-pointer hover:bg-muted-foreground/20" onClick={() => onEditModels(c)}>
              {c.supportedModels.length} model{c.supportedModels.length !== 1 ? 's' : ''}
            </Badge>
          ) : (
            <Badge variant="secondary" className="ml-auto text-xs px-1.5 cursor-pointer hover:bg-muted-foreground/20" onClick={() => onEditModels(c)}>
              All models
            </Badge>
          )}
        </div>

        {/* Quota Panel */}
        <div className="space-y-2">
          <div className="flex items-center justify-between text-xs">
            <span className="text-muted-foreground">Usage & Quota</span>
            <Button variant="ghost" size="icon" className="h-5 w-5 hover:bg-muted" onClick={handleCheckQuota} title="Refresh quota" disabled={quotaLoading}>
              <RefreshCw size={10} className={cn("text-muted-foreground", quotaLoading && "animate-spin")} />
            </Button>
          </div>
          <div className="rounded-md border border-border/50 bg-muted/10 p-2.5 min-h-[56px] flex flex-col justify-center relative overflow-hidden">
            {(!quotaResult && !quotaLoading && !c.isActive) ? (
              <p className="text-[11px] text-muted-foreground text-center italic">Quota check unavailable for inactive connection.</p>
            ) : (!quotaResult && !quotaLoading) ? (
              <div className="text-center">
                <p className="text-[11px] text-muted-foreground mb-2">Quota not loaded</p>
                <Button variant="outline" size="sm" className="h-6 text-[10px]" onClick={handleCheckQuota}>Load Quota</Button>
              </div>
            ) : (
              <QuotaPanel data={quotaResult} loading={quotaLoading} />
            )}
          </div>
        </div>

        {/* Issues & Warnings */}
        {(isRL || c.lastError || c.backoffLevel > 0 || lockCount > 0) && (
          <div className="space-y-1.5">
            {isRL && (
              <div className="flex items-center gap-1.5 rounded-md bg-amber-500/10 px-2.5 py-1.5 text-[11px] text-amber-700 dark:text-amber-400 w-full">
                <Clock className="h-3 w-3 shrink-0" />
                <span>Rate Limited: {secsToHuman(rlSecs)}</span>
              </div>
            )}
            {c.backoffLevel > 0 && (
              <div className="flex items-center gap-1.5 rounded-md bg-amber-500/10 px-2.5 py-1.5 text-[11px] text-amber-700 dark:text-amber-400 w-full">
                <RefreshCw className="h-3 w-3 shrink-0" />
                <span>Backoff: Level {c.backoffLevel}/7</span>
              </div>
            )}
            {lockCount > 0 && (
              <div className="flex items-center gap-1.5 rounded-md bg-amber-500/10 px-2.5 py-1.5 text-[11px] text-amber-700 dark:text-amber-400 w-full">
                <Lock className="h-3 w-3 shrink-0" />
                <span>{lockCount} model(s) locked manually</span>
              </div>
            )}
            {c.lastError && (
              <div className="flex items-start gap-1.5 rounded-md bg-destructive/10 px-2.5 py-1.5 text-[11px] text-destructive dark:text-red-400 w-full">
                <AlertTriangle className="mt-0.5 h-3.5 w-3.5 shrink-0" />
                <span className="break-words line-clamp-2" title={c.lastError}>{c.lastError}</span>
              </div>
            )}
          </div>
        )}

        {/* Separator */}
        <div className="h-px bg-border/60" />

        {/* Actions Menu */}
        <div className="flex items-center gap-1">
          <Button variant="ghost" size="icon" className="h-7 w-7" onClick={handleTest} title="Test Connection">
            <TestTube className="h-3 w-3" />
          </Button>
          <Button variant="ghost" size="icon" className="h-7 w-7" onClick={() => onEditModels(c)} title="Edit Models">
            <Settings2 className="h-3 w-3" />
          </Button>
          
          <DropdownMenu>
            <DropdownMenuTrigger asChild>
              <Button variant="ghost" size="icon" className="h-7 w-7">
                <MoreHorizontal className="h-3 w-3" />
              </Button>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="start" className="w-44">
              {(isRL || c.backoffLevel > 0) && (
                <DropdownMenuItem onClick={handleResetCooldown} className="gap-2 text-xs text-amber-600">
                  <RefreshCw size={13} /> Reset Cooldown
                </DropdownMenuItem>
              )}
              <DropdownMenuItem onClick={handleTest} className="gap-2 text-xs">
                <TestTube size={13} /> Test Connection
              </DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>

          <div className="flex-1 flex justify-center">
            {testResult && !testResult.loading && (
              <span className={cn('text-[10px] font-medium px-2 py-0.5 rounded-sm', testResult.status === 'ok' ? 'bg-emerald-500/10 text-emerald-600' : 'bg-destructive/10 text-destructive')}>
                {testResult.status === 'ok' ? '✓ OK' : '✗ Failed'}
              </span>
            )}
            {testResult?.loading && <Loader2 size={12} className="animate-spin text-muted-foreground" />}
          </div>

          <Button variant="ghost" size="icon" className="h-7 w-7" onClick={handleToggle} title={c.isActive ? 'Deactivate' : 'Activate'}>
            <Switch checked={c.isActive} onCheckedChange={handleToggle} className="scale-[0.55] pointer-events-none" />
          </Button>
          <Button variant="ghost" size="icon" className="h-7 w-7 text-destructive hover:text-destructive hover:bg-destructive/10" onClick={() => onDelete(c.id, c.name)} title="Remove">
            <Trash2 className="h-3 w-3" />
          </Button>
        </div>
      </CardContent>
    </Card>
  )
}
