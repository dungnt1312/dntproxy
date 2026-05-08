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

interface GenerateDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  onGenerate: (name: string) => Promise<void>
}

export function GenerateDialog({ open, onOpenChange, onGenerate }: GenerateDialogProps) {
  const [keyName, setKeyName] = useState('')
  const [generating, setGenerating] = useState(false)

  const handleGenerate = async () => {
    if (!keyName.trim()) return

    try {
      setGenerating(true)
      await onGenerate(keyName.trim())
      setKeyName('')
    } finally {
      setGenerating(false)
    }
  }

  const handleClose = () => {
    onOpenChange(false)
    setKeyName('')
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
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
        </div>
        <DialogFooter>
          <Button variant="outline" onClick={handleClose}>
            Cancel
          </Button>
          <Button onClick={handleGenerate} disabled={generating || !keyName.trim()}>
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
