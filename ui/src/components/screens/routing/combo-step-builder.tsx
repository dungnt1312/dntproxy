import { useState, useMemo, useEffect, useRef } from 'react'
import { GripVertical, Trash2, Pin, Shuffle, ChevronDown } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Label } from '@/components/ui/label'
import { Badge } from '@/components/ui/badge'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { RadioGroup, RadioGroupItem } from '@/components/ui/radio-group'
import { cn } from '@/lib/utils'
import { ConnectionOption, UiModel } from './types'

export interface ComboStep {
  id: string
  provider: string
  model: string
  accountMode: 'auto' | 'pinned'
  accountId?: string
  order: number
}

interface ComboStepBuilderProps {
  steps: ComboStep[]
  connections: ConnectionOption[]
  models: UiModel[]
  onChange: (steps: ComboStep[]) => void
}

export function ComboStepBuilder({ steps, connections, models, onChange }: ComboStepBuilderProps) {
  const [selectedProvider, setSelectedProvider] = useState<string>('')
  const [selectedModel, setSelectedModel] = useState<string>('')
  const [accountMode, setAccountMode] = useState<'auto' | 'pinned'>('auto')
  const [selectedAccount, setSelectedAccount] = useState<string>('')
  const [showQuickAdd, setShowQuickAdd] = useState(false)
  const [highlightedStepId, setHighlightedStepId] = useState<string | null>(null)
  const stepsEndRef = useRef<HTMLDivElement>(null)

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
    return connections.filter(c => c.provider === selectedProvider)
  }, [connections, selectedProvider])

  function handleAddStep() {
    if (!selectedProvider || !selectedModel) return

    const newStep: ComboStep = {
      id: `step-${Date.now()}`,
      provider: selectedProvider,
      model: selectedModel,
      accountMode,
      accountId: accountMode === 'pinned' ? selectedAccount : undefined,
      order: steps.length,
    }

    onChange([...steps, newStep])

    // Highlight new step
    setHighlightedStepId(newStep.id)
    setTimeout(() => setHighlightedStepId(null), 2000)

    // Scroll to new step
    setTimeout(() => {
      stepsEndRef.current?.scrollIntoView({ behavior: 'smooth', block: 'nearest' })
    }, 100)

    // Smart reset: keep provider & model, only reset account to auto
    setAccountMode('auto')
    setSelectedAccount('')
    
    // Show quick add options after first step
    if (steps.length === 0) {
      setShowQuickAdd(true)
    }
  }

  function handleQuickAddWithAccount(accountId: string) {
    if (!selectedProvider || !selectedModel) return

    const newStep: ComboStep = {
      id: `step-${Date.now()}`,
      provider: selectedProvider,
      model: selectedModel,
      accountMode: 'pinned',
      accountId,
      order: steps.length,
    }

    onChange([...steps, newStep])
    
    // Highlight new step
    setHighlightedStepId(newStep.id)
    setTimeout(() => setHighlightedStepId(null), 2000)

    // Scroll to new step
    setTimeout(() => {
      stepsEndRef.current?.scrollIntoView({ behavior: 'smooth', block: 'nearest' })
    }, 100)
    
    // Keep form state for next quick add
    setAccountMode('auto')
    setSelectedAccount('')
  }

  function handleChangeModel() {
    setShowQuickAdd(false)
    setSelectedModel('')
    setAccountMode('auto')
    setSelectedAccount('')
  }

  function handleChangeProvider() {
    setShowQuickAdd(false)
    setSelectedProvider('')
    setSelectedModel('')
    setAccountMode('auto')
    setSelectedAccount('')
  }

  function handleDeleteStep(stepId: string) {
    const newSteps = steps
      .filter(s => s.id !== stepId)
      .map((s, idx) => ({ ...s, order: idx }))
    onChange(newSteps)
  }

  function handleMoveStep(stepId: string, direction: 'up' | 'down') {
    const index = steps.findIndex(s => s.id === stepId)
    if (index === -1) return
    if (direction === 'up' && index === 0) return
    if (direction === 'down' && index === steps.length - 1) return

    const newSteps = [...steps]
    const targetIndex = direction === 'up' ? index - 1 : index + 1
    ;[newSteps[index], newSteps[targetIndex]] = [newSteps[targetIndex], newSteps[index]]
    
    // Update order
    newSteps.forEach((s, idx) => { s.order = idx })
    onChange(newSteps)
  }

  const canAddStep = selectedProvider && selectedModel && (accountMode === 'auto' || selectedAccount)

  // Keyboard shortcut: Enter to add step
  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Enter' && e.ctrlKey && canAddStep) {
        e.preventDefault()
        handleAddStep()
      }
    }
    window.addEventListener('keydown', handleKeyDown)
    return () => window.removeEventListener('keydown', handleKeyDown)
  }, [canAddStep, selectedProvider, selectedModel, accountMode, selectedAccount])

  return (
    <div className="space-y-4">
      {/* Step Builder Form */}
      <div className="rounded-lg border bg-muted/20 p-4 space-y-4">
        <div className="flex items-center gap-2 mb-2">
          <div className="h-1.5 w-1.5 rounded-full bg-violet-500" />
          <h4 className="text-sm font-semibold">Build a Combo</h4>
        </div>
        <p className="text-xs text-muted-foreground -mt-2">
          Build each combo step in sequence: provider, model, then account. This allows repeating the same provider and model with different accounts.
        </p>

        <div className="grid grid-cols-3 gap-3">
          {/* 1. Provider */}
          <div className="space-y-1.5">
            <Label className="text-xs font-medium">1. Provider</Label>
            <Select value={selectedProvider} onValueChange={(val) => {
              setSelectedProvider(val)
              setSelectedModel('')
              setSelectedAccount('')
            }}>
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
                // Strip provider prefix if present (e.g., "openai/gpt-5.4" -> "gpt-5.4")
                const parts = val.split('/')
                const modelName = parts.length > 1 ? parts.slice(1).join('/') : val
                setSelectedModel(modelName)
              }}
              disabled={!selectedProvider}
            >
              <SelectTrigger className="h-9">
                <SelectValue placeholder={selectedProvider ? "Pick provider first" : "Select model"} />
              </SelectTrigger>
              <SelectContent>
                {availableModels.map(m => (
                  <SelectItem key={m.id} value={m.id}>
                    {m.name || m.id}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>

          {/* 3. Account */}
          <div className="space-y-1.5">
            <Label className="text-xs font-medium">3. Account</Label>
            <Select 
              value={accountMode === 'auto' ? 'auto' : selectedAccount} 
              onValueChange={(val) => {
                if (val === 'auto') {
                  setAccountMode('auto')
                  setSelectedAccount('')
                } else {
                  setAccountMode('pinned')
                  setSelectedAccount(val)
                }
              }}
              disabled={!selectedProvider}
            >
              <SelectTrigger className="h-9">
                <SelectValue placeholder="Auto-select account at runtime" />
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
        {selectedProvider && selectedModel && (
          <div className="rounded-md bg-background border p-3 text-xs">
            <div className="text-muted-foreground mb-1">Preview:</div>
            <div className="font-mono">
              {selectedProvider}/{selectedModel}
              {accountMode === 'pinned' && selectedAccount && (
                <span className="text-violet-600 dark:text-violet-400">@{selectedAccount}</span>
              )}
            </div>
          </div>
        )}

        <Button 
          onClick={handleAddStep} 
          disabled={!canAddStep}
          className="w-full h-9 bg-violet-600 hover:bg-violet-700"
          size="sm"
          title="Add step (Ctrl+Enter)"
        >
          Add step
        </Button>

        {/* Quick Add Section */}
        {showQuickAdd && selectedProvider && selectedModel && availableAccounts.length > 0 && (
          <div className="rounded-md border bg-emerald-50 dark:bg-emerald-950/20 p-3 space-y-2">
            <div className="flex items-center justify-between">
              <div className="flex items-center gap-2">
                <div className="h-1.5 w-1.5 rounded-full bg-emerald-500" />
                <span className="text-xs font-semibold text-emerald-700 dark:text-emerald-400">
                  Quick Add More
                </span>
              </div>
              <div className="flex gap-1">
                <Button
                  variant="ghost"
                  size="sm"
                  className="h-6 text-xs"
                  onClick={handleChangeModel}
                >
                  Change Model
                </Button>
                <Button
                  variant="ghost"
                  size="sm"
                  className="h-6 text-xs"
                  onClick={handleChangeProvider}
                >
                  Change Provider
                </Button>
              </div>
            </div>
            <p className="text-xs text-muted-foreground">
              Same model ({selectedModel}), different account:
            </p>
            <div className="flex flex-wrap gap-2">
              {availableAccounts.map(acc => (
                <Button
                  key={acc.id}
                  variant="outline"
                  size="sm"
                  className="h-7 text-xs gap-1.5 bg-background hover:bg-emerald-100 dark:hover:bg-emerald-900/30"
                  onClick={() => handleQuickAddWithAccount(acc.id)}
                >
                  <Pin className="h-3 w-3" />
                  {acc.name}
                </Button>
              ))}
              <Button
                variant="outline"
                size="sm"
                className="h-7 text-xs gap-1.5 bg-background hover:bg-emerald-100 dark:hover:bg-emerald-900/30"
                onClick={() => {
                  handleAddStep()
                }}
              >
                <Shuffle className="h-3 w-3" />
                Auto-select
              </Button>
            </div>
          </div>
        )}
      </div>

      {/* Steps List */}
      <div className="space-y-2">
        <Label className="text-xs font-medium">Combo Steps ({steps.length})</Label>
        
        {steps.length === 0 ? (
          <div className="rounded-lg border border-dashed p-8 text-center">
            <p className="text-sm text-muted-foreground">
              No steps added yet. Use the form above to add your first step.
            </p>
          </div>
        ) : (
          <div className="space-y-2">
            {steps.map((step, index) => {
              const connection = connections.find(c => c.id === step.accountId)
              const model = models.find(m => m.id === step.model)
              const isHighlighted = highlightedStepId === step.id
              
              return (
                <div
                  key={step.id}
                  className={cn(
                    "group flex items-center gap-3 rounded-lg border bg-card p-3 hover:bg-muted/30 transition-all",
                    isHighlighted && "ring-2 ring-emerald-500 bg-emerald-50 dark:bg-emerald-950/20"
                  )}
                >
                  {/* Drag Handle */}
                  <div className="flex flex-col gap-0.5 opacity-0 group-hover:opacity-100 transition-opacity cursor-move">
                    <Button
                      variant="ghost"
                      size="icon"
                      className="h-4 w-4 p-0"
                      onClick={() => handleMoveStep(step.id, 'up')}
                      disabled={index === 0}
                    >
                      <ChevronDown className="h-3 w-3 rotate-180" />
                    </Button>
                    <Button
                      variant="ghost"
                      size="icon"
                      className="h-4 w-4 p-0"
                      onClick={() => handleMoveStep(step.id, 'down')}
                      disabled={index === steps.length - 1}
                    >
                      <ChevronDown className="h-3 w-3" />
                    </Button>
                  </div>

                  {/* Step Number */}
                  <div className="flex h-7 w-7 items-center justify-center rounded-md bg-violet-500/10 text-sm font-semibold text-violet-600 dark:text-violet-400 shrink-0">
                    {index + 1}
                  </div>

                  {/* Step Content */}
                  <div className="flex-1 min-w-0 space-y-1">
                    <div className="flex items-center gap-2">
                      <Badge variant="outline" className="text-xs font-mono">
                        {step.provider}
                      </Badge>
                      <span className="text-sm font-medium truncate">
                        {model?.name || step.model}
                      </span>
                    </div>
                    <div className="flex items-center gap-1.5 text-xs">
                      {step.accountMode === 'pinned' && connection ? (
                        <>
                          <Pin className="h-3 w-3 text-violet-500" />
                          <span className="text-muted-foreground">
                            Pinned: <span className="font-medium text-foreground">{connection.name}</span>
                          </span>
                        </>
                      ) : (
                        <>
                          <Shuffle className="h-3 w-3 text-muted-foreground" />
                          <span className="text-muted-foreground">Auto-select at runtime</span>
                        </>
                      )}
                    </div>
                  </div>

                  {/* Delete Button */}
                  <Button
                    variant="ghost"
                    size="icon"
                    className="h-7 w-7 text-muted-foreground hover:text-destructive opacity-0 group-hover:opacity-100 transition-opacity"
                    onClick={() => handleDeleteStep(step.id)}
                  >
                    <Trash2 className="h-3.5 w-3.5" />
                  </Button>
                </div>
              )
            })}
            <div ref={stepsEndRef} />
          </div>
        )}

        {steps.length > 0 && (
          <p className="text-xs text-muted-foreground mt-2">
            💡 Order matters — backend combo routing uses this list with the global strategy.
          </p>
        )}
      </div>
    </div>
  )
}
