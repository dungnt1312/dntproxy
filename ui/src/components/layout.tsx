import { Outlet, NavLink } from 'react-router-dom'
import {
  LayoutDashboard, Link2, Layers, Box, KeyRound,
  Settings, Terminal, Database, Play, Zap
} from 'lucide-react'

const nav = [
  { to: '/', icon: LayoutDashboard, label: 'Dashboard' },
  { to: '/connections', icon: Link2, label: 'Connections' },
  { to: '/combos', icon: Layers, label: 'Combos' },
  { to: '/models', icon: Box, label: 'Models' },
  { to: '/keys', icon: KeyRound, label: 'API Keys' },
  { to: '/playground', icon: Play, label: 'Playground' },
  { to: '/logs', icon: Terminal, label: 'Logs' },
  { to: '/backup', icon: Database, label: 'Backup' },
  { to: '/settings', icon: Settings, label: 'Settings' },
]

export default function Layout() {
  return (
    <div className="flex min-h-dvh flex-col md:flex-row relative">
      {/* Animated background mesh */}
      <div className="bg-mesh" />

      {/* Sidebar */}
      <aside className="w-full md:w-60 shrink-0 glass md:m-3 md:mr-0 md:rounded-2xl flex flex-col relative z-10 md:h-[calc(100dvh-24px)] md:sticky md:top-3">
        {/* Logo */}
        <div className="px-5 py-4 border-b border-[var(--border)]">
          <div className="flex items-center gap-2.5">
            <div className="w-8 h-8 rounded-xl bg-gradient-to-br from-[var(--accent)] to-[#7c3aed] flex items-center justify-center shadow-lg shadow-[var(--accent)]/20">
              <Zap size={16} className="text-white" />
            </div>
            <div>
              <h1 className="text-sm font-bold tracking-tight font-[var(--font-heading)]" style={{ fontFamily: 'var(--font-heading)' }}>dntproxy</h1>
              <p className="text-[10px] text-[var(--text-dim)] leading-none mt-0.5">AI Router</p>
            </div>
          </div>
        </div>

        {/* Navigation */}
        <nav className="flex-1 p-2.5 flex md:block gap-1 overflow-x-auto md:overflow-y-auto md:overflow-x-hidden">
          {nav.map(({ to, icon: Icon, label }) => (
            <NavLink
              key={to}
              to={to}
              end={to === '/'}
              className={({ isActive }) =>
                `shrink-0 flex items-center gap-2.5 px-3 py-2 rounded-xl text-[13px] font-medium transition-all duration-200 group relative cursor-pointer ${
                  isActive
                    ? 'text-white bg-white/[0.06]'
                    : 'text-[var(--text-muted)] hover:text-[var(--text)] hover:bg-white/[0.04]'
                }`
              }
            >
              {({ isActive }) => (
                <>
                  {isActive && (
                    <span className="absolute left-0 top-1/2 -translate-y-1/2 w-[3px] h-5 rounded-r-full bg-[var(--accent)] shadow-[0_0_8px_var(--accent)]" />
                  )}
                  <Icon size={16} className={isActive ? 'text-[var(--accent)]' : 'opacity-60 group-hover:opacity-100 transition-opacity'} />
                  <span>{label}</span>
                </>
              )}
            </NavLink>
          ))}
        </nav>

        {/* Version */}
        <div className="hidden md:flex items-center px-5 py-3 border-t border-[var(--border)]">
          <span className="chip chip-muted text-[10px]">v0.1.0</span>
        </div>
      </aside>

      {/* Main */}
      <main className="flex-1 overflow-auto p-4 md:p-6 relative z-10">
        <Outlet />
      </main>
    </div>
  )
}
