import { useEffect, useState, useRef } from 'react'
import { Loader2, Pencil, Check, X } from 'lucide-react'
import { Input } from '@/components/ui/input'
import { Button } from '@/components/ui/button'
import { toast } from 'sonner'
import type { Connection } from '@/types/connections'

interface InlineNameProps {
  conn: Connection
  onRename: (name: string) => Promise<void>
}

export default function InlineName({ conn, onRename }: InlineNameProps) {
  const [editing, setEditing] = useState(false)
  const [value, setValue] = useState(conn.name)
  const [saving, setSaving] = useState(false)
  const inputRef = useRef<HTMLInputElement>(null)

  useEffect(() => { if (editing) inputRef.current?.select() }, [editing])

  const startEditing = () => { setValue(conn.name); setEditing(true) }

  const save = async () => {
    const next = value.trim()
    if (!next || next === conn.name) { setEditing(false); setValue(conn.name); return }
    setSaving(true)
    try {
      await onRename(next)
      setEditing(false)
    } catch (e: unknown) {
      toast.error(e instanceof Error && e.message ? e.message : 'Failed to rename connection')
    } finally {
      setSaving(false)
    }
  }

  const cancel = () => { setValue(conn.name); setEditing(false) }

  if (editing) {
    return (
      <form
        className="flex items-center gap-1"
        onSubmit={e => { e.preventDefault(); void save() }}
      >
        <Input
          ref={inputRef}
          value={value}
          onChange={e => setValue(e.target.value)}
          onKeyDown={e => { if (e.key === 'Escape') { e.preventDefault(); cancel() } }}
          aria-label="Connection name"
          className="h-8 w-48 text-sm"
          disabled={saving}
          autoComplete="off"
          data-1p-ignore
        />
        <Button type="submit" variant="ghost" size="icon" className="size-8" disabled={saving} aria-label="Save name">
          {saving ? <Loader2 className="size-4 animate-spin" /> : <Check className="size-4 text-success" />}
        </Button>
        <Button type="button" variant="ghost" size="icon" className="size-8" onClick={cancel} disabled={saving} aria-label="Cancel rename">
          <X className="size-4" />
        </Button>
      </form>
    )
  }

  return (
    <div className="group/name flex min-w-0 items-center gap-1">
      <h2 className="truncate text-base font-semibold">{conn.name}</h2>
      <Button
        variant="ghost"
        size="icon"
        className="size-7 shrink-0 text-muted-foreground opacity-60 transition-opacity hover:opacity-100 focus-visible:opacity-100 group-hover/name:opacity-100"
        onClick={startEditing}
        title="Rename"
        aria-label="Rename connection"
      >
        <Pencil className="size-3.5" />
      </Button>
    </div>
  )
}
