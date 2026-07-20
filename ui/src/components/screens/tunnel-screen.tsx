import { useEffect, useState, useCallback, useRef } from 'react'
import { motion } from 'framer-motion'
import {
  Globe,
  Copy,
  Check,
  Loader2,
  AlertTriangle,
  ExternalLink,
  RefreshCw,
  Zap,
  ZapOff,
  CircleAlert,
} from 'lucide-react'
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Skeleton } from '@/components/ui/skeleton'
import { Separator } from '@/components/ui/separator'
import { toast } from 'sonner'
import { goApi } from '@/lib/go-api'

interface TunnelStatus {
  enabled: boolean
  running: boolean
  starting: boolean
  provider: string
  tunnelUrl: string
  shortId: string
  publicUrl: string
  lastError: string
}

const containerVariants = {
  hidden: { opacity: 0 },
  visible: {
    opacity: 1,
    transition: { staggerChildren: 0.08 },
  },
}

const itemVariants = {
  hidden: { opacity: 0, y: 16 },
  visible: { opacity: 1, y: 0, transition: { duration: 0.3 } },
}

export default function TunnelScreen() {
  const [status, setStatus] = useState<TunnelStatus | null>(null)
  const [loading, setLoading] = useState(true)
  const [actionLoading, setActionLoading] = useState(false)
  const [copied, setCopied] = useState(false)
  const pollRef = useRef<ReturnType<typeof setInterval> | null>(null)
  const safetyTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null)

  const stopPolling = useCallback(() => {
    if (pollRef.current) {
      clearInterval(pollRef.current)
      pollRef.current = null
    }
    if (safetyTimerRef.current) {
      clearTimeout(safetyTimerRef.current)
      safetyTimerRef.current = null
    }
  }, [])

  const fetchStatus = useCallback(async () => {
    try {
      const data = await goApi.getTunnelStatus()
      setStatus(data)
      return data
    } catch {
      return null
    }
  }, [])

  useEffect(() => {
    setLoading(true)
    fetchStatus().finally(() => setLoading(false))
    return stopPolling
  }, [fetchStatus, stopPolling])

  const startPolling = useCallback(() => {
    stopPolling()
    pollRef.current = setInterval(async () => {
      const data = await fetchStatus()
      if (!data) return // fetch failed, keep polling
      if (data.running && !data.starting && data.tunnelUrl) {
        // Fully connected: process alive + URL ready
        stopPolling()
        toast.success('Tunnel connected!', { description: data.tunnelUrl })
      } else if (data.lastError) {
        stopPolling()
        toast.error('Tunnel failed', { description: data.lastError })
      }
      // Otherwise keep polling: starting=true, or running but URL not yet propagated
    }, 2000)
    // Safety: stop polling after 90s and refresh status one last time
    safetyTimerRef.current = setTimeout(() => {
      stopPolling()
      fetchStatus()
    }, 90000)
  }, [fetchStatus, stopPolling])

  const handleEnable = async () => {
    setActionLoading(true)
    try {
      await goApi.enableTunnel()
      setStatus(prev => prev ? { ...prev, starting: true, lastError: '' } : null)
      toast.info('Starting tunnel...', { description: 'Downloading binary & connecting. This may take ~35 seconds.' })
      startPolling()
    } catch (error: unknown) {
      const message = error instanceof Error ? error.message : 'Unknown error'
      toast.error('Failed to start tunnel', { description: message })
    } finally {
      setActionLoading(false)
    }
  }

  const handleDisable = async () => {
    setActionLoading(true)
    stopPolling()
    try {
      await goApi.disableTunnel()
      setStatus(prev => prev ? { ...prev, enabled: false, running: false, starting: false, tunnelUrl: '', publicUrl: '', lastError: '' } : null)
      toast.success('Tunnel stopped')
    } catch (error: unknown) {
      const message = error instanceof Error ? error.message : 'Unknown error'
      toast.error('Failed to stop tunnel', { description: message })
    } finally {
      setActionLoading(false)
    }
  }

  const copyUrl = async (url: string) => {
    await navigator.clipboard.writeText(url)
    setCopied(true)
    toast.success('Copied to clipboard')
    setTimeout(() => setCopied(false), 2000)
  }

  if (loading) {
    return (
      <motion.div variants={containerVariants} initial="hidden" animate="visible" className="space-y-6">
        <motion.div variants={itemVariants}><Skeleton className="h-8 w-48" /></motion.div>
        <motion.div variants={itemVariants}><Skeleton className="h-40 w-full rounded-xl" /></motion.div>
        <motion.div variants={itemVariants}><Skeleton className="h-24 w-full rounded-xl" /></motion.div>
      </motion.div>
    )
  }

  const isRunning = status?.running ?? false
  const isStarting = status?.starting ?? false
  const tunnelUrl = status?.tunnelUrl || status?.publicUrl || ''
  const hasError = !!status?.lastError

  return (
    <motion.div variants={containerVariants} initial="hidden" animate="visible" className="space-y-6">
      {/* Header */}
      <motion.div variants={itemVariants} className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold flex items-center gap-2">
            <Globe className="w-6 h-6" />
            Cloudflare Tunnel
          </h1>
          <p className="text-muted-foreground mt-1">
            Expose your local proxy to the internet securely.
          </p>
        </div>
        <Button variant="ghost" size="icon" onClick={() => fetchStatus()} disabled={actionLoading}>
          <RefreshCw className="w-4 h-4" />
        </Button>
      </motion.div>

      {/* Main Control Card */}
      <motion.div variants={itemVariants}>
        <Card className={isRunning ? 'border-green-500/30' : hasError ? 'border-destructive/30' : ''}>
          <CardContent className="pt-6">
            <div className="flex items-center justify-between">
              <div className="flex items-center gap-3">
                <div className={`w-3 h-3 rounded-full ${isRunning ? 'bg-green-500 animate-pulse' : isStarting ? 'bg-yellow-500 animate-pulse' : 'bg-muted-foreground/30'}`} />
                <div>
                  <p className="font-semibold text-lg">
                    {isRunning ? 'Connected' : isStarting ? 'Starting...' : 'Disconnected'}
                  </p>
                  <p className="text-sm text-muted-foreground">
                    {isRunning ? 'Tunnel is active and forwarding traffic' : isStarting ? 'Downloading binary & connecting' : 'Tunnel is not running'}
                  </p>
                </div>
              </div>
              <div>
                {isRunning ? (
                  <Button variant="destructive" onClick={handleDisable} disabled={actionLoading}>
                    {actionLoading ? <Loader2 className="w-4 h-4 mr-2 animate-spin" /> : <ZapOff className="w-4 h-4 mr-2" />}
                    Stop Tunnel
                  </Button>
                ) : isStarting ? (
                  <Button variant="outline" onClick={handleDisable} disabled={actionLoading}>
                    {actionLoading ? <Loader2 className="w-4 h-4 mr-2 animate-spin" /> : <ZapOff className="w-4 h-4 mr-2" />}
                    Cancel
                  </Button>
                ) : (
                  <Button onClick={handleEnable} disabled={actionLoading}>
                    {actionLoading ? <Loader2 className="w-4 h-4 mr-2 animate-spin" /> : <Zap className="w-4 h-4 mr-2" />}
                    Start Tunnel
                  </Button>
                )}
              </div>
            </div>

            {/* Starting progress */}
            {isStarting && (
              <div className="mt-4 flex items-center gap-2 text-sm text-muted-foreground">
                <Loader2 className="w-4 h-4 animate-spin" />
                <span>This may take up to 35 seconds...</span>
              </div>
            )}

            {/* Tunnel URL */}
            {isRunning && tunnelUrl && (
              <div className="mt-4 p-3 bg-green-500/5 border border-green-500/20 rounded-lg">
                <p className="text-xs text-muted-foreground mb-1">Public URL</p>
                <div className="flex items-center gap-2">
                  <code className="flex-1 text-sm font-mono break-all text-green-600 dark:text-green-400">
                    {tunnelUrl}
                  </code>
                  <Button variant="ghost" size="icon" className="h-8 w-8 shrink-0" onClick={() => copyUrl(tunnelUrl)}>
                    {copied ? <Check className="w-4 h-4 text-green-600" /> : <Copy className="w-4 h-4" />}
                  </Button>
                  <Button variant="ghost" size="icon" className="h-8 w-8 shrink-0" onClick={() => window.open(tunnelUrl, '_blank')}>
                    <ExternalLink className="w-4 h-4" />
                  </Button>
                </div>
              </div>
            )}

            {/* Error message */}
            {hasError && !isStarting && (
              <Alert variant="destructive" className="mt-4">
                <CircleAlert className="h-4 w-4" />
                <AlertDescription>{status!.lastError}</AlertDescription>
              </Alert>
            )}
          </CardContent>
        </Card>
      </motion.div>

      {/* Details */}
      {(isRunning || status?.shortId) && (
        <motion.div variants={itemVariants}>
          <Card>
            <CardHeader className="pb-3">
              <CardTitle className="text-base">Details</CardTitle>
            </CardHeader>
            <CardContent>
              <div className="grid grid-cols-2 sm:grid-cols-3 gap-4 text-sm">
                <div>
                  <p className="text-muted-foreground">Provider</p>
                  <p className="font-medium capitalize mt-0.5">{status?.provider || 'cloudflare'}</p>
                </div>
                <div>
                  <p className="text-muted-foreground">Short ID</p>
                  <p className="font-mono mt-0.5">{status?.shortId || '—'}</p>
                </div>
                <div>
                  <p className="text-muted-foreground">Status</p>
                  <Badge variant={isRunning ? 'default' : 'secondary'} className="mt-0.5">
                    {isRunning ? 'Active' : 'Inactive'}
                  </Badge>
                </div>
              </div>
            </CardContent>
          </Card>
        </motion.div>
      )}

      {/* Security Warning */}
      <motion.div variants={itemVariants}>
        <Alert>
          <AlertTriangle className="h-4 w-4" />
          <AlertDescription>
            API key authentication is always required. Only share dashboard keys with trusted operators before enabling a public tunnel.
          </AlertDescription>
        </Alert>
      </motion.div>
    </motion.div>
  )
}
