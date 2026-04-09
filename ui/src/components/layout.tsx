import { Outlet, NavLink } from 'react-router-dom'
import { LayoutDashboard, Link2, Layers, Box, KeyRound, Settings, Terminal, Database } from 'lucide-react'

const nav = [
  { to: '/', icon: LayoutDashboard, label: 'Dashboard' },
  { to: '/connections', icon: Link2, label: 'Connections' },
  { to: '/combos', icon: Layers, label: 'Combos' },
  { to: '/models', icon: Box, label: 'Models' },
  { to: '/keys', icon: KeyRound, label: 'API Keys' },
  { to: '/logs', icon: Terminal, label: 'Logs' },
  { to: '/backup', icon: Database, label: 'Backup' },
  { to: '/settings', icon: Settings, label: 'Settings' },
]

export default function Layout() {
  return (
    <div className="flex h-screen">
      <aside className="w-56 shrink-0 border-r border-[var(--border)] bg-[var(--bg-card)] flex flex-col">
        <div className="p-4 border-b border-[var(--border)]">
          <h1 className="text-lg font-bold tracking-tight">dntproxy</h1>
          <p className="text-xs text-[var(--text-muted)]">Kiro AI Router</p>
        </div>
        <nav className="flex-1 p-2 space-y-0.5">
          {nav.map(({ to, icon: Icon, label }) => (
            <NavLink
              key={to}
              to={to}
              end={to === '/'}
              className={({ isActive }) =>
                `flex items-center gap-2.5 px-3 py-2 rounded-lg text-sm transition-colors ${
                  isActive
                    ? 'bg-[var(--accent)] text-white'
                    : 'text-[var(--text-muted)] hover:bg-[var(--bg-hover)] hover:text-[var(--text)]'
                }`
              }
            >
              <Icon size={16} />
              {label}
            </NavLink>
          ))}
        </nav>
        <div className="p-3 border-t border-[var(--border)] text-xs text-[var(--text-muted)]">
          v0.1.0
        </div>
      </aside>
      <main className="flex-1 overflow-auto p-6">
        <Outlet />
      </main>
    </div>
  )
}
