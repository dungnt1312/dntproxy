import { useCallback, useEffect, useMemo, useState } from 'react'
import { Search, Layers, Plus, Trash2, ChevronRight, Link2, Zap } from 'lucide-react'
import { toast } from 'sonner'

import { goApi } from '@/lib/go-api'

interface UiModel {
  id: string
  name: string
  provider: string
  models?: string[]
}

type AliasMap = Record<string, string>

function providerLabel(provider: string) {
  if (provider === 'kiro') return 'Kiro'
  if (provider === 'openai') return 'OpenAI'
  if (provider === 'combo') return 'Combo'
  if (provider === 'alias') return 'Alias'
  return provider || 'Other'
}

export default function ModelsScreen() {
  const [models, setModels] = useState<UiModel[]>([])
  const [aliases, setAliases] = useState<AliasMap>({})
  const [loading, setLoading] = useState(true)
  const [search, setSearch] = useState('')

  const [showAdd, setShowAdd] = useState(false)
  const [aliasInput, setAliasInput] = useState('')
  const [modelInput, setModelInput] = useState('')
  const [savingAlias, setSavingAlias] = useState(false)

  const fetchAll = useCallback(async () => {
    setLoading(true)
    try {
      const [modelsData, aliasesData] = await Promise.all([
        goApi.getModels().catch(() => []),
        goApi.getAliases().catch(() => ({})),
      ])

      setModels(Array.isArray(modelsData) ? modelsData : [])
      setAliases(aliasesData || {})
    } catch {
      toast.error('Failed to load models')
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    fetchAll()
  }, [fetchAll])

  const q = search.trim().toLowerCase()

  const filteredModels = useMemo(
    () => (!q ? models : models.filter((m) => m.id.toLowerCase().includes(q) || m.name.toLowerCase().includes(q))),
    [models, q]
  )

  const sections = useMemo(() => {
    const byProvider: Record<string, UiModel[]> = {}
    filteredModels.forEach((model) => {
      if (!byProvider[model.provider]) byProvider[model.provider] = []
      byProvider[model.provider].push(model)
    })
    return Object.entries(byProvider)
  }, [filteredModels])

  const filteredAliases = useMemo(() => {
    const entries = Object.entries(aliases)
    if (!q) return entries
    return entries.filter(([alias, target]) => alias.toLowerCase().includes(q) || target.toLowerCase().includes(q))
  }, [aliases, q])

  async function handleAddAlias() {
    if (!aliasInput.trim() || !modelInput.trim()) {
      toast.error('Alias and target model are required')
      return
    }

    setSavingAlias(true)
    try {
      await goApi.setAlias(aliasInput.trim(), modelInput.trim())
      setAliasInput('')
      setModelInput('')
      setShowAdd(false)
      toast.success('Alias created')
      await fetchAll()
    } catch {
      toast.error('Failed to create alias')
    } finally {
      setSavingAlias(false)
    }
  }

  async function handleDeleteAlias(alias: string) {
    try {
      await goApi.deleteAlias(alias)
      toast.success('Alias deleted')
      await fetchAll()
    } catch {
      toast.error('Failed to delete alias')
    }
  }

  return (
    <div className="space-y-6">
      <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <div>
          <h1 className="text-2xl font-bold tracking-tight">Models & Aliases</h1>
          <p className="text-sm text-muted-foreground">Models are sourced from active provider connections, aliases, and backend combos.</p>
        </div>
      </div>

      <div className="rounded-lg border p-3">
        <div className="relative">
          <Search className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
          <input
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            placeholder="Filter models by id or name"
            className="h-9 w-full rounded-md border bg-background pl-9 pr-3 text-sm"
          />
        </div>
      </div>

      {loading ? (
        <div className="text-sm text-muted-foreground">Loading models...</div>
      ) : (
        <div className="space-y-4">
          {sections.length === 0 ? (
            <div className="rounded-lg border border-dashed p-8 text-center text-sm text-muted-foreground">
              No models found.
            </div>
          ) : (
            sections.map(([provider, list]) => (
              <div key={provider} className="rounded-lg border">
                <div className="flex items-center justify-between border-b px-4 py-2">
                  <div className="flex items-center gap-2 text-sm font-medium">
                    <Layers className="h-4 w-4 text-muted-foreground" />
                    {providerLabel(provider)}
                  </div>
                  <span className="text-xs text-muted-foreground">{list.length}</span>
                </div>

                <div className="divide-y">
                  {list.map((item) => (
                    <div key={item.id} className="flex flex-col gap-1 px-4 py-2 text-sm sm:flex-row sm:items-center sm:justify-between">
                      <div className="min-w-0">
                        <div className="truncate font-medium">{item.name}</div>
                        <div className="truncate font-mono text-xs text-muted-foreground">{item.id}</div>
                      </div>
                      {item.models && item.models.length > 0 && (
                        <span className="self-start text-xs text-muted-foreground sm:self-auto">{item.models.length} items</span>
                      )}
                    </div>
                  ))}
                </div>
              </div>
            ))
          )}
        </div>
      )}

      <div id="aliases" className="space-y-3 rounded-lg border p-4">
        <div className="flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
          <h2 className="text-sm font-semibold">Aliases</h2>
          <button
            onClick={() => setShowAdd((v) => !v)}
            className="inline-flex h-8 items-center gap-1 self-start rounded-md border px-2 py-1 text-xs hover:bg-accent"
          >
            <Plus className="h-3 w-3" /> Add Alias
          </button>
        </div>

        {showAdd && (
          <div className="grid gap-2 sm:grid-cols-[1fr_1fr_auto]">
            <input
              value={aliasInput}
              onChange={(e) => setAliasInput(e.target.value)}
              placeholder="alias"
              className="h-9 rounded-md border px-3 text-sm"
            />
            <input
              value={modelInput}
              onChange={(e) => setModelInput(e.target.value)}
              placeholder="provider/model"
              className="h-9 rounded-md border px-3 text-sm"
            />
            <button
              onClick={handleAddAlias}
              disabled={savingAlias}
              className="inline-flex h-9 items-center justify-center rounded-md bg-primary px-3 text-sm text-primary-foreground"
            >
              {savingAlias ? 'Saving...' : 'Save'}
            </button>
          </div>
        )}

        {filteredAliases.length === 0 ? (
          <div className="text-sm text-muted-foreground">No aliases configured.</div>
        ) : (
          <div className="space-y-2">
            {filteredAliases.map(([alias, target]) => (
              <div key={alias} className="flex flex-col gap-2 rounded-md border px-3 py-2 sm:flex-row sm:items-center sm:justify-between">
                <div className="min-w-0 text-sm">
                  <div className="font-mono">{alias}</div>
                  <div className="font-mono text-xs text-muted-foreground">{target}</div>
                </div>
                <button
                  onClick={() => handleDeleteAlias(alias)}
                  className="inline-flex h-8 w-8 items-center justify-center self-start rounded-md text-destructive hover:bg-destructive/10 sm:self-auto"
                >
                  <Trash2 className="h-4 w-4" />
                </button>
              </div>
            ))}
          </div>
        )}

        <div className="flex items-center gap-2 text-xs text-muted-foreground">
          <Link2 className="h-3 w-3" />
          Use aliases as model IDs in the playground and client requests.
          <ChevronRight className="h-3 w-3" />
          <Zap className="h-3 w-3" />
        </div>
      </div>
    </div>
  )
}
