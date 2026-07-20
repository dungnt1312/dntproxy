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
import { CopyButton } from '../api-keys/copy-button'
import type { Tenant, TenantKeyPayload } from './types'

interface TenantKeyDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  tenant: Tenant | null
  onGenerate: (payload: TenantKeyPayload) => Promise<{ key: string } | null>
}

export function TenantKeyDialog({
  open,
  onOpenChange,
  tenant,
  onGenerate,
}: TenantKeyDialogProps) {
  const [keyName, setKeyName] = useState('')
  const [dashboardAccess, setDashboardAccess] = useState(false)
  const [generating, setGenerating] = useState(false)
  const [generatedKey, setGeneratedKey] = useState('')

  const handleClose = () => {
    onOpenChange(false)
    setKeyName('')
    setDashboardAccess(false)
    setGeneratedKey('')
  }

  const handleGenerate = async () => {
    if (!keyName.trim() || !tenant) return
    try {
      setGenerating(true)
      const result = await onGenerate({
        name: keyName.trim(),
        dashboardAccess,
      })
      if (result?.key) {
        setGeneratedKey(result.key)
      }
    } finally {
      setGenerating(false)
    }
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-lg">
        <DialogHeader>
          <DialogTitle>Generate Key for {tenant?.name ?? 'Tenant'}</DialogTitle>
          <DialogDescription>
            {generatedKey
              ? 'Copy the key now — it will not be shown again.'
              : `The new key will be pinned to tenant "${tenant?.slug}" and scoped to its resources.`}
          </DialogDescription>
        </DialogHeader>

        {generatedKey ? (
          <div className="space-y-3 py-2">
            <div className="flex items-center gap-2">
              <code className="flex-1 text-xs bg-muted px-3 py-2 rounded font-mono break-all select-all">
                {generatedKey}
              </code>
              <CopyButton text={generatedKey} label="Key" />
            </div>
            <p className="text-xs text-muted-foreground">
              Share this key securely with the tenant. They will use it as the
              Bearer token for all proxy and dashboard requests.
            </p>
          </div>
        ) : (
          <div className="space-y-4 py-2">
            <div className="space-y-2">
              <Label htmlFor="tenantKeyName">Key Name</Label>
              <Input
                id="tenantKeyName"
                placeholder="e.g., Production, Dev"
                value={keyName}
                onChange={(e) => setKeyName(e.target.value)}
                autoFocus
              />
            </div>
            <div className="flex items-center justify-between rounded-lg border p-3">
              <div className="space-y-0.5">
                <Label htmlFor="tenantKeyDashboard">Dashboard Access</Label>
                <p className="text-xs text-muted-foreground">
                  Allow this key to access the management dashboard (scoped to
                  this tenant's resources only).
                </p>
              </div>
              <Switch
                id="tenantKeyDashboard"
                checked={dashboardAccess}
                onCheckedChange={setDashboardAccess}
              />
            </div>
          </div>
        )}

        <DialogFooter>
          {generatedKey ? (
            <Button onClick={handleClose}>Done</Button>
          ) : (
            <>
              <Button variant="outline" onClick={handleClose}>
                Cancel
              </Button>
              <Button
                onClick={handleGenerate}
                disabled={generating || !keyName.trim()}
              >
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
            </>
          )}
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
