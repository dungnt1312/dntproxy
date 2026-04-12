import { useEffect, useState, useCallback } from 'react'
import { motion } from 'framer-motion'
import {
  Globe,
  Power,
  Copy,
  Check,
  Loader2,
  AlertTriangle,
  ExternalLink,
  RefreshCw,
  Info,
} from 'lucide-react'
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Skeleton } from '@/components/ui/skeleton'
import { Separator } from '@/components/ui/separator'
import { toast } from 'sonner'
import { goApi } from '@/lib/go-api'

interface TunnelStatus {
  enabled: boolean
  running: boolean
  provider: string
  tunnelUrl: string
  shortId: string
  publicUrl: string
}

const containerVariants = {
  hidden: { opacity: 0 },
  visible: {
    opacity: 1,
    transition: { staggerChildren: 0.1 },
  },
}

const itemVariants = {
  hidden: { opacity: 0, y: 20 },
  visible: { opacity: 1, y: 0, transition: { duration: 0.4 } },
}

export default function TunnelScreen() {
  const [status, setStatus] = useState<TunnelStatus | null>(null)
  const [loading, setLoading] = useState(true)
  const [actionLoading, setActionLoading] = useState(false)
  const [copied, setCopied] = useState(false)

  const fetchStatus = useCallback(async () => {
    try {
      setLoading(true)
      const data = await goApi.getTunnelStatus()
      setStatus(data)
    } catch (error) {
      toast.error('Failed to fetch tunnel status')
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    fetchStatus()
  }, [fetchStatus])

  const handleEnable = async () => {
    setActionLoading(true)
    try {
      await goApi.enableTunnel()
      toast.success('Tunnel starting...', { description: 'It may take up to 30 seconds to connect.' })
      // Poll status every 2s until connected
      const poll = setInterval(async () => {
        const data = await goApi.getTunnelStatus()
        setStatus(data)
        if (data.running) {
          clearInterval(poll)
          toast.success('Tunnel connected!')
        }
      }, 2000)
      // Stop polling after 60s
      setTimeout(() => clearInterval(poll), 60000)
    } catch (error: unknown) {
      const message = error instanceof Error ? error.message : 'Unknown error'
      toast.error('Failed to start tunnel', { description: message })
    } finally {
      setActionLoading(false)
    }
  }

  const handleDisable = async () => {
    setActionLoading(true)
    try {
      await goApi.disableTunnel()
      setStatus((prev) => prev ? { ...prev, enabled: false, running: false, tunnelUrl: '', publicUrl: '' } : null)
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
    toast.success('URL copied to clipboard')
    setTimeout(() => setCopied(false), 2000)
  }

  if (loading) {
    return (
      <motion.div variants={containerVariants} initial="hidden" animate="visible" className="space-y-6">
        <motion.div variants={itemVariants}>
          <Skeleton className="h-8 w-48" />
        </motion.div>
        <motion.div variants={itemVariants}>
          <Skeleton className="h-32 w-full rounded-xl" />
        </motion.div>
      </motion.div>
    )
  }

  const isRunning = status?.running ?? false
  const tunnelUrl = status?.tunnelUrl || status?.publicUrl || ''

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
            Expose your local proxy to the internet via a secure Cloudflare quick tunnel.
          </p>
        </div>
        <div className="flex gap-2">
          <Button variant="outline" size="sm" onClick={fetchStatus} disabled={actionLoading}>
            <RefreshCw className="w-4 h-4 mr-1" />
            Refresh
          </Button>
          {isRunning ? (
            <Button variant="destructive" size="sm" onClick={handleDisable} disabled={actionLoading}>
              {actionLoading ? <Loader2 className="w-4 h-4 mr-1 animate-spin" /> : <Power className="w-4 h-4 mr-1" />}
              Stop
            </Button>
          ) : (
            <Button size="sm" onClick={handleEnable} disabled={actionLoading}>
              {actionLoading ? <Loader2 className="w-4 h-4 mr-1 animate-spin" /> : <Power className="w-4 h-4 mr-1" />}
              Start
            </Button>
          )}
        </div>
      </motion.div>

      {/* Status Alert */}
      {isRunning && tunnelUrl && (
        <motion.div variants={itemVariants}>
          <Alert className="border-green-500/50 bg-green-500/5">
            <Globe className="h-4 w-4 text-green-600" />
            <AlertTitle className="text-green-600">Tunnel is running</AlertTitle>
            <AlertDescription>
              <div className="flex items-center gap-2 mt-2">
                <code className="flex-1 bg-green-500/10 px-2 py-1 rounded text-sm break-all">
                  {tunnelUrl}
                </code>
                <Button variant="ghost" size="sm" onClick={() => copyUrl(tunnelUrl)}>
                  {copied ? <Check className="w-4 h-4" /> : <Copy className="w-4 h-4" />}
                </Button>
                <Button variant="ghost" size="sm" onClick={() => window.open(tunnelUrl, '_blank')}>
                  <ExternalLink className="w-4 h-4" />
                </Button>
              </div>
            </AlertDescription>
          </Alert>
        </motion.div>
      )}

      {/* Status Card */}
      <motion.div variants={itemVariants}>
        <Card>
          <CardHeader>
            <CardTitle>Status</CardTitle>
            <CardDescription>Current tunnel connection details</CardDescription>
          </CardHeader>
          <CardContent className="space-y-4">
            <div className="grid grid-cols-2 gap-4 text-sm">
              <div>
                <span className="text-muted-foreground">Enabled</span>
                <div className="mt-1">
                  <Badge variant={status?.enabled ? 'default' : 'secondary'}>
                    {status?.enabled ? 'Yes' : 'No'}
                  </Badge>
                </div>
              </div>
              <div>
                <span className="text-muted-foreground">Running</span>
                <div className="mt-1">
                  <Badge variant={isRunning ? 'default' : 'destructive'}>
                    {isRunning ? 'Connected' : 'Disconnected'}
                  </Badge>
                </div>
              </div>
              <div>
                <span className="text-muted-foreground">Provider</span>
                <div className="mt-1 font-medium capitalize">
                  {status?.provider || '—'}
                </div>
              </div>
              <div>
                <span className="text-muted-foreground">Short ID</span>
                <div className="mt-1 font-mono text-sm">
                  {status?.shortId || '—'}
                </div>
              </div>
            </div>

            <Separator />

            <div>
              <span className="text-muted-foreground">Tunnel URL</span>
              <div className="mt-1 font-mono text-sm break-all text-muted-foreground">
                {tunnelUrl || 'Not connected'}
              </div>
            </div>
          </CardContent>
        </Card>
      </motion.div>

      {/* Info Card */}
      <motion.div variants={itemVariants}>
        <Card>
          <CardHeader>
            <CardTitle className="flex items-center gap-2">
              <Info className="w-4 h-4" />
              How it works
            </CardTitle>
            <CardDescription>
              Cloudflare quick tunnels provide a public URL to your local proxy
            </CardDescription>
          </CardHeader>
          <CardContent>
            <ul className="space-y-2 text-sm text-muted-foreground">
              <li className="flex items-start gap-2">
                <span className="text-primary mt-0.5">1.</span>
                Click <strong>Start</strong> to download cloudflared (if needed) and create a quick tunnel.
              </li>
              <li className="flex items-start gap-2">
                <span className="text-primary mt-0.5">2.</span>
                A public <code className="bg-muted px-1 rounded">trycloudflare.com</code> URL will be generated.
              </li>
              <li className="flex items-start gap-2">
                <span className="text-primary mt-0.5">3.</span>
                Use this URL as your API endpoint in any AI client.
              </li>
              <li className="flex items-start gap-2">
                <span className="text-primary mt-0.5">4.</span>
                The tunnel stops when you click <strong>Stop</strong> or restart the proxy.
              </li>
            </ul>

            <Alert className="mt-4">
              <AlertTriangle className="h-4 w-4" />
              <AlertTitle>Security note</AlertTitle>
              <AlertDescription>
                Make sure <strong>Require API Key</strong> is enabled in Settings before starting a tunnel to protect your proxy from unauthorized access.
              </AlertDescription>
            </Alert>
          </CardContent>
        </Card>
      </motion.div>

      {/* CLI Usage */}
      <motion.div variants={itemVariants}>
        <Card>
          <CardHeader>
            <CardTitle>CLI Usage</CardTitle>
            <CardDescription>Manage tunnels from the command line</CardDescription>
          </CardHeader>
          <CardContent>
            <div className="space-y-2 text-sm font-mono bg-muted p-4 rounded-lg">
              <p><span className="text-muted-foreground"># Start tunnel</span></p>
              <p>dntproxy tunnel enable</p>
              <Separator className="my-2" />
              <p><span className="text-muted-foreground"># Stop tunnel</span></p>
              <p>dntproxy tunnel disable</p>
              <Separator className="my-2" />
              <p><span className="text-muted-foreground"># Check status</span></p>
              <p>dntproxy tunnel status</p>
            </div>
          </CardContent>
        </Card>
      </motion.div>
    </motion.div>
  )
}
