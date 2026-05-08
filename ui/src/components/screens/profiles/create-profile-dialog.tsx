import { useState } from 'react'
import { toast } from 'sonner'

import { goApi } from '@/lib/go-api'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { AliasEditor } from './alias-editor'
import type { AliasRow } from './types'

type CreateProfileDialogProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
  onSuccess: () => void
}

export function CreateProfileDialog({ open, onOpenChange, onSuccess }: CreateProfileDialogProps) {
  const [formName, setFormName] = useState('')
  const [formDescription, setFormDescription] = useState('')
  const [formAliases, setFormAliases] = useState<AliasRow[]>([{ alias: '', model: '' }])
  const [saving, setSaving] = useState(false)

  function addAliasRow() {
    setFormAliases((prev) => [...prev, { alias: '', model: '' }])
  }

  function removeAliasRow(index: number) {
    setFormAliases((prev) => prev.filter((_, i) => i !== index))
  }

  function updateAliasRow(index: number, field: 'alias' | 'model', value: string) {
    setFormAliases((prev) => prev.map((row, i) => (i === index ? { ...row, [field]: value } : row)))
  }

  async function handleCreate() {
    const name = formName.trim()
    if (!name) {
      toast.error('Profile name is required')
      return
    }

    const aliases: Record<string, string> = {}
    for (const row of formAliases) {
      const a = row.alias.trim()
      const m = row.model.trim()
      if (a && m) aliases[a] = m
    }

    if (Object.keys(aliases).length === 0) {
      toast.error('At least one alias mapping is required')
      return
    }

    setSaving(true)
    try {
      await goApi.createProfile({ name, description: formDescription.trim(), aliases })
      toast.success(`Profile "${name}" created`)
      onOpenChange(false)
      // Reset form
      setFormName('')
      setFormDescription('')
      setFormAliases([{ alias: '', model: '' }])
      onSuccess()
    } catch (error) {
      toast.error(error instanceof Error ? error.message : 'Failed to create profile')
    } finally {
      setSaving(false)
    }
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-xl">
        <DialogHeader>
          <DialogTitle>Create Profile</DialogTitle>
          <DialogDescription>
            Define model alias mappings. When activated, these aliases route CLI tool requests to
            your chosen providers.
          </DialogDescription>
        </DialogHeader>
        <div className="space-y-4 py-2">
          <div className="grid grid-cols-2 gap-4">
            <div className="space-y-2">
              <Label htmlFor="profile-name">Name</Label>
              <Input
                id="profile-name"
                value={formName}
                onChange={(e) => setFormName(e.target.value)}
                placeholder="my-profile"
              />
            </div>
            <div className="space-y-2">
              <Label htmlFor="profile-desc">Description</Label>
              <Input
                id="profile-desc"
                value={formDescription}
                onChange={(e) => setFormDescription(e.target.value)}
                placeholder="Optional description"
              />
            </div>
          </div>
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
          <Button onClick={handleCreate} disabled={saving}>
            {saving ? 'Creating…' : 'Create Profile'}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
