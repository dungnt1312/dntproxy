import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { api } from '../api'
import { Link2, Layers, Box, KeyRound } from 'lucide-react'

export default function Dashboard() {
  const [stats, setStats] = useState({ connections: 0, combos: 0, models: 0, keys: 0 })
  const [conns, setConns] = useState<any[]>([])

  useEffect(() => {
    Promise.all([
      api.getConnections(),
      api.getCombos(),
      api.getModels(),
      api.getKeys(),
    ]).then(([c, co, m, k]) => {
      setStats({ connections: c?.length || 0, combos: co?.length || 0, models: m?.length || 0, keys: k?.length || 0 })
      setConns(c || [])
    })
  }, [])

  const cards = [
    { label: 'Connections', value: stats.connections, icon: Link2, color: 'text-blue-400', href: '/connections' },
    { label: 'Combos', value: stats.combos, icon: Layers, color: 'text-purple-400', href: '/combos' },
    { label: 'Models', value: stats.models, icon: Box, color: 'text-green-400', href: '/models' },
    { label: 'API Keys', value: stats.keys, icon: KeyRound, color: 'text-yellow-400', href: '/keys' },
  ]

  return (
    <div>
      <h2 className="text-2xl font-bold mb-6">Dashboard</h2>

      <div className="grid grid-cols-1 sm:grid-cols-2 xl:grid-cols-4 gap-4 mb-8">
        {cards.map(({ label, value, icon: Icon, color, href }) => (
          <Link key={label} to={href} className="bg-[var(--bg-card)] rounded-xl p-4 border border-[var(--border)] hover:border-[var(--accent)] transition-colors">
            <div className="flex items-center justify-between mb-2">
              <span className="text-sm text-[var(--text-muted)]">{label}</span>
              <Icon size={18} className={color} />
            </div>
            <div className="text-3xl font-bold">{value}</div>
          </Link>
        ))}
      </div>

      <h3 className="text-lg font-semibold mb-3">Connection Status</h3>
      {conns.length === 0 ? (
        <p className="text-[var(--text-muted)]">No connections. <Link to="/connections" className="text-[var(--accent)] underline">Add one</Link></p>
      ) : (
        <div className="space-y-2">
          {conns.map((c: any) => (
            <div key={c.id} className="bg-[var(--bg-card)] rounded-lg p-3 border border-[var(--border)] flex items-center justify-between">
              <div>
                <span className="font-medium">{c.name}</span>
                <span className="text-xs text-[var(--text-muted)] ml-2">{c.providerName || c.authMethod}</span>
              </div>
              <div className="flex items-center gap-2">
                {c.hasToken && <span className="w-2 h-2 rounded-full bg-[var(--success)]" />}
                <span className={`text-xs px-2 py-0.5 rounded ${c.isActive ? 'bg-green-900/40 text-green-400' : 'bg-red-900/40 text-red-400'}`}>
                  {c.rateLimitedUntil && new Date(c.rateLimitedUntil) > new Date() ? 'cooldown' : c.isActive ? 'active' : 'disabled'}
                </span>
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  )
}
