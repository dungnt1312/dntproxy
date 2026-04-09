import { useEffect, useState } from 'react'
import { api } from '../api'
import { Plus, Trash2, TestTube, Power, Search, Upload, Shield, Settings2 } from 'lucide-react'

const PROVIDERS = [
  { id: 'kiro', name: 'Kiro AI', icon: '🤖' },
  { id: 'openai', name: 'OpenAI', icon: '🟢' },
  { id: 'openai-compatible', name: 'OpenAI Compatible', icon: '🔌' },
]

type ImportMode = 'detect' | 'file' | 'manual'

export default function Connections() {
  const [conns, setConns] = useState<any[]>([])
  const [showAdd, setShowAdd] = useState(false)
  const [provider, setProvider] = useState('kiro')
  const [importMode, setImportMode] = useState<ImportMode>('detect')
  const [form, setForm] = useState({ refreshToken: '', clientId: '', clientSecret: '', region: '', authMethod: 'builder-id' })
  const [openaiForm, setOpenaiForm] = useState({ name: '', apiKey: '', supportedModels: '' })
  const [customForm, setCustomForm] = useState({ name: '', apiKey: '', baseUrl: '', supportedModels: '' })
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')
  const [success, setSuccess] = useState('')
  const [testResult, setTestResult] = useState<Record<string, any>>({})
  const [expandedModels, setExpandedModels] = useState<Record<string, boolean>>({})
  const [editModels, setEditModels] = useState<Record<string, string>>({})

  const load = () => api.getConnections().then(data => setConns(data || [])).catch(() => {})
  useEffect(() => { load() }, [])

  const resetForm = () => {
    setForm({ refreshToken: '', clientId: '', clientSecret: '', region: '', authMethod: 'builder-id' })
    setOpenaiForm({ name: '', apiKey: '', supportedModels: '' })
    setCustomForm({ name: '', apiKey: '', baseUrl: '', supportedModels: '' })
    setError('')
    setSuccess('')
  }

  const parseSupportedModels = (str: string): string[] => {
    return str.split('\n').map(s => s.trim()).filter(Boolean)
  }

  // Auto-detect from Kiro IDE
  const handleDetect = async () => {
    setLoading(true)
    setError('')
    setSuccess('')
    try {
      const res = await api.detectKiroToken()
      if (res.found) {
        setSuccess(`Detected ${res.authMethod} token (${res.region || 'us-east-1'}). Importing...`)
        await api.importConnection({
          refreshToken: res.refreshToken,
          clientId: res.clientId || '',
          clientSecret: res.clientSecret || '',
          region: res.region || '',
          authMethod: res.authMethod || 'builder-id',
        })
        setSuccess('Connection imported successfully!')
        setShowAdd(false)
        resetForm()
        load()
      } else {
        setError(res.error || 'No Kiro token found. Make sure Kiro IDE is logged in.')
      }
    } catch (e: any) {
      setError(e.message)
    } finally {
      setLoading(false)
    }
  }

  // Import from uploaded JSON file
  const handleFileUpload = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0]
    if (!file) return
    setLoading(true)
    setError('')
    setSuccess('')
    try {
      const text = await file.text()
      const data = JSON.parse(text)
      if (!data.refreshToken) {
        setError('Invalid file: missing refreshToken field')
        return
      }
      await api.importConnection({
        refreshToken: data.refreshToken,
        clientId: data.clientId || '',
        clientSecret: data.clientSecret || '',
        region: data.region || '',
        authMethod: data.authMethod?.toLowerCase() || 'builder-id',
      })
      setSuccess('Connection imported from file!')
      setShowAdd(false)
      resetForm()
      load()
    } catch (e: any) {
      setError(e.message)
    } finally {
      setLoading(false)
    }
  }

  // Manual import (Kiro)
  const handleManualImport = async () => {
    setLoading(true)
    setError('')
    setSuccess('')
    try {
      await api.importConnection(form)
      setSuccess('Connection imported!')
      setShowAdd(false)
      resetForm()
      load()
    } catch (e: any) {
      setError(e.message)
    } finally {
      setLoading(false)
    }
  }

  // Add OpenAI connection
  const handleAddOpenAI = async () => {
    setLoading(true)
    setError('')
    setSuccess('')
    try {
      const models = parseSupportedModels(openaiForm.supportedModels)
      await api.addOpenAIConnection({
        name: openaiForm.name || undefined,
        apiKey: openaiForm.apiKey,
        supportedModels: models.length > 0 ? models : undefined,
      })
      setSuccess('OpenAI connection added!')
      setShowAdd(false)
      resetForm()
      load()
    } catch (e: any) {
      setError(e.message)
    } finally {
      setLoading(false)
    }
  }

  // Add Custom OpenAI Compatible connection
  const handleAddCustom = async () => {
    setLoading(true)
    setError('')
    setSuccess('')
    try {
      const models = parseSupportedModels(customForm.supportedModels)
      await api.addCustomConnection({
        name: customForm.name || undefined,
        apiKey: customForm.apiKey || undefined,
        baseUrl: customForm.baseUrl,
        supportedModels: models.length > 0 ? models : undefined,
      })
      setSuccess('Custom connection added!')
      setShowAdd(false)
      resetForm()
      load()
    } catch (e: any) {
      setError(e.message)
    } finally {
      setLoading(false)
    }
  }

  const handleDelete = async (id: string) => {
    if (!confirm('Remove this connection?')) return
    await api.deleteConnection(id)
    load()
  }

  const handleTest = async (id: string) => {
    setTestResult(prev => ({ ...prev, [id]: { loading: true } }))
    try {
      const res = await api.testConnection(id)
      setTestResult(prev => ({ ...prev, [id]: res }))
    } catch (e: any) {
      setTestResult(prev => ({ ...prev, [id]: { status: 'error', message: e.message } }))
    }
  }

  const handleToggle = async (id: string, isActive: boolean) => {
    await api.updateConnection(id, { isActive: !isActive })
    load()
  }

  const handleSaveModels = async (id: string) => {
    const models = parseSupportedModels(editModels[id] || '')
    await api.updateConnection(id, { supportedModels: models, setModels: true })
    setExpandedModels(prev => ({ ...prev, [id]: false }))
    load()
  }

  const toggleModelEdit = (conn: any) => {
    const id = conn.id
    if (expandedModels[id]) {
      setExpandedModels(prev => ({ ...prev, [id]: false }))
    } else {
      setEditModels(prev => ({ ...prev, [id]: (conn.supportedModels || []).join('\n') }))
      setExpandedModels(prev => ({ ...prev, [id]: true }))
    }
  }

  const getProviderIcon = (provider: string) => {
    const p = PROVIDERS.find(pp => pp.id === provider)
    return p ? p.icon : '📦'
  }

  const getProviderLabel = (provider: string) => {
    const p = PROVIDERS.find(pp => pp.id === provider)
    return p ? p.name : provider
  }

  return (
    <div>
      <div className="flex items-center justify-between mb-6">
        <h2 className="text-2xl font-bold">Connections</h2>
        <button onClick={() => { setShowAdd(!showAdd); resetForm() }} className="flex items-center gap-1.5 px-3 py-1.5 bg-[var(--accent)] hover:bg-[var(--accent-hover)] rounded-lg text-sm transition-colors">
          <Plus size={16} /> Add Connection
        </button>
      </div>

      {showAdd && (
        <div className="bg-[var(--bg-card)] rounded-xl p-4 border border-[var(--border)] mb-6 space-y-4">
          {/* Provider Selection */}
          <div>
            <label className="block text-xs text-[var(--text-muted)] mb-2">Provider</label>
            <div className="flex gap-2 flex-wrap">
              {PROVIDERS.map(p => (
                <button key={p.id} onClick={() => { setProvider(p.id); resetForm() }}
                  className={`flex items-center gap-2 px-4 py-2 rounded-lg border text-sm transition-colors ${
                    provider === p.id ? 'border-[var(--accent)] bg-[var(--accent)]/10 text-[var(--accent)]' : 'border-[var(--border)] hover:border-[var(--bg-hover)]'
                  }`}>
                  <span>{p.icon}</span> {p.name}
                </button>
              ))}
            </div>
          </div>

          {/* Kiro Import Modes */}
          {provider === 'kiro' && (
            <>
              <div>
                <label className="block text-xs text-[var(--text-muted)] mb-2">Import Method</label>
                <div className="flex gap-2">
                  <button onClick={() => setImportMode('detect')}
                    className={`flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-sm transition-colors ${
                      importMode === 'detect' ? 'bg-[var(--accent)] text-white' : 'bg-[var(--bg)] text-[var(--text-muted)] hover:text-[var(--text)]'
                    }`}>
                    <Search size={14} /> Detect from IDE
                  </button>
                  <button onClick={() => setImportMode('file')}
                    className={`flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-sm transition-colors ${
                      importMode === 'file' ? 'bg-[var(--accent)] text-white' : 'bg-[var(--bg)] text-[var(--text-muted)] hover:text-[var(--text)]'
                    }`}>
                    <Upload size={14} /> Import File
                  </button>
                  <button onClick={() => setImportMode('manual')}
                    className={`flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-sm transition-colors ${
                      importMode === 'manual' ? 'bg-[var(--accent)] text-white' : 'bg-[var(--bg)] text-[var(--text-muted)] hover:text-[var(--text)]'
                    }`}>
                    Paste Token
                  </button>
                </div>
              </div>

              {/* Detect from IDE */}
              {importMode === 'detect' && (
                <div className="space-y-3">
                  <p className="text-sm text-[var(--text-muted)]">
                    Auto-detect Kiro token from <code className="text-xs bg-[var(--bg)] px-1.5 py-0.5 rounded">~/.aws/sso/cache/kiro-auth-token.json</code>
                  </p>
                  <p className="text-xs text-[var(--text-muted)]">Make sure you're logged into Kiro IDE first.</p>
                  <button onClick={handleDetect} disabled={loading}
                    className="flex items-center gap-1.5 px-4 py-2 bg-[var(--accent)] hover:bg-[var(--accent-hover)] rounded-lg text-sm disabled:opacity-50 transition-colors">
                    <Search size={16} /> {loading ? 'Detecting...' : 'Detect & Import'}
                  </button>
                </div>
              )}

              {/* Import File */}
              {importMode === 'file' && (
                <div className="space-y-3">
                  <p className="text-sm text-[var(--text-muted)]">
                    Upload a <code className="text-xs bg-[var(--bg)] px-1.5 py-0.5 rounded">kiro-auth-token.json</code> file.
                  </p>
                  <label className="flex items-center gap-2 px-4 py-3 bg-[var(--bg)] border border-dashed border-[var(--border)] rounded-lg cursor-pointer hover:border-[var(--accent)] transition-colors">
                    <Upload size={16} className="text-[var(--text-muted)]" />
                    <span className="text-sm text-[var(--text-muted)]">{loading ? 'Importing...' : 'Choose JSON file...'}</span>
                    <input type="file" accept=".json" onChange={handleFileUpload} className="hidden" disabled={loading} />
                  </label>
                </div>
              )}

              {/* Manual Paste */}
              {importMode === 'manual' && (
                <div className="space-y-3">
                  <div>
                    <label className="block text-xs text-[var(--text-muted)] mb-1">Refresh Token *</label>
                    <input value={form.refreshToken} onChange={e => setForm({ ...form, refreshToken: e.target.value })}
                      placeholder="aorAAAAAG..."
                      className="w-full bg-[var(--bg)] border border-[var(--border)] rounded-lg px-3 py-2 text-sm focus:border-[var(--accent)] outline-none" />
                  </div>
                  <div className="grid grid-cols-2 gap-3">
                    <div>
                      <label className="block text-xs text-[var(--text-muted)] mb-1">Auth Method</label>
                      <select value={form.authMethod} onChange={e => setForm({ ...form, authMethod: e.target.value })}
                        className="w-full bg-[var(--bg)] border border-[var(--border)] rounded-lg px-3 py-2 text-sm">
                        <option value="builder-id">AWS Builder ID</option>
                        <option value="idc">AWS IAM Identity Center</option>
                      </select>
                    </div>
                    <div>
                      <label className="block text-xs text-[var(--text-muted)] mb-1">Region</label>
                      <input value={form.region} onChange={e => setForm({ ...form, region: e.target.value })}
                        placeholder="us-east-1"
                        className="w-full bg-[var(--bg)] border border-[var(--border)] rounded-lg px-3 py-2 text-sm" />
                    </div>
                  </div>
                  <div className="grid grid-cols-2 gap-3">
                    <div>
                      <label className="block text-xs text-[var(--text-muted)] mb-1">Client ID (optional)</label>
                      <input value={form.clientId} onChange={e => setForm({ ...form, clientId: e.target.value })}
                        className="w-full bg-[var(--bg)] border border-[var(--border)] rounded-lg px-3 py-2 text-sm" />
                    </div>
                    <div>
                      <label className="block text-xs text-[var(--text-muted)] mb-1">Client Secret (optional)</label>
                      <input value={form.clientSecret} onChange={e => setForm({ ...form, clientSecret: e.target.value })}
                        className="w-full bg-[var(--bg)] border border-[var(--border)] rounded-lg px-3 py-2 text-sm" />
                    </div>
                  </div>
                  <button onClick={handleManualImport} disabled={loading || !form.refreshToken}
                    className="px-4 py-2 bg-[var(--accent)] hover:bg-[var(--accent-hover)] rounded-lg text-sm disabled:opacity-50 transition-colors">
                    {loading ? 'Validating...' : 'Import'}
                  </button>
                </div>
              )}
            </>
          )}

          {/* OpenAI Form */}
          {provider === 'openai' && (
            <div className="space-y-3">
              <div>
                <label className="block text-xs text-[var(--text-muted)] mb-1">Name (optional)</label>
                <input value={openaiForm.name} onChange={e => setOpenaiForm({ ...openaiForm, name: e.target.value })}
                  placeholder="My OpenAI Account"
                  className="w-full bg-[var(--bg)] border border-[var(--border)] rounded-lg px-3 py-2 text-sm focus:border-[var(--accent)] outline-none" />
              </div>
              <div>
                <label className="block text-xs text-[var(--text-muted)] mb-1">API Key *</label>
                <input type="password" value={openaiForm.apiKey} onChange={e => setOpenaiForm({ ...openaiForm, apiKey: e.target.value })}
                  placeholder="sk-..."
                  className="w-full bg-[var(--bg)] border border-[var(--border)] rounded-lg px-3 py-2 text-sm font-mono focus:border-[var(--accent)] outline-none" />
              </div>
              <div>
                <label className="block text-xs text-[var(--text-muted)] mb-1">
                  <Shield size={12} className="inline mr-1" />
                  Supported Models <span className="text-[var(--text-muted)]">(optional, one per line, empty = all)</span>
                </label>
                <textarea value={openaiForm.supportedModels} onChange={e => setOpenaiForm({ ...openaiForm, supportedModels: e.target.value })}
                  rows={3} placeholder={"gpt-4o\no3\ngpt-4.1*"}
                  className="w-full bg-[var(--bg)] border border-[var(--border)] rounded-lg px-3 py-2 text-sm font-mono focus:border-[var(--accent)] outline-none resize-none" />
                <p className="text-xs text-[var(--text-muted)] mt-1">Use * for wildcard: <code className="bg-[var(--bg)] px-1 rounded">gpt-4*</code> matches all GPT-4 models</p>
              </div>
              <button onClick={handleAddOpenAI} disabled={loading || !openaiForm.apiKey}
                className="px-4 py-2 bg-[var(--accent)] hover:bg-[var(--accent-hover)] rounded-lg text-sm disabled:opacity-50 transition-colors">
                {loading ? 'Adding...' : 'Add OpenAI'}
              </button>
            </div>
          )}

          {/* OpenAI Compatible Form */}
          {provider === 'openai-compatible' && (
            <div className="space-y-3">
              <div>
                <label className="block text-xs text-[var(--text-muted)] mb-1">Name (optional)</label>
                <input value={customForm.name} onChange={e => setCustomForm({ ...customForm, name: e.target.value })}
                  placeholder="My Custom API"
                  className="w-full bg-[var(--bg)] border border-[var(--border)] rounded-lg px-3 py-2 text-sm focus:border-[var(--accent)] outline-none" />
              </div>
              <div>
                <label className="block text-xs text-[var(--text-muted)] mb-1">Base URL *</label>
                <input value={customForm.baseUrl} onChange={e => setCustomForm({ ...customForm, baseUrl: e.target.value })}
                  placeholder="https://api.together.xyz"
                  className="w-full bg-[var(--bg)] border border-[var(--border)] rounded-lg px-3 py-2 text-sm font-mono focus:border-[var(--accent)] outline-none" />
                <p className="text-xs text-[var(--text-muted)] mt-1">The proxy will call <code className="bg-[var(--bg)] px-1 rounded">{customForm.baseUrl || 'https://...'}/v1/chat/completions</code></p>
              </div>
              <div>
                <label className="block text-xs text-[var(--text-muted)] mb-1">API Key (optional)</label>
                <input type="password" value={customForm.apiKey} onChange={e => setCustomForm({ ...customForm, apiKey: e.target.value })}
                  placeholder="sk-..."
                  className="w-full bg-[var(--bg)] border border-[var(--border)] rounded-lg px-3 py-2 text-sm font-mono focus:border-[var(--accent)] outline-none" />
              </div>
              <div>
                <label className="block text-xs text-[var(--text-muted)] mb-1">
                  <Shield size={12} className="inline mr-1" />
                  Supported Models <span className="text-[var(--text-muted)]">(optional, one per line, empty = all)</span>
                </label>
                <textarea value={customForm.supportedModels} onChange={e => setCustomForm({ ...customForm, supportedModels: e.target.value })}
                  rows={3} placeholder={"llama-3.1-70b\nmistral-*"}
                  className="w-full bg-[var(--bg)] border border-[var(--border)] rounded-lg px-3 py-2 text-sm font-mono focus:border-[var(--accent)] outline-none resize-none" />
                <p className="text-xs text-[var(--text-muted)] mt-1">Use * for wildcard: <code className="bg-[var(--bg)] px-1 rounded">llama-*</code> matches all Llama models</p>
              </div>
              <button onClick={handleAddCustom} disabled={loading || !customForm.baseUrl}
                className="px-4 py-2 bg-[var(--accent)] hover:bg-[var(--accent-hover)] rounded-lg text-sm disabled:opacity-50 transition-colors">
                {loading ? 'Adding...' : 'Add Custom'}
              </button>
            </div>
          )}

          {error && <p className="text-sm text-[var(--danger)]">{error}</p>}
          {success && <p className="text-sm text-[var(--success)]">{success}</p>}
        </div>
      )}

      {conns.length === 0 ? (
        <p className="text-[var(--text-muted)]">No connections configured.</p>
      ) : (
        <div className="space-y-3">
          {conns.map((c: any) => (
            <div key={c.id} className="bg-[var(--bg-card)] rounded-xl p-4 border border-[var(--border)]">
              <div className="flex items-center justify-between mb-2">
                <div className="flex items-center gap-2">
                  <span className={`w-2 h-2 rounded-full ${c.isActive && (c.hasToken || c.hasApiKey) ? 'bg-[var(--success)]' : 'bg-[var(--text-muted)]'}`} />
                  <span className="font-medium">{c.name}</span>
                  <span className="text-xs text-[var(--text-muted)] bg-[var(--bg)] px-2 py-0.5 rounded flex items-center gap-1">
                    <span>{getProviderIcon(c.provider)}</span> {getProviderLabel(c.provider)}
                  </span>
                  {c.providerName || c.authMethod ? (
                    <span className="text-xs text-[var(--text-muted)] bg-[var(--bg)] px-2 py-0.5 rounded">{c.providerName || c.authMethod}</span>
                  ) : null}
                  {c.authType === 'apikey' && (
                    <span className="text-xs text-amber-400 bg-amber-400/10 px-2 py-0.5 rounded">API Key</span>
                  )}
                </div>
                <div className="flex items-center gap-1">
                  <button onClick={() => toggleModelEdit(c)} className="p-1.5 hover:bg-[var(--bg-hover)] rounded-lg transition-colors" title="Supported Models">
                    <Settings2 size={14} className={`${c.supportedModels?.length > 0 ? 'text-[var(--accent)]' : 'text-[var(--text-muted)]'}`} />
                  </button>
                  <button onClick={() => handleTest(c.id)} className="p-1.5 hover:bg-[var(--bg-hover)] rounded-lg transition-colors" title="Test">
                    <TestTube size={14} className="text-[var(--text-muted)]" />
                  </button>
                  <button onClick={() => handleToggle(c.id, c.isActive)} className="p-1.5 hover:bg-[var(--bg-hover)] rounded-lg transition-colors" title={c.isActive ? 'Disable' : 'Enable'}>
                    <Power size={14} className={c.isActive ? 'text-[var(--success)]' : 'text-[var(--text-muted)]'} />
                  </button>
                  <button onClick={() => handleDelete(c.id)} className="p-1.5 hover:bg-[var(--bg-hover)] rounded-lg transition-colors" title="Remove">
                    <Trash2 size={14} className="text-[var(--danger)]" />
                  </button>
                </div>
              </div>

              {/* Info row */}
              <div className="text-xs text-[var(--text-muted)] space-x-4">
                {c.email && <span>Email: {c.email}</span>}
                {c.baseUrl && <span className="font-mono">URL: {c.baseUrl}</span>}
                {c.expiresAt && <span>Expires: {new Date(c.expiresAt).toLocaleString()}</span>}
                {c.lastError && <span className="text-[var(--danger)]">Error: {c.lastError.slice(0, 80)}</span>}
              </div>

              {/* Supported models badges */}
              {c.supportedModels?.length > 0 && !expandedModels[c.id] && (
                <div className="flex flex-wrap gap-1.5 mt-2">
                  <span className="text-xs text-[var(--text-muted)]">Models:</span>
                  {c.supportedModels.map((m: string, i: number) => (
                    <span key={i} className="text-xs font-mono bg-[var(--accent)]/10 text-[var(--accent)] px-2 py-0.5 rounded">{m}</span>
                  ))}
                </div>
              )}

              {/* Model edit panel */}
              {expandedModels[c.id] && (
                <div className="mt-3 pt-3 border-t border-[var(--border)] space-y-2">
                  <label className="block text-xs text-[var(--text-muted)]">
                    <Shield size={12} className="inline mr-1" />
                    Supported Models (one per line, empty = all models)
                  </label>
                  <textarea
                    value={editModels[c.id] || ''}
                    onChange={e => setEditModels(prev => ({ ...prev, [c.id]: e.target.value }))}
                    rows={3}
                    placeholder={"claude-opus-4*\nclaude-sonnet-4*"}
                    className="w-full bg-[var(--bg)] border border-[var(--border)] rounded-lg px-3 py-2 text-xs font-mono focus:border-[var(--accent)] outline-none resize-none"
                  />
                  <p className="text-xs text-[var(--text-muted)]">Use * for wildcard: <code className="bg-[var(--bg)] px-1 rounded">claude-*</code> matches all Claude models</p>
                  <div className="flex gap-2">
                    <button onClick={() => handleSaveModels(c.id)}
                      className="px-3 py-1 bg-[var(--accent)] hover:bg-[var(--accent-hover)] rounded-lg text-xs transition-colors">
                      Save
                    </button>
                    <button onClick={() => setExpandedModels(prev => ({ ...prev, [c.id]: false }))}
                      className="px-3 py-1 bg-[var(--bg)] hover:bg-[var(--bg-hover)] rounded-lg text-xs transition-colors text-[var(--text-muted)]">
                      Cancel
                    </button>
                  </div>
                </div>
              )}

              {testResult[c.id] && (
                <div className={`mt-2 text-xs px-2 py-1 rounded ${testResult[c.id].status === 'ok' ? 'bg-green-900/30 text-green-400' : 'bg-red-900/30 text-red-400'}`}>
                  {testResult[c.id].loading ? 'Testing...' : testResult[c.id].status === 'ok' ? 'Connection OK' : testResult[c.id].message}
                </div>
              )}
            </div>
          ))}
        </div>
      )}
    </div>
  )
}
