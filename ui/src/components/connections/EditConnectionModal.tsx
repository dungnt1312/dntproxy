import { useState } from 'react'
import { Loader2, Link2, AlertTriangle } from 'lucide-react'
import { api } from '../../api'
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
} from '@/components/ui/dialog'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'

interface EditConnectionModalProps {
  conn: any
  onSuccess: (message: string) => void
  onClose: () => void
}

export default function EditConnectionModal({ conn, onSuccess, onClose }: EditConnectionModalProps) {
  const [name, setName] = useState(conn.name || '')
  const [apiKey, setApiKey] = useState('')
  const [baseUrl, setBaseUrl] = useState(conn.baseUrl || '')
  
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')

  const handleSave = async () => {
    setLoading(true)
    setError('')
    try {
      const payload: any = {}
      if (name.trim() && name !== conn.name) payload.name = name.trim()
      if (apiKey.trim()) payload.apiKey = apiKey.trim()
      if (baseUrl !== conn.baseUrl) payload.baseUrl = baseUrl.trim()
      
      if (Object.keys(payload).length > 0) {
        await api.updateConnection(conn.id, payload)
        onSuccess('Connection updated!')
      } else {
        onClose()
      }
    } catch (e: any) {
      setError(e.message)
    } finally {
      setLoading(false)
    }
  }

  return (
    <Dialog open onOpenChange={open => { if (!open) onClose() }}>
      <DialogContent className="max-w-md max-h-[90vh] flex flex-col overflow-hidden">
        <DialogHeader>
          <div className="flex items-center gap-2">
            <div className="flex h-8 w-8 items-center justify-center rounded-md bg-primary/10">
              <Link2 className="h-4 w-4 text-primary" />
            </div>
            <div>
              <DialogTitle>Edit Connection</DialogTitle>
              <DialogDescription>Update configuration for this connection.</DialogDescription>
            </div>
          </div>
        </DialogHeader>

        <div className="flex-1 overflow-y-auto space-y-4 py-4">
          <div className="space-y-3">
            <div className="space-y-1">
              <Label className="text-xs">Display Name</Label>
              <Input 
                value={name} 
                onChange={e => setName(e.target.value)} 
                placeholder="Connection Name" 
                className="text-xs" 
                autoComplete="off"
                data-1p-ignore
              />
            </div>
            
            <div className="space-y-1">
              <Label className="text-xs">API Key <span className="opacity-70 font-normal">(leave blank to keep current)</span></Label>
              <Input 
                type="password" 
                value={apiKey} 
                onChange={e => setApiKey(e.target.value)} 
                placeholder="Enter new API key" 
                className="text-xs font-mono" 
                autoComplete="new-password"
                data-1p-ignore
              />
            </div>

            <div className="space-y-1">
              <Label className="text-xs">Base URL</Label>
              <Input 
                value={baseUrl} 
                onChange={e => setBaseUrl(e.target.value)} 
                placeholder="https://api.openai.com/v1" 
                className="text-xs font-mono" 
                autoComplete="off"
                data-1p-ignore
              />
            </div>
          </div>

          {error && (
            <div className="flex items-center gap-2 rounded-lg border border-destructive/30 bg-destructive/10 px-3 py-2 text-xs text-destructive">
              <AlertTriangle size={13} className="shrink-0" />
              {error}
            </div>
          )}

          <div className="flex justify-end gap-2 pt-2">
            <Button variant="outline" onClick={onClose} disabled={loading}>
              Cancel
            </Button>
            <Button onClick={handleSave} disabled={loading}>
              {loading ? <><Loader2 className="w-4 h-4 mr-2 animate-spin" /> Saving...</> : 'Save Changes'}
            </Button>
          </div>
        </div>
      </DialogContent>
    </Dialog>
  )
}
