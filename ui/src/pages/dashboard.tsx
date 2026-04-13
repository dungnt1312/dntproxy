import { useEffect, useState } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import { api } from '../api'
import { Link2, Layers, Box, KeyRound, Plus, ChevronRight, Zap } from 'lucide-react'
import { getProviderInfo, getProviderLabel } from '../components/connections/helpers'

export default function Dashboard() {
  const navigate = useNavigate()
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
    { label: 'Connections', value: stats.connections, icon: Link2, colorClass: 'stat-card-blue', href: '/connections' },
    { label: 'Combos', value: stats.combos, icon: Layers, colorClass: 'stat-card-purple', href: '/combos' },
    { label: 'Models', value: stats.models, icon: Box, colorClass: 'stat-card-green', href: '/models' },
    { label: 'API Keys', value: stats.keys, icon: KeyRound, colorClass: 'stat-card-amber', href: '/keys' },
  ]

  return (
    <div>
      <div className="page-header">
        <div>
          <h2 className="page-title">Dashboard</h2>
          <p className="page-subtitle">Overview of your AI proxy infrastructure.</p>
        </div>
        <button onClick={() => navigate('/playground')} className="btn-primary">
          <Zap size={16} /> Playground
        </button>
      </div>

      <div className="grid grid-cols-1 sm:grid-cols-2 xl:grid-cols-4 gap-4 mb-8">
        {cards.map(({ label, value, icon: Icon, colorClass, href }) => (
          <Link key={label} to={href} className={`glass stat-card ${colorClass} cursor-pointer group`}>
            <div className="flex items-center justify-between mb-2">
              <span className="text-xs text-[var(--text-muted)] font-medium">{label}</span>
              <Icon size={16} className="opacity-40 group-hover:opacity-70 transition-opacity" />
            </div>
            <div className="text-3xl font-bold" style={{ fontFamily: 'var(--font-heading)' }}>{value}</div>
          </Link>
        ))}
      </div>

      <div className="flex items-center justify-between mb-3">
        <h3 className="text-sm font-semibold" style={{ fontFamily: 'var(--font-heading)' }}>Connection Status</h3>
        <button onClick={() => navigate('/connections')} className="text-xs text-[var(--accent)] hover:text-[var(--accent-hover)] font-medium flex items-center gap-1 cursor-pointer">
          View all <ChevronRight size={12} />
        </button>
      </div>

      {conns.length === 0 ? (
        <div className="empty-state py-10">
          <div className="empty-state-icon">
            <Link2 size={24} />
          </div>
          <h3>No connections</h3>
          <p>Add your first AI provider to start routing requests.</p>
          <button onClick={() => navigate('/connections')} className="btn-primary">
            <Plus size={16} /> Add Connection
          </button>
        </div>
      ) : (
        <div className="space-y-2">
          {conns.map((c: any) => {
            const info = getProviderInfo(c.provider)
            const isRL = c.rateLimitedUntil && new Date(c.rateLimitedUntil) > new Date()
            return (
              <div key={c.id} className="glass-sm px-4 py-3 flex items-center justify-between hover:border-[var(--border-hover)] transition-all cursor-pointer group" onClick={() => navigate('/connections')}>
                <div className="flex items-center gap-3 min-w-0">
                  <div className="shrink-0 rounded-lg overflow-hidden flex shadow-sm scale-90">
                    {info.icon}
                  </div>
                  <div className="min-w-0">
                    <span className="font-medium text-sm block truncate">{c.name}</span>
                    <span className="text-[10px] text-[var(--text-dim)]">{getProviderLabel(c.provider)}</span>
                  </div>
                </div>
                <div className="flex items-center gap-2">
                  {c.hasToken && <span className="w-2 h-2 rounded-full bg-[var(--success)]" />}
                  <span className={`chip text-[10px] ${
                    isRL ? 'chip-warning' : c.isActive ? 'chip-success' : 'chip-muted'
                  }`}>
                    {isRL ? 'cooldown' : c.isActive ? 'active' : 'disabled'}
                  </span>
                  <ChevronRight size={14} className="text-[var(--text-dim)] opacity-0 group-hover:opacity-100 transition-opacity" />
                </div>
              </div>
            )
          })}
        </div>
      )}
    </div>
  )
}
