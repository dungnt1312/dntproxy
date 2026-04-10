import { useState } from 'react'
import { Plus, X, Search, Check } from 'lucide-react'
import { MODELS_CONFIG, getModelsByProvider, getModelIdPrefixForProvider } from '../models-config'

interface ModelSelectorProps {
  selected: string[]
  onChange: (models: string[]) => void
  provider?: string
  allowedModels?: string[] // if set, only show these model IDs
}

export default function ModelSelector({
  selected,
  onChange,
  provider,
  allowedModels,
}: ModelSelectorProps) {
  const [customInput, setCustomInput] = useState('')
  const [showCustomInput, setShowCustomInput] = useState(false)
  const [searchQuery, setSearchQuery] = useState('')

  const baseModels = provider ? getModelsByProvider(provider) : MODELS_CONFIG
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

  const modelIdPrefix = provider ? (getModelIdPrefixForProvider(provider) ?? provider) : null

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

  return (
    <div className="space-y-3">
      {/* Selected known model tags */}
      {selectedKnownModels.length > 0 && (
        <div className="flex flex-wrap gap-2 p-3 bg-[var(--accent-glow)] rounded-xl border border-[var(--accent)]/20">
          <span className="text-[10px] text-[var(--accent)] w-full mb-0.5 font-medium uppercase tracking-wide">Allowed models:</span>
          {selectedKnownModels.map((modelId) => {
            return (
              <span
                key={modelId}
                className="inline-flex items-center gap-1 px-2 py-0.5 bg-[var(--accent)]/15 border border-[var(--accent)]/25 rounded-md text-[11px] font-mono"
              >
                <span className="text-[var(--text-muted)]">{modelId}</span>
                <button
                  onClick={() => removeModel(modelId)}
                  aria-label={`Remove allowed model ${modelId}`}
                  className="ml-0.5 hover:bg-[var(--accent)]/20 rounded-sm p-0.5 transition-colors cursor-pointer"
                >
                  <X size={10} className="text-[var(--accent)]" />
                </button>
              </span>
            )
          })}
        </div>
      )}

      {/* Custom models tags */}
      {customModels.length > 0 && (
        <div className="flex flex-wrap gap-2 p-3 bg-[var(--purple-glow)] rounded-xl border border-[var(--purple)]/20">
          <span className="text-[10px] text-[var(--purple)] w-full mb-0.5 font-medium uppercase tracking-wide">Custom models:</span>
          {customModels.map((modelId) => {
            return (
              <span
                key={modelId}
                className="inline-flex items-center gap-1 px-2 py-0.5 bg-[var(--purple)]/15 border border-[var(--purple)]/25 rounded-md text-[11px] font-mono"
              >
                <span className="text-[var(--text-muted)]">{modelId}</span>
                <button
                  onClick={() => removeModel(modelId)}
                  aria-label={`Remove custom model ${modelId}`}
                  className="ml-0.5 hover:bg-[var(--purple)]/20 rounded-sm p-0.5 transition-colors cursor-pointer"
                >
                  <X size={10} className="text-[var(--purple)]" />
                </button>
              </span>
            )
          })}
        </div>
      )}

      {/* Status indicator */}
      {isAllAllowed ? (
        <div className="chip chip-success text-xs w-full justify-center py-2">
          All models allowed. Select models below to restrict.
        </div>
      ) : (
        <div className="flex items-center justify-between text-xs text-[var(--text-muted)] glass-sm px-3 py-2">
          <span>Restricted to {selected.length} selected model(s).</span>
          <button
            onClick={() => onChange([])}
            className="text-[var(--accent)] hover:text-[var(--accent-hover)] transition-colors cursor-pointer font-medium"
          >
            Clear restrictions
          </button>
        </div>
      )}

      {/* Search */}
      <div className="relative">
        <Search size={13} className="absolute left-3 top-1/2 -translate-y-1/2 text-[var(--text-dim)]" />
        <input
          type="text"
          value={searchQuery}
          onChange={e => setSearchQuery(e.target.value)}
          placeholder="Search models..."
          className="glass-input w-full pl-9 text-xs"
        />
      </div>

      {/* Model list - checked = allowed */}
      <div className="max-h-64 overflow-y-auto space-y-0.5 glass-sm p-1.5">
        {filteredModels.length === 0 && (
          <p className="text-xs text-[var(--text-dim)] py-4 text-center">No models found</p>
        )}
        {filteredModels.map((model) => {
          const isSelected = isSelectedModel(model.id)
          return (
            <button
              key={model.id}
              onClick={() => toggleModel(model.id)}
              className={`w-full flex items-center gap-3 px-3 py-2 rounded-lg text-left text-xs cursor-pointer transition-all ${
                isSelected
                  ? 'text-[var(--text)] bg-[var(--accent-glow)]'
                  : 'text-[var(--text-muted)] hover:text-[var(--text)] hover:bg-white/[0.03]'
              }`}
            >
              <span
                className={`w-4 h-4 border-2 rounded flex items-center justify-center transition-colors shrink-0 ${
                  isSelected
                    ? 'border-[var(--accent)] bg-[var(--accent)]'
                    : 'border-[var(--border)]'
                }`}
              >
                {isSelected && <Check size={10} className="text-white" />}
              </span>
              <span className="flex-1 font-mono truncate">{model.id}</span>
              {isSelected && <span className="text-[10px] text-[var(--accent)] shrink-0">selected</span>}
            </button>
          )
        })}
      </div>

      {/* Add custom model */}
      <div>
        {showCustomInput ? (
          <div className="flex gap-2">
            <input
              type="text"
              value={customInput}
              onChange={e => setCustomInput(e.target.value)}
              onKeyDown={e => {
                if (e.key === 'Enter') addCustomModel()
                if (e.key === 'Escape') setShowCustomInput(false)
              }}
              placeholder={modelIdPrefix ? `${modelIdPrefix}/model-name` : 'provider/model-name'}
              className="glass-input flex-1 text-xs font-mono"
              autoFocus
            />
            <button
              onClick={addCustomModel}
              disabled={!customInput.trim()}
              className="btn-primary text-xs py-2"
            >
              Add
            </button>
            <button
              onClick={() => {
                setShowCustomInput(false)
                setCustomInput('')
              }}
              className="btn-ghost text-xs py-2"
            >
              Cancel
            </button>
          </div>
        ) : (
          <button
            onClick={() => setShowCustomInput(true)}
            className="flex items-center gap-1.5 text-xs text-[var(--purple)] hover:text-[var(--purple)] font-medium transition-colors cursor-pointer"
          >
            <Plus size={13} />
            Add custom model
          </button>
        )}
      </div>
    </div>
  )
}
