import { useEffect, useState } from 'react'
import { api } from '../api'
import { Save } from 'lucide-react'

export default function Settings() {
  const [settings, setSettings] = useState<any>(null)
  const [saved, setSaved] = useState(false)

  useEffect(() => { api.getSettings().then(setSettings).catch(() => {}) }, [])

  const handleSave = async () => {
    await api.updateSettings(settings)
    setSaved(true)
    setTimeout(() => setSaved(false), 2000)
  }

  if (!settings) return <p className="text-[var(--text-muted)]">Loading...</p>

  return (
    <div>
      <div className="flex items-center justify-between mb-6">
        <h2 className="text-2xl font-bold">Settings</h2>
        <button onClick={handleSave} className="flex items-center gap-1.5 px-3 py-1.5 bg-[var(--accent)] hover:bg-[var(--accent-hover)] rounded-lg text-sm transition-colors">
          <Save size={16} /> {saved ? 'Saved!' : 'Save'}
        </button>
      </div>

      <div className="space-y-4 max-w-lg">
        <div className="bg-[var(--bg-card)] rounded-xl p-4 border border-[var(--border)] space-y-4">
          <div>
            <label className="block text-sm font-medium mb-1">Port</label>
            <input type="number" value={settings.port || 20128} onChange={e => setSettings({ ...settings, port: parseInt(e.target.value) || 20128 })} className="w-full bg-[var(--bg)] border border-[var(--border)] rounded-lg px-3 py-2 text-sm" />
            <p className="text-xs text-[var(--text-muted)] mt-1">Requires restart to take effect.</p>
          </div>

          <div>
            <label className="block text-sm font-medium mb-1">Combo Strategy</label>
            <select value={settings.comboStrategy || 'fallback'} onChange={e => setSettings({ ...settings, comboStrategy: e.target.value })} className="w-full bg-[var(--bg)] border border-[var(--border)] rounded-lg px-3 py-2 text-sm">
              <option value="fallback">Fallback (try in order)</option>
              <option value="round-robin">Round Robin (rotate)</option>
            </select>
          </div>

          <div className="flex items-center justify-between">
            <div>
              <label className="block text-sm font-medium">Require API Key</label>
              <p className="text-xs text-[var(--text-muted)]">Enforce Bearer token on /v1/* routes.</p>
            </div>
            <button onClick={() => setSettings({ ...settings, requireApiKey: !settings.requireApiKey })} className={`w-11 h-6 rounded-full transition-colors relative ${settings.requireApiKey ? 'bg-[var(--accent)]' : 'bg-[var(--border)]'}`}>
              <span className={`absolute top-0.5 w-5 h-5 rounded-full bg-white transition-transform ${settings.requireApiKey ? 'left-5.5' : 'left-0.5'}`} />
            </button>
          </div>

          <div>
            <label className="block text-sm font-medium mb-1">Sticky Round Robin Limit</label>
            <input type="number" value={settings.stickyRoundRobinLimit || 3} onChange={e => setSettings({ ...settings, stickyRoundRobinLimit: parseInt(e.target.value) || 3 })} className="w-full bg-[var(--bg)] border border-[var(--border)] rounded-lg px-3 py-2 text-sm" />
          </div>
        </div>

        <div className="bg-[var(--bg-card)] rounded-xl p-4 border border-[var(--border)]">
          <h3 className="text-sm font-medium mb-2">Endpoint Info</h3>
          <div className="text-xs text-[var(--text-muted)] space-y-1 font-mono">
            <p>API: http://localhost:{settings.port || 20128}/v1/chat/completions</p>
            <p>Models: http://localhost:{settings.port || 20128}/v1/models</p>
          </div>
        </div>
      </div>
    </div>
  )
}
