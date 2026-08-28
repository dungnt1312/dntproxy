import { useState, useEffect } from 'react'
import { Plus, X, Search, Check, Loader2, CheckCircle2, XCircle, TestTube } from 'lucide-react'
import { api } from '../api'
import { getModelProviderId } from '@/lib/provider-registry'

interface RegistryModel {
  id: string
  name: string
  provider: string
}

export interface ModelTestState {
  status: 'ok' | 'error' | 'loading'
  message?: string
}

export interface ModelSelectorTestProps {
  results: Record<string, ModelTestState>
  onTest: (modelId: string) => void
  onTestAll?: () => void
  running?: boolean
  progress?: string
}

interface ModelSelectorProps {
  selected: string[]
  onChange: (models: string[]) => void
  provider?: string
  routePrefix?: string
  allowedModels?: string[]
  /** When provided, each selected model tag gets an inline test button. */
  test?: ModelSelectorTestProps
}

export default function ModelSelector({
  selected,
  onChange,
  provider,
  routePrefix,
  allowedModels,
  test,
}: ModelSelectorProps) {
  const [customInput, setCustomInput] = useState('')
  const [showCustomInput, setShowCustomInput] = useState(false)
  const [searchQuery, setSearchQuery] = useState('')
  const [registryModels, setRegistryModels] = useState<RegistryModel[]>([])
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    let cancelled = false
    setLoading(true)
    api.getModelRegistry()
      .then((res: any) => {
        if (cancelled) return
        const models: RegistryModel[] = []
        if (res.models) {
          for (const [key, m] of Object.entries(res.models)) {
            const def = m as any
            if (def.provider && def.id) {
              models.push({ id: key, name: def.name || def.id, provider: def.provider })
            }
          }
        }
        setRegistryModels(models)
      })
      .catch(() => {
        if (!cancelled) setRegistryModels([])
      })
      .finally(() => {
        if (!cancelled) setLoading(false)
      })
    return () => { cancelled = true }
  }, [])

  const modelProvider = provider ? getModelProviderId(provider) : undefined
  const baseModels = modelProvider
    ? registryModels.filter(m => m.provider === modelProvider || m.provider === provider)
    : registryModels
  const models = allowedModels
    ? baseModels.filter(m => allowedModels.includes(m.id))
    : baseModels
  const customModels = selected.filter((modelId) => !baseModels.some((model) => model.id === modelId))
  const selectedKnownModels = selected.filter((modelId) => models.some((model) => model.id === modelId))

  const filteredModels = searchQuery
    ? models.filter(
      (model) =>
        model.id.toLowerCase().includes(searchQuery.toLowerCase()) ||
        model.name.toLowerCase().includes(searchQuery.toLowerCase()),
      )
    : models

  const isSelectedModel = (modelId: string) => selectedKnownModels.includes(modelId)
  const isAllAllowed = selected.length === 0

  const toggleModel = (modelId: string): void => {
    if (isSelectedModel(modelId)) {
      onChange(selected.filter((currentModel) => currentModel !== modelId))
      return
    }

    onChange([...selectedKnownModels, modelId, ...customModels])
  }

  const removeModel = (modelId: string): void => {
    onChange(selected.filter((m) => m !== modelId))
  }

  const modelIdPrefix = routePrefix || (modelProvider ?? provider ?? null)

  const addCustomModel = (): void => {
    const trimmed = customInput.trim()
    if (!trimmed) return
    const modelId =
      modelIdPrefix && !trimmed.includes('/') ? `${modelIdPrefix}/${trimmed}` : trimmed
    if (!selected.includes(modelId)) {
      onChange([...selected, modelId])
    }
    setCustomInput('')
    setShowCustomInput(false)
  }

  const testIcon = (state: ModelTestState | undefined) => {
    if (!test) return null
    if (state?.status === 'loading') return <Loader2 size={11} className="animate-spin text-muted-foreground" />
    if (state?.status === 'ok') return <CheckCircle2 size={11} className="text-emerald-600" />
    if (state?.status === 'error') return <XCircle size={11} className="text-destructive" />
    return <TestTube size={11} className="text-muted-foreground/60" />
  }

  const testTitle = (state: ModelTestState | undefined) => {
    if (!test) return undefined
    if (state?.status === 'loading') return 'Testing…'
    if (state?.status === 'ok') return 'Model works on this connection'
    if (state?.status === 'error') return state.message || 'Model test failed'
    return 'Test this model on the connection'
  }

  return (
    <div className="space-y-3">
      {/* Selected models (known + custom) with inline test + remove actions */}
      {selected.length > 0 && (
        <div className="space-y-2 rounded-lg border border-border bg-muted/20 p-3">
          <div className="flex items-center justify-between gap-2">
            <span className="text-[10px] font-bold uppercase tracking-wider text-muted-foreground">
              Selected models · {selected.length}
            </span>
            <div className="flex items-center gap-3">
              {test?.onTestAll && (
                <button
                  onClick={test.onTestAll}
                  disabled={test.running}
                  className="flex items-center gap-1 text-xs font-medium text-muted-foreground transition-colors hover:text-foreground disabled:cursor-not-allowed disabled:opacity-60"
                >
                  {test.running
                    ? <Loader2 size={12} className="animate-spin" />
                    : <TestTube size={12} />}
                  {test.running ? `Testing ${test.progress ?? '…'}` : 'Test all'}
                </button>
              )}
              <button
                onClick={() => onChange([])}
                className="text-xs font-medium text-muted-foreground transition-colors hover:text-foreground"
              >
                Clear all
              </button>
            </div>
          </div>
          <div className="flex flex-wrap gap-1.5">
            {[...selectedKnownModels, ...customModels].map((modelId) => {
              const isCustom = customModels.includes(modelId)
              const state = test?.results[modelId]
              return (
                <span
                  key={modelId}
                  title={isCustom ? 'Custom model (not in registry)' : undefined}
                  className={`inline-flex items-center gap-1 rounded-md border px-2 py-1 text-[11px] font-mono text-foreground shadow-sm ${
                    isCustom
                      ? 'border-dashed border-amber-500/50 bg-amber-500/5'
                      : state?.status === 'ok'
                        ? 'border-emerald-500/30 bg-emerald-500/5'
                        : state?.status === 'error'
                          ? 'border-destructive/30 bg-destructive/5'
                          : 'border-border bg-background'
                  }`}
                >
                  {test && (
                    <button
                      onClick={() => test.onTest(modelId)}
                      disabled={state?.status === 'loading'}
                      title={testTitle(state)}
                      aria-label={testTitle(state)}
                      className="cursor-pointer p-0.5 transition-transform hover:scale-110"
                    >
                      {testIcon(state)}
                    </button>
                  )}
                  <span className="max-w-[220px] truncate">{modelId}</span>
                  <button
                    onClick={() => removeModel(modelId)}
                    aria-label={`Remove ${modelId}`}
                    className="ml-0.5 cursor-pointer rounded-sm p-0.5 text-muted-foreground transition-colors hover:bg-muted hover:text-foreground"
                  >
                    <X size={11} />
                  </button>
                </span>
              )
            })}
          </div>
        </div>
      )}

      {/* Status indicator */}
      {isAllAllowed && (
        <div className="flex w-full items-center justify-center rounded-md border border-emerald-500/20 bg-emerald-500/10 px-3 py-2 text-xs font-medium text-emerald-600 dark:text-emerald-400">
          All models allowed. Select models below to restrict.
        </div>
      )}

      {/* Search */}
      <div className="relative">
        <Search size={14} className="absolute left-3 top-1/2 -translate-y-1/2 text-muted-foreground" />
        <input
          type="text"
          value={searchQuery}
          onChange={e => setSearchQuery(e.target.value)}
          placeholder="Search models..."
          className="flex h-9 w-full rounded-md border border-input bg-transparent px-3 py-1 pl-9 text-sm shadow-sm transition-colors placeholder:text-muted-foreground focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring"
        />
      </div>

      {/* Model list - checked = allowed */}
      <div className="max-h-64 overflow-y-auto p-1 border border-border bg-background rounded-md shadow-sm">
        {loading && (
          <div className="flex items-center justify-center gap-2 py-6 text-muted-foreground">
            <Loader2 size={14} className="animate-spin" />
            <span className="text-sm">Loading models…</span>
          </div>
        )}
        {!loading && filteredModels.length === 0 && (
          <p className="text-sm text-muted-foreground py-6 text-center">No models found</p>
        )}
        <div className="space-y-0.5">
          {filteredModels.map((model) => {
            const isSelected = isSelectedModel(model.id)
            return (
              <button
                key={model.id}
                onClick={() => toggleModel(model.id)}
                className={`w-full flex items-center gap-3 px-2 py-2 rounded-sm text-sm text-left cursor-pointer transition-colors ${
                  isSelected
                    ? 'bg-accent text-accent-foreground font-medium'
                    : 'text-foreground hover:bg-muted'
                }`}
              >
                <div
                  className={`w-4 h-4 rounded-sm border flex items-center justify-center shrink-0 ${
                    isSelected
                      ? 'bg-primary text-primary-foreground border-primary'
                      : 'border-input bg-transparent'
                  }`}
                >
                  {isSelected && <Check size={12} strokeWidth={3} />}
                </div>
                <span className="flex-1 font-mono truncate">{model.id}</span>
              </button>
            )
          })}
        </div>
      </div>

      {/* Add custom model */}
      <div className="pt-2">
        {showCustomInput ? (
          <div className="flex gap-2 items-center bg-muted/30 p-2 rounded-md border border-border">
            <input
              type="text"
              value={customInput}
              onChange={e => setCustomInput(e.target.value)}
              onKeyDown={e => {
                if (e.key === 'Enter') addCustomModel()
                if (e.key === 'Escape') setShowCustomInput(false)
              }}
              placeholder={modelIdPrefix ? `${modelIdPrefix}/model-name` : 'provider/model-name'}
              className="flex h-8 w-full rounded-md border border-input bg-background px-3 py-1 text-sm shadow-sm transition-colors placeholder:text-muted-foreground focus-visible:outline-none flex-1 font-mono"
              autoFocus
            />
            <button
              onClick={addCustomModel}
              disabled={!customInput.trim()}
              className="inline-flex items-center justify-center rounded-md text-xs font-semibold focus-visible:outline-none bg-primary text-primary-foreground hover:bg-primary/90 h-8 px-3"
            >
              Add
            </button>
            <button
              onClick={() => {
                setShowCustomInput(false)
                setCustomInput('')
              }}
              className="inline-flex items-center justify-center rounded-md text-xs font-medium hover:bg-muted hover:text-foreground text-muted-foreground h-8 px-3"
            >
              Cancel
            </button>
          </div>
        ) : (
          <button
            onClick={() => setShowCustomInput(true)}
            className="flex items-center gap-1.5 text-sm text-muted-foreground hover:text-foreground font-medium transition-colors cursor-pointer ml-1"
          >
            <Plus size={14} />
            Add custom model
          </button>
        )}
      </div>
    </div>
  )
}
