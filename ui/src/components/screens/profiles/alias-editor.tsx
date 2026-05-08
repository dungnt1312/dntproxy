import { Plus, ArrowRight, Trash2 } from 'lucide-react'

import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import type { AliasRow } from './types'

type AliasEditorProps = {
  aliases: AliasRow[]
  onAdd: () => void
  onRemove: (index: number) => void
  onUpdate: (index: number, field: 'alias' | 'model', value: string) => void
}

export function AliasEditor({ aliases, onAdd, onRemove, onUpdate }: AliasEditorProps) {
  return (
    <div className="space-y-3">
      <div className="flex items-center justify-between">
        <Label>Alias Mappings</Label>
        <Button type="button" variant="outline" size="sm" onClick={onAdd} className="gap-1.5">
          <Plus className="h-3 w-3" />
          Add Row
        </Button>
      </div>
      <div className="space-y-2 rounded-lg border bg-muted/20 p-3">
        {aliases.length === 0 && (
          <p className="py-2 text-center text-sm text-muted-foreground">
            No aliases. Click "Add Row" to start.
          </p>
        )}
        {aliases.map((row, index) => (
          <div key={index} className="flex items-center gap-2">
            <Input
              value={row.alias}
              onChange={(e) => onUpdate(index, 'alias', e.target.value)}
              placeholder="claude-sonnet"
              className="flex-1 font-mono text-xs"
            />
            <ArrowRight className="h-3.5 w-3.5 shrink-0 text-muted-foreground" />
            <Input
              value={row.model}
              onChange={(e) => onUpdate(index, 'model', e.target.value)}
              placeholder="kr/claude-sonnet-4.5"
              className="flex-1 font-mono text-xs"
            />
            <Button
              type="button"
              variant="ghost"
              size="icon"
              className="h-8 w-8 shrink-0 text-muted-foreground hover:text-destructive"
              onClick={() => onRemove(index)}
            >
              <Trash2 className="h-3.5 w-3.5" />
            </Button>
          </div>
        ))}
      </div>
      <p className="text-xs text-muted-foreground">
        Map model names (as sent by CLI tools) to provider/model targets in dntproxy.
      </p>
    </div>
  )
}
