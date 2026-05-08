import { useEffect, useState, useMemo, useCallback } from 'react'
import { motion } from 'framer-motion'
import {
  Plus,
  Copy,
  CopyCheck,
  Trash2,
  Key,
  Loader2,
  ShieldCheck,
  Shield,
  Search,
  Eye,
  EyeOff,
  Check,
} from 'lucide-react'
import { Card, CardContent } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Skeleton } from '@/components/ui/skeleton'
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog'
import { toast } from 'sonner'
import { goApi } from '@/lib/go-api'
import { cn } from '@/lib/utils'

interface ApiKey {
  id: string
  name: string
  key: string
  isActive: boolean
  createdAt: string
  updatedAt: string
}

function formatDate(dateStr: string) {
  const date = new Date(dateStr)
  if (Number.isNaN(date.getTime())) return dateStr
  return date.toLocaleDateString(undefined, {
    month: 'short',
    day: 'numeric',
    year: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
  })
}

function maskKey(key: string) {
  if (!key) return ''
  if (key.length <= 12) return key
  return key.slice(0, 7) + '...' + key.slice(-4)
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
  visible: { opacity: 1, y: 0, transition: { duration: 0.35 } },
}

function StatCard({
  icon: Icon,
  label,
  value,
  sub,
  className,
}: {
  icon: React.ElementType
  label: string
  value: number | string
  sub?: string
  className?: string
}) {
  return (
    <motion.div variants={itemVariants}>
      <Card className={cn('relative overflow-hidden', className)}>
        <CardContent className="flex items-center gap-4 p-4">
          <div className="flex size-10 shrink-0 items-center justify-center rounded-lg bg-primary/10">
            <Icon className="size-5 text-primary" />
          </div>
          <div className="min-w-0">
            <p className="text-xs font-medium text-muted-foreground uppercase tracking-wide">{label}</p>
            <p className="text-2xl font-bold leading-tight">{value}</p>
            {sub && <p className="text-xs text-muted-foreground mt-0.5">{sub}</p>}
          </div>
        </CardContent>
      </Card>
    </motion.div>
  )
}

function CopyButton({ text, label }: { text: string; label: string }) {
  const [copied, setCopied] = useState(false)

  const handleCopy = () => {
    navigator.clipboard.writeText(text).then(() => {
      setCopied(true)
      toast.success(`${label} copied to clipboard`)
      setTimeout(() => setCopied(false), 2000)
    }).catch(() => {
      toast.error('Failed to copy to clipboard')
    })
  }

  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <Button
          variant="ghost"
          size="icon"
          className="size-8"
          onClick={handleCopy}
        >
          {copied ? (
            <CopyCheck className="size-3.5 text-emerald-500" />
          ) : (
            <Copy className="size-3.5" />
          )}
        </Button>
      </TooltipTrigger>
      <TooltipContent>{copied ? 'Copied!' : `Copy ${label}`}</TooltipContent>
    </Tooltip>
  )
}

export default function ApiKeysScreen() {
  const [keys, setKeys] = useState<ApiKey[]>([])
  const [loading, setLoading] = useState(true)
  const [searchQuery, setSearchQuery] = useState('')
  const [generateOpen, setGenerateOpen] = useState(false)
  const [newKeyName, setNewKeyName] = useState('')
  const [generating, setGenerating] = useState(false)
  const [showKeyOpen, setShowKeyOpen] = useState(false)
  const [createdKey, setCreatedKey] = useState('')
  const [createdKeyName, setCreatedKeyName] = useState('')
  const [deleteTarget, setDeleteTarget] = useState<ApiKey | null>(null)
  const [deleting, setDeleting] = useState(false)
  const [revealedKeys, setRevealedKeys] = useState<Set<string>>(new Set())

  const fetchKeys = useCallback(async () => {
    try {
      setLoading(true)
      const json = await goApi.getKeys()
      setKeys(json)
    } catch {
      toast.error('Failed to load API keys')
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    fetchKeys()
  }, [fetchKeys])

  const filteredKeys = useMemo(() => {
    const q = searchQuery.trim().toLowerCase()
    if (!q) return keys
    return keys.filter((k) =>
      k.name.toLowerCase().includes(q) || k.key.toLowerCase().includes(q)
    )
  }, [keys, searchQuery])

  const stats = useMemo(() => {
    const total = keys.length
    const active = keys.filter((k) => k.isActive).length
    return { total, active }
  }, [keys])

  const toggleReveal = (id: string) => {
    setRevealedKeys((prev) => {
      const next = new Set(prev)
      if (next.has(id)) next.delete(id)
      else next.add(id)
      return next
    })
  }

  const handleGenerate = async () => {
    if (!newKeyName.trim()) {
      toast.error('Please enter a key name')
      return
    }

    try {
      setGenerating(true)
      const json = await goApi.createKey(newKeyName.trim())
      setCreatedKey(json.key)
      setCreatedKeyName(json.name)
      setGenerateOpen(false)
      setNewKeyName('')
      setShowKeyOpen(true)
      toast.success('API key generated successfully')
      await fetchKeys()
    } catch {
      toast.error('Failed to generate API key')
    } finally {
      setGenerating(false)
    }
  }

  const handleDelete = async () => {
    if (!deleteTarget) return
    try {
      setDeleting(true)
      await goApi.deleteKey(deleteTarget.id)
      setDeleteTarget(null)
      await fetchKeys()
      toast.success(`Key "${deleteTarget.name}" deleted`)
    } catch {
      toast.error('Failed to delete API key')
    } finally {
      setDeleting(false)
    }
  }

  const copyToClipboard = (text: string, label: string) => {
    navigator.clipboard.writeText(text).then(() => {
      toast.success(`${label} copied to clipboard`)
    }).catch(() => {
      toast.error('Failed to copy to clipboard')
    })
  }

  return (
    <motion.div
      className="space-y-6"
      variants={containerVariants}
      initial="hidden"
      animate="visible"
    >
      {/* Header */}
      <motion.div variants={itemVariants} className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-4">
        <div>
          <h1 className="text-2xl font-bold tracking-tight">API Keys</h1>
          <p className="text-muted-foreground mt-1 text-sm">
            Generate and manage API keys for authenticating requests to your proxy server
          </p>
        </div>
        <Button onClick={() => setGenerateOpen(true)} className="self-start shrink-0">
          <Plus className="size-4" />
          Generate New Key
        </Button>
      </motion.div>

      {/* Stats */}
      <div className="grid grid-cols-2 lg:grid-cols-3 gap-4">
        <StatCard icon={Key} label="Total Keys" value={stats.total} />
        <StatCard icon={ShieldCheck} label="Active" value={stats.active} sub={stats.total > 0 ? `${Math.round((stats.active / stats.total) * 100)}% of total` : undefined} />
        <StatCard icon={Shield} label="Inactive" value={stats.total - stats.active} className="hidden lg:block" />
      </div>

      {/* Search + Table */}
      <motion.div variants={itemVariants}>
        <Card>
          <CardContent className="p-0">
            {loading ? (
              <div className="p-6 space-y-4">
                <Skeleton className="h-9 w-full max-w-sm" />
                {Array.from({ length: 3 }).map((_, i) => (
                  <div key={i} className="flex items-center gap-4">
                    <Skeleton className="h-5 w-24" />
                    <Skeleton className="h-5 flex-1 max-w-xs" />
                    <Skeleton className="h-5 w-16" />
                    <Skeleton className="h-5 w-32" />
                    <Skeleton className="size-8" />
                  </div>
                ))}
              </div>
            ) : keys.length === 0 ? (
              <div className="text-center py-20">
                <div className="size-20 rounded-2xl bg-muted/60 mx-auto flex items-center justify-center mb-5">
                  <Key className="size-10 text-muted-foreground/60" />
                </div>
                <h3 className="font-semibold text-lg">No API keys yet</h3>
                <p className="text-sm text-muted-foreground mt-2 mb-6 max-w-sm mx-auto">
                  Create an API key to authenticate requests to your proxy server.
                  You can use it with any OpenAI-compatible client.
                </p>
                <Button onClick={() => setGenerateOpen(true)}>
                  <Plus className="size-4" />
                  Generate Your First Key
                </Button>
              </div>
            ) : (
              <>
                {/* Search bar */}
                <div className="flex items-center gap-3 px-4 pt-4 pb-2">
                  <div className="relative flex-1 max-w-sm">
                    <Search className="absolute left-3 top-1/2 -translate-y-1/2 size-4 text-muted-foreground" />
                    <Input
                      placeholder="Search keys..."
                      value={searchQuery}
                      onChange={(e) => setSearchQuery(e.target.value)}
                      className="pl-9 h-9"
                    />
                  </div>
                  <span className="text-xs text-muted-foreground">
                    {filteredKeys.length} of {keys.length} keys
                  </span>
                </div>

                {filteredKeys.length === 0 ? (
                  <div className="text-center py-12 text-muted-foreground">
                    <Search className="size-8 mx-auto mb-3 opacity-50" />
                    <p className="text-sm">No keys match your search</p>
                  </div>
                ) : (
                  <>
                    {/* Desktop Table */}
                    <div className="hidden overflow-x-auto md:block">
                      <Table>
                        <TableHeader>
                          <TableRow>
                            <TableHead>Name</TableHead>
                            <TableHead>Key</TableHead>
                            <TableHead>Status</TableHead>
                            <TableHead>Created</TableHead>
                            <TableHead className="text-right">Actions</TableHead>
                          </TableRow>
                        </TableHeader>
                        <TableBody>
                          {filteredKeys.map((apiKey) => {
                            const isRevealed = revealedKeys.has(apiKey.id)
                            return (
                              <TableRow key={apiKey.id}>
                                <TableCell>
                                  <div className="flex items-center gap-2">
                                    <div className={cn(
                                      'size-2 rounded-full shrink-0',
                                      apiKey.isActive ? 'bg-emerald-500' : 'bg-gray-400'
                                    )} />
                                    <span className="font-medium">{apiKey.name}</span>
                                  </div>
                                </TableCell>
                                <TableCell>
                                  <div className="flex items-center gap-1">
                                    <code className="text-xs bg-muted px-2 py-1 rounded font-mono max-w-[280px] truncate select-all">
                                      {isRevealed ? apiKey.key : maskKey(apiKey.key)}
                                    </code>
                                    <Tooltip>
                                      <TooltipTrigger asChild>
                                        <Button
                                          variant="ghost"
                                          size="icon"
                                          className="size-7"
                                          onClick={() => toggleReveal(apiKey.id)}
                                        >
                                          {isRevealed ? <EyeOff className="size-3" /> : <Eye className="size-3" />}
                                        </Button>
                                      </TooltipTrigger>
                                      <TooltipContent>{isRevealed ? 'Hide key' : 'Reveal key'}</TooltipContent>
                                    </Tooltip>
                                    <CopyButton text={apiKey.key} label="Key" />
                                  </div>
                                </TableCell>
                                <TableCell>
                                  <Badge
                                    variant="outline"
                                    className={cn(
                                      'font-medium',
                                      apiKey.isActive
                                        ? 'bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 border-emerald-500/20'
                                        : 'bg-gray-500/10 text-gray-500 dark:text-gray-400 border-gray-500/20'
                                    )}
                                  >
                                    {apiKey.isActive ? 'Active' : 'Inactive'}
                                  </Badge>
                                </TableCell>
                                <TableCell className="text-muted-foreground text-sm">
                                  {formatDate(apiKey.createdAt)}
                                </TableCell>
                                <TableCell className="text-right">
                                  <div className="flex items-center justify-end gap-1">
                                    <Tooltip>
                                      <TooltipTrigger asChild>
                                        <Button
                                          variant="ghost"
                                          size="icon"
                                          className="size-8 text-destructive hover:text-destructive hover:bg-destructive/10"
                                          onClick={() => setDeleteTarget(apiKey)}
                                        >
                                          <Trash2 className="size-3.5" />
                                        </Button>
                                      </TooltipTrigger>
                                      <TooltipContent>Delete key</TooltipContent>
                                    </Tooltip>
                                  </div>
                                </TableCell>
                              </TableRow>
                            )
                          })}
                        </TableBody>
                      </Table>
                    </div>

                    {/* Mobile Cards */}
                    <div className="space-y-3 p-4 md:hidden">
                      {filteredKeys.map((apiKey) => {
                        const isRevealed = revealedKeys.has(apiKey.id)
                        return (
                          <div key={apiKey.id} className="rounded-lg border bg-card p-4 space-y-3">
                            <div className="flex items-start justify-between gap-3">
                              <div className="flex items-center gap-2 min-w-0">
                                <div className={cn(
                                  'size-2 rounded-full shrink-0 mt-1.5',
                                  apiKey.isActive ? 'bg-emerald-500' : 'bg-gray-400'
                                )} />
                                <div className="min-w-0">
                                  <p className="font-medium truncate">{apiKey.name}</p>
                                  <p className="text-xs text-muted-foreground mt-0.5">{formatDate(apiKey.createdAt)}</p>
                                </div>
                              </div>
                              <Badge
                                variant="outline"
                                className={cn(
                                  'shrink-0 font-medium',
                                  apiKey.isActive
                                    ? 'bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 border-emerald-500/20'
                                    : 'bg-gray-500/10 text-gray-500 dark:text-gray-400 border-gray-500/20'
                                )}
                              >
                                {apiKey.isActive ? 'Active' : 'Inactive'}
                              </Badge>
                            </div>

                            <div className="flex items-center gap-1.5">
                              <code className="min-w-0 flex-1 truncate rounded bg-muted px-2 py-1.5 font-mono text-xs select-all">
                                {isRevealed ? apiKey.key : maskKey(apiKey.key)}
                              </code>
                              <Button
                                variant="ghost"
                                size="icon"
                                className="size-7 shrink-0"
                                onClick={() => toggleReveal(apiKey.id)}
                              >
                                {isRevealed ? <EyeOff className="size-3" /> : <Eye className="size-3" />}
                              </Button>
                              <CopyButton text={apiKey.key} label="Key" />
                            </div>

                            <div className="flex items-center justify-end gap-1 pt-1 border-t">
                              <Button
                                variant="ghost"
                                size="sm"
                                className="h-7 text-xs text-destructive hover:text-destructive hover:bg-destructive/10"
                                onClick={() => setDeleteTarget(apiKey)}
                              >
                                <Trash2 className="size-3" />
                                Delete
                              </Button>
                            </div>
                          </div>
                        )
                      })}
                    </div>
                  </>
                )}
              </>
            )}
          </CardContent>
        </Card>
      </motion.div>

      {/* Usage Tip */}
      {keys.length > 0 && (
        <motion.div variants={itemVariants}>
          <Card className="border-dashed">
            <CardContent className="flex items-start gap-3 p-4">
              <div className="size-8 rounded-lg bg-blue-500/10 flex items-center justify-center shrink-0 mt-0.5">
                <Key className="size-4 text-blue-500" />
              </div>
              <div className="text-sm">
                <p className="font-medium">Using your API key</p>
                <p className="text-muted-foreground mt-1">
                  Pass your key as <code className="bg-muted px-1.5 py-0.5 rounded text-xs font-mono">Authorization: Bearer sk-dnt-...</code> header
                  with any OpenAI-compatible request to <code className="bg-muted px-1.5 py-0.5 rounded text-xs font-mono">/v1/chat/completions</code>.
                </p>
              </div>
            </CardContent>
          </Card>
        </motion.div>
      )}

      {/* Generate Key Dialog */}
      <Dialog open={generateOpen} onOpenChange={setGenerateOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Generate New API Key</DialogTitle>
            <DialogDescription>
              Create a new API key for authenticating requests to your proxy server.
              The full key will only be shown once after generation.
            </DialogDescription>
          </DialogHeader>
          <div className="space-y-4 py-2">
            <div className="space-y-2">
              <Label htmlFor="keyName">Key Name</Label>
              <Input
                id="keyName"
                placeholder="e.g., Production App, Dev Testing"
                value={newKeyName}
                onChange={(e) => setNewKeyName(e.target.value)}
                onKeyDown={(e) => {
                  if (e.key === 'Enter') handleGenerate()
                }}
                autoFocus
              />
              <p className="text-xs text-muted-foreground">
                A descriptive name to help you identify this key later
              </p>
            </div>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => { setGenerateOpen(false); setNewKeyName('') }}>
              Cancel
            </Button>
            <Button onClick={handleGenerate} disabled={generating || !newKeyName.trim()}>
              {generating ? (
                <>
                  <Loader2 className="size-4 animate-spin" />
                  Generating...
                </>
              ) : (
                <>
                  <Key className="size-4" />
                  Generate Key
                </>
              )}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Show Created Key Dialog */}
      <Dialog open={showKeyOpen} onOpenChange={setShowKeyOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle className="flex items-center gap-2">
              <div className="size-8 rounded-lg bg-emerald-500/10 flex items-center justify-center">
                <ShieldCheck className="size-4 text-emerald-600" />
              </div>
              Key Generated Successfully
            </DialogTitle>
            <DialogDescription>
              Key <strong>&quot;{createdKeyName}&quot;</strong> has been created. Copy the key below and store it safely.
            </DialogDescription>
          </DialogHeader>
          <div className="space-y-4 py-2">
            <div className="space-y-2">
              <Label>API Key</Label>
              <div className="flex items-center gap-2">
                <code className="flex-1 text-sm bg-muted px-3 py-2.5 rounded-md font-mono break-all select-all">
                  {createdKey}
                </code>
                <Button
                  variant="outline"
                  size="icon"
                  className="shrink-0"
                  onClick={() => copyToClipboard(createdKey, 'API key')}
                >
                  <Copy className="size-4" />
                </Button>
              </div>
            </div>
            <div className="flex items-start gap-2 rounded-lg border border-amber-500/30 bg-amber-500/5 p-3">
              <ShieldCheck className="size-4 text-amber-600 shrink-0 mt-0.5" />
              <p className="text-xs text-amber-700 dark:text-amber-400">
                <strong>Important:</strong> This is the only time the full API key will be displayed.
                Make sure to copy it now. The list screen only shows a masked value.
              </p>
            </div>
          </div>
          <DialogFooter>
            <Button
              onClick={() => {
                copyToClipboard(createdKey, 'API key')
              }}
              variant="outline"
              className="gap-2"
            >
              <Copy className="size-4" />
              Copy Key
            </Button>
            <Button onClick={() => setShowKeyOpen(false)} className="gap-2">
              <Check className="size-4" />
              Done
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Delete Confirmation */}
      <AlertDialog open={!!deleteTarget} onOpenChange={(open) => { if (!open) setDeleteTarget(null) }}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Delete API Key</AlertDialogTitle>
            <AlertDialogDescription>
              Are you sure you want to delete the key <strong>&quot;{deleteTarget?.name}&quot;</strong>? This action
              cannot be undone. Any applications using this key will immediately lose access.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={deleting}>Cancel</AlertDialogCancel>
            <AlertDialogAction
              onClick={(e) => {
                e.preventDefault()
                handleDelete()
              }}
              disabled={deleting}
              className="bg-destructive text-white hover:bg-destructive/90"
            >
              {deleting ? (
                <>
                  <Loader2 className="size-4 animate-spin" />
                  Deleting...
                </>
              ) : (
                <>
                  <Trash2 className="size-4" />
                  Delete
                </>
              )}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </motion.div>
  )
}
