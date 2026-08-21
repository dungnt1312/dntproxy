import { useMemo } from 'react'
import { Pin, Shuffle, ChevronRight, Sparkles } from 'lucide-react'
import { Badge } from '@/components/ui/badge'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { cn } from '@/lib/utils'
import { providersMatch } from '@/lib/provider-registry'
import { buildPlaygroundModelId, getSelectableModelValue } from './stream-utils'

interface Model {
  id: string
  name?: string
  provider: string
  routePrefix?: string
  displayName?: string
}

interface Connection {
  id: string
  name: string
  provider: string
  isActive: boolean
  supportedModels?: string[]
  routePrefix?: string
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
  const displayProvider = (m: Model) => m.routePrefix || m.provider

  // Get unique providers from models
  const providers = useMemo(() => {
    const providerSet = new Set(models.map(displayProvider))
    return Array.from(providerSet).sort()
  }, [models])

  // Filter models by selected provider
  const availableModels = useMemo(() => {
    if (!selectedProvider) return []
    return models.filter(m => displayProvider(m) === selectedProvider)
  }, [models, selectedProvider])

  // Filter connections by selected provider and model
  const availableAccounts = useMemo(() => {
    if (!selectedProvider) return []
    
    const filtered = connections.filter(c => {
      if (!c.isActive) return false
      if (c.routePrefix && c.routePrefix === selectedProvider) return true
      return providersMatch(c.provider, selectedProvider) || providersMatch((c as { publicPrefix?: string }).publicPrefix, selectedProvider)
    })

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
    return buildPlaygroundModelId(selectedProvider, selectedModel, selectedAccount)
  }, [selectedProvider, selectedModel, selectedAccount])

  return (
    <div className="space-y-4">
      {/* Compact horizontal flow */}
      <div className="grid gap-2 md:flex md:flex-wrap md:items-center">
        {/* Provider */}
        <div className="grid gap-1 md:flex md:items-center md:gap-2">
          <span id="playground-provider-label" className="text-xs text-muted-foreground font-medium">Provider</span>
          <Select 
            value={selectedProvider} 
            onValueChange={onProviderChange}
            disabled={disabled}
          >
            <SelectTrigger aria-labelledby="playground-provider-label" className="h-8 w-full md:w-[140px]">
              <SelectValue placeholder="Select..." />
            </SelectTrigger>
            <SelectContent>
              {providers.map(p => (
                <SelectItem key={p} value={p}>
                  <Badge variant="secondary" className="text-xs">
                    {p}
                  </Badge>
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>

        {selectedProvider && (
          <>
            <ChevronRight className="hidden h-4 w-4 text-muted-foreground md:block" />
            
            {/* Model */}
            <div className="grid gap-1 md:flex md:items-center md:gap-2">
              <span id="playground-model-label" className="text-xs text-muted-foreground font-medium">Model</span>
              <Select 
                value={selectedModel} 
                onValueChange={(val) => {
                  onModelChange(val)
                }}
                disabled={disabled}
              >
                <SelectTrigger aria-labelledby="playground-model-label" className="h-8 w-full md:w-[240px]">
                  <SelectValue placeholder="Select model..." />
                </SelectTrigger>
                <SelectContent>
                  {availableModels.map(m => {
                    // Strip provider prefix for value to match selectedModel state
                    const modelValue = getSelectableModelValue(m)
                    return (
                      <SelectItem key={m.id} value={modelValue}>
                        <div className="flex items-center gap-2">
                          <Sparkles className="h-3 w-3 text-violet-500" />
                          <span className="truncate">{m.displayName || m.name || m.id}</span>
                        </div>
                      </SelectItem>
                    )
                  })}
                </SelectContent>
              </Select>
            </div>
          </>
        )}

        {selectedProvider && selectedModel && selectedProvider !== 'combo' && selectedProvider !== 'alias' && (
          <>
            <ChevronRight className="hidden h-4 w-4 text-muted-foreground md:block" />
            
            {/* Account */}
            <div className="grid gap-1 md:flex md:items-center md:gap-2">
              <span id="playground-account-label" className="text-xs text-muted-foreground font-medium">Account</span>
              <Select 
                value={selectedAccount} 
                onValueChange={onAccountChange}
                disabled={disabled}
              >
                <SelectTrigger className={cn(
                  "h-8 w-full md:w-[180px]",
                  selectedAccount === 'auto' && "text-muted-foreground"
                )} aria-labelledby="playground-account-label">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="auto">
                    <div className="flex items-center gap-2">
                      <Shuffle className="h-3.5 w-3.5 opacity-50" />
                      <span>Auto-select</span>
                    </div>
                  </SelectItem>
                  {availableAccounts.length > 0 && (
                    <>
                      <div className="px-2 py-1.5 text-xs font-semibold text-muted-foreground border-t mt-1 pt-2">
                        Pin to account
                      </div>
                      {availableAccounts.map(acc => (
                        <SelectItem key={acc.id} value={acc.id}>
                          <div className="flex items-center gap-2">
                            <Pin className="h-3.5 w-3.5 text-violet-500" />
                            <span>{acc.name}</span>
                          </div>
                        </SelectItem>
                      ))}
                    </>
                  )}
                </SelectContent>
              </Select>
            </div>
          </>
        )}
      </div>

      {/* Preview badge */}
      {finalModelString && (
        <div className="flex items-center gap-2">
          <Badge variant="outline" className="font-mono text-xs px-2 py-1">
            {finalModelString}
          </Badge>
          {selectedAccount !== 'auto' && (
            <Badge variant="secondary" className="text-xs gap-1">
              <Pin className="h-3 w-3" />
              Pinned
            </Badge>
          )}
        </div>
      )}
    </div>
  )
}
