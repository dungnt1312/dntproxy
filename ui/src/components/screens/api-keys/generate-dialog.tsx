import { useState } from 'react'
import { Key, Loader2 } from 'lucide-react'
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
import { PermissionsEditor } from './permissions-editor'
import type { ApiKeyCreatePayload, ConnectionOption, ModelOption } from './types'

interface GenerateDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  onGenerate: (payload: ApiKeyCreatePayload) => Promise<void>
  connections: ConnectionOption[]
  models: ModelOption[]
}

export function GenerateDialog({ open, onOpenChange, onGenerate, connections, models }: GenerateDialogProps) {
  const [keyName, setKeyName] = useState('')
  const [restricted, setRestricted] = useState(false)
  const [allowedConnectionIds, setAllowedConnectionIds] = useState<string[]>([])
  const [allowedModels, setAllowedModels] = useState<string[]>([])
  const [generating, setGenerating] = useState(false)
  const restrictedWithoutSelection = restricted && allowedConnectionIds.length === 0 && allowedModels.length === 0

  const handleGenerate = async () => {
    if (!keyName.trim()) return

    try {
      setGenerating(true)
      await onGenerate({
        name: keyName.trim(),
        allowedConnectionIds: restricted ? allowedConnectionIds : [],
        allowedModels: restricted ? allowedModels : [],
      })
      setKeyName('')
      setRestricted(false)
      setAllowedConnectionIds([])
      setAllowedModels([])
    } finally {
      setGenerating(false)
    }
  }

  const handleClose = () => {
    onOpenChange(false)
    setKeyName('')
    setRestricted(false)
    setAllowedConnectionIds([])
    setAllowedModels([])
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
