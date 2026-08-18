import { useState } from 'react'
import { Key, Loader2 } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Switch } from '@/components/ui/switch'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { PermissionsEditor } from './permissions-editor'
import { useAppStore } from '@/stores/app-store'
import type { ApiKeyCreatePayload, ConnectionOption, ModelOption } from './types'

interface GenerateDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  onGenerate: (payload: ApiKeyCreatePayload) => Promise<void>
  connections: ConnectionOption[]
  models: ModelOption[]
}

export function GenerateDialog({ open, onOpenChange, onGenerate, connections, models }: GenerateDialogProps) {
  const session = useAppStore((s) => s.session)
  const isAdmin = Boolean(session?.isAdmin)
  const [keyName, setKeyName] = useState('')
  const [dashboardAccess, setDashboardAccess] = useState(false)
  const [restricted, setRestricted] = useState(false)
  const [allowedConnectionIds, setAllowedConnectionIds] = useState<string[]>([])
  const [allowedModels, setAllowedModels] = useState<string[]>([])
  const [tenantId, setTenantId] = useState('')
  const [generating, setGenerating] = useState(false)
  const restrictedWithoutSelection = restricted && allowedConnectionIds.length === 0 && allowedModels.length === 0

  const handleGenerate = async () => {
    if (!keyName.trim()) return

    try {
      setGenerating(true)
      await onGenerate({
        name: keyName.trim(),
        dashboardAccess,
        allowedConnectionIds: restricted ? allowedConnectionIds : [],
        allowedModels: restricted ? allowedModels : [],
        tenantId: buildTenantIdForNewKey(tenantId, session),
      })
      setKeyName('')
      setDashboardAccess(false)
      setRestricted(false)
      setAllowedConnectionIds([])
      setAllowedModels([])
      setTenantId('')
    } finally {
      setGenerating(false)
    }
  }

  const handleClose = () => {
    onOpenChange(false)
    setKeyName('')
    setDashboardAccess(false)
    setRestricted(false)
    setAllowedConnectionIds([])
    setAllowedModels([])
    setTenantId('')
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-3xl">
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
              value={keyName}
              onChange={(e) => setKeyName(e.target.value)}
              onKeyDown={(e) => {
                if (e.key === 'Enter') handleGenerate()
              }}
              autoFocus
            />
            <p className="text-xs text-muted-foreground">
              A descriptive name to help you identify this key later
            </p>
          </div>

          {/* Tenant assignment — only admins can pick a tenant for the new key.
              Non-admins inherit their own tenant automatically. */}
          {isAdmin ? (
            <div className="space-y-2">
              <Label htmlFor="tenantId">Tenant ID (optional)</Label>
              <Input
                id="tenantId"
                placeholder="e.g., acme-corp (leave blank for admin/global key)"
                value={tenantId}
                onChange={(e) => setTenantId(e.target.value)}
              />
              <p className="text-xs text-muted-foreground">
                Assign this key to a tenant to scope its access. Leave blank to create
                an admin (global) key that can see all tenants.
              </p>
            </div>
          ) : null}

          {isAdmin ? (
          <div className="flex items-center justify-between rounded-lg border p-3">
            <div className="space-y-0.5">
              <Label htmlFor="dashboardAccess">Dashboard Access</Label>
              <p className="text-xs text-muted-foreground">
                Allow this key to access the management dashboard
              </p>
            </div>
            <Switch
              id="dashboardAccess"
              checked={dashboardAccess}
              onCheckedChange={setDashboardAccess}
            />
          </div>
          ) : null}
          <PermissionsEditor
            value={{ allowedConnectionIds, allowedModels }}
            onChange={(next) => {
              setAllowedConnectionIds(next.allowedConnectionIds)
              setAllowedModels(next.allowedModels)
            }}
            restricted={restricted}
            onRestrictedChange={setRestricted}
            connections={connections}
            models={models}
          />
          {restrictedWithoutSelection && (
            <p className="text-xs text-destructive">Select at least one connection or model, or use unrestricted access.</p>
          )}
        </div>
        <DialogFooter>
          <Button variant="outline" onClick={handleClose}>
            Cancel
          </Button>
          <Button onClick={handleGenerate} disabled={generating || !keyName.trim() || restrictedWithoutSelection}>
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
  )
}

/**
 * buildTenantIdForNewKey decides the tenantId to send when creating a new key.
 *
 * - Admins can type any tenant id (or leave blank for a global admin key).
 * - Non-admins are forced to their own tenant id (cannot escalate to admin
 *   or impersonate another tenant).
 */
function buildTenantIdForNewKey(
  input: string,
  session: { tenantId?: string; isAdmin?: boolean } | null,
): string | undefined {
  if (session?.isAdmin) {
    const trimmed = input.trim()
    return trimmed === '' ? undefined : trimmed
  }
  // Non-admin: pin to own tenant.
  return session?.tenantId || undefined
}

