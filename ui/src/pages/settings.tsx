import { useEffect, useState } from 'react'
import { api } from '../api'
import { Save, Check, Settings } from 'lucide-react'

export default function SettingsPage() {
  const [settings, setSettings] = useState<any>(null)
  const [saved, setSaved] = useState(false)

  useEffect(() => { api.getSettings().then(setSettings).catch(() => {}) }, [])

  const handleSave = async () => {
    await api.updateSettings(settings)
    setSaved(true)
    setTimeout(() => setSaved(false), 2000)
  }

  if (!settings) return (
    <div className="flex items-center justify-center h-40">
      <div className="skeleton w-48 h-4" />
    </div>
  )

  return (
    <div>
      <div className="page-header">
        <div>
          <h2 className="page-title">Settings</h2>
          <p className="page-subtitle">Configure proxy behavior, security, and routing strategy.</p>
        </div>
        <button onClick={handleSave} className="btn-primary">
          {saved ? <Check size={16} /> : <Save size={16} />}
          {saved ? 'Saved!' : 'Save'}
        </button>
      </div>

      <div className="space-y-4 max-w-lg">
        <div className="glass p-5 space-y-5">
          <div>
            <label className="block text-sm font-medium mb-1.5" style={{ fontFamily: 'var(--font-heading)' }}>Port</label>
            <input type="number" value={settings.port || 20128} onChange={e => setSettings({ ...settings, port: parseInt(e.target.value) || 20128 })} className="glass-input w-full" />
            <p className="text-xs text-[var(--text-dim)] mt-1.5">Requires restart to take effect.</p>
          </div>

          <div>
            <label className="block text-sm font-medium mb-1.5" style={{ fontFamily: 'var(--font-heading)' }}>Combo Strategy</label>
            <select value={settings.comboStrategy || 'fallback'} onChange={e => setSettings({ ...settings, comboStrategy: e.target.value })} className="glass-select w-full">
              <option value="fallback">Fallback (try in order)</option>
              <option value="round-robin">Round Robin (rotate)</option>
            </select>
          </div>

          <div className="flex items-center justify-between py-1">
            <div>
              <label className="block text-sm font-medium" style={{ fontFamily: 'var(--font-heading)' }}>Require API Key</label>
              <p className="text-xs text-[var(--text-dim)] mt-0.5">Enforce Bearer token on /v1/* routes.</p>
            </div>
            <button
              onClick={() => setSettings({ ...settings, requireApiKey: !settings.requireApiKey })}
              role="switch"
              aria-checked={Boolean(settings.requireApiKey)}
              aria-label="Toggle Require API Key"
              className={`w-11 h-6 rounded-full transition-colors relative cursor-pointer ${settings.requireApiKey ? 'bg-[var(--accent)]' : 'bg-[var(--border)]'}`}
            >
              <span className={`absolute top-0.5 w-5 h-5 rounded-full bg-white shadow transition-transform ${settings.requireApiKey ? 'left-5.5' : 'left-0.5'}`} />
            </button>
          </div>

          <div>
            <label className="block text-sm font-medium mb-1.5" style={{ fontFamily: 'var(--font-heading)' }}>Sticky Round Robin Limit</label>
            <input type="number" value={settings.stickyRoundRobinLimit || 3} onChange={e => setSettings({ ...settings, stickyRoundRobinLimit: parseInt(e.target.value) || 3 })} className="glass-input w-full" />
          </div>
        </div>

        <div className="glass p-5">
          <h3 className="text-sm font-medium mb-3 flex items-center gap-2" style={{ fontFamily: 'var(--font-heading)' }}>
            <Settings size={14} className="text-[var(--text-dim)]" /> Endpoint Info
          </h3>
          <div className="text-xs text-[var(--text-muted)] space-y-1.5 font-mono glass-sm p-3">
            <p>API: http://localhost:{settings.port || 20128}/v1/chat/completions</p>
            <p>Models: http://localhost:{settings.port || 20128}/v1/models</p>
          </div>
        </div>
      </div>
    </div>
  )
}
