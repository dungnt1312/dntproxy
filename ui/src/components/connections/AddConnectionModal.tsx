import { useState, useEffect, useRef, useCallback } from 'react'
import {
  Loader2, Search, Upload, Shield, ExternalLink,
  Globe, GitBranch, Link2, CheckCircle2, AlertTriangle, Play
} from 'lucide-react'
import { api } from '../../api'
import type { ImportMode, DeviceCodeState, SocialLoginState } from './helpers'
import { ProviderLogoIcon } from './helpers'
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
} from '@/components/ui/dialog'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'
import { cn } from '@/lib/utils'

interface AddConnectionModalProps {
  onSuccess: (message: string) => void
  onClose: () => void
}

export default function AddConnectionModal({ onSuccess, onClose }: AddConnectionModalProps) {
  const [provider, setProvider] = useState('kiro')
  const [importMode, setImportMode] = useState<ImportMode>('detect')
  const [form, setForm] = useState({ refreshToken: '', clientId: '', clientSecret: '', region: '', authMethod: 'builder-id' })
  const [openaiForm, setOpenaiForm] = useState({ name: '', apiKey: '', supportedModels: '' })
  const [customForm, setCustomForm] = useState({ name: '', apiKey: '', baseUrl: '', routePrefix: '', modelPrefix: '', supportedModels: '' })
  const [glmForm, setGlmForm] = useState({ name: '', apiKey: '', baseUrl: '', supportedModels: '' })
  const [minimaxForm, setMinimaxForm] = useState({ name: '', apiKey: '', baseUrl: '', supportedModels: '' })
  const [anthropicForm, setAnthropicForm] = useState({ name: '', apiKey: '', baseUrl: '', supportedModels: '' })
  const [geminiForm, setGeminiForm] = useState({ name: '', apiKey: '', baseUrl: '', supportedModels: '' })
  const [qwenMode, setQwenMode] = useState<'oauth' | 'apikey'>('oauth')
  const [qwenForm, setQwenForm] = useState({ name: '', apiKey: '', baseUrl: '', supportedModels: '' })
  const [qwenDeviceCode, setQwenDeviceCode] = useState<DeviceCodeState | null>(null)
  const [qwenPolling, setQwenPolling] = useState(false)
  const qwenPollRef = useRef<ReturnType<typeof setTimeout> | null>(null)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')
  const [deviceCode, setDeviceCode] = useState<DeviceCodeState | null>(null)
  const [polling, setPolling] = useState(false)
  const pollTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null)
  const [idcForm, setIdcForm] = useState({ startUrl: '', region: '' })
  const [socialLogin, setSocialLogin] = useState<SocialLoginState | null>(null)
  const [socialCallbackUrl, setSocialCallbackUrl] = useState('')
  const [socialProvider, setSocialProvider] = useState<'google' | 'github'>('google')
  const [openaiMode, setOpenaiMode] = useState<'oauth' | 'apikey'>('oauth')
  const [openaiOAuthSession, setOpenaiOAuthSession] = useState<{ sessionId: string; authUrl: string } | null>(null)
  const [openaiManualCallback, setOpenaiManualCallback] = useState('')
  const openaiPollRef = useRef<ReturnType<typeof setInterval> | null>(null)
  const [xaiOAuthSession, setXaiOAuthSession] = useState<{ sessionId: string; authUrl: string; state?: string; redirectUri?: string } | null>(null)
  const [xaiManualCallback, setXaiManualCallback] = useState('')

  useEffect(() => {
    return () => {
      if (pollTimerRef.current) clearTimeout(pollTimerRef.current)
      if (openaiPollRef.current) clearInterval(openaiPollRef.current)
      if (qwenPollRef.current) clearTimeout(qwenPollRef.current)
    }
  }, [])

  const resetForm = () => {
    setForm({ refreshToken: '', clientId: '', clientSecret: '', region: '', authMethod: 'builder-id' })
    setOpenaiForm({ name: '', apiKey: '', supportedModels: '' })
    setCustomForm({ name: '', apiKey: '', baseUrl: '', routePrefix: '', modelPrefix: '', supportedModels: '' })
    setGlmForm({ name: '', apiKey: '', baseUrl: '', supportedModels: '' })
    setMinimaxForm({ name: '', apiKey: '', baseUrl: '', supportedModels: '' })
    setAnthropicForm({ name: '', apiKey: '', baseUrl: '', supportedModels: '' })
    setGeminiForm({ name: '', apiKey: '', baseUrl: '', supportedModels: '' })
    setQwenForm({ name: '', apiKey: '', baseUrl: '', supportedModels: '' })
    setQwenDeviceCode(null); setQwenPolling(false)
    if (qwenPollRef.current) { clearTimeout(qwenPollRef.current); qwenPollRef.current = null }
    setIdcForm({ startUrl: '', region: '' })
    setDeviceCode(null); setPolling(false)
    setSocialLogin(null); setSocialCallbackUrl('')
    setOpenaiOAuthSession(null); setOpenaiManualCallback('')
    setXaiOAuthSession(null); setXaiManualCallback('')
    if (openaiPollRef.current) { clearInterval(openaiPollRef.current); openaiPollRef.current = null }
    setError('')
    if (pollTimerRef.current) { clearTimeout(pollTimerRef.current); pollTimerRef.current = null }
  }

  const parseSupportedModels = (str: string) => str.split('\n').map(s => s.trim()).filter(Boolean)

  const done = (msg: string) => { resetForm(); onSuccess(msg) }

  const handleStartBuilderID = async () => {
    setLoading(true); setError('')
    try {
      const res = await api.startBuilderID()
      setDeviceCode(res); setPolling(true); startPolling(res.sessionId, res.interval)
    } catch (e: any) { setError(e.message) } finally { setLoading(false) }
  }

  const handleStartIDC = async () => {
    if (!idcForm.startUrl) { setError('Start URL is required'); return }
    setLoading(true); setError('')
    try {
      const res = await api.startIDC({ startUrl: idcForm.startUrl, region: idcForm.region || undefined })
      setDeviceCode(res); setPolling(true); startPolling(res.sessionId, res.interval)
    } catch (e: any) { setError(e.message) } finally { setLoading(false) }
  }

  const startPolling = useCallback((sessionId: string, interval: number) => {
    const ms = Math.max(interval, 3) * 1000
    const poll = async () => {
      try {
        const res = await api.pollAuth(sessionId)
        if (res.status === 'pending') { pollTimerRef.current = setTimeout(poll, ms); return }
        if (res.status === 'success') { done(`Connected! ${res.email ? `(${res.email})` : res.name}`); return }
        setError(res.errorDescription || res.error || 'Authorization failed')
        setDeviceCode(null); setPolling(false)
      } catch (e: any) { setError(e.message); setDeviceCode(null); setPolling(false) }
    }
    pollTimerRef.current = setTimeout(poll, ms)
  }, [])

  const handleStartSocial = async () => {
    setLoading(true); setError('')
    try {
      const res = await api.startSocialLogin(socialProvider)
      setSocialLogin({ ...res, provider: socialProvider })
    } catch (e: any) { setError(e.message) } finally { setLoading(false) }
  }

  const handleExchangeSocial = async () => {
    if (!socialLogin || !socialCallbackUrl) return
    setLoading(true); setError('')
    try {
      await api.exchangeSocialCode({ sessionId: socialLogin.sessionId, callbackUrl: socialCallbackUrl })
      done('Social login connected!')
    } catch (e: any) { setError(e.message) } finally { setLoading(false) }
  }

  const handleDetect = async () => {
    setLoading(true); setError('')
    try {
      const res = await api.detectKiroToken()
      if (res.found) {
        await api.importConnection({ refreshToken: res.refreshToken, clientId: res.clientId || '', clientSecret: res.clientSecret || '', region: res.region || '', authMethod: res.authMethod || 'builder-id' })
        done('Connection imported!')
      } else setError(res.error || 'No Kiro token found.')
    } catch (e: any) { setError(e.message) } finally { setLoading(false) }
  }

  const handleFileUpload = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0]; if (!file) return
    setLoading(true); setError('')
    try {
      const data = JSON.parse(await file.text())
      if (!data.refreshToken) { setError('Invalid file: missing refreshToken'); return }
      await api.importConnection({ refreshToken: data.refreshToken, clientId: data.clientId || '', clientSecret: data.clientSecret || '', region: data.region || '', authMethod: data.authMethod?.toLowerCase() || 'builder-id' })
      done('Imported!')
    } catch (e: any) { setError(e.message) } finally { setLoading(false) }
  }

  const handleManualImport = async () => {
    setLoading(true); setError('')
    try { await api.importConnection(form); done('Imported!') } catch (e: any) { setError(e.message) } finally { setLoading(false) }
  }

  const handleAddOpenAI = async () => {
    setLoading(true); setError('')
    try {
      const models = parseSupportedModels(openaiForm.supportedModels)
      await api.addOpenAIConnection({ name: openaiForm.name || undefined, apiKey: openaiForm.apiKey, supportedModels: models.length > 0 ? models : undefined })
      done('OpenAI added!')
    } catch (e: any) { setError(e.message) } finally { setLoading(false) }
  }

  const handleAddCustom = async () => {
    setLoading(true); setError('')
    try {
      const models = parseSupportedModels(customForm.supportedModels)
      await api.addCustomConnection({
        name: customForm.name || undefined,
        apiKey: customForm.apiKey || undefined,
        baseUrl: customForm.baseUrl,
        routePrefix: customForm.routePrefix || undefined,
        modelPrefix: customForm.modelPrefix || undefined,
        supportedModels: models.length > 0 ? models : undefined,
      })
      done('Custom added!')
    } catch (e: any) { setError(e.message) } finally { setLoading(false) }
  }

  const handleAddGLM = async () => {
    setLoading(true); setError('')
    try {
      const models = parseSupportedModels(glmForm.supportedModels)
      await api.addGLMConnection({ name: glmForm.name || undefined, apiKey: glmForm.apiKey, baseUrl: glmForm.baseUrl || undefined, supportedModels: models.length > 0 ? models : undefined })
      done('GLM added!')
    } catch (e: any) { setError(e.message) } finally { setLoading(false) }
  }

  const handleAddMiniMax = async () => {
    setLoading(true); setError('')
    try {
      const models = parseSupportedModels(minimaxForm.supportedModels)
      await api.addMiniMaxConnection({ name: minimaxForm.name || undefined, apiKey: minimaxForm.apiKey, baseUrl: minimaxForm.baseUrl || undefined, supportedModels: models.length > 0 ? models : undefined })
      done('MiniMax added!')
    } catch (e: any) { setError(e.message) } finally { setLoading(false) }
  }

  const handleStartQwenOAuth = async () => {
    setLoading(true); setError('')
    try {
      const res = await api.startQwenOAuth()
      setQwenDeviceCode(res); setQwenPolling(true); startQwenPolling(res.sessionId, res.interval)
    } catch (e: any) { setError(e.message) } finally { setLoading(false) }
  }

  const startQwenPolling = useCallback((sessionId: string, interval: number) => {
    const ms = Math.max(interval, 3) * 1000
    const poll = async () => {
      try {
        const res = await api.pollQwenOAuth(sessionId)
        if (res.status === 'pending') { qwenPollRef.current = setTimeout(poll, ms); return }
        if (res.status === 'success') { done(`Qwen connected! ${res.email ? `(${res.email})` : res.name}`); return }
        setError(res.errorDescription || res.error || 'Authorization failed')
        setQwenDeviceCode(null); setQwenPolling(false)
      } catch (e: any) { setError(e.message); setQwenDeviceCode(null); setQwenPolling(false) }
    }
    qwenPollRef.current = setTimeout(poll, ms)
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  const handleAddQwenAPIKey = async () => {
    setLoading(true); setError('')
    try {
      const models = parseSupportedModels(qwenForm.supportedModels)
      await api.addQwenConnection({ name: qwenForm.name || undefined, apiKey: qwenForm.apiKey, baseUrl: qwenForm.baseUrl || undefined, supportedModels: models.length > 0 ? models : undefined })
      done('Qwen added!')
    } catch (e: any) { setError(e.message) } finally { setLoading(false) }
  }

  const handleAddAnthropic = async () => {
    setLoading(true); setError('')
    try {
      const models = parseSupportedModels(anthropicForm.supportedModels)
      await api.addAnthropicConnection({ name: anthropicForm.name || undefined, apiKey: anthropicForm.apiKey, baseUrl: anthropicForm.baseUrl || undefined, supportedModels: models.length > 0 ? models : undefined })
      done('Anthropic added!')
    } catch (e: any) { setError(e.message) } finally { setLoading(false) }
  }

  const handleAddGemini = async () => {
    setLoading(true); setError('')
    try {
      const models = parseSupportedModels(geminiForm.supportedModels)
      await api.addGeminiConnection({ name: geminiForm.name || undefined, apiKey: geminiForm.apiKey, baseUrl: geminiForm.baseUrl || undefined, supportedModels: models.length > 0 ? models : undefined })
      done('Gemini added!')
    } catch (e: any) { setError(e.message) } finally { setLoading(false) }
  }

  const handleStartXAIOAuth = async () => {
    setLoading(true); setError('')
    try {
      const res = await api.startXAIOAuth()
      setXaiOAuthSession({ sessionId: res.sessionId, authUrl: res.authUrl, state: res.state, redirectUri: res.redirectUri })
      window.open(res.authUrl, '_blank')
    } catch (e: any) { setError(e.message) } finally { setLoading(false) }
  }

  const handleExchangeXAIOAuth = async () => {
    if (!xaiOAuthSession) return
    const callback = xaiManualCallback.trim()
    if (!callback) { setError('Callback URL is required'); return }
    setLoading(true); setError('')
    try {
      await api.exchangeXAIOAuth(xaiOAuthSession.sessionId, callback)
      done('Grok Build connected!')
    } catch (e: any) { setError(e.message) } finally { setLoading(false) }
  }

  const DeviceCodePanel = () => deviceCode ? (
    <div className="space-y-3 mt-4 rounded-lg border bg-muted/40 p-4">
      <div className="text-center">
        <p className="text-xs text-muted-foreground mb-2">Enter this code on the authorization page:</p>
        <div className="text-2xl font-mono font-bold tracking-[0.2em] text-primary mb-3">{deviceCode.userCode}</div>
        <Button asChild size="sm" className="gap-2">
          <a href={deviceCode.verificationUriComplete || deviceCode.verificationUri} target="_blank" rel="noopener noreferrer">
            <ExternalLink size={14} /> Open Authorization Page
          </a>
        </Button>
      </div>
      {polling && (
        <div className="flex items-center justify-center gap-2 text-xs text-muted-foreground mt-2">
          <Loader2 size={12} className="animate-spin" /> Waiting for authorization ({deviceCode.interval}s)…
        </div>
      )}
    </div>
  ) : null

  const providerTabs = [
    { id: 'kiro', name: 'AWS / Kiro', icon: <ProviderLogoIcon provider="kiro" size={14} /> },
    { id: 'openai', name: 'OpenAI', icon: <ProviderLogoIcon provider="openai" size={14} /> },
    { id: 'qwen', name: 'Qwen', icon: <ProviderLogoIcon provider="qwen" size={14} /> },
    { id: 'xai', name: 'Grok', icon: <ProviderLogoIcon provider="xai" size={14} /> },
    { id: 'glm', name: 'GLM', icon: <ProviderLogoIcon provider="glm" size={14} /> },
    { id: 'minimax', name: 'MiniMax', icon: <ProviderLogoIcon provider="minimax" size={14} /> },
    { id: 'anthropic', name: 'Anthropic', icon: <ProviderLogoIcon provider="anthropic" size={14} /> },
    { id: 'gemini', name: 'Gemini', icon: <ProviderLogoIcon provider="gemini" size={14} /> },
    { id: 'openai-compatible', name: 'Custom', icon: <ProviderLogoIcon provider="openai-compatible" size={14} /> },
  ]

  const kiroModes = [
    { id: 'detect' as ImportMode, label: 'Auto Detect', icon: <Search size={13} /> },
    { id: 'builder-id' as ImportMode, label: 'Builder ID', icon: <ExternalLink size={13} /> },
    { id: 'social' as ImportMode, label: 'Social Login', icon: <Globe size={13} /> },
    { id: 'idc' as ImportMode, label: 'IAM IDC', icon: <Shield size={13} /> },
    { id: 'file' as ImportMode, label: 'Import File', icon: <Upload size={13} /> },
    { id: 'manual' as ImportMode, label: 'Manual', icon: <Play size={13} /> },
  ]

  return (
    <Dialog open onOpenChange={open => { if (!open) onClose() }}>
      <DialogContent className="max-w-2xl max-h-[90vh] flex flex-col overflow-hidden">
        <DialogHeader>
          <div className="flex items-center gap-2">
            <div className="flex h-8 w-8 items-center justify-center rounded-md bg-primary/10">
              <Link2 className="h-4 w-4 text-primary" />
            </div>
            <div>
              <DialogTitle>Add Connection</DialogTitle>
              <DialogDescription>Configure your AI provider account.</DialogDescription>
            </div>
          </div>
        </DialogHeader>

        <div className="flex-1 overflow-y-auto space-y-4 py-2">
          {/* Provider tabs */}
          <div className="flex gap-1 rounded-lg border bg-muted/40 p-1 w-fit mx-auto">
            {providerTabs.map(p => (
              <button
                key={p.id}
                onClick={() => { setProvider(p.id); resetForm() }}
                className={cn(
                  'flex items-center gap-1.5 px-3 py-1.5 rounded-md text-xs font-medium transition-colors cursor-pointer',
                  provider === p.id
                    ? 'bg-background text-foreground shadow-sm border'
                    : 'text-muted-foreground hover:text-foreground'
                )}
              >
                {p.icon} {p.name}
              </button>
            ))}
          </div>

          <div className="rounded-lg border bg-muted/20 p-4 min-h-[280px]">
            {/* Kiro */}
            {provider === 'kiro' && (
              <div className="space-y-4">
                <div className="flex flex-wrap gap-1.5 justify-center">
                  {kiroModes.map(m => (
                    <button
                      key={m.id}
                      onClick={() => { setImportMode(m.id); setDeviceCode(null); setPolling(false); setSocialLogin(null); setError('') }}
                      className={cn(
                        'flex items-center gap-1.5 px-3 py-1.5 rounded-full text-xs font-medium transition-colors cursor-pointer border',
                        importMode === m.id
                          ? 'bg-primary/10 text-primary border-primary/30'
                          : 'bg-transparent text-muted-foreground border-transparent hover:bg-muted'
                      )}
                    >
                      {m.icon} {m.label}
                    </button>
                  ))}
                </div>

                <div className="border-t pt-4">
                  {importMode === 'detect' && (
                    <div className="text-center space-y-3 max-w-sm mx-auto">
                      <p className="text-xs text-muted-foreground">Auto-detect credentials from <code className="bg-muted px-1 rounded">kiro-auth-token.json</code></p>
                      <Button onClick={handleDetect} disabled={loading} size="sm" className="gap-2">
                        {loading ? <Loader2 size={13} className="animate-spin" /> : <Search size={13} />}
                        {loading ? 'Detecting…' : 'Scan & Import'}
                      </Button>
                    </div>
                  )}

                  {importMode === 'builder-id' && (
                    <div className="text-center space-y-3 max-w-sm mx-auto">
                      <p className="text-xs text-muted-foreground">Authenticate via AWS Builder ID (Device Code Flow).</p>
                      {!deviceCode && (
                        <Button onClick={handleStartBuilderID} disabled={loading} size="sm" className="gap-2">
                          {loading ? <Loader2 size={13} className="animate-spin" /> : <ExternalLink size={13} />}
                          Start Login
                        </Button>
                      )}
                      <DeviceCodePanel />
                    </div>
                  )}

                  {importMode === 'idc' && (
                    <div className="space-y-3 max-w-sm mx-auto">
                      <p className="text-xs text-muted-foreground text-center">AWS IAM Identity Center (Enterprise SSO).</p>
                      {!deviceCode && (
                        <>
                          <Input value={idcForm.startUrl} onChange={e => setIdcForm({ ...idcForm, startUrl: e.target.value })} placeholder="Start URL (https://mycompany.awsapps.com/start)" className="text-xs" />
                          <Input value={idcForm.region} onChange={e => setIdcForm({ ...idcForm, region: e.target.value })} placeholder="Region (e.g. us-east-1)" className="text-xs" />
                          <Button onClick={handleStartIDC} disabled={loading || !idcForm.startUrl} size="sm" className="w-full gap-2">
                            {loading ? <Loader2 size={13} className="animate-spin" /> : <ExternalLink size={13} />}
                            Start Login
                          </Button>
                        </>
                      )}
                      <DeviceCodePanel />
                    </div>
                  )}

                  {importMode === 'social' && (
                    <div className="space-y-3 max-w-sm mx-auto text-center">
                      <p className="text-xs text-muted-foreground">Authenticate with Google or GitHub via Kiro Identity.</p>
                      {!socialLogin && (
                        <>
                          <div className="flex gap-2 justify-center">
                            {(['google', 'github'] as const).map(p => (
                              <button
                                key={p}
                                onClick={() => setSocialProvider(p)}
                                className={cn(
                                  'flex items-center gap-1.5 px-4 py-2 rounded-lg text-xs font-medium cursor-pointer transition-colors border',
                                  socialProvider === p ? 'bg-primary/10 text-primary border-primary/30' : 'bg-transparent text-muted-foreground border-border hover:bg-muted'
                                )}
                              >
                                {p === 'google' ? <Globe size={13} /> : <GitBranch size={13} />}
                                {p === 'google' ? 'Google' : 'GitHub'}
                              </button>
                            ))}
                          </div>
                          <Button onClick={handleStartSocial} disabled={loading} size="sm" className="gap-2">
                            {loading ? <Loader2 size={13} className="animate-spin" /> : <Globe size={13} />}
                            Start Login
                          </Button>
                        </>
                      )}
                      {socialLogin && (
                        <div className="space-y-3 text-left">
                          <div className="rounded-lg border bg-muted/40 p-3 text-xs text-muted-foreground space-y-1">
                            <p>1. Login page opened in browser.</p>
                            <p>2. After login, copy the <code className="bg-muted px-1 rounded">kiro://</code> URL.</p>
                            <p>3. Paste it below to complete authorization.</p>
                          </div>
                          <Input value={socialCallbackUrl} onChange={e => setSocialCallbackUrl(e.target.value)} placeholder="kiro://kiro.kiroAgent/authenticate-success?..." className="text-xs font-mono" />
                          <div className="flex gap-2">
                            <Button onClick={handleExchangeSocial} disabled={loading || !socialCallbackUrl} size="sm" className="flex-1">
                              {loading ? 'Processing…' : 'Submit'}
                            </Button>
                            <Button asChild variant="outline" size="sm">
                              <a href={socialLogin.loginUrl} target="_blank" rel="noopener noreferrer"><ExternalLink size={13} /></a>
                            </Button>
                          </div>
                        </div>
                      )}
                    </div>
                  )}

                  {importMode === 'file' && (
                    <div className="text-center space-y-3 max-w-sm mx-auto">
                      <p className="text-xs text-muted-foreground">Upload <code className="bg-muted px-1 rounded">kiro-auth-token.json</code></p>
                      <label className="flex flex-col items-center justify-center gap-2 p-6 rounded-lg border border-dashed cursor-pointer hover:border-primary hover:bg-muted/40 transition-colors">
                        <Upload size={20} className="text-muted-foreground" />
                        <span className="text-xs font-medium">{loading ? 'Processing…' : 'Select JSON file'}</span>
                        <input type="file" accept=".json" onChange={handleFileUpload} className="hidden" disabled={loading} />
                      </label>
                    </div>
                  )}

                  {importMode === 'manual' && (
                    <div className="space-y-3 max-w-md mx-auto">
                      <Textarea value={form.refreshToken} onChange={e => setForm({ ...form, refreshToken: e.target.value })} placeholder="Refresh Token *" rows={3} className="text-xs font-mono" />
                      <div className="grid grid-cols-2 gap-3">
                        <select value={form.authMethod} onChange={e => setForm({ ...form, authMethod: e.target.value })} className="flex h-9 w-full rounded-md border border-input bg-transparent px-3 py-1 text-xs shadow-sm">
                          <option value="builder-id">AWS Builder ID</option>
                          <option value="idc">IAM Identity Center</option>
                          <option value="social">Social Login</option>
                        </select>
                        <Input value={form.region} onChange={e => setForm({ ...form, region: e.target.value })} placeholder="Region (optional)" className="text-xs" />
                      </div>
                      <Button onClick={handleManualImport} disabled={loading || !form.refreshToken} size="sm" className="w-full">
                        {loading ? 'Validating…' : 'Import Configuration'}
                      </Button>
                    </div>
                  )}
                </div>
              </div>
            )}

            {/* OpenAI */}
            {provider === 'openai' && (
              <div className="space-y-4 max-w-md mx-auto">
                <div className="flex gap-1.5 justify-center">
                  {(['oauth', 'apikey'] as const).map(mode => (
                    <button
                      key={mode}
                      onClick={() => { setOpenaiMode(mode); setOpenaiOAuthSession(null); setOpenaiManualCallback(''); setError('') }}
                      className={cn(
                        'flex items-center gap-1.5 px-4 py-1.5 rounded-full text-xs font-medium cursor-pointer transition-colors border',
                        openaiMode === mode ? 'bg-emerald-500/10 text-emerald-600 border-emerald-500/30' : 'bg-transparent text-muted-foreground border-transparent hover:bg-muted'
                      )}
                    >
                      {mode === 'oauth' ? <><Globe size={13} /> OAuth Flow</> : <><Shield size={13} /> API Key</>}
                    </button>
                  ))}
                </div>

                {openaiMode === 'oauth' && (
                  <div className="space-y-3 text-center">
                    {!openaiOAuthSession ? (
                      <>
                        <p className="text-xs text-muted-foreground">Securely connect your OpenAI account via OAuth PKCE flow.</p>
                        <Button onClick={async () => {
                          setLoading(true); setError('')
                          try {
                            const res = await api.startOpenAIOAuth()
                            setOpenaiOAuthSession({ sessionId: res.sessionId, authUrl: res.authUrl })
                            window.open(res.authUrl, '_blank')
                            openaiPollRef.current = setInterval(async () => {
                              try {
                                const poll = await api.pollOpenAIOAuth(res.sessionId)
                                if (poll.status === 'pending') return
                                if (openaiPollRef.current) clearInterval(openaiPollRef.current)
                                if (poll.status === 'success') { done(`Connected! ${poll.email || poll.name || ''}`) }
                                else { setError(poll.error || 'Authorization failed'); setOpenaiOAuthSession(null) }
                              } catch (e: any) {
                                if (openaiPollRef.current) clearInterval(openaiPollRef.current)
                                setError(e.message); setOpenaiOAuthSession(null)
                              }
                            }, 2000)
                          } catch (e: any) { setError(e.message) } finally { setLoading(false) }
                        }} disabled={loading} size="sm" className="gap-2 bg-emerald-600 hover:bg-emerald-700">
                          {loading ? <Loader2 size={13} className="animate-spin" /> : <ExternalLink size={13} />}
                          Start Login
                        </Button>
                      </>
                    ) : (
                      <div className="space-y-3 text-left">
                        <div className="rounded-lg border bg-muted/40 p-4 text-center">
                          <Loader2 size={20} className="animate-spin text-emerald-600 mx-auto mb-2" />
                          <p className="text-sm font-medium">Waiting for authorization…</p>
                          <a href={openaiOAuthSession.authUrl} target="_blank" rel="noopener noreferrer" className="text-xs text-emerald-600 hover:underline inline-flex items-center gap-1 mt-1">
                            Open browser manually <ExternalLink size={10} />
                          </a>
                        </div>
                        <div>
                          <p className="text-[10px] text-muted-foreground mb-1 flex items-center gap-1"><AlertTriangle size={10} /> Paste callback URL if not redirected automatically:</p>
                          <div className="flex gap-2">
                            <Input value={openaiManualCallback} onChange={e => setOpenaiManualCallback(e.target.value)} placeholder="http://localhost:1455/auth/callback?..." className="text-xs font-mono" />
                            <Button onClick={async () => {
                              if (!openaiManualCallback.trim()) return
                              setLoading(true); setError('')
                              try {
                                if (openaiPollRef.current) clearInterval(openaiPollRef.current)
                                const poll = await api.pollOpenAIOAuth(openaiOAuthSession.sessionId, openaiManualCallback.trim())
                                if (poll.status === 'success') { done(`Connected! ${poll.email || poll.name || ''}`) }
                                else { setError(poll.error || 'Authorization failed'); setOpenaiOAuthSession(null) }
                              } catch (e: any) { setError(e.message); setOpenaiOAuthSession(null) } finally { setLoading(false) }
                            }} disabled={loading || !openaiManualCallback.trim()} size="sm">
                              {loading ? <Loader2 size={12} className="animate-spin" /> : 'Submit'}
                            </Button>
                          </div>
                        </div>
                        <Button variant="outline" size="sm" className="w-full" onClick={() => {
                          if (openaiPollRef.current) clearInterval(openaiPollRef.current)
                          setOpenaiOAuthSession(null); setOpenaiManualCallback(''); setError('')
                        }}>Cancel</Button>
                      </div>
                    )}
                  </div>
                )}

                {openaiMode === 'apikey' && (
                  <div className="space-y-3">
                    <Input type="password" value={openaiForm.apiKey} onChange={e => setOpenaiForm({ ...openaiForm, apiKey: e.target.value })} placeholder="API Key (sk-proj-…) *" className="text-xs font-mono" />
                    <Input value={openaiForm.name} onChange={e => setOpenaiForm({ ...openaiForm, name: e.target.value })} placeholder="Display Name (optional)" className="text-xs" />
                    <Textarea value={openaiForm.supportedModels} onChange={e => setOpenaiForm({ ...openaiForm, supportedModels: e.target.value })} placeholder="Supported Models (one per line, optional)" rows={3} className="text-xs font-mono" />
                    <Button onClick={handleAddOpenAI} disabled={loading || !openaiForm.apiKey} size="sm" className="w-full bg-emerald-600 hover:bg-emerald-700">
                      {loading ? 'Adding…' : 'Add Connection'}
                    </Button>
                  </div>
                )}
              </div>
            )}

            {/* Custom API */}
            {provider === 'openai-compatible' && (
              <div className="space-y-3 max-w-md mx-auto">
                <Input value={customForm.baseUrl} onChange={e => setCustomForm({ ...customForm, baseUrl: e.target.value })} placeholder="Base URL (e.g. https://api.together.xyz/v1) *" className="text-xs font-mono" />
                <div className="grid grid-cols-2 gap-3">
                  <Input type="password" value={customForm.apiKey} onChange={e => setCustomForm({ ...customForm, apiKey: e.target.value })} placeholder="API Key (optional)" className="text-xs font-mono" />
                  <Input value={customForm.name} onChange={e => setCustomForm({ ...customForm, name: e.target.value })} placeholder="Display Name" className="text-xs" />
                </div>
                <div className="grid grid-cols-2 gap-3">
                  <Input value={customForm.routePrefix} onChange={e => setCustomForm({ ...customForm, routePrefix: e.target.value })} placeholder="Route prefix (e.g. windsurf)" className="text-xs font-mono" />
                  <Input value={customForm.modelPrefix} onChange={e => setCustomForm({ ...customForm, modelPrefix: e.target.value })} placeholder="Model prefix to strip" className="text-xs font-mono" />
                </div>
                <Textarea value={customForm.supportedModels} onChange={e => setCustomForm({ ...customForm, supportedModels: e.target.value })} placeholder="Supported Models (one per line, optional)" rows={3} className="text-xs font-mono" />
                <Button onClick={handleAddCustom} disabled={loading || !customForm.baseUrl} size="sm" className="w-full bg-purple-600 hover:bg-purple-700">
                  {loading ? 'Adding…' : 'Add Custom Connection'}
                </Button>
              </div>
            )}

            {/* Grok Build */}
            {provider === 'xai' && (
              <div className="space-y-4 max-w-md mx-auto">
                <div className="text-center space-y-1">
                  <p className="text-xs text-muted-foreground">Connect your Grok Build account with xAI OAuth. dntproxy stores the refresh token locally and routes models as <code className="bg-muted px-1 rounded">grok/&lt;model&gt;</code>.</p>
                </div>

                {!xaiOAuthSession ? (
                  <Button onClick={handleStartXAIOAuth} disabled={loading} size="sm" className="w-full gap-2 bg-slate-900 hover:bg-slate-800 text-white">
                    {loading ? <Loader2 size={13} className="animate-spin" /> : <ExternalLink size={13} />}
                    {loading ? 'Starting…' : 'Connect Grok Build'}
                  </Button>
                ) : (
                  <div className="space-y-3">
                    <div className="rounded-lg border bg-muted/40 p-4 text-sm space-y-2">
                      <p className="font-medium">Finish Grok authorization</p>
                      <ol className="list-decimal list-inside text-xs text-muted-foreground space-y-1">
                        <li>Complete login in the browser.</li>
                        <li>Copy the final callback URL from the browser.</li>
                        <li>Paste it below to save the connection.</li>
                      </ol>
                      {xaiOAuthSession.redirectUri && (
                        <p className="text-[10px] text-muted-foreground break-all">Expected redirect: <code className="bg-muted px-1 rounded">{xaiOAuthSession.redirectUri}</code></p>
                      )}
                    </div>
                    <Input value={xaiManualCallback} onChange={e => setXaiManualCallback(e.target.value)} placeholder="http://127.0.0.1:56121/callback?code=...&state=..." className="text-xs font-mono" />
                    <div className="flex gap-2">
                      <Button onClick={handleExchangeXAIOAuth} disabled={loading || !xaiManualCallback.trim()} size="sm" className="flex-1 bg-slate-900 hover:bg-slate-800 text-white">
                        {loading ? <><Loader2 size={13} className="animate-spin mr-2" />Saving…</> : 'Save Connection'}
                      </Button>
                      <Button asChild variant="outline" size="sm">
                        <a href={xaiOAuthSession.authUrl} target="_blank" rel="noopener noreferrer"><ExternalLink size={13} /></a>
                      </Button>
                      <Button variant="outline" size="sm" onClick={() => { setXaiOAuthSession(null); setXaiManualCallback(''); setError('') }}>Cancel</Button>
                    </div>
                  </div>
                )}
              </div>
            )}

            {/* GLM (Zhipu AI) */}
            {provider === 'glm' && (
              <div className="space-y-3 max-w-md mx-auto">
                <p className="text-xs text-muted-foreground text-center">Connect to <a href="https://open.bigmodel.cn" target="_blank" rel="noopener noreferrer" className="text-[#0066FF] hover:underline">Zhipu AI (bigmodel.cn)</a> — GLM models.</p>
                <Input type="password" value={glmForm.apiKey} onChange={e => setGlmForm({ ...glmForm, apiKey: e.target.value })} placeholder="API Key *" className="text-xs font-mono" />
                <div className="grid grid-cols-2 gap-3">
                  <Input value={glmForm.name} onChange={e => setGlmForm({ ...glmForm, name: e.target.value })} placeholder="Display Name (optional)" className="text-xs" />
                  <Input value={glmForm.baseUrl} onChange={e => setGlmForm({ ...glmForm, baseUrl: e.target.value })} placeholder="Base URL (default: bigmodel.cn)" className="text-xs font-mono" />
                </div>
                <Textarea value={glmForm.supportedModels} onChange={e => setGlmForm({ ...glmForm, supportedModels: e.target.value })} placeholder="Supported Models (one per line, auto-populated if empty)" rows={3} className="text-xs font-mono" />
                <Button onClick={handleAddGLM} disabled={loading || !glmForm.apiKey} size="sm" className="w-full bg-[#0066FF] hover:bg-[#0055DD]">
                  {loading ? 'Adding…' : 'Add GLM Connection'}
                </Button>
              </div>
            )}

            {/* MiniMax */}
            {provider === 'minimax' && (
              <div className="space-y-3 max-w-md mx-auto">
                <p className="text-xs text-muted-foreground text-center">Connect to <a href="https://platform.minimax.io" target="_blank" rel="noopener noreferrer" className="text-[#FF6B35] hover:underline">MiniMax Platform</a> — M2 series models.</p>
                <Input type="password" value={minimaxForm.apiKey} onChange={e => setMinimaxForm({ ...minimaxForm, apiKey: e.target.value })} placeholder="API Key *" className="text-xs font-mono" />
                <div className="grid grid-cols-2 gap-3">
                  <Input value={minimaxForm.name} onChange={e => setMinimaxForm({ ...minimaxForm, name: e.target.value })} placeholder="Display Name (optional)" className="text-xs" />
                  <Input value={minimaxForm.baseUrl} onChange={e => setMinimaxForm({ ...minimaxForm, baseUrl: e.target.value })} placeholder="Base URL (default: api.minimax.io)" className="text-xs font-mono" />
                </div>
                <Textarea value={minimaxForm.supportedModels} onChange={e => setMinimaxForm({ ...minimaxForm, supportedModels: e.target.value })} placeholder="Supported Models (one per line, auto-populated if empty)" rows={3} className="text-xs font-mono" />
                <Button onClick={handleAddMiniMax} disabled={loading || !minimaxForm.apiKey} size="sm" className="w-full bg-[#FF6B35] hover:bg-[#E85A25]">
                  {loading ? 'Adding…' : 'Add MiniMax Connection'}
                </Button>
              </div>
            )}

            {/* Anthropic */}
            {provider === 'anthropic' && (
              <div className="space-y-3 max-w-md mx-auto">
                <p className="text-xs text-muted-foreground text-center">Connect Claude models with an Anthropic API key from <a href="https://console.anthropic.com/settings/keys" target="_blank" rel="noopener noreferrer" className="text-amber-600 hover:underline">Anthropic Console</a>.</p>
                <Input type="password" value={anthropicForm.apiKey} onChange={e => setAnthropicForm({ ...anthropicForm, apiKey: e.target.value })} placeholder="API Key *" className="text-xs font-mono" />
                <div className="grid grid-cols-2 gap-3">
                  <Input value={anthropicForm.name} onChange={e => setAnthropicForm({ ...anthropicForm, name: e.target.value })} placeholder="Display Name (optional)" className="text-xs" />
                  <Input value={anthropicForm.baseUrl} onChange={e => setAnthropicForm({ ...anthropicForm, baseUrl: e.target.value })} placeholder="Base URL (optional)" className="text-xs font-mono" />
                </div>
                <Textarea value={anthropicForm.supportedModels} onChange={e => setAnthropicForm({ ...anthropicForm, supportedModels: e.target.value })} placeholder="Supported Models (one per line, auto-populated if empty)" rows={3} className="text-xs font-mono" />
                <Button onClick={handleAddAnthropic} disabled={loading || !anthropicForm.apiKey} size="sm" className="w-full bg-amber-600 hover:bg-amber-700">
                  {loading ? 'Adding…' : 'Add Anthropic Connection'}
                </Button>
              </div>
            )}

            {/* Gemini */}
            {provider === 'gemini' && (
              <div className="space-y-3 max-w-md mx-auto">
                <p className="text-xs text-muted-foreground text-center">Connect Google Gemini models with an API key from <a href="https://aistudio.google.com/app/apikey" target="_blank" rel="noopener noreferrer" className="text-blue-500 hover:underline">Google AI Studio</a>.</p>
                <Input type="password" value={geminiForm.apiKey} onChange={e => setGeminiForm({ ...geminiForm, apiKey: e.target.value })} placeholder="API Key *" className="text-xs font-mono" />
                <div className="grid grid-cols-2 gap-3">
                  <Input value={geminiForm.name} onChange={e => setGeminiForm({ ...geminiForm, name: e.target.value })} placeholder="Display Name (optional)" className="text-xs" />
                  <Input value={geminiForm.baseUrl} onChange={e => setGeminiForm({ ...geminiForm, baseUrl: e.target.value })} placeholder="Base URL (optional)" className="text-xs font-mono" />
                </div>
                <Textarea value={geminiForm.supportedModels} onChange={e => setGeminiForm({ ...geminiForm, supportedModels: e.target.value })} placeholder="Supported Models (one per line, auto-populated if empty)" rows={3} className="text-xs font-mono" />
                <Button onClick={handleAddGemini} disabled={loading || !geminiForm.apiKey} size="sm" className="w-full bg-blue-500 hover:bg-blue-600">
                  {loading ? 'Adding…' : 'Add Gemini Connection'}
                </Button>
              </div>
            )}

            {/* Qwen */}
            {provider === 'qwen' && (
              <div className="space-y-3 max-w-md mx-auto">
                <p className="text-xs text-muted-foreground text-center">Connect to <a href="https://qwen.ai" target="_blank" rel="noopener noreferrer" className="text-[#6366F1] hover:underline">Qwen AI</a> — Free coding models via OAuth or paid via API key.</p>

                {/* Mode selector */}
                <div className="flex gap-1 p-0.5 bg-muted rounded-lg">
                  {([['oauth', '🔓 OAuth (Free)'], ['apikey', '🔑 API Key']] as const).map(([mode, label]) => (
                    <button key={mode} onClick={() => setQwenMode(mode)} className={cn('flex-1 text-xs py-1.5 rounded-md transition-all font-medium', qwenMode === mode ? 'bg-background text-foreground shadow-sm' : 'text-muted-foreground hover:text-foreground')}>{label}</button>
                  ))}
                </div>

                {qwenMode === 'oauth' && (
                  <div className="space-y-3">
                    {!qwenDeviceCode ? (
                      <Button onClick={handleStartQwenOAuth} disabled={loading} size="sm" className="w-full bg-[#6366F1] hover:bg-[#5558E6]">
                        {loading ? <><Loader2 size={14} className="animate-spin mr-2" />Starting…</> : <><Play size={14} className="mr-2" />Start Qwen Login</>}
                      </Button>
                    ) : (
                      <div className="space-y-3 rounded-lg border bg-muted/40 p-4">
                        <div className="text-center">
                          <p className="text-xs text-muted-foreground mb-2">Enter this code on qwen.ai:</p>
                          <div className="text-2xl font-mono font-bold tracking-[0.2em] text-[#6366F1] mb-3">{qwenDeviceCode.userCode}</div>
                          <Button asChild size="sm" className="gap-2 bg-[#6366F1] hover:bg-[#5558E6]">
                            <a href={qwenDeviceCode.verificationUriComplete || qwenDeviceCode.verificationUri} target="_blank" rel="noopener noreferrer">
                              <ExternalLink size={14} /> Open Qwen Authorization
                            </a>
                          </Button>
                        </div>
                        {qwenPolling && (
                          <div className="flex items-center justify-center gap-2 text-xs text-muted-foreground mt-2">
                            <Loader2 size={12} className="animate-spin" /> Waiting for authorization ({qwenDeviceCode.interval}s)…
                          </div>
                        )}
                      </div>
                    )}
                    <p className="text-[10px] text-muted-foreground text-center">Free tier: ~1,000–2,000 requests/day. No credit card needed.</p>
                  </div>
                )}

                {qwenMode === 'apikey' && (
                  <div className="space-y-3">
                    <p className="text-[10px] text-muted-foreground">Get API key from <a href="https://dashscope.aliyun.com" target="_blank" rel="noopener noreferrer" className="text-[#6366F1] hover:underline">DashScope Console</a>.</p>
                    <Input type="password" value={qwenForm.apiKey} onChange={e => setQwenForm({ ...qwenForm, apiKey: e.target.value })} placeholder="API Key *" className="text-xs font-mono" />
                    <div className="grid grid-cols-2 gap-3">
                      <Input value={qwenForm.name} onChange={e => setQwenForm({ ...qwenForm, name: e.target.value })} placeholder="Display Name (optional)" className="text-xs" />
                      <Input value={qwenForm.baseUrl} onChange={e => setQwenForm({ ...qwenForm, baseUrl: e.target.value })} placeholder="Base URL (default: DashScope)" className="text-xs font-mono" />
                    </div>
                    <Textarea value={qwenForm.supportedModels} onChange={e => setQwenForm({ ...qwenForm, supportedModels: e.target.value })} placeholder="Supported Models (one per line, auto-populated if empty)" rows={3} className="text-xs font-mono" />
                    <Button onClick={handleAddQwenAPIKey} disabled={loading || !qwenForm.apiKey} size="sm" className="w-full bg-[#6366F1] hover:bg-[#5558E6]">
                      {loading ? 'Adding…' : 'Add Qwen Connection'}
                    </Button>
                  </div>
                )}
              </div>
            )}
          </div>

          {error && (
            <div className="flex items-center gap-2 rounded-lg border border-destructive/30 bg-destructive/10 px-3 py-2 text-xs text-destructive">
              <AlertTriangle size={13} className="shrink-0" />
              {error}
            </div>
          )}
        </div>
      </DialogContent>
    </Dialog>
  )
}
