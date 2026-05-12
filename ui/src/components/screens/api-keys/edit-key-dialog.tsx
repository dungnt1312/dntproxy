import { useEffect, useState } from 'react'
import { Loader2, Save } from 'lucide-react'
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
import type { ApiKey, ApiKeyUpdatePayload, ConnectionOption, ModelOption } from './types'

interface EditKeyDialogProps {
  apiKey: ApiKey | null
  open: boolean
  onOpenChange: (open: boolean) => void
  onSave: (id: string, payload: ApiKeyUpdatePayload) => Promise<void>
  connections: ConnectionOption[]
  models: ModelOption[]
}

export function EditKeyDialog({ apiKey, open, onOpenChange, onSave, connections, models }: EditKeyDialogProps) {
  const [name, setName] = useState('')
  const [isActive, setIsActive] = useState(true)
  const [restricted, setRestricted] = useState(false)
  const [allowedConnectionIds, setAllowedConnectionIds] = useState<string[]>([])
  const [allowedModels, setAllowedModels] = useState<string[]>([])
  const [saving, setSaving] = useState(false)
  const restrictedWithoutSelection = restricted && allowedConnectionIds.length === 0 && allowedModels.length === 0

  useEffect(() => {
    if (!apiKey) return
    setName(apiKey.name)
    setIsActive(apiKey.isActive)
    setAllowedConnectionIds(apiKey.allowedConnectionIds)
    setAllowedModels(apiKey.allowedModels)
    setRestricted(apiKey.allowedConnectionIds.length > 0 || apiKey.allowedModels.length > 0)
  }, [apiKey])

  const handleSave = async () => {
    if (!apiKey || !name.trim()) return

    try {
      setSaving(true)
      await onSave(apiKey.id, {
        name: name.trim(),
        isActive,
        allowedConnectionIds: restricted ? allowedConnectionIds : [],
        allowedModels: restricted ? allowedModels : [],
      })
      onOpenChange(false)
    } finally {
      setSaving(false)
    }
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-3xl">
        <DialogHeader>
          <DialogTitle>Edit API Key</DialogTitle>
          <DialogDescription>Update key metadata and request routing permissions.</DialogDescription>
        </DialogHeader>
        <div className="space-y-5 py-2">
          <div className="grid gap-4 sm:grid-cols-[1fr_auto]">
            <div className="space-y-2">
              <Label htmlFor="editKeyName">Key Name</Label>
              <Input id="editKeyName" value={name} onChange={(event) => setName(event.target.value)} />
            </div>
            <div className="flex items-end gap-3 rounded-md border px-3 py-2">
              <Switch id="editKeyActive" checked={isActive} onCheckedChange={setIsActive} />
              <Label htmlFor="editKeyActive" className="pb-0.5">{isActive ? 'Active' : 'Inactive'}</Label>
            </div>
          </div>
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
          <Button variant="outline" onClick={() => onOpenChange(false)} disabled={saving}>Cancel</Button>
          <Button onClick={handleSave} disabled={saving || !name.trim() || restrictedWithoutSelection}>
            {saving ? <Loader2 className="size-4 animate-spin" /> : <Save className="size-4" />}
            Save
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
