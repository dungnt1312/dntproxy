import { useMemo } from 'react'
import { Checkbox } from '@/components/ui/checkbox'
import { Label } from '@/components/ui/label'
import { RadioGroup, RadioGroupItem } from '@/components/ui/radio-group'
import { ScrollArea } from '@/components/ui/scroll-area'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import type { ConnectionOption, ModelOption } from './types'

interface PermissionsValue {
  allowedConnectionIds: string[]
  allowedModels: string[]
}

interface PermissionsEditorProps {
  value: PermissionsValue
  onChange: (value: PermissionsValue) => void
  connections: ConnectionOption[]
  models: ModelOption[]
  restricted: boolean
  onRestrictedChange: (restricted: boolean) => void
}

function groupByProvider<T extends { provider: string }>(items: T[]) {
  return items.reduce<Record<string, T[]>>((acc, item) => {
    const provider = item.provider || 'unknown'
    acc[provider] = acc[provider] || []
    acc[provider].push(item)
    return acc
  }, {})
}

function toggleValue(values: string[], id: string) {
  return values.includes(id) ? values.filter((value) => value !== id) : [...values, id]
}

export function PermissionsEditor({ value, onChange, connections, models, restricted, onRestrictedChange }: PermissionsEditorProps) {
  const connectionGroups = useMemo(() => groupByProvider(connections), [connections])
  const modelGroups = useMemo(() => groupByProvider(models), [models])

  const setMode = (mode: string) => {
    onRestrictedChange(mode === 'restricted')
    if (mode === 'unrestricted') {
      onChange({ allowedConnectionIds: [], allowedModels: [] })
    }
  }

  const updateConnections = (id: string) => {
    onChange({
      ...value,
      allowedConnectionIds: toggleValue(value.allowedConnectionIds, id),
    })
  }

  const updateModels = (id: string) => {
    onChange({
      ...value,
      allowedModels: toggleValue(value.allowedModels, id),
    })
  }

  return (
    <div className="space-y-4">
      <RadioGroup value={restricted ? 'restricted' : 'unrestricted'} onValueChange={setMode} className="grid grid-cols-1 gap-2 sm:grid-cols-2">
        <Label className="flex cursor-pointer items-start gap-3 rounded-md border p-3">
          <RadioGroupItem value="unrestricted" className="mt-0.5" />
          <span>
            <span className="block text-sm font-medium">Unrestricted</span>
            <span className="block text-xs text-muted-foreground">Can use all available connections and models.</span>
          </span>
        </Label>
        <Label className="flex cursor-pointer items-start gap-3 rounded-md border p-3">
          <RadioGroupItem value="restricted" className="mt-0.5" />
          <span>
            <span className="block text-sm font-medium">Restricted</span>
            <span className="block text-xs text-muted-foreground">Limit by selected connections and/or models.</span>
          </span>
        </Label>
      </RadioGroup>

      {restricted && (
        <div className="grid gap-4 lg:grid-cols-2">
          <section className="space-y-2">
            <div className="flex items-center justify-between gap-2">
              <Label>Connections</Label>
              <Badge variant="outline">{value.allowedConnectionIds.length || 'Any'}</Badge>
            </div>
            <ScrollArea className="h-56 rounded-md border">
              <div className="space-y-4 p-3">
                {Object.entries(connectionGroups).map(([provider, items]) => (
                  <div key={provider} className="space-y-2">
                    <p className="text-xs font-medium uppercase text-muted-foreground">{provider}</p>
                    {items.map((connection) => (
                      <Label key={connection.id} className="flex cursor-pointer items-center gap-2 text-sm">
                        <Checkbox checked={value.allowedConnectionIds.includes(connection.id)} onCheckedChange={() => updateConnections(connection.id)} />
                        <span className="min-w-0 flex-1 truncate">{connection.name}</span>
                        {!connection.isActive && <Badge variant="secondary">Inactive</Badge>}
                      </Label>
                    ))}
                  </div>
                ))}
                {connections.length === 0 && <p className="text-sm text-muted-foreground">No connections configured.</p>}
              </div>
            </ScrollArea>
          </section>

          <section className="space-y-2">
            <div className="flex items-center justify-between gap-2">
              <Label>Models</Label>
              <Badge variant="outline">{value.allowedModels.length || 'Any'}</Badge>
            </div>
            <ScrollArea className="h-56 rounded-md border">
              <div className="space-y-4 p-3">
                {Object.entries(modelGroups).map(([provider, items]) => (
                  <div key={provider} className="space-y-2">
                    <p className="text-xs font-medium uppercase text-muted-foreground">{provider}</p>
                    {items.map((model) => (
                      <Label key={model.id} className="flex cursor-pointer items-center gap-2 text-sm">
                        <Checkbox checked={value.allowedModels.includes(model.id)} onCheckedChange={() => updateModels(model.id)} />
                        <span className="min-w-0 flex-1 truncate">{model.displayName || model.modelId || model.id}</span>
                      </Label>
                    ))}
                  </div>
                ))}
                {models.length === 0 && <p className="text-sm text-muted-foreground">No models available.</p>}
              </div>
            </ScrollArea>
          </section>

          <div className="lg:col-span-2">
            <Button
              variant="outline"
              size="sm"
              onClick={() => {
                onChange({ allowedConnectionIds: [], allowedModels: [] })
                onRestrictedChange(false)
              }}
            >
              Clear restrictions
            </Button>
          </div>
        </div>
      )}
    </div>
  )
}
