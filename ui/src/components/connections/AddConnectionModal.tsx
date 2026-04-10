import { useState, useEffect, useRef, useCallback } from 'react'
import {
  X, Loader2, Search, Upload, Shield, ExternalLink,
  Globe, GitBranch, Link2, CheckCircle2, AlertTriangle, Play
} from 'lucide-react'
import { api } from '../../api'
import type { ImportMode, DeviceCodeState, SocialLoginState } from './helpers'
import { AwsLogo, OpenAILogo, CustomLogo } from './helpers'

interface AddConnectionModalProps {
  onSuccess: (message: string) => void
  onClose: () => void
}

export default function AddConnectionModal({ onSuccess, onClose }: AddConnectionModalProps) {
  const [provider, setProvider] = useState('kiro')
  const [importMode, setImportMode] = useState<ImportMode>('detect')
  const [form, setForm] = useState({ refreshToken: '', clientId: '', clientSecret: '', region: '', authMethod: 'builder-id' })
  const [openaiForm, setOpenaiForm] = useState({ name: '', apiKey: '', supportedModels: '' })
  const [customForm, setCustomForm] = useState({ name: '', apiKey: '', baseUrl: '', supportedModels: '' })
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')
  const [success, setSuccess] = useState('')
  const [deviceCode, setDeviceCode] = useState<DeviceCodeState | null>(null)
  const [polling, setPolling] = useState(false)
  const pollTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null)
  const [idcForm, setIdcForm] = useState({ startUrl: '', region: '' })
  const [socialLogin, setSocialLogin] = useState<SocialLoginState | null>(null)
  const [socialCallbackUrl, setSocialCallbackUrl] = useState('')
  const [socialProvider, setSocialProvider] = useState<'google' | 'github'>('google')
  const [openaiMode, setOpenaiMode] = useState<'oauth' | 'apikey'>('oauth')
  const [openaiOAuthSession, setOpenaiOAuthSession] = useState<{sessionId: string; authUrl: string} | null>(null)
  const [openaiManualCallback, setOpenaiManualCallback] = useState('')
  const openaiPollRef = useRef<ReturnType<typeof setInterval> | null>(null)

  useEffect(() => {
    return () => {
      if (pollTimerRef.current) clearTimeout(pollTimerRef.current)
      if (openaiPollRef.current) clearInterval(openaiPollRef.current)
    }
  }, [])

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => { if (e.key === 'Escape') onClose() }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [onClose])

  const resetForm = () => {
    setForm({ refreshToken: '', clientId: '', clientSecret: '', region: '', authMethod: 'builder-id' })
    setOpenaiForm({ name: '', apiKey: '', supportedModels: '' })
    setCustomForm({ name: '', apiKey: '', baseUrl: '', supportedModels: '' })
    setIdcForm({ startUrl: '', region: '' })
    setDeviceCode(null); setPolling(false)
    setSocialLogin(null); setSocialCallbackUrl('')
    setOpenaiOAuthSession(null); setOpenaiManualCallback('')
    if (openaiPollRef.current) { clearInterval(openaiPollRef.current); openaiPollRef.current = null }
    setError(''); setSuccess('')
    if (pollTimerRef.current) { clearTimeout(pollTimerRef.current); pollTimerRef.current = null }
  }

  const parseSupportedModels = (str: string) => str.split('\n').map(s => s.trim()).filter(Boolean)

  const done = (msg: string) => {
    resetForm()
    onSuccess(msg)
  }

  // ── Functions (AWS/IDC/Social/Detect/Upload/Manual/OpenAI/Custom)
  const handleStartBuilderID = async () => {
    setLoading(true); setError(''); setSuccess('')
    try {
      const res = await api.startBuilderID()
      setDeviceCode(res); setPolling(true); startPolling(res.sessionId, res.interval)
    } catch (e: any) { setError(e.message) }
    finally { setLoading(false) }
  }

  const handleStartIDC = async () => {
    if (!idcForm.startUrl) { setError('Start URL is required'); return }
    setLoading(true); setError(''); setSuccess('')
    try {
      const res = await api.startIDC({ startUrl: idcForm.startUrl, region: idcForm.region || undefined })
      setDeviceCode(res); setPolling(true); startPolling(res.sessionId, res.interval)
    } catch (e: any) { setError(e.message) }
    finally { setLoading(false) }
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
    setLoading(true); setError(''); setSuccess('')
    try {
      const res = await api.startSocialLogin(socialProvider)
      setSocialLogin({ ...res, provider: socialProvider })
      window.open(res.loginUrl, '_blank')
    } catch (e: any) { setError(e.message) }
    finally { setLoading(false) }
  }

  const handleExchangeSocial = async () => {
    if (!socialLogin || !socialCallbackUrl) return
    setLoading(true); setError('')
    try {
      await api.exchangeSocialCode({ sessionId: socialLogin.sessionId, callbackUrl: socialCallbackUrl })
      done('Social login connected!')
    } catch (e: any) { setError(e.message) }
    finally { setLoading(false) }
  }

  const handleDetect = async () => {
    setLoading(true); setError(''); setSuccess('')
    try {
      const res = await api.detectKiroToken()
      if (res.found) {
        await api.importConnection({ refreshToken: res.refreshToken, clientId: res.clientId || '', clientSecret: res.clientSecret || '', region: res.region || '', authMethod: res.authMethod || 'builder-id' })
        done('Connection imported!')
      } else setError(res.error || 'No Kiro token found.')
    } catch (e: any) { setError(e.message) }
    finally { setLoading(false) }
  }

  const handleFileUpload = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0]; if (!file) return
    setLoading(true); setError(''); setSuccess('')
    try {
      const data = JSON.parse(await file.text())
      if (!data.refreshToken) { setError('Invalid file: missing refreshToken'); return }
      await api.importConnection({ refreshToken: data.refreshToken, clientId: data.clientId || '', clientSecret: data.clientSecret || '', region: data.region || '', authMethod: data.authMethod?.toLowerCase() || 'builder-id' })
      done('Imported!')
    } catch (e: any) { setError(e.message) }
    finally { setLoading(false) }
  }

  const handleManualImport = async () => {
    setLoading(true); setError(''); setSuccess('')
    try { await api.importConnection(form); done('Imported!') } catch (e: any) { setError(e.message) } finally { setLoading(false) }
  }

  const handleAddOpenAI = async () => {
    setLoading(true); setError(''); setSuccess('')
    try {
      const models = parseSupportedModels(openaiForm.supportedModels)
      await api.addOpenAIConnection({ name: openaiForm.name || undefined, apiKey: openaiForm.apiKey, supportedModels: models.length > 0 ? models : undefined })
      done('OpenAI added!')
    } catch (e: any) { setError(e.message) } finally { setLoading(false) }
  }

  const handleAddCustom = async () => {
    setLoading(true); setError(''); setSuccess('')
    try {
      const models = parseSupportedModels(customForm.supportedModels)
      await api.addCustomConnection({ name: customForm.name || undefined, apiKey: customForm.apiKey || undefined, baseUrl: customForm.baseUrl, supportedModels: models.length > 0 ? models : undefined })
      done('Custom added!')
    } catch (e: any) { setError(e.message) } finally { setLoading(false) }
  }

  const DeviceCodePanel = () => deviceCode ? (
    <div className="space-y-3 mt-4 glass-sm p-4 animate-fade-in border-[var(--accent)]/30">
      <div className="text-center">
        <p className="text-xs text-[var(--text-muted)] mb-2">Enter this code on the authorization page:</p>
        <div className="text-2xl font-mono font-bold tracking-[0.2em] text-[var(--accent)] mb-3">{deviceCode.userCode}</div>
        <a href={deviceCode.verificationUriComplete || deviceCode.verificationUri} target="_blank" rel="noopener noreferrer"
          className="btn-primary flex items-center justify-center gap-2 mx-auto decoration-transparent">
          <ExternalLink size={14} /> Open Authorization Page
        </a>
      </div>
      {polling && (
        <div className="flex items-center justify-center gap-2 text-xs text-[var(--accent)] bg-[var(--accent-glow)] rounded-lg py-2 mt-2">
          <Loader2 size={12} className="animate-spin" /> Waiting for authorization ({deviceCode.interval}s)…
        </div>
      )}
    </div>
  ) : null

  return (
    <div className="modal-overlay" role="presentation" onMouseDown={e => { if (e.target === e.currentTarget) onClose() }}>
      <div role="dialog" aria-modal="true" aria-labelledby="add-conn-title" className="modal-content sm:max-w-2xl" onMouseDown={e => e.stopPropagation()}>
        
        {/* Header */}
        <div className="modal-header pb-4 pt-5 px-5">
          <div className="flex items-center gap-3 min-w-0">
            <div className="w-10 h-10 rounded-xl bg-[var(--accent-glow)] border border-[var(--accent)]/20 flex items-center justify-center shrink-0">
              <Link2 size={18} className="text-[var(--accent)]" />
            </div>
            <div>
              <h3 id="add-conn-title" className="modal-title text-base">Add Connection</h3>
              <p className="modal-subtitle text-xs">Configure your AI provider account.</p>
            </div>
          </div>
          <button onClick={onClose} className="btn-icon shrink-0" aria-label="Close"><X size={16} /></button>
        </div>

        <div className="modal-body p-5 space-y-6">
          {/* Provider Tabs */}
          <div className="flex bg-[var(--bg-card)] rounded-lg p-1 border border-[var(--border)] overflow-x-auto gap-1 hide-scrollbar w-fit mx-auto">
            {[
              { id: 'kiro', name: 'AWS / Kiro', icon: <AwsLogo size={14} />, color: '#FF9900' },
              { id: 'openai', name: 'OpenAI', icon: <OpenAILogo size={14} />, color: '#10a37f' },
              { id: 'openai-compatible', name: 'Custom API', icon: <CustomLogo size={14} />, color: '#a855f7' }
            ].map(p => (
              <button key={p.id} onClick={() => { setProvider(p.id); resetForm() }}
                 className={`flex items-center gap-2 px-4 py-2 text-xs font-medium rounded-md transition-colors whitespace-nowrap cursor-pointer ${provider === p.id ? 'bg-[var(--bg-surface)] text-[var(--text)] shadow-sm border border-[var(--border)]' : 'text-[var(--text-muted)] hover:text-[var(--text)] hover:bg-white/[0.02]'}`}>
                 {p.icon} {p.name}
              </button>
            ))}
          </div>

          <div className="animate-fade-in glass p-5 min-h-[300px]">
            {/* Kiro Config */}
            {provider === 'kiro' && (
              <div className="space-y-5">
                <div className="flex flex-wrap gap-2 justify-center">
                  {[
                    { id: 'detect' as ImportMode, label: 'Auto Detect', icon: <Search size={14} /> },
                    { id: 'builder-id' as ImportMode, label: 'Builder ID', icon: <ExternalLink size={14} /> },
                    { id: 'social' as ImportMode, label: 'Social Login', icon: <Globe size={14} /> },
                    { id: 'idc' as ImportMode, label: 'IAM IDC', icon: <Shield size={14} /> },
                    { id: 'file' as ImportMode, label: 'Import File', icon: <Upload size={14} /> },
                    { id: 'manual' as ImportMode, label: 'Manual', icon: <Play size={14} /> }
                  ].map(m => (
                    <button key={m.id} onClick={() => { setImportMode(m.id); setDeviceCode(null); setPolling(false); setSocialLogin(null); setError(''); setSuccess('') }}
                      className={`flex items-center gap-1.5 px-3 py-1.5 rounded-full text-xs transition-colors cursor-pointer border ${importMode === m.id ? 'bg-[var(--accent-glow)] text-[var(--accent)] border-[var(--accent)]/30 font-medium' : 'bg-transparent text-[var(--text-muted)] border-transparent hover:bg-white/[0.04]'}`}>
                      {m.icon} {m.label}
                    </button>
                  ))}
                </div>
                
                <div className="border-t border-[var(--border)] pt-5">
                  {importMode === 'detect' && (
                    <div className="text-center max-w-sm mx-auto space-y-4">
                      <p className="text-xs text-[var(--text-muted)] leading-relaxed">Automatically discover credentials from <code className="font-mono bg-white/[0.05] px-1 rounded">kiro-auth-token.json</code></p>
                      <button onClick={handleDetect} disabled={loading} className="btn-primary w-full max-w-[200px] mx-auto">
                        {loading ? <Loader2 size={14} className="animate-spin" /> : <Search size={14} />} 
                        {loading ? 'Detecting…' : 'Scan & Import'}
                      </button>
                    </div>
                  )}

                  {importMode === 'builder-id' && (
                    <div className="text-center max-w-sm mx-auto space-y-4">
                      <p className="text-xs text-[var(--text-muted)]">Authenticate via AWS Builder ID (Device Code Flow).</p>
                      {!deviceCode && (
                        <button onClick={handleStartBuilderID} disabled={loading} className="btn-primary w-full max-w-[200px] mx-auto">
                          {loading ? <Loader2 size={14} className="animate-spin" /> : <ExternalLink size={14} />} Start Login
                        </button>
                      )}
                      <DeviceCodePanel />
                    </div>
                  )}

                  {importMode === 'idc' && (
                    <div className="max-w-sm mx-auto space-y-4">
                      <p className="text-xs text-[var(--text-muted)] text-center">AWS IAM Identity Center (Enterprise SSO).</p>
                      {!deviceCode && (
                        <>
                          <div className="space-y-3">
                            <div>
                              <input value={idcForm.startUrl} onChange={e => setIdcForm({ ...idcForm, startUrl: e.target.value })} placeholder="Start URL (https://mycompany.awsapps.com/start)" className="glass-input w-full text-xs" />
                            </div>
                            <div>
                              <input value={idcForm.region} onChange={e => setIdcForm({ ...idcForm, region: e.target.value })} placeholder="Region (e.g. us-east-1)" className="glass-input w-full text-xs" />
                            </div>
                          </div>
                          <button onClick={handleStartIDC} disabled={loading || !idcForm.startUrl} className="btn-primary w-full">
                            {loading ? <Loader2 size={14} className="animate-spin" /> : <ExternalLink size={14} />} Start Login
                          </button>
                        </>
                      )}
                      <DeviceCodePanel />
                    </div>
                  )}

                  {importMode === 'social' && (
                    <div className="max-w-sm mx-auto space-y-4 text-center">
                      <p className="text-xs text-[var(--text-muted)]">Authenticate with Google or GitHub via Kiro Identity.</p>
                      {!socialLogin && (
                        <>
                          <div className="flex gap-2 justify-center">
                            {(['google', 'github'] as const).map(p => (
                              <button key={p} onClick={() => setSocialProvider(p)} className={`flex items-center gap-1.5 px-4 py-2 rounded-lg text-xs font-medium cursor-pointer transition-colors border ${socialProvider === p ? 'bg-[var(--accent-glow)] text-[var(--accent)] border-[var(--accent)]/30' : 'bg-transparent text-[var(--text-muted)] border-[var(--border)] hover:bg-white/[0.04]'}`}>
                                {p === 'google' ? <Globe size={14} /> : <GitBranch size={14} />} {p === 'google' ? 'Google' : 'GitHub'}
                              </button>
                            ))}
                          </div>
                          <button onClick={handleStartSocial} disabled={loading} className="btn-primary w-full max-w-[200px] mx-auto mt-2">
                            {loading ? <Loader2 size={14} className="animate-spin" /> : <Globe size={14} />} Start Login
                          </button>
                        </>
                      )}
                      {socialLogin && (
                        <div className="space-y-3 text-left">
                          <div className="glass-sm p-3 text-xs text-[var(--text-muted)] space-y-1.5 border-blue-500/20">
                            <p>1. The login page has been opened in your browser.</p>
                            <p>2. After logging in, copy the <code className="bg-white/[0.05] px-1 rounded">kiro://</code> URL.</p>
                            <p>3. Paste it below to complete authorization.</p>
                          </div>
                          <input value={socialCallbackUrl} onChange={e => setSocialCallbackUrl(e.target.value)} placeholder="kiro://kiro.kiroAgent/authenticate-success?..." className="glass-input w-full text-xs font-mono" />
                          <div className="flex gap-2">
                            <button onClick={handleExchangeSocial} disabled={loading || !socialCallbackUrl} className="btn-primary flex-1 text-xs">
                              {loading ? 'Processing…' : 'Submit'}
                            </button>
                            <a href={socialLogin.loginUrl} target="_blank" rel="noopener noreferrer" className="btn-ghost flex items-center justify-center text-xs px-3 decoration-transparent" title="Re-open browser">
                              <ExternalLink size={14} />
                            </a>
                          </div>
                        </div>
                      )}
                    </div>
                  )}

                  {importMode === 'file' && (
                    <div className="text-center max-w-sm mx-auto space-y-4">
                      <p className="text-xs text-[var(--text-muted)]">Upload the <code className="font-mono bg-white/[0.05] px-1 rounded">kiro-auth-token.json</code> config file.</p>
                      <label className="flex flex-col items-center justify-center gap-2 p-6 glass-sm border-dashed hover:border-[var(--accent)] hover:text-[var(--accent)] transition-colors cursor-pointer rounded-xl group select-none">
                        <Upload size={20} className="text-[var(--text-muted)] group-hover:text-[var(--accent)] transition-colors" />
                        <span className="text-xs font-medium">{loading ? 'Processing…' : 'Select JSON file'}</span>
                        <input type="file" accept=".json" onChange={handleFileUpload} className="hidden" disabled={loading} />
                      </label>
                    </div>
                  )}

                  {importMode === 'manual' && (
                    <div className="space-y-3 max-w-md mx-auto">
                      <textarea value={form.refreshToken} onChange={e => setForm({ ...form, refreshToken: e.target.value })} placeholder="Refresh Token *" rows={3} className="glass-input w-full text-xs font-mono" />
                      <div className="grid grid-cols-2 gap-3">
                        <select value={form.authMethod} onChange={e => setForm({ ...form, authMethod: e.target.value })} className="glass-select w-full text-xs">
                          <option value="builder-id">AWS Builder ID</option>
                          <option value="idc">IAM Identity Center</option>
                          <option value="social">Social Login</option>
                        </select>
                        <input value={form.region} onChange={e => setForm({ ...form, region: e.target.value })} placeholder="Region (optional)" className="glass-input w-full text-xs" />
                      </div>
                      <button onClick={handleManualImport} disabled={loading || !form.refreshToken} className="btn-primary w-full text-xs py-2">
                        {loading ? 'Validating…' : 'Import Configuration'}
                      </button>
                    </div>
                  )}
                </div>
              </div>
            )}

            {/* OpenAI Config */}
            {provider === 'openai' && (
              <div className="space-y-5 max-w-md mx-auto">
                <div className="flex gap-2 justify-center mb-5">
                  <button onClick={() => { setOpenaiMode('oauth'); setOpenaiOAuthSession(null); setOpenaiManualCallback(''); setError(''); setSuccess('') }}
                    className={`flex items-center gap-1.5 px-4 py-1.5 rounded-full text-xs font-medium cursor-pointer transition-colors border ${openaiMode === 'oauth' ? 'bg-[#10a37f]/10 text-[#10a37f] border-[#10a37f]/30' : 'bg-transparent text-[var(--text-muted)] border-transparent hover:bg-white/[0.04]'}`}>
                    <Globe size={14} /> OAuth Flow
                  </button>
                  <button onClick={() => { setOpenaiMode('apikey'); setOpenaiOAuthSession(null); setOpenaiManualCallback(''); setError(''); setSuccess('') }}
                    className={`flex items-center gap-1.5 px-4 py-1.5 rounded-full text-xs font-medium cursor-pointer transition-colors border ${openaiMode === 'apikey' ? 'bg-[#10a37f]/10 text-[#10a37f] border-[#10a37f]/30' : 'bg-transparent text-[var(--text-muted)] border-transparent hover:bg-white/[0.04]'}`}>
                    <Shield size={14} /> API Key
                  </button>
                </div>

                {openaiMode === 'oauth' && (
                  <div className="space-y-4 text-center">
                    {!openaiOAuthSession && (
                      <>
                        <p className="text-xs text-[var(--text-muted)] leading-relaxed">Securely connect your OpenAI account without sharing passwords. You will be redirected to official OpenAI login.</p>
                        <button onClick={async () => {
                          setLoading(true); setError(''); setSuccess('')
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
                        }} disabled={loading} className="btn-primary w-full max-w-[200px] mx-auto bg-[#10a37f] hover:bg-[#0d8a6b] border-transparent shadow-[#10a37f]/20">
                          {loading ? <Loader2 size={14} className="animate-spin" /> : <ExternalLink size={14} />} Start Login
                        </button>
                      </>
                    )}

                    {openaiOAuthSession && (
                      <div className="space-y-3 text-left">
                        <div className="glass-sm p-4 border-[#10a37f]/30 text-center animate-fade-in">
                          <Loader2 size={24} className="animate-spin text-[#10a37f] mx-auto mb-2" />
                          <h4 className="font-medium text-sm text-[var(--text)]">Waiting for authorization</h4>
                          <a href={openaiOAuthSession.authUrl} target="_blank" rel="noopener noreferrer" className="text-xs text-[#10a37f] hover:underline mt-1 inline-flex items-center gap-1 decoration-transparent">
                            Open browser manually <ExternalLink size={10} />
                          </a>
                        </div>
                        <div className="pt-2">
                          <p className="text-[10px] text-[var(--text-dim)] mb-1 flex items-center gap-1"><AlertTriangle size={10} /> If not redirected automatically, paste the callback URL below:</p>
                          <div className="flex gap-2">
                            <input value={openaiManualCallback} onChange={e => setOpenaiManualCallback(e.target.value)} placeholder="http://localhost:1455/auth/callback?..." className="glass-input flex-1 text-xs font-mono" />
                            <button onClick={async () => {
                              if (!openaiManualCallback.trim()) return
                              setLoading(true); setError('')
                              try {
                                if (openaiPollRef.current) clearInterval(openaiPollRef.current)
                                const poll = await api.pollOpenAIOAuth(openaiOAuthSession.sessionId, openaiManualCallback.trim())
                                if (poll.status === 'success') { done(`Connected! ${poll.email || poll.name || ''}`) }
                                else { setError(poll.error || 'Authorization failed'); setOpenaiOAuthSession(null) }
                              } catch (e: any) { setError(e.message); setOpenaiOAuthSession(null) }
                              finally { setLoading(false) }
                            }} disabled={loading || !openaiManualCallback.trim()} className="btn-primary text-xs bg-[#10a37f] hover:bg-[#0d8a6b] border-transparent">
                              {loading ? <Loader2 size={12} className="animate-spin" /> : 'Submit'}
                            </button>
                          </div>
                        </div>
                        <button onClick={() => {
                          if (openaiPollRef.current) clearInterval(openaiPollRef.current)
                          setOpenaiOAuthSession(null); setOpenaiManualCallback(''); setError('')
                        }} className="btn-ghost w-full text-xs mt-2">Cancel</button>
                      </div>
                    )}
                  </div>
                )}

                {openaiMode === 'apikey' && (
                  <div className="space-y-3">
                    <input type="password" value={openaiForm.apiKey} onChange={e => setOpenaiForm({ ...openaiForm, apiKey: e.target.value })} placeholder="API Key (sk-proj-…) *" className="glass-input w-full text-xs font-mono" />
                    <input value={openaiForm.name} onChange={e => setOpenaiForm({ ...openaiForm, name: e.target.value })} placeholder="Display Name (optional)" className="glass-input w-full text-xs" />
                    <textarea value={openaiForm.supportedModels} onChange={e => setOpenaiForm({ ...openaiForm, supportedModels: e.target.value })} placeholder="Supported Models (one per line, optional)" rows={3} className="glass-input w-full text-xs font-mono" />
                    <button onClick={handleAddOpenAI} disabled={loading || !openaiForm.apiKey} className="btn-primary w-full text-xs py-2 bg-[#10a37f] hover:bg-[#0d8a6b] border-transparent shadow-[#10a37f]/20">
                      {loading ? 'Adding…' : 'Add Connection'}
                    </button>
                  </div>
                )}
              </div>
            )}

            {/* Custom API Config */}
            {provider === 'openai-compatible' && (
              <div className="space-y-3 max-w-md mx-auto pt-2">
                <div>
                  <input value={customForm.baseUrl} onChange={e => setCustomForm({ ...customForm, baseUrl: e.target.value })} placeholder="Base URL (e.g. https://api.together.xyz/v1) *" className="glass-input w-full text-xs font-mono" />
                </div>
                <div className="grid grid-cols-2 gap-3">
                  <input type="password" value={customForm.apiKey} onChange={e => setCustomForm({ ...customForm, apiKey: e.target.value })} placeholder="API Key (sk-…)" className="glass-input w-full text-xs font-mono" />
                  <input value={customForm.name} onChange={e => setCustomForm({ ...customForm, name: e.target.value })} placeholder="Display Name" className="glass-input w-full text-xs" />
                </div>
                <textarea value={customForm.supportedModels} onChange={e => setCustomForm({ ...customForm, supportedModels: e.target.value })} placeholder="Supported Models (one per line, optional)" rows={3} className="glass-input w-full text-xs font-mono" />
                <button onClick={handleAddCustom} disabled={loading || !customForm.baseUrl} className="btn-primary w-full text-xs py-2 bg-[var(--purple)] hover:bg-[#9333ea] border-transparent shadow-[var(--purple)]/20 shadow-md">
                  {loading ? 'Adding…' : 'Add Custom Connection'}
                </button>
              </div>
            )}
          </div>

          {(error || success) && (
            <div className={`p-3 rounded-xl border text-xs flex items-center gap-2 animate-slide-up ${error ? 'bg-[var(--danger-glow)] text-[var(--danger)] border-[var(--danger)]/20' : 'bg-[var(--success-glow)] text-[var(--success)] border-[var(--success)]/20'}`}>
              {error ? <AlertTriangle size={14} className="shrink-0" /> : <CheckCircle2 size={14} className="shrink-0" />}
              <span className="leading-relaxed">{error || success}</span>
            </div>
          )}
        </div>
      </div>
    </div>
  )
}
