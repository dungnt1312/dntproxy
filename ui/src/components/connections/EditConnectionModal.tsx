import { useId, useState } from 'react'
import { Loader2, AlertTriangle } from 'lucide-react'
import { api } from '../../api'
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogDescription,
} from '@/components/ui/dialog'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { SecretInput } from '@/components/ui/secret-input'
import type { Connection } from '@/types/connections'

interface EditConnectionModalProps {
  conn: Connection
  onSuccess: (message: string) => void
  onClose: () => void
}

/** API-key style credentials can be rotated here; OAuth tokens are refreshed by the backend. */
function usesApiKey(conn: Connection): boolean {
  return conn.authType === 'apikey' || Boolean(conn.apiKey) || conn.authMethod === 'apikey'
}

export default function EditConnectionModal({ conn, onSuccess, onClose }: EditConnectionModalProps) {
  const id = useId()
  const [name, setName] = useState(conn.name || '')
  const [apiKey, setApiKey] = useState('')
  const [baseUrl, setBaseUrl] = useState(conn.baseUrl || '')
  const [routePrefix, setRoutePrefix] = useState(conn.routePrefix || '')
  const [modelPrefix, setModelPrefix] = useState(conn.modelPrefix || '')
  const [priority, setPriority] = useState(String(conn.priority ?? 0))
  const [weight, setWeight] = useState(String(conn.weight || 100))

  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')

  const showApiKey = usesApiKey(conn)
  const showBaseUrl = showApiKey || Boolean(conn.baseUrl)
  const showPrefixes = conn.provider === 'openai-compatible'

  const handleSave = async (e: React.FormEvent) => {
    e.preventDefault()
    setError('')

    const nextPriority = Number.parseInt(priority, 10)
    const nextWeight = Number.parseInt(weight, 10)
    if (Number.isNaN(nextPriority) || nextPriority < 0) return setError('Priority must be a number ≥ 0.')
    if (Number.isNaN(nextWeight) || nextWeight < 1) return setError('Weight must be a number ≥ 1.')
    if (!name.trim()) return setError('Name cannot be empty.')

    const payload: Record<string, unknown> = {}
    if (name.trim() !== conn.name) payload.name = name.trim()
    if (apiKey.trim()) payload.apiKey = apiKey.trim()
    if (showBaseUrl && baseUrl.trim() !== (conn.baseUrl || '')) payload.baseUrl = baseUrl.trim()
    if (showPrefixes && routePrefix.trim() !== (conn.routePrefix || '')) payload.routePrefix = routePrefix.trim()
    if (showPrefixes && modelPrefix.trim() !== (conn.modelPrefix || '')) payload.modelPrefix = modelPrefix.trim()
    if (nextPriority !== (conn.priority ?? 0)) payload.priority = nextPriority
    if (nextWeight !== (conn.weight || 100)) payload.weight = nextWeight

    if (Object.keys(payload).length === 0) return onClose()

    setLoading(true)
    try {
      await api.updateConnection(conn.id, payload)
      onSuccess('Connection updated')
    } catch (err: unknown) {
      setError(err instanceof Error && err.message ? err.message : 'Failed to update connection')
    } finally {
      setLoading(false)
    }
  }

  return (
    <Dialog open onOpenChange={open => { if (!open && !loading) onClose() }}>
      <DialogContent className="flex max-h-[90vh] max-w-md flex-col overflow-hidden">
        <DialogHeader>
          <DialogTitle>Edit connection</DialogTitle>
          <DialogDescription>Update the configuration of “{conn.name}”.</DialogDescription>
        </DialogHeader>

        <form onSubmit={handleSave} className="flex min-h-0 flex-1 flex-col">
          <div className="flex-1 space-y-4 overflow-y-auto py-2 pr-1">
            <div className="space-y-1.5">
              <Label htmlFor={`${id}-name`}>Display name</Label>
              <Input id={`${id}-name`} value={name} onChange={e => setName(e.target.value)} autoComplete="off" data-1p-ignore />
            </div>

            {showApiKey && (
              <div className="space-y-1.5">
                <Label htmlFor={`${id}-key`}>API key</Label>
                <SecretInput
                  id={`${id}-key`}
                  name="connection-api-key"
                  revealLabel="API key"
                  value={apiKey}
                  onChange={e => setApiKey(e.target.value)}
                  placeholder="Leave blank to keep the current key"
                  className="font-mono"
                />
              </div>
            )}

            {showBaseUrl && (
              <div className="space-y-1.5">
                <Label htmlFor={`${id}-url`}>Base URL</Label>
                <Input
                  id={`${id}-url`}
                  type="url"
                  value={baseUrl}
                  onChange={e => setBaseUrl(e.target.value)}
                  placeholder="Provider default"
                  className="font-mono"
                  autoComplete="off"
                  data-1p-ignore
                />
              </div>
            )}

            {showPrefixes && (
              <div className="grid grid-cols-2 gap-3">
                <div className="space-y-1.5">
                  <Label htmlFor={`${id}-route`}>Route prefix</Label>
                  <Input id={`${id}-route`} value={routePrefix} onChange={e => setRoutePrefix(e.target.value)} placeholder="e.g. windsurf" className="font-mono" autoComplete="off" data-1p-ignore />
                </div>
                <div className="space-y-1.5">
                  <Label htmlFor={`${id}-model`}>Model prefix to strip</Label>
                  <Input id={`${id}-model`} value={modelPrefix} onChange={e => setModelPrefix(e.target.value)} placeholder="optional" className="font-mono" autoComplete="off" data-1p-ignore />
                </div>
              </div>
            )}

            <div className="grid grid-cols-2 gap-3">
              <div className="space-y-1.5">
                <Label htmlFor={`${id}-prio`}>Priority</Label>
                <Input id={`${id}-prio`} type="number" inputMode="numeric" min={0} value={priority} onChange={e => setPriority(e.target.value)} aria-describedby={`${id}-routing-help`} />
              </div>
              <div className="space-y-1.5">
                <Label htmlFor={`${id}-weight`}>Weight</Label>
                <Input id={`${id}-weight`} type="number" inputMode="numeric" min={1} value={weight} onChange={e => setWeight(e.target.value)} aria-describedby={`${id}-routing-help`} />
              </div>
              <p id={`${id}-routing-help`} className="col-span-2 text-xs text-muted-foreground">
                Lower priority is tried first; within the same priority, higher weight gets more traffic.
              </p>
            </div>

            {error && (
              <div role="alert" className="flex items-center gap-2 rounded-md border border-destructive/30 bg-destructive/10 px-3 py-2 text-sm text-destructive">
                <AlertTriangle className="size-4 shrink-0" />
                {error}
              </div>
            )}
          </div>

          <DialogFooter className="pt-4">
            <Button type="button" variant="outline" onClick={onClose} disabled={loading}>Cancel</Button>
            <Button type="submit" disabled={loading} className="gap-2">
              {loading && <Loader2 className="size-4 animate-spin" />}
              {loading ? 'Saving…' : 'Save changes'}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}
