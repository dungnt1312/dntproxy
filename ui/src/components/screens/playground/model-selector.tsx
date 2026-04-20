import { useMemo } from 'react'
import { Pin, Shuffle } from 'lucide-react'
import { Label } from '@/components/ui/label'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'

interface Model {
  id: string
  name?: string
  provider: string
  displayName?: string
}

interface Connection {
  id: string
  name: string
  provider: string
  isActive: boolean
  supportedModels?: string[]
}

interface ModelSelectorProps {
  models: Model[]
  connections: Connection[]
  selectedProvider: string
  selectedModel: string
  selectedAccount: string
  onProviderChange: (provider: string) => void
  onModelChange: (model: string) => void
  onAccountChange: (account: string) => void
  disabled?: boolean
}

export function ModelSelector({
  models,
  connections,
  selectedProvider,
  selectedModel,
  selectedAccount,
  onProviderChange,
  onModelChange,
  onAccountChange,
  disabled,
}: ModelSelectorProps) {
  // Get unique providers from models
  const providers = useMemo(() => {
    const providerSet = new Set(models.map(m => m.provider))
    return Array.from(providerSet).sort()
  }, [models])

  // Filter models by selected provider
  const availableModels = useMemo(() => {
    if (!selectedProvider) return []
    return models.filter(m => m.provider === selectedProvider)
  }, [models, selectedProvider])

  // Filter connections by selected provider and model
  const availableAccounts = useMemo(() => {
    if (!selectedProvider) return []
    
    const filtered = connections.filter(c => 
      c.isActive && c.provider === selectedProvider
    )

    // If model is selected, further filter by supportedModels
    if (selectedModel && filtered.length > 0) {
      return filtered.filter(conn => {
        if (!conn.supportedModels || conn.supportedModels.length === 0) {
          return true // No restriction
        }
        // Check if model is supported (strip provider prefix)
        const modelShortId = selectedModel.includes('/') 
          ? selectedModel.split('/').slice(1).join('/')
          : selectedModel
        return conn.supportedModels.includes(selectedModel) || 
               conn.supportedModels.includes(modelShortId)
      })
    }

    return filtered
  }, [connections, selectedProvider, selectedModel])

  // Build final model string for API
  const finalModelString = useMemo(() => {
    if (!selectedProvider || !selectedModel) return ''
    
    const base = `${selectedProvider}/${selectedModel}`
    if (selectedAccount && selectedAccount !== 'auto') {
      return `${base}@${selectedAccount}`
    }
    return base
  }, [selectedProvider, selectedModel, selectedAccount])

  return (
    <div className="space-y-3">
      <div className="grid grid-cols-3 gap-3">
        {/* 1. Provider */}
        <div className="space-y-1.5">
          <Label className="text-xs font-medium">1. Provider</Label>
          <Select 
            value={selectedProvider} 
            onValueChange={(val) => {
              onProviderChange(val)
              onModelChange('')
              onAccountChange('auto')
            }}
            disabled={disabled}
          >
            <SelectTrigger className="h-9">
              <SelectValue placeholder="Select provider" />
            </SelectTrigger>
            <SelectContent>
              {providers.map(p => (
                <SelectItem key={p} value={p}>{p}</SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>

        {/* 2. Model */}
        <div className="space-y-1.5">
          <Label className="text-xs font-medium">2. Model</Label>
          <Select 
            value={selectedModel} 
            onValueChange={(val) => {
              // Strip provider prefix if present
              const parts = val.split('/')
              const modelName = parts.length > 1 ? parts.slice(1).join('/') : val
              onModelChange(modelName)
            }}
            disabled={disabled || !selectedProvider}
          >
            <SelectTrigger className="h-9">
              <SelectValue placeholder={selectedProvider ? "Select model" : "Pick provider first"} />
            </SelectTrigger>
            <SelectContent>
              {availableModels.map(m => (
                <SelectItem key={m.id} value={m.id}>
                  {m.displayName || m.name || m.id}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>

        {/* 3. Account */}
        <div className="space-y-1.5">
          <Label className="text-xs font-medium">3. Account</Label>
          <Select 
            value={selectedAccount} 
            onValueChange={onAccountChange}
            disabled={disabled || !selectedProvider}
          >
            <SelectTrigger className="h-9">
              <SelectValue placeholder="Auto-select at runtime" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="auto">
                <div className="flex items-center gap-2">
                  <Shuffle className="h-3.5 w-3.5 opacity-50" />
                  Auto-select at runtime
                </div>
              </SelectItem>
              {availableAccounts.length > 0 && (
                <>
                  <div className="px-2 py-1.5 text-xs font-semibold text-muted-foreground">
                    Pin to specific account
                  </div>
                  {availableAccounts.map(acc => (
                    <SelectItem key={acc.id} value={acc.id}>
                      <div className="flex items-center gap-2">
                        <Pin className="h-3.5 w-3.5 opacity-50" />
                        {acc.name}
                      </div>
                    </SelectItem>
                  ))}
                </>
              )}
            </SelectContent>
          </Select>
        </div>
      </div>

      {/* Preview */}
      {finalModelString && (
        <div className="rounded-md bg-muted/50 border p-2 text-xs">
          <div className="text-muted-foreground mb-1">Model string:</div>
          <div className="font-mono text-sm">
            {finalModelString}
          </div>
        </div>
      )}
    </div>
  )
}
