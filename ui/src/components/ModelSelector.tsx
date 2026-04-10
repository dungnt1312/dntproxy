import { useState } from 'react'
import { Plus, X, Search, Check } from 'lucide-react'
import { MODELS_CONFIG, PROVIDER_CONFIGS, getModelsByProvider, getModelIdPrefixForProvider } from '../models-config'

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

  const providerConfig = provider ? PROVIDER_CONFIGS[provider] : null

  return (
    <div className="space-y-3">
      {/* Selected known model tags */}
      {selectedKnownModels.length > 0 && (
        <div className="flex flex-wrap gap-2 p-3 bg-[var(--accent)]/10 rounded-lg border border-[var(--accent)]/30">
          <span className="text-xs text-[var(--accent)] w-full mb-1">Allowed models:</span>
          {selectedKnownModels.map((modelId) => {
            const config = providerConfig || PROVIDER_CONFIGS.other
            return (
              <span
                key={modelId}
                className="inline-flex items-center gap-1.5 px-2.5 py-1 bg-[var(--accent)]/20 border border-[var(--accent)]/40 rounded-lg text-xs"
              >
                <span>{config.icon}</span>
                <span className="text-blue-200">{modelId}</span>
                <button
                  onClick={() => removeModel(modelId)}
                  aria-label={`Remove allowed model ${modelId}`}
                  className="ml-0.5 hover:bg-[var(--accent)]/30 rounded-sm p-0.5 transition-colors"
                >
                  <X size={12} className="text-blue-200" />
                </button>
              </span>
            )
          })}
        </div>
      )}

      {/* Custom models tags */}
      {customModels.length > 0 && (
        <div className="flex flex-wrap gap-2 p-3 bg-purple-500/10 rounded-lg border border-purple-500/30">
          <span className="text-xs text-purple-400 w-full mb-1">Custom models:</span>
          {customModels.map((modelId) => {
            const config = providerConfig || PROVIDER_CONFIGS.other
            return (
              <span
                key={modelId}
                className="inline-flex items-center gap-1.5 px-2.5 py-1 bg-purple-500/20 border border-purple-500/40 rounded-lg text-xs"
              >
                <span>{config.icon}</span>
                <span className="text-purple-300">{modelId}</span>
                <button
                  onClick={() => removeModel(modelId)}
                  aria-label={`Remove custom model ${modelId}`}
                  className="ml-0.5 hover:bg-purple-500/30 rounded-sm p-0.5 transition-colors"
                >
                  <X size={12} className="text-purple-400" />
                </button>
              </span>
            )
          })}
        </div>
      )}

      {/* Status indicator */}
      {isAllAllowed ? (
        <div className="text-xs text-green-400 bg-green-500/10 rounded-lg px-3 py-2 border border-green-500/30">
          All models allowed. Select models below to restrict this connection.
        </div>
      ) : (
        <div className="flex items-center justify-between text-xs text-[var(--text-muted)] bg-[var(--bg)] rounded-lg px-3 py-2 border border-[var(--border)]">
          <span>Restricted to {selected.length} selected model(s).</span>
          <button
            onClick={() => onChange([])}
            className="text-[var(--accent)] hover:text-[var(--accent-hover)] transition-colors"
          >
            Clear restrictions
          </button>
        </div>
      )}

      {/* Search */}
      <div className="relative">
        <Search size={14} className="absolute left-3 top-1/2 -translate-y-1/2 text-[var(--text-muted)]" />
        <input
          type="text"
          value={searchQuery}
          onChange={e => setSearchQuery(e.target.value)}
          placeholder="Search models..."
          className="w-full bg-[var(--bg)] border border-[var(--border)] rounded-lg pl-9 pr-3 py-2 text-sm focus:border-[var(--accent)] outline-none"
        />
      </div>

      {/* Model list - checked = allowed */}
      <div className="max-h-64 overflow-y-auto space-y-1 bg-[var(--bg)] rounded-lg border border-[var(--border)] p-2">
        {filteredModels.length === 0 && (
          <p className="text-sm text-[var(--text-muted)] py-4 text-center">No models found</p>
        )}
        {filteredModels.map((model) => {
          const isSelected = isSelectedModel(model.id)
          return (
            <button
              key={model.id}
              onClick={() => toggleModel(model.id)}
              className={`w-full flex items-center gap-3 px-3 py-2 rounded-lg text-left text-sm transition-colors ${
                isSelected
                  ? 'text-[var(--text)] bg-[var(--accent)]/10'
                  : 'text-[var(--text-muted)] hover:text-[var(--text)] hover:bg-[var(--bg-hover)]'
              }`}
            >
              <span
                className={`w-5 h-5 border-2 rounded flex items-center justify-center transition-colors ${
                  isSelected
                    ? 'border-[var(--accent)] bg-[var(--accent)]'
                    : 'border-[var(--border)]'
                }`}
              >
                {isSelected && <Check size={12} className="text-white" />}
              </span>
              <span className="flex-1 font-medium font-mono">{model.id}</span>
              {isSelected && <span className="text-xs text-[var(--accent)]">selected</span>}
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
              className="flex-1 bg-[var(--bg)] border border-[var(--border)] rounded-lg px-3 py-2 text-sm font-mono focus:border-[var(--accent)] outline-none"
              autoFocus
            />
            <button
              onClick={addCustomModel}
              disabled={!customInput.trim()}
              className="px-3 py-2 bg-[var(--accent)] hover:bg-[var(--accent-hover)] disabled:opacity-50 rounded-lg text-sm transition-colors"
            >
              Add
            </button>
            <button
              onClick={() => {
                setShowCustomInput(false)
                setCustomInput('')
              }}
              className="px-3 py-2 bg-[var(--bg)] hover:bg-[var(--bg-hover)] rounded-lg text-sm transition-colors"
            >
              Cancel
            </button>
          </div>
        ) : (
          <button
            onClick={() => setShowCustomInput(true)}
            className="flex items-center gap-2 text-sm text-purple-400 hover:text-purple-300 transition-colors"
          >
            <Plus size={14} />
            Add custom model
          </button>
        )}
      </div>
    </div>
  )
}
