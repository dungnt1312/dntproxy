import { useState, useEffect } from 'react'
import { Loader2 } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'
import { Switch } from '@/components/ui/switch'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import type { Tenant, TenantCreatePayload, TenantUpdatePayload } from './types'

interface TenantFormDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  // When `tenant` is provided → edit mode; otherwise → create mode.
  tenant?: Tenant | null
  onSubmit: (payload: TenantCreatePayload | TenantUpdatePayload) => Promise<void>
}

export function TenantFormDialog({
  open,
  onOpenChange,
  tenant,
  onSubmit,
}: TenantFormDialogProps) {
  const isEdit = Boolean(tenant)
  const [slug, setSlug] = useState('')
  const [name, setName] = useState('')
  const [notes, setNotes] = useState('')
  const [active, setActive] = useState(true)
  const [submitting, setSubmitting] = useState(false)

  // Reset form whenever the dialog opens or the target tenant changes.
  useEffect(() => {
    if (open) {
      setSlug(tenant?.slug ?? '')
      setName(tenant?.name ?? '')
      setNotes(tenant?.notes ?? '')
      setActive(tenant?.status !== 'disabled')
    }
  }, [open, tenant])

  const handleSubmit = async () => {
    if (!isEdit && !slug.trim()) return
    try {
      setSubmitting(true)
      if (isEdit) {
        const payload: TenantUpdatePayload = {
          name: name.trim(),
          notes: notes.trim(),
          status: active ? 'active' : 'disabled',
        }
        await onSubmit(payload)
      } else {
        const payload: TenantCreatePayload = {
          slug: slug.trim(),
          name: name.trim() || undefined,
          notes: notes.trim() || undefined,
        }
        await onSubmit(payload)
      }
      onOpenChange(false)
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-lg">
        <DialogHeader>
          <DialogTitle>{isEdit ? 'Edit Tenant' : 'Create Tenant'}</DialogTitle>
          <DialogDescription>
            {isEdit
              ? 'Update tenant details or change its status.'
              : 'Register a new tenant. The slug becomes the API key prefix (e.g. "acme" → sk-dnt-acme-…).'}
          </DialogDescription>
        </DialogHeader>
        <div className="space-y-4 py-2">
          <div className="space-y-2">
            <Label htmlFor="tenantSlug">Slug</Label>
            <Input
              id="tenantSlug"
              placeholder="e.g., acme-corp"
              value={slug}
              onChange={(e) => setSlug(e.target.value)}
              disabled={isEdit}
              autoFocus={!isEdit}
            />
            <p className="text-xs text-muted-foreground">
              {isEdit
                ? 'Slug cannot be changed after creation.'
                : 'Lowercase letters, digits, hyphens. 2-32 chars. Reserved: global, admin, all, default.'}
            </p>
          </div>
          <div className="space-y-2">
            <Label htmlFor="tenantName">Display Name</Label>
            <Input
              id="tenantName"
              placeholder="e.g., ACME Corporation"
              value={name}
              onChange={(e) => setName(e.target.value)}
              autoFocus={isEdit}
            />
          </div>
          <div className="space-y-2">
            <Label htmlFor="tenantNotes">Notes (optional)</Label>
            <Textarea
              id="tenantNotes"
              placeholder="Internal notes about this tenant…"
              value={notes}
              onChange={(e) => setNotes(e.target.value)}
              rows={3}
            />
          </div>
          {isEdit && (
            <div className="flex items-center justify-between rounded-lg border p-3">
              <div className="space-y-0.5">
                <Label htmlFor="tenantActive">Active</Label>
                <p className="text-xs text-muted-foreground">
                  When disabled, every request using this tenant's API keys is
                  rejected with 403.
                </p>
              </div>
              <Switch
                id="tenantActive"
                checked={active}
                onCheckedChange={setActive}
              />
            </div>
          )}
        </div>
        <DialogFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)}>
            Cancel
          </Button>
          <Button
            onClick={handleSubmit}
            disabled={submitting || (!isEdit && !slug.trim())}
          >
            {submitting ? (
              <>
                <Loader2 className="size-4 animate-spin" />
                Saving...
              </>
            ) : isEdit ? (
              'Save Changes'
            ) : (
              'Create Tenant'
            )}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
