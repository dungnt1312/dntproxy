import { Copy, Check, ShieldCheck } from 'lucide-react'
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
import { toast } from 'sonner'

interface ShowKeyDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  keyValue: string
  keyName: string
}

export function ShowKeyDialog({ open, onOpenChange, keyValue, keyName }: ShowKeyDialogProps) {
  const copyToClipboard = (text: string, label: string) => {
    navigator.clipboard.writeText(text).then(() => {
      toast.success(`${label} copied to clipboard`)
    }).catch(() => {
      toast.error('Failed to copy to clipboard')
    })
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle className="flex items-center gap-2">
            <div className="size-8 rounded-lg bg-emerald-500/10 flex items-center justify-center">
              <ShieldCheck className="size-4 text-emerald-600" />
            </div>
            Key Generated Successfully
          </DialogTitle>
          <DialogDescription>
            Key <strong>&quot;{keyName}&quot;</strong> has been created. Copy the key below and store it safely.
          </DialogDescription>
        </DialogHeader>
        <div className="space-y-4 py-2">
          <div className="space-y-2">
            <Label>API Key</Label>
            <div className="flex items-center gap-2">
              <code className="flex-1 text-sm bg-muted px-3 py-2.5 rounded-md font-mono break-all select-all">
                {keyValue}
              </code>
              <Button
                variant="outline"
                size="icon"
                className="shrink-0"
                onClick={() => copyToClipboard(keyValue, 'API key')}
              >
                <Copy className="size-4" />
              </Button>
            </div>
          </div>
          <div className="flex items-start gap-2 rounded-lg border border-amber-500/30 bg-amber-500/5 p-3">
            <ShieldCheck className="size-4 text-amber-600 shrink-0 mt-0.5" />
            <p className="text-xs text-amber-700 dark:text-amber-400">
              <strong>Important:</strong> This is the only time the full API key will be displayed.
              Make sure to copy it now. The list screen only shows a masked value.
            </p>
          </div>
        </div>
        <DialogFooter>
          <Button
            onClick={() => copyToClipboard(keyValue, 'API key')}
            variant="outline"
            className="gap-2"
          >
            <Copy className="size-4" />
            Copy Key
          </Button>
          <Button onClick={() => onOpenChange(false)} className="gap-2">
            <Check className="size-4" />
            Done
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
