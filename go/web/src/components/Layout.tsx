import { useState } from 'react'
import { NavLink, Outlet } from 'react-router-dom'
import { cn } from '@/lib/utils'

const navItems = [
  { to: '/', label: 'Dashboard', icon: '\u25EB' },
  { to: '/index', label: 'Index', icon: '\u27F3' },
  { to: '/query', label: 'Query', icon: '\u2315' },
  { to: '/settings', label: 'Settings', icon: '\u2699' },
]

export function Layout() {
  const [mobileOpen, setMobileOpen] = useState(false)

  return (
    <div className="flex h-screen bg-zinc-950 text-zinc-100">
      {/* Mobile header */}
      <div className="fixed top-0 left-0 right-0 z-30 flex items-center h-12 px-4 border-b border-zinc-800 bg-zinc-950 md:hidden">
        <button
          onClick={() => setMobileOpen(!mobileOpen)}
          className="p-1 mr-3 text-zinc-400 hover:text-zinc-100"
          aria-label="Toggle menu"
        >
          <svg width="20" height="20" viewBox="0 0 20 20" fill="currentColor">
            <rect y="3" width="20" height="2" rx="1" />
            <rect y="9" width="20" height="2" rx="1" />
            <rect y="15" width="20" height="2" rx="1" />
          </svg>
        </button>
        <span className="text-sm font-bold tracking-tight">Carto</span>
      </div>

      {/* Mobile overlay */}
      {mobileOpen && (
        <div
          className="fixed inset-0 z-40 bg-black/50 md:hidden"
          onClick={() => setMobileOpen(false)}
        />
      )}

      {/* Sidebar */}
      <aside
        className={cn(
          'fixed inset-y-0 left-0 z-50 w-56 border-r border-zinc-800 bg-zinc-950 flex flex-col transition-transform duration-200 md:static md:translate-x-0',
          mobileOpen ? 'translate-x-0' : '-translate-x-full'
        )}
      >
        <div className="p-4 border-b border-zinc-800">
          <h1 className="text-xl font-bold tracking-tight">Carto</h1>
          <p className="text-xs text-zinc-500">Codebase Intelligence</p>
        </div>
        <nav className="flex-1 p-2 space-y-1">
          {navItems.map((item) => (
            <NavLink
              key={item.to}
              to={item.to}
              end={item.to === '/'}
              onClick={() => setMobileOpen(false)}
              className={({ isActive }) =>
                cn(
                  'flex items-center gap-2 px-3 py-2 rounded-md text-sm transition-colors',
                  isActive
                    ? 'bg-zinc-800 text-zinc-100'
                    : 'text-zinc-400 hover:text-zinc-100 hover:bg-zinc-800/50'
                )
              }
            >
              <span className="text-base">{item.icon}</span>
              {item.label}
            </NavLink>
          ))}
        </nav>
      </aside>

      {/* Main content */}
      <main className="flex-1 overflow-y-auto p-4 pt-16 md:p-6 md:pt-6">
        <Outlet />
      </main>
    </div>
  )
}
