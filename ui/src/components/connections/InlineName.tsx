import { useEffect, useState, useRef } from 'react'
import { Loader2, Pencil, Check, X } from 'lucide-react'
import { Input } from '@/components/ui/input'
import { Button } from '@/components/ui/button'
import { toast } from 'sonner'
import type { Connection } from '@/types/connections'

interface InlineNameProps {
  conn: Connection
  onRename: (id: string, name: string) => Promise<void>
}

export default function InlineName({ conn, onRename }: InlineNameProps) {
  const [editing, setEditing] = useState(false)
  const [value, setValue] = useState(conn.name)
  const [saving, setSaving] = useState(false)
  const inputRef = useRef<HTMLInputElement>(null)

  useEffect(() => { if (editing) inputRef.current?.focus() }, [editing])

  const save = async () => {
    if (!value.trim() || value === conn.name) { setEditing(false); return }
    setSaving(true)
    try {
      await onRename(conn.id, value.trim())
      setEditing(false)
    } catch (e: any) {
      toast.error(e.message || 'Failed to rename connection')
    } finally {
      setSaving(false)
    }
  }

  const cancel = () => { setValue(conn.name); setEditing(false) }

  if (editing) {
    return (
      <div className="flex items-center gap-1">
        <Input
          ref={inputRef}
          value={value}
          onChange={e => setValue(e.target.value)}
          onKeyDown={e => { if (e.key === 'Enter') save(); if (e.key === 'Escape') cancel() }}
          onClick={e => e.stopPropagation()}
          className="h-6 w-44 text-sm px-2 py-0"
        />
        <Button variant="ghost" size="icon" className="h-5 w-5" onClick={e => { e.stopPropagation(); save() }} disabled={saving}>
          {saving ? <Loader2 size={11} className="animate-spin" /> : <Check size={11} className="text-emerald-500" />}
        </Button>
        <Button variant="ghost" size="icon" className="h-5 w-5" onClick={e => { e.stopPropagation(); cancel() }}>
          <X size={11} className="text-muted-foreground" />
        </Button>
      </div>
    )
  }

  return (
    <div className="flex items-center gap-1 group/name">
      <span className="font-semibold text-sm truncate">{conn.name}</span>
      <Button
        variant="ghost"
        size="icon"
        className="h-5 w-5 opacity-0 group-hover/name:opacity-100 transition-opacity"
        onClick={e => { e.stopPropagation(); setEditing(true) }}
        title="Rename"
        aria-label="Rename connection"
      >
        <Pencil size={10} className="text-muted-foreground" />
      </Button>
    </div>
  )
}
