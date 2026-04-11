import { useEffect, useState, useCallback } from 'react'
import { motion } from 'framer-motion'
import {
  Plus,
  Copy,
  Trash2,
  Key,
  Loader2,
  ShieldCheck,
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

interface ApiKey {
  id: string
  name: string
  keyMasked: string
  isActive: boolean
  createdAt: string
  updatedAt: string
}

function formatDate(dateStr: string) {
  const date = new Date(dateStr)
  if (Number.isNaN(date.getTime())) return dateStr
  return date.toLocaleString()
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

export default function ApiKeysScreen() {
  const [keys, setKeys] = useState<ApiKey[]>([])
  const [loading, setLoading] = useState(true)
  const [generateOpen, setGenerateOpen] = useState(false)
  const [newKeyName, setNewKeyName] = useState('')
  const [generating, setGenerating] = useState(false)
  const [showKeyOpen, setShowKeyOpen] = useState(false)
  const [createdKey, setCreatedKey] = useState<string>('')
  const [createdKeyName, setCreatedKeyName] = useState('')
  const [deleteTarget, setDeleteTarget] = useState<ApiKey | null>(null)
  const [deleting, setDeleting] = useState(false)

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
          <h1 className="text-2xl font-bold">API Keys</h1>
          <p className="text-muted-foreground mt-1">
            Manage API keys for authenticating requests to your proxy
          </p>
        </div>
        <Button onClick={() => setGenerateOpen(true)} className="self-start">
          <Plus className="size-4" />
          Generate New Key
        </Button>
      </motion.div>

      {/* Keys Table */}
      <motion.div variants={itemVariants}>
        <Card>
          <CardContent className="p-0">
            {loading ? (
              <div className="p-6 space-y-4">
                {Array.from({ length: 3 }).map((_, i) => (
                  <div key={i} className="flex items-center gap-4">
                    <Skeleton className="h-5 w-24" />
                    <Skeleton className="h-5 flex-1 max-w-xs" />
                    <Skeleton className="h-5 w-16" />
                    <Skeleton className="h-5 w-16" />
                    <Skeleton className="size-8" />
                  </div>
                ))}
              </div>
            ) : keys.length === 0 ? (
              <div className="text-center py-16">
                <div className="size-16 rounded-full bg-muted mx-auto flex items-center justify-center mb-4">
                  <Key className="size-8 text-muted-foreground" />
                </div>
                <h3 className="font-semibold text-lg">No API keys yet</h3>
                <p className="text-sm text-muted-foreground mt-1 mb-4 max-w-sm mx-auto">
                  Create an API key to authenticate requests to your proxy server.
                </p>
                <Button onClick={() => setGenerateOpen(true)} variant="outline">
                  <Plus className="size-4" />
                  Generate New Key
                </Button>
              </div>
            ) : (
              <>
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
                    {keys.map((apiKey) => (
                      <TableRow key={apiKey.id}>
                        <TableCell className="font-medium">{apiKey.name}</TableCell>
                        <TableCell>
                          <div className="flex items-center gap-2">
                            <code className="text-xs bg-muted px-2 py-1 rounded font-mono max-w-[280px] truncate">
                              {apiKey.keyMasked}
                            </code>
                          </div>
                        </TableCell>
                        <TableCell>
                          <Badge
                            variant="outline"
                            className={
                              apiKey.isActive
                                ? 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-400 border-emerald-200 dark:border-emerald-800'
                                : 'bg-gray-100 text-gray-600 dark:bg-gray-800 dark:text-gray-400 border-gray-200 dark:border-gray-700'
                            }
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
                    ))}
                  </TableBody>
                </Table>
              </div>

              <div className="space-y-3 p-4 md:hidden">
                {keys.map((apiKey) => (
                  <div key={apiKey.id} className="space-y-3 rounded-lg border p-3">
                    <div className="flex items-start justify-between gap-3">
                      <div className="min-w-0">
                        <p className="truncate font-medium">{apiKey.name}</p>
                        <p className="text-xs text-muted-foreground">Created {formatDate(apiKey.createdAt)}</p>
                      </div>
                      <Badge
                        variant="outline"
                        className={
                          apiKey.isActive
                            ? 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-400 border-emerald-200 dark:border-emerald-800'
                            : 'bg-gray-100 text-gray-600 dark:bg-gray-800 dark:text-gray-400 border-gray-200 dark:border-gray-700'
                        }
                      >
                        {apiKey.isActive ? 'Active' : 'Inactive'}
                      </Badge>
                    </div>

                    <div className="flex items-center gap-2">
                      <code className="min-w-0 flex-1 truncate rounded bg-muted px-2 py-1 font-mono text-xs">
                        {apiKey.keyMasked}
                      </code>
                    </div>

                    <div className="flex items-center justify-between gap-2 text-xs text-muted-foreground">
                      <span>Masked by backend</span>
                      <Button
                        variant="ghost"
                        size="icon"
                        className="size-8 text-destructive hover:text-destructive hover:bg-destructive/10"
                        onClick={() => setDeleteTarget(apiKey)}
                      >
                        <Trash2 className="size-3.5" />
                      </Button>
                    </div>
                  </div>
                ))}
              </div>
              </>
            )}
          </CardContent>
        </Card>
      </motion.div>

      {/* Generate Key Dialog */}
      <Dialog open={generateOpen} onOpenChange={setGenerateOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Generate New API Key</DialogTitle>
            <DialogDescription>
              Create a new API key. The backend only returns the full key once after generation.
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
              <ShieldCheck className="size-5 text-emerald-600" />
              Key Generated Successfully
            </DialogTitle>
            <DialogDescription>
              Key &quot;{createdKeyName}&quot; has been created. Copy the key below and store it safely.
            </DialogDescription>
          </DialogHeader>
          <div className="space-y-4 py-2">
            <div className="space-y-2">
              <Label>API Key</Label>
              <div className="flex items-center gap-2">
                <code className="flex-1 text-sm bg-muted px-3 py-2 rounded-md font-mono break-all">
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
            <p className="text-xs text-amber-700 dark:text-amber-400">
              <strong>Important:</strong> This is the only time the full API key will be displayed.
              Make sure to copy it now. The list screen only shows the backend-masked value.
            </p>
          </div>
          <DialogFooter>
            <Button onClick={() => setShowKeyOpen(false)} className="w-full sm:w-auto sm:min-w-[100px]">
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
              Are you sure you want to delete the key &quot;{deleteTarget?.name}&quot;? This action
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
