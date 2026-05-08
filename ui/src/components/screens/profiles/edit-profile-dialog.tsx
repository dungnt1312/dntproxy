import { useState, useEffect } from 'react'
import { toast } from 'sonner'

import { goApi } from '@/lib/go-api'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { AliasEditor } from './alias-editor'
import type { ProfileData, AliasRow } from './types'

type EditProfileDialogProps = {
  open: boolean
  profile: ProfileData | null
  activeProfile: string
  onOpenChange: (open: boolean) => void
  onSuccess: () => void
}

export function EditProfileDialog({
  open,
  profile,
  activeProfile,
  onOpenChange,
  onSuccess,
}: EditProfileDialogProps) {
  const [formAliases, setFormAliases] = useState<AliasRow[]>([{ alias: '', model: '' }])
  const [saving, setSaving] = useState(false)

  useEffect(() => {
    if (profile) {
      const aliasRows = Object.entries(profile.aliases || {}).map(([alias, model]) => ({
        alias,
        model,
      }))
      setFormAliases(aliasRows.length > 0 ? aliasRows : [{ alias: '', model: '' }])
    }
  }, [profile])

  function addAliasRow() {
    setFormAliases((prev) => [...prev, { alias: '', model: '' }])
  }

  function removeAliasRow(index: number) {
    setFormAliases((prev) => prev.filter((_, i) => i !== index))
  }

  function updateAliasRow(index: number, field: 'alias' | 'model', value: string) {
    setFormAliases((prev) => prev.map((row, i) => (i === index ? { ...row, [field]: value } : row)))
  }

  async function handleEdit() {
    if (!profile) return

    const addAliases: Record<string, string> = {}
    const currentAliasKeys = new Set<string>()

    for (const row of formAliases) {
      const a = row.alias.trim()
      const m = row.model.trim()
      if (a && m) {
        addAliases[a] = m
        currentAliasKeys.add(a)
      }
    }

    // Find removed aliases
    const removeAliases = Object.keys(profile.aliases || {}).filter(
      (key) => !currentAliasKeys.has(key),
    )

    setSaving(true)
    try {
      await goApi.updateProfile(profile.name, { addAliases, removeAliases })
      toast.success(`Profile "${profile.name}" updated`)
      onOpenChange(false)
      onSuccess()
    } catch (error) {
      toast.error(error instanceof Error ? error.message : 'Failed to update profile')
    } finally {
      setSaving(false)
    }
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-xl">
        <DialogHeader>
          <DialogTitle>Edit Profile: {profile?.name}</DialogTitle>
          <DialogDescription>
            Update the alias mappings for this profile.
            {profile?.name === activeProfile &&
              ' Changes will immediately affect active routing.'}
          </DialogDescription>
        </DialogHeader>
        <div className="space-y-4 py-2">
          <AliasEditor
            aliases={formAliases}
            onAdd={addAliasRow}
            onRemove={removeAliasRow}
            onUpdate={updateAliasRow}
          />
        </div>
        <DialogFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)}>
            Cancel
          </Button>
          <Button onClick={handleEdit} disabled={saving}>
            {saving ? 'Saving…' : 'Save Changes'}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
