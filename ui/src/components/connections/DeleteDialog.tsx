import { useState } from 'react'
import { Loader2, Trash2 } from 'lucide-react'
import {
  AlertDialog,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog'
import { Button } from '@/components/ui/button'

interface DeleteDialogProps {
  target: { id: string; name: string }
  onConfirm: (id: string) => Promise<void>
  onClose: () => void
}

export default function DeleteDialog({ target, onConfirm, onClose }: DeleteDialogProps) {
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')

  const handleConfirm = async () => {
    setLoading(true)
    setError('')
    try {
      await onConfirm(target.id)
      onClose()
    } catch (e: any) {
      setError(e.message || 'Failed to remove connection')
      setLoading(false)
    }
  }

  return (
    <AlertDialog open onOpenChange={open => { if (!open) onClose() }}>
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>Remove Connection</AlertDialogTitle>
          <AlertDialogDescription>
            Are you sure you want to remove <span className="font-semibold text-foreground">{target.name}</span>?
            All tokens, settings, and quota data will be permanently deleted.
          </AlertDialogDescription>
        </AlertDialogHeader>
        {error && (
          <div role="alert" className="rounded-md border border-destructive/30 bg-destructive/10 px-3 py-2 text-xs text-destructive">
            {error}
          </div>
        )}
        <AlertDialogFooter>
          <Button variant="outline" onClick={onClose} disabled={loading}>Cancel</Button>
          <Button variant="destructive" onClick={handleConfirm} disabled={loading} className="gap-2">
            {loading ? <Loader2 size={14} className="animate-spin" /> : <Trash2 size={14} />}
            {loading ? 'Removing…' : 'Remove'}
          </Button>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  )
}
