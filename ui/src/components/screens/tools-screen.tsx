import { useEffect, useState, useCallback } from 'react'
import { motion } from 'framer-motion'
import {
  Wrench,
  Check,
  X,
  Loader2,
  RefreshCw,
  Settings2,
  RotateCcw,
  Zap,
} from 'lucide-react'
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Skeleton } from '@/components/ui/skeleton'
import { Separator } from '@/components/ui/separator'
import { toast } from 'sonner'
import { goApi } from '@/lib/go-api'

interface ToolStatus {
  id: string
  name: string
  installed: boolean
  configured: boolean
  configPath?: string
  proxyUrl?: string
  backupExists?: boolean
}

const containerVariants = {
  hidden: { opacity: 0 },
  visible: {
    opacity: 1,
    transition: { staggerChildren: 0.06 },
  },
}

const itemVariants = {
  hidden: { opacity: 0, y: 16 },
  visible: { opacity: 1, y: 0, transition: { duration: 0.3 } },
}

export default function ToolsScreen() {
  const [tools, setTools] = useState<ToolStatus[]>([])
  const [loading, setLoading] = useState(true)
  const [actionLoading, setActionLoading] = useState<string | null>(null)

  const fetchTools = useCallback(async () => {
    try {
      const data = await goApi.getTools()
      setTools(Array.isArray(data) ? data : [])
    } catch {
      toast.error('Failed to load tools')
    }
  }, [])

  useEffect(() => {
    setLoading(true)
    fetchTools().finally(() => setLoading(false))
  }, [fetchTools])

  const handleConfigure = async (id: string, name: string) => {
    setActionLoading(id)
    try {
      await goApi.configureTool(id)
      toast.success(`${name} configured`, { description: 'Now routing through dntproxy' })
      await fetchTools()
    } catch (error: unknown) {
      const message = error instanceof Error ? error.message : 'Unknown error'
      toast.error(`Failed to configure ${name}`, { description: message })
    } finally {
      setActionLoading(null)
    }
  }

  const handleReset = async (id: string, name: string) => {
    setActionLoading(id + '-reset')
    try {
      await goApi.resetTool(id)
      toast.success(`${name} reset`, { description: 'Reverted to direct provider access' })
      await fetchTools()
    } catch (error: unknown) {
      const message = error instanceof Error ? error.message : 'Unknown error'
      toast.error(`Failed to reset ${name}`, { description: message })
    } finally {
      setActionLoading(null)
    }
  }

  const handleConfigureAll = async () => {
    setActionLoading('all')
    try {
      const result = await goApi.configureAllTools()
      const count = result?.count ?? 0
      toast.success(`${count} tool(s) configured`)
      await fetchTools()
    } catch (error: unknown) {
      const message = error instanceof Error ? error.message : 'Unknown error'
      toast.error('Failed to configure tools', { description: message })
    } finally {
      setActionLoading(null)
    }
  }

  const handleResetAll = async () => {
    setActionLoading('all-reset')
    try {
      const result = await goApi.resetAllTools()
      const count = result?.count ?? 0
      toast.success(`${count} tool(s) reset`)
      await fetchTools()
    } catch (error: unknown) {
      const message = error instanceof Error ? error.message : 'Unknown error'
      toast.error('Failed to reset tools', { description: message })
    } finally {
      setActionLoading(null)
    }
  }

  const installedTools = tools.filter(t => t.installed)
  const configuredTools = tools.filter(t => t.configured)

  if (loading) {
    return (
      <motion.div variants={containerVariants} initial="hidden" animate="visible" className="space-y-6">
        <motion.div variants={itemVariants}><Skeleton className="h-8 w-48" /></motion.div>
        <motion.div variants={itemVariants}><Skeleton className="h-32 w-full rounded-xl" /></motion.div>
        <motion.div variants={itemVariants}><Skeleton className="h-64 w-full rounded-xl" /></motion.div>
      </motion.div>
    )
  }

  return (
    <motion.div variants={containerVariants} initial="hidden" animate="visible" className="space-y-6">
      {/* Header */}
      <motion.div variants={itemVariants} className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold flex items-center gap-2">
            <Wrench className="w-6 h-6" />
            AI Tools
          </h1>
          <p className="text-muted-foreground mt-1">
            Configure AI coding tools to route through dntproxy.
          </p>
        </div>
        <Button variant="ghost" size="icon" onClick={() => fetchTools()} disabled={!!actionLoading}>
          <RefreshCw className="w-4 h-4" />
        </Button>
      </motion.div>

      {/* Summary Card */}
      <motion.div variants={itemVariants}>
        <Card>
          <CardContent className="pt-6">
            <div className="flex items-center justify-between">
              <div className="flex items-center gap-6">
                <div className="text-center">
                  <p className="text-2xl font-bold">{installedTools.length}</p>
                  <p className="text-xs text-muted-foreground">Detected</p>
                </div>
                <Separator orientation="vertical" className="h-10" />
                <div className="text-center">
                  <p className="text-2xl font-bold text-green-600 dark:text-green-400">{configuredTools.length}</p>
                  <p className="text-xs text-muted-foreground">Configured</p>
                </div>
              </div>
              <div className="flex gap-2">
                {installedTools.length > 0 && (
                  <Button
                    size="sm"
                    onClick={handleConfigureAll}
                    disabled={!!actionLoading}
                  >
                    {actionLoading === 'all' ? (
                      <Loader2 className="w-4 h-4 mr-2 animate-spin" />
                    ) : (
                      <Zap className="w-4 h-4 mr-2" />
                    )}
                    Configure All
                  </Button>
                )}
                {configuredTools.length > 0 && (
                  <Button
                    size="sm"
                    variant="outline"
                    onClick={handleResetAll}
                    disabled={!!actionLoading}
                  >
                    {actionLoading === 'all-reset' ? (
                      <Loader2 className="w-4 h-4 mr-2 animate-spin" />
                    ) : (
                      <RotateCcw className="w-4 h-4 mr-2" />
                    )}
                    Reset All
                  </Button>
                )}
              </div>
            </div>
          </CardContent>
        </Card>
      </motion.div>

      {/* Tools List */}
      <motion.div variants={itemVariants}>
        <Card>
          <CardHeader className="pb-3">
            <CardTitle className="text-base">Supported Tools</CardTitle>
            <CardDescription>
              Tools detected on this system are shown with their configuration status.
            </CardDescription>
          </CardHeader>
          <CardContent>
            <div className="space-y-3">
              {tools.map((tool) => (
                <div
                  key={tool.id}
                  className={`flex items-center justify-between p-3 rounded-lg border ${
                    tool.configured
                      ? 'border-green-500/30 bg-green-500/5'
                      : tool.installed
                        ? 'border-border'
                        : 'border-border/50 opacity-60'
                  }`}
                >
                  <div className="flex items-center gap-3 min-w-0">
                    <div className={`w-2.5 h-2.5 rounded-full shrink-0 ${
                      tool.configured ? 'bg-green-500' : tool.installed ? 'bg-muted-foreground/40' : 'bg-muted-foreground/20'
                    }`} />
                    <div className="min-w-0">
                      <div className="flex items-center gap-2">
                        <p className="font-medium text-sm">{tool.name}</p>
                        {tool.installed ? (
                          <Badge variant="secondary" className="text-xs">installed</Badge>
                        ) : (
                          <Badge variant="outline" className="text-xs opacity-60">not found</Badge>
                        )}
                        {tool.configured && (
                          <Badge variant="default" className="text-xs bg-green-600">proxied</Badge>
                        )}
                      </div>
                      {tool.configured && tool.proxyUrl && (
                        <p className="text-xs text-muted-foreground mt-0.5 truncate">
                          {tool.proxyUrl}
                        </p>
                      )}
                      {tool.configPath && tool.installed && (
                        <p className="text-xs text-muted-foreground/70 mt-0.5 truncate font-mono">
                          {tool.configPath}
                        </p>
                      )}
                    </div>
                  </div>

                  <div className="flex items-center gap-2 shrink-0 ml-3">
                    {tool.installed && !tool.configured && (
                      <Button
                        size="sm"
                        variant="outline"
                        onClick={() => handleConfigure(tool.id, tool.name)}
                        disabled={!!actionLoading}
                      >
                        {actionLoading === tool.id ? (
                          <Loader2 className="w-3.5 h-3.5 animate-spin" />
                        ) : (
                          <Settings2 className="w-3.5 h-3.5 mr-1.5" />
                        )}
                        Configure
                      </Button>
                    )}
                    {tool.configured && (
                      <>
                        <Button
                          size="sm"
                          variant="ghost"
                          onClick={() => handleConfigure(tool.id, tool.name)}
                          disabled={!!actionLoading}
                          title="Reconfigure"
                        >
                          {actionLoading === tool.id ? (
                            <Loader2 className="w-3.5 h-3.5 animate-spin" />
                          ) : (
                            <RefreshCw className="w-3.5 h-3.5" />
                          )}
                        </Button>
                        <Button
                          size="sm"
                          variant="ghost"
                          className="text-destructive hover:text-destructive"
                          onClick={() => handleReset(tool.id, tool.name)}
                          disabled={!!actionLoading}
                          title="Reset to defaults"
                        >
                          {actionLoading === tool.id + '-reset' ? (
                            <Loader2 className="w-3.5 h-3.5 animate-spin" />
                          ) : (
                            <RotateCcw className="w-3.5 h-3.5" />
                          )}
                        </Button>
                      </>
                    )}
                  </div>
                </div>
              ))}

              {tools.length === 0 && (
                <p className="text-sm text-muted-foreground text-center py-6">
                  No supported tools found in the registry.
                </p>
              )}
            </div>
          </CardContent>
        </Card>
      </motion.div>
    </motion.div>
  )
}
