import { useState } from 'react'
import { ArrowRight } from 'lucide-react'
import { toast } from 'sonner'

import { goApi } from '@/lib/go-api'
import { Button } from '@/components/ui/button'
import { Label } from '@/components/ui/label'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import type { PresetData } from './types'

type PresetDialogProps = {
  open: boolean
  presets: PresetData[]
  onOpenChange: (open: boolean) => void
  onSuccess: () => void
}

export function PresetDialog({ open, presets, onOpenChange, onSuccess }: PresetDialogProps) {
  const [selectedPreset, setSelectedPreset] = useState('')
  const [saving, setSaving] = useState(false)

  async function handleCreateFromPreset() {
    if (!selectedPreset) {
      toast.error('Select a preset')
      return
    }

    setSaving(true)
    try {
      await goApi.createProfileFromPreset(selectedPreset)
      toast.success(`Profile created from preset "${selectedPreset}"`)
      onOpenChange(false)
      setSelectedPreset('')
      onSuccess()
    } catch (error) {
      toast.error(error instanceof Error ? error.message : 'Failed to create from preset')
    } finally {
      setSaving(false)
    }
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-lg">
        <DialogHeader>
          <DialogTitle>Create from Preset</DialogTitle>
          <DialogDescription>
            Choose a built-in preset to quickly create a profile with pre-configured alias
            mappings.
          </DialogDescription>
        </DialogHeader>
        <div className="space-y-4 py-2">
          <div className="space-y-2">
            <Label>Preset</Label>
            <Select value={selectedPreset} onValueChange={setSelectedPreset}>
              <SelectTrigger>
                <SelectValue placeholder="Select a preset…" />
              </SelectTrigger>
              <SelectContent>
                {presets.map((preset) => (
                  <SelectItem key={preset.name} value={preset.name}>
                    <span className="font-medium">{preset.name}</span>
                    <span className="ml-2 text-muted-foreground">— {preset.description}</span>
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>

          {selectedPreset && (
            <div className="space-y-2 rounded-lg border bg-muted/20 p-3">
              <p className="text-xs font-semibold uppercase tracking-wide text-muted-foreground">
                Preview
              </p>
              {presets
                .find((p) => p.name === selectedPreset)
                ?.aliases &&
                Object.entries(presets.find((p) => p.name === selectedPreset)!.aliases).map(
                  ([alias, model]) => (
                    <div key={alias} className="flex items-center gap-2 text-xs">
                      <span className="font-mono text-foreground">{alias}</span>
                      <ArrowRight className="h-3 w-3 shrink-0 text-muted-foreground/60" />
                      <span className="font-mono text-muted-foreground">{model}</span>
                    </div>
                  ),
                )}
            </div>
          )}
        </div>
        <DialogFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)}>
            Cancel
          </Button>
          <Button onClick={handleCreateFromPreset} disabled={saving || !selectedPreset}>
            {saving ? 'Creating…' : 'Create from Preset'}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
