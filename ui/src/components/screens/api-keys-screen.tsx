import { useEffect, useState, useMemo, useCallback } from 'react'
import { motion } from 'framer-motion'
import {
  Plus,
  Trash2,
  Key,
  ShieldCheck,
  Shield,
  Search,
  Loader2,
} from 'lucide-react'
import { Card, CardContent } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Skeleton } from '@/components/ui/skeleton'
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
import { StatCard } from './api-keys/stat-card'
import { KeysTable } from './api-keys/keys-table'
import { KeysMobile } from './api-keys/keys-mobile'
import { GenerateDialog } from './api-keys/generate-dialog'
import { ShowKeyDialog } from './api-keys/show-key-dialog'
import { EditKeyDialog } from './api-keys/edit-key-dialog'
import type { ApiKey, ApiKeyCreatePayload, ApiKeyUpdatePayload, ConnectionOption, ModelOption } from './api-keys/types'

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

export default function ApiKeysScreen() {
  const [keys, setKeys] = useState<ApiKey[]>([])
  const [connections, setConnections] = useState<ConnectionOption[]>([])
  const [models, setModels] = useState<ModelOption[]>([])
  const [loading, setLoading] = useState(true)
  const [searchQuery, setSearchQuery] = useState('')
  const [generateOpen, setGenerateOpen] = useState(false)
  const [showKeyOpen, setShowKeyOpen] = useState(false)
  const [createdKey, setCreatedKey] = useState('')
  const [createdKeyName, setCreatedKeyName] = useState('')
  const [deleteTarget, setDeleteTarget] = useState<ApiKey | null>(null)
  const [editTarget, setEditTarget] = useState<ApiKey | null>(null)
  const [deleting, setDeleting] = useState(false)
  const [revealedKeys, setRevealedKeys] = useState<Set<string>>(new Set())

  const fetchData = useCallback(async () => {
    try {
      setLoading(true)
      const [keysData, connectionsData, modelsData] = await Promise.all([
        goApi.getKeys(),
        goApi.getConnections(),
        goApi.getModels(),
      ])
      setKeys(keysData)
      setConnections(Array.isArray(connectionsData) ? connectionsData : [])
      setModels(Array.isArray(modelsData) ? modelsData : [])
    } catch {
      toast.error('Failed to load API key data')
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    fetchData()
  }, [fetchData])

  const filteredKeys = useMemo(() => {
    const q = searchQuery.trim().toLowerCase()
    if (!q) return keys
    return keys.filter((k) =>
      k.name.toLowerCase().includes(q) ||
      k.key.toLowerCase().includes(q) ||
      k.allowedConnectionIds.some((id) => id.toLowerCase().includes(q)) ||
      k.allowedModels.some((model) => model.toLowerCase().includes(q))
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

  const handleGenerate = async (payload: ApiKeyCreatePayload) => {
    const json = await goApi.createKey(payload)
    setCreatedKey(json.key)
    setCreatedKeyName(json.name)
    setGenerateOpen(false)
    setShowKeyOpen(true)
    toast.success('API key generated successfully')
    await fetchData()
  }

  const handleUpdate = async (id: string, payload: ApiKeyUpdatePayload) => {
    try {
      await goApi.updateKey(id, payload)
      await fetchData()
      toast.success('API key updated')
    } catch (error) {
      toast.error(error instanceof Error ? error.message : 'Failed to update API key')
      throw error
    }
  }

  const handleDelete = async () => {
    if (!deleteTarget) return
    try {
      setDeleting(true)
      await goApi.deleteKey(deleteTarget.id)
      setDeleteTarget(null)
      await fetchData()
      toast.success(`Key "${deleteTarget.name}" deleted`)
    } catch {
      toast.error('Failed to delete API key')
    } finally {
      setDeleting(false)
    }
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
                    <KeysTable
                      keys={filteredKeys}
                      revealedKeys={revealedKeys}
                      onToggleReveal={toggleReveal}
                      onDelete={setDeleteTarget}
                      onEdit={setEditTarget}
                      formatDate={formatDate}
                      maskKey={maskKey}
                    />
                    <KeysMobile
                      keys={filteredKeys}
                      revealedKeys={revealedKeys}
                      onToggleReveal={toggleReveal}
                      onDelete={setDeleteTarget}
                      onEdit={setEditTarget}
                      formatDate={formatDate}
                      maskKey={maskKey}
                    />
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

      {/* Dialogs */}
      <GenerateDialog
        open={generateOpen}
        onOpenChange={setGenerateOpen}
        onGenerate={handleGenerate}
        connections={connections}
        models={models}
      />

      <EditKeyDialog
        apiKey={editTarget}
        open={!!editTarget}
        onOpenChange={(open) => { if (!open) setEditTarget(null) }}
        onSave={handleUpdate}
        connections={connections}
        models={models}
      />

      <ShowKeyDialog
        open={showKeyOpen}
        onOpenChange={setShowKeyOpen}
        keyValue={createdKey}
        keyName={createdKeyName}
      />

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
