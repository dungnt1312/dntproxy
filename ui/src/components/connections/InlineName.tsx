import { useEffect, useState, useRef } from 'react'
import { Loader2, Pencil, Check, X } from 'lucide-react'

interface InlineNameProps {
  conn: any
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
    await onRename(conn.id, value.trim())
    setSaving(false)
    setEditing(false)
  }

  const cancel = () => { setValue(conn.name); setEditing(false) }

  if (editing) {
    return (
      <div className="flex items-center gap-1">
        <input
          ref={inputRef}
          value={value}
          onChange={e => setValue(e.target.value)}
          onKeyDown={e => { if (e.key === 'Enter') save(); if (e.key === 'Escape') cancel() }}
          className="bg-[var(--bg)] border border-[var(--accent)] rounded px-2 py-0.5 text-sm font-medium outline-none w-48"
          onClick={e => e.stopPropagation()}
        />
        <button
          onClick={e => { e.stopPropagation(); save() }}
          disabled={saving}
          aria-label="Save connection name"
          className="p-0.5 text-[var(--success)] hover:opacity-80 disabled:opacity-50"
        >
          {saving ? <Loader2 size={12} className="animate-spin" /> : <Check size={13} />}
        </button>
        <button
          onClick={e => { e.stopPropagation(); cancel() }}
          aria-label="Cancel editing connection name"
          className="p-0.5 text-[var(--text-muted)] hover:opacity-80"
        >
          <X size={13} />
        </button>
      </div>
    )
  }

  return (
    <div className="flex items-center gap-1 group/name">
      <span className="font-semibold text-[var(--text)] line-clamp-1 truncate">{conn.name}</span>
      <button
        onClick={(e) => { e.stopPropagation(); setEditing(true) }}
        aria-label={`Rename ${conn.name}`}
        className="opacity-0 group-hover/name:opacity-100 p-0.5 text-[var(--text-muted)] hover:text-[var(--text)] transition-opacity"
        title="Rename"
      >
        <Pencil size={11} />
      </button>
    </div>
  )
}
