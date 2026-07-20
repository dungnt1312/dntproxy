import { useEffect, useState, useCallback } from 'react'
import { useNavigate } from 'react-router-dom'
import { motion } from 'framer-motion'
import {
  Plus,
  Building2,
  Key,
  Edit3,
  Trash2,
  Ban,
  CheckCircle2,
  Loader2,
  ShieldAlert,
  KeyRound,
} from 'lucide-react'
import { Card, CardContent } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Skeleton } from '@/components/ui/skeleton'
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from '@/components/ui/tooltip'
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
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { toast } from 'sonner'
import { goApi } from '@/lib/go-api'
import { useAppStore } from '@/stores/app-store'
import { cn } from '@/lib/utils'
import { TenantFormDialog } from './tenants/tenant-form-dialog'
import { TenantKeyDialog } from './tenants/tenant-key-dialog'
import { TenantKeysDialog } from './tenants/tenant-keys-dialog'
import { CopyButton } from './api-keys/copy-button'
import type { Tenant, TenantCreatePayload, TenantUpdatePayload } from './tenants/types'

const containerVariants = {
  hidden: { opacity: 0 },
  visible: { opacity: 1, transition: { staggerChildren: 0.06 } },
}
const itemVariants = {
  hidden: { opacity: 0, y: 16 },
  visible: { opacity: 1, y: 0, transition: { duration: 0.35 } },
}

function formatDate(dateStr?: string) {
  if (!dateStr) return '—'
  const d = new Date(dateStr)
  if (Number.isNaN(d.getTime())) return dateStr
  return d.toLocaleDateString(undefined, {
    month: 'short',
    day: 'numeric',
    year: 'numeric',
  })
}

export default function TenantsScreen() {
  const navigate = useNavigate()
  const session = useAppStore((s) => s.session)
  const [tenants, setTenants] = useState<Tenant[]>([])
  const [loading, setLoading] = useState(true)
  const [createOpen, setCreateOpen] = useState(false)
  const [editTarget, setEditTarget] = useState<Tenant | null>(null)
  const [keyTarget, setKeyTarget] = useState<Tenant | null>(null)
  const [viewKeysTarget, setViewKeysTarget] = useState<Tenant | null>(null)
  const [deleteTarget, setDeleteTarget] = useState<Tenant | null>(null)
  const [deleting, setDeleting] = useState(false)
  // Holds the auto-generated default key shown once after tenant creation.
  const [newTenantKey, setNewTenantKey] = useState<{ tenantName: string; key: string } | null>(null)

  const fetchTenants = useCallback(async () => {
    try {
      setLoading(true)
      const data = await goApi.getTenants()
      setTenants(data)
    } catch {
      toast.error('Failed to load tenants')
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    // Admin-only gate: redirect non-admins back to dashboard.
    if (session && !session.isAdmin) {
      navigate('/', { replace: true })
      return
    }
    if (session?.isAdmin) {
      fetchTenants()
    }
  }, [session, navigate, fetchTenants])

  const handleCreate = async (payload: TenantCreatePayload | TenantUpdatePayload) => {
    try {
      const res = await goApi.createTenant(payload as TenantCreatePayload)
      toast.success('Tenant created')
      await fetchTenants()
      // Show the auto-generated default key once — it cannot be recovered later.
      if (res?.defaultKey?.key) {
        setNewTenantKey({
          tenantName: (payload as TenantCreatePayload).name || (payload as TenantCreatePayload).slug,
          key: res.defaultKey.key,
        })
      }
    } catch (e) {
      toast.error(e instanceof Error ? e.message : 'Failed to create tenant')
      throw e
    }
  }

  const handleUpdate = async (payload: TenantCreatePayload | TenantUpdatePayload) => {
    if (!editTarget) return
    try {
      await goApi.updateTenant(editTarget.id, payload as TenantUpdatePayload)
      toast.success('Tenant updated')
      setEditTarget(null)
      await fetchTenants()
    } catch (e) {
      toast.error(e instanceof Error ? e.message : 'Failed to update tenant')
      throw e
    }
  }

  const handleGenerateKey = async (payload: {
    name: string
    dashboardAccess?: boolean
  }) => {
    if (!keyTarget) return null
    try {
      const res = await goApi.generateTenantKey(keyTarget.id, payload)
      toast.success(`Key generated for ${keyTarget.slug}`)
      await fetchTenants()
      return { key: res.key }
    } catch (e) {
      toast.error(e instanceof Error ? e.message : 'Failed to generate key')
      throw e
    }
  }

  const handleDelete = async () => {
    if (!deleteTarget) return
    try {
      setDeleting(true)
      await goApi.deleteTenant(deleteTarget.id, false)
      toast.success(`Tenant "${deleteTarget.slug}" deleted`)
      setDeleteTarget(null)
      await fetchTenants()
    } catch (e) {
      toast.error(e instanceof Error ? e.message : 'Failed to delete tenant')
    } finally {
      setDeleting(false)
    }
  }

  const toggleStatus = async (t: Tenant) => {
    const next = t.status === 'disabled' ? 'active' : 'disabled'
    try {
      await goApi.updateTenant(t.id, { status: next })
      toast.success(`Tenant ${next === 'disabled' ? 'disabled' : 'enabled'}`)
      await fetchTenants()
    } catch (e) {
      toast.error(e instanceof Error ? e.message : 'Failed to update tenant')
    }
  }

  // Still loading session → blank (App.tsx shows loading state).
  if (!session) return null

  return (
    <motion.div
      className="space-y-6"
      variants={containerVariants}
      initial="hidden"
      animate="visible"
    >
      <motion.div variants={itemVariants} className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-4">
        <div>
          <h1 className="text-2xl font-bold tracking-tight">Tenants</h1>
          <p className="text-muted-foreground mt-1 text-sm">
            Admin-only. Register tenants, provision API keys, and disable access.
          </p>
        </div>
        <Button onClick={() => setCreateOpen(true)} className="self-start shrink-0">
          <Plus className="size-4" />
          New Tenant
        </Button>
      </motion.div>

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
                    <Skeleton className="h-5 w-32" />
                    <Skeleton className="size-8" />
                  </div>
                ))}
              </div>
            ) : tenants.length === 0 ? (
              <div className="text-center py-20">
                <div className="size-20 rounded-2xl bg-muted/60 mx-auto flex items-center justify-center mb-5">
                  <Building2 className="size-10 text-muted-foreground/60" />
                </div>
                <h3 className="font-semibold text-lg">No tenants yet</h3>
                <p className="text-sm text-muted-foreground mt-2 mb-6 max-w-sm mx-auto">
                  Register a tenant to provision API keys scoped to its own
                  connections, combos, and logs.
                </p>
                <Button onClick={() => setCreateOpen(true)}>
                  <Plus className="size-4" />
                  Create First Tenant
                </Button>
              </div>
            ) : (
              <div className="hidden overflow-x-auto md:block">
                <table className="w-full text-sm">
                  <thead>
                    <tr className="border-b text-left text-xs uppercase tracking-wider text-muted-foreground">
                      <th className="px-4 py-3 font-medium">Tenant</th>
                      <th className="px-4 py-3 font-medium">Status</th>
                      <th className="px-4 py-3 font-medium">Resources</th>
                      <th className="px-4 py-3 font-medium">Created</th>
                      <th className="px-4 py-3 text-right font-medium">Actions</th>
                    </tr>
                  </thead>
                  <tbody>
                    {tenants.map((t) => (
                      <tr key={t.id} className="border-b last:border-0 hover:bg-muted/30">
                        <td className="px-4 py-3">
                          <div className="flex flex-col gap-0.5">
                            <span className="font-medium">{t.name}</span>
                            <code className="text-xs text-muted-foreground font-mono">
                              {t.slug}
                            </code>
                            {t.notes && (
                              <span className="text-xs text-muted-foreground truncate max-w-xs">
                                {t.notes}
                              </span>
                            )}
                          </div>
                        </td>
                        <td className="px-4 py-3">
                          <StatusBadge status={t.status} />
                        </td>
                        <td className="px-4 py-3">
                          <div className="flex gap-3 text-xs text-muted-foreground">
                            <span title="Connections">{t.connections ?? 0} conns</span>
                            <span title="Combos">{t.combos ?? 0} combos</span>
                            <span title="API Keys">{t.keys ?? 0} keys</span>
                          </div>
                        </td>
                        <td className="px-4 py-3 text-muted-foreground">
                          {formatDate(t.createdAt)}
                        </td>
                        <td className="px-4 py-3 text-right">
                          <div className="flex items-center justify-end gap-1">
                            <ActionBtn
                              tooltip="View / Copy Keys"
                              onClick={() => setViewKeysTarget(t)}
                              icon={KeyRound}
                            />
                            <ActionBtn
                              tooltip="Generate Key"
                              onClick={() => setKeyTarget(t)}
                              icon={Key}
                            />
                            <ActionBtn
                              tooltip={t.status === 'disabled' ? 'Enable' : 'Disable'}
                              onClick={() => toggleStatus(t)}
                              icon={t.status === 'disabled' ? CheckCircle2 : Ban}
                              className={
                                t.status === 'disabled'
                                  ? 'text-emerald-600 hover:bg-emerald-500/10'
                                  : 'text-amber-600 hover:bg-amber-500/10'
                              }
                            />
                            <ActionBtn
                              tooltip="Edit"
                              onClick={() => setEditTarget(t)}
                              icon={Edit3}
                            />
                            <ActionBtn
                              tooltip="Delete"
                              onClick={() => setDeleteTarget(t)}
                              icon={Trash2}
                              className="text-destructive hover:bg-destructive/10"
                            />
                          </div>
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            )}

            {/* Mobile list */}
            {!loading && tenants.length > 0 && (
              <div className="md:hidden divide-y">
                {tenants.map((t) => (
                  <div key={t.id} className="p-4 space-y-2">
                    <div className="flex items-start justify-between gap-2">
                      <div className="min-w-0">
                        <p className="font-medium truncate">{t.name}</p>
                        <code className="text-xs text-muted-foreground font-mono">
                          {t.slug}
                        </code>
                      </div>
                      <StatusBadge status={t.status} />
                    </div>
                    <div className="flex gap-3 text-xs text-muted-foreground">
                      <span>{t.connections ?? 0} conns</span>
                      <span>{t.combos ?? 0} combos</span>
                      <span>{t.keys ?? 0} keys</span>
                    </div>
                    {t.notes && (
                      <p className="text-xs text-muted-foreground truncate">{t.notes}</p>
                    )}
                    <div className="flex gap-2 pt-1">
                      <Button size="sm" variant="outline" onClick={() => setViewKeysTarget(t)}>
                        <KeyRound className="size-3.5" /> Keys
                      </Button>
                      <Button size="sm" variant="outline" onClick={() => setKeyTarget(t)}>
                        <Key className="size-3.5" /> New
                      </Button>
                      <Button size="sm" variant="outline" onClick={() => toggleStatus(t)}>
                        {t.status === 'disabled' ? 'Enable' : 'Disable'}
                      </Button>
                      <Button size="sm" variant="outline" onClick={() => setEditTarget(t)}>
                        <Edit3 className="size-3.5" /> Edit
                      </Button>
                      <Button
                        size="sm"
                        variant="outline"
                        className="text-destructive"
                        onClick={() => setDeleteTarget(t)}
                      >
                        <Trash2 className="size-3.5" />
                      </Button>
                    </div>
                  </div>
                ))}
              </div>
            )}
          </CardContent>
        </Card>
      </motion.div>

      {/* Tip */}
      {!loading && tenants.length > 0 && (
        <motion.div variants={itemVariants}>
          <Card className="border-dashed">
            <CardContent className="flex items-start gap-3 p-4">
              <div className="size-8 rounded-lg bg-amber-500/10 flex items-center justify-center shrink-0 mt-0.5">
                <ShieldAlert className="size-4 text-amber-500" />
              </div>
              <div className="text-sm">
                <p className="font-medium">Disabling a tenant</p>
                <p className="text-muted-foreground mt-1">
                  Disabling a tenant immediately rejects every request using that
                  tenant's API keys with 403. Resources are preserved — re-enable
                  any time. Deleting (without cascade) leaves resources orphaned.
                </p>
              </div>
            </CardContent>
          </Card>
        </motion.div>
      )}

      <TenantFormDialog
        open={createOpen}
        onOpenChange={setCreateOpen}
        onSubmit={handleCreate}
      />
      <TenantFormDialog
        open={!!editTarget}
        onOpenChange={(o) => !o && setEditTarget(null)}
        tenant={editTarget}
        onSubmit={handleUpdate}
      />
      <TenantKeyDialog
        open={!!keyTarget}
        onOpenChange={(o) => !o && setKeyTarget(null)}
        tenant={keyTarget}
        onGenerate={handleGenerateKey}
      />
      <TenantKeysDialog
        open={!!viewKeysTarget}
        onOpenChange={(o) => !o && setViewKeysTarget(null)}
        tenant={viewKeysTarget}
      />

      <AlertDialog
        open={!!deleteTarget}
        onOpenChange={(o) => !o && setDeleteTarget(null)}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Delete Tenant</AlertDialogTitle>
            <AlertDialogDescription>
              Delete tenant <strong>&quot;{deleteTarget?.name}&quot;</strong> (slug{' '}
              <code className="bg-muted px-1 rounded text-xs">{deleteTarget?.slug}</code>)?
              Resources (connections, combos, keys) are kept by default. This action
              cannot be undone.
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

      {/* Default key reveal — shown once after tenant creation */}
      <Dialog open={!!newTenantKey} onOpenChange={(o) => !o && setNewTenantKey(null)}>
        <DialogContent className="max-w-lg">
          <DialogHeader>
            <DialogTitle>Tenant Created</DialogTitle>
            <DialogDescription>
              A default dashboard key has been generated for{' '}
              <strong>{newTenantKey?.tenantName}</strong>. Copy it now — it
              won't be shown again.
            </DialogDescription>
          </DialogHeader>
          <div className="flex items-center gap-2 rounded-md border bg-muted px-3 py-2 font-mono text-xs break-all">
            <span className="flex-1 select-all">{newTenantKey?.key}</span>
            <CopyButton text={newTenantKey?.key ?? ''} label="API key" />
          </div>
          <DialogFooter>
            <Button onClick={() => setNewTenantKey(null)}>Done</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

    </motion.div>
  )
}

function StatusBadge({ status }: { status: string }) {
  const isDisabled = status === 'disabled'
  return (
    <Badge
      variant="outline"
      className={cn(
        'font-medium',
        isDisabled
          ? 'bg-amber-500/10 text-amber-600 dark:text-amber-400 border-amber-500/20'
          : 'bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 border-emerald-500/20',
      )}
    >
      {isDisabled ? 'Disabled' : 'Active'}
    </Badge>
  )
}

interface ActionBtnProps {
  tooltip: string
  onClick: () => void
  icon: React.ElementType
  className?: string
}

function ActionBtn({ tooltip, onClick, icon: Icon, className }: ActionBtnProps) {
  return (
    <Tooltip delayDuration={0}>
      <TooltipTrigger asChild>
        <Button
          variant="ghost"
          size="icon"
          className={cn('size-8', className)}
          onClick={onClick}
        >
          <Icon className="size-3.5" />
        </Button>
      </TooltipTrigger>
      <TooltipContent>{tooltip}</TooltipContent>
    </Tooltip>
  )
}
