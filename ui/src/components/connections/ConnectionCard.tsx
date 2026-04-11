import { useState, useEffect } from 'react'
import {
  TestTube, Settings2, MoreHorizontal, RefreshCw,
  Trash2, BarChart2, Loader2, Play, ChevronRight, AlertTriangle, Clock, Lock
} from 'lucide-react'
import { api } from '../../api'
import { getModelName } from '../../models-config'
import InlineName from './InlineName'
import { TokenBar, QuotaPanel, getProviderInfo, secsToHuman } from './helpers'
import { Card, CardContent } from '@/components/ui/card'
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
  const [quotaOpen, setQuotaOpen] = useState(false)
  const [modelTestResults, setModelTestResults] = useState<Record<string, { status: 'ok' | 'error' | 'loading'; message?: string }>>({})

  useEffect(() => {
    if (initialQuotaResult !== undefined) setQuotaResult(initialQuotaResult)
  }, [initialQuotaResult])

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
    setQuotaOpen(true)
    try {
      const res = await api.checkQuota(c.id)
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
    <Card className={cn('transition-all', !c.isActive && 'opacity-60', hasIssue && c.isActive && 'border-amber-500/40')}>
      <CardContent className="p-4 space-y-3">
        {/* Header row */}
        <div className="flex items-start gap-3">
          <div className={cn('relative flex h-10 w-10 shrink-0 items-center justify-center rounded-lg border', providerInfo.colorClass)}>
            {providerInfo.icon}
            <span className={cn('absolute -bottom-0.5 -right-0.5 h-2.5 w-2.5 rounded-full border-2 border-background',
              !c.isActive ? 'bg-muted-foreground' : isRL ? 'bg-amber-500' : hasIssue ? 'bg-destructive' : 'bg-emerald-500'
            )} />
          </div>

          <div className="flex-1 min-w-0 space-y-1">
            <div className="flex items-center gap-2 flex-wrap">
              <InlineName conn={c} onRename={handleRename} />
              <span className="text-[11px] text-muted-foreground truncate">
                {c.email || c.baseUrl?.replace('https://', '') || c.authMethod || 'API Key'}
              </span>
            </div>
            <div className="flex items-center gap-1.5 flex-wrap">
              <TokenBar conn={c} />
              {isRL && (
                <Badge variant="outline" className="gap-1 text-amber-600 border-amber-500/30 bg-amber-500/10 text-[10px] py-0 h-5">
                  <Clock size={9} /> RL: {secsToHuman(rlSecs)}
                </Badge>
              )}
              {c.backoffLevel > 0 && (
                <Badge variant="outline" className="gap-1 text-amber-600 border-amber-500/30 bg-amber-500/10 text-[10px] py-0 h-5">
                  <RefreshCw size={9} /> Backoff: {c.backoffLevel}/7
                </Badge>
              )}
              {lockCount > 0 && (
                <Badge variant="outline" className="gap-1 text-amber-600 border-amber-500/30 bg-amber-500/10 text-[10px] py-0 h-5">
                  <Lock size={9} /> {lockCount} locked
                </Badge>
              )}
              {c.lastError && (
                <Badge variant="outline" className="gap-1 text-destructive border-destructive/30 bg-destructive/10 text-[10px] py-0 h-5 max-w-[200px] truncate" title={c.lastError}>
                  <AlertTriangle size={9} /> {c.lastError.slice(0, 30)}{c.lastError.length > 30 ? '…' : ''}
                </Badge>
              )}
              {testResult && !testResult.loading && (
                <span className={cn('text-[10px] font-medium', testResult.status === 'ok' ? 'text-emerald-600' : 'text-destructive')}>
                  {testResult.status === 'ok' ? '✓ OK' : '✗ Failed'}
                </span>
              )}
              {testResult?.loading && <Loader2 size={10} className="animate-spin text-muted-foreground" />}
            </div>
          </div>

          <div className="flex items-center gap-1.5 shrink-0">
            <Switch checked={c.isActive} onCheckedChange={handleToggle} className="scale-75" />
            <Button variant="ghost" size="icon" className="h-7 w-7" onClick={handleTest} title="Test connection">
              <TestTube size={13} />
            </Button>
            <Button variant="ghost" size="icon" className="h-7 w-7" onClick={() => onEditModels(c)} title="Edit models">
              <Settings2 size={13} />
            </Button>
            <DropdownMenu>
              <DropdownMenuTrigger asChild>
                <Button variant="ghost" size="icon" className="h-7 w-7">
                  <MoreHorizontal size={14} />
                </Button>
              </DropdownMenuTrigger>
              <DropdownMenuContent align="end" className="w-44">
                <DropdownMenuItem onClick={handleCheckQuota} className="gap-2 text-xs">
                  <BarChart2 size={13} /> Check Quota
                </DropdownMenuItem>
                {(isRL || c.backoffLevel > 0) && (
                  <DropdownMenuItem onClick={handleResetCooldown} className="gap-2 text-xs text-amber-600">
                    <RefreshCw size={13} /> Reset Cooldown
                  </DropdownMenuItem>
                )}
                <DropdownMenuSeparator />
                <DropdownMenuItem onClick={() => onDelete(c.id, c.name)} className="gap-2 text-xs text-destructive focus:text-destructive">
                  <Trash2 size={13} /> Remove
                </DropdownMenuItem>
              </DropdownMenuContent>
            </DropdownMenu>
          </div>
        </div>

        {/* Models + Quota */}
        <div className="border-t pt-3">
          <div className="flex items-start gap-3">
            <div className="flex-1 min-w-0">
              <p className="text-[10px] font-semibold uppercase tracking-wider text-muted-foreground mb-2">Supported Models</p>
              {c.supportedModels?.length ? (
                <div className="flex flex-wrap gap-1.5">
                  {c.supportedModels.slice(0, 6).map((m: string, i: number) => {
                    const mTest = modelTestResults[m]
                    return (
                      <div key={i} className="relative group/model">
                        <Badge
                          variant="outline"
                          className={cn(
                            'text-[11px] font-mono cursor-default',
                            mTest?.status === 'ok' && 'text-emerald-600 border-emerald-500/30 bg-emerald-500/10',
                            mTest?.status === 'error' && 'text-destructive border-destructive/30 bg-destructive/10',
                          )}
                        >
                          {mTest?.status === 'loading' && <Loader2 size={9} className="animate-spin mr-1" />}
                          {getModelName(m)}
                        </Badge>
                        <button
                          onClick={() => handleTestModel(m)}
                          className="absolute inset-0 flex items-center justify-center opacity-0 group-hover/model:opacity-100 bg-primary rounded text-primary-foreground transition-opacity"
                        >
                          <Play size={9} fill="currentColor" />
                        </button>
                      </div>
                    )
                  })}
                  {c.supportedModels.length > 6 && (
                    <Button variant="outline" size="sm" className="h-6 text-[11px] gap-1 px-2" onClick={() => onEditModels(c)}>
                      +{c.supportedModels.length - 6} <ChevronRight size={10} />
                    </Button>
                  )}
                </div>
              ) : (
                <p className="text-xs text-muted-foreground">All models allowed</p>
              )}
            </div>

            {(quotaResult || quotaLoading) && quotaOpen && (
              <div className="shrink-0 w-52 rounded-lg border bg-muted/30 p-3">
                <QuotaPanel data={quotaResult} loading={quotaLoading} />
              </div>
            )}
          </div>
        </div>
      </CardContent>
    </Card>
  )
}
