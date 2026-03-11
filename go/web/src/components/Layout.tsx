import { useState, useEffect } from 'react'
import { NavLink, Outlet } from 'react-router-dom'
import { cn } from '@/lib/utils'
import { apiFetch } from '@/lib/api'
import { useTheme } from './ThemeProvider'

const navItems = [
  { to: '/', label: 'Dashboard', icon: '◫' },
  { to: '/index', label: 'Index', icon: '⟳' },
  { to: '/query', label: 'Query', icon: '⌕' },
  { to: '/settings', label: 'Settings', icon: '⚙' },
]

function ThemeToggle() {
  const { resolved, setTheme } = useTheme()
  return (
    <button
      onClick={() => setTheme(resolved === 'dark' ? 'light' : 'dark')}
      className="p-2 rounded-md text-muted-foreground hover:text-foreground hover:bg-accent transition-colors"
      aria-label="Toggle theme"
      title={resolved === 'dark' ? 'Switch to light mode' : 'Switch to dark mode'}
    >
      {resolved === 'dark' ? (
        <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
          <circle cx="12" cy="12" r="5" />
          <line x1="12" y1="1" x2="12" y2="3" />
          <line x1="12" y1="21" x2="12" y2="23" />
          <line x1="4.22" y1="4.22" x2="5.64" y2="5.64" />
          <line x1="18.36" y1="18.36" x2="19.78" y2="19.78" />
          <line x1="1" y1="12" x2="3" y2="12" />
          <line x1="21" y1="12" x2="23" y2="12" />
          <line x1="4.22" y1="19.78" x2="5.64" y2="18.36" />
          <line x1="18.36" y1="5.64" x2="19.78" y2="4.22" />
        </svg>
      ) : (
        <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
          <path d="M21 12.79A9 9 0 1 1 11.21 3 7 7 0 0 0 21 12.79z" />
        </svg>
      )}
    </button>
  )
}

function SidebarContent({ health, onNavClick }: { health: { memories_healthy: boolean } | null; onNavClick?: () => void }) {
  return (
    <>
      {/* Logo */}
      <div className="px-4 py-3 border-b border-border h-12 flex items-center">
        <span className="text-xl font-bold tracking-tight font-[var(--font-display)]"><span className="text-primary">C</span>arto</span>
      </div>

      {/* Navigation */}
      <nav className="flex-1 p-2 space-y-0.5">
        {navItems.map((item) => (
          <NavLink
            key={item.to}
            to={item.to}
            end={item.to === '/'}
            onClick={onNavClick}
            aria-label={item.label}
            className={({ isActive }) =>
              cn(
                'flex items-center gap-3 px-3 py-2 rounded-md text-base transition-colors',
                isActive
                  ? 'bg-primary/10 text-primary font-medium'
                  : 'text-muted-foreground hover:bg-muted hover:text-foreground'
              )
            }
          >
            <span className="shrink-0" aria-hidden="true">{item.icon}</span>
            <span>{item.label}</span>
          </NavLink>
        ))}
      </nav>

      {/* Server status */}
      <div className="px-4 py-3 border-t border-border/50">
        <p className="text-xs font-medium uppercase tracking-wider text-muted-foreground mb-1">Server</p>
        <div className="flex items-center gap-2 text-sm">
          <span
            className={cn(
              'inline-block w-2 h-2 rounded-full shrink-0',
              health === null
                ? 'bg-muted-foreground'
                : health.memories_healthy
                  ? 'bg-green-500'
                  : 'bg-red-500'
            )}
          />
          <span>
            {health === null
              ? 'Memories: ...'
              : health.memories_healthy
                ? 'Memories: OK'
                : 'Memories: Down'}
          </span>
        </div>
      </div>

      {/* About link */}
      <div className="px-2 pb-1">
        <NavLink
          to="/about"
          onClick={onNavClick}
          aria-label="About"
          className={({ isActive }) =>
            cn(
              'flex items-center gap-3 px-3 py-2 rounded-md text-sm transition-colors',
              isActive
                ? 'bg-primary/10 text-primary font-medium'
                : 'text-muted-foreground hover:bg-muted hover:text-foreground'
            )
          }
        >
          <span className="shrink-0" aria-hidden="true">ℹ</span>
          <span>About</span>
        </NavLink>
      </div>

      {/* Footer */}
      <div className="flex items-center justify-between px-4 py-3 border-t border-border/50">
        <span className="text-xs text-muted-foreground">v1.1.0</span>
        <ThemeToggle />
      </div>
    </>
  )
}

export function Layout() {
  const [mobileOpen, setMobileOpen] = useState(false)
  const [health, setHealth] = useState<{ memories_healthy: boolean } | null>(null)

  useEffect(() => {
    apiFetch<{ memories_healthy: boolean }>('/health').then(setHealth).catch(() => {})
  }, [])

  return (
    <div className="flex h-screen bg-background text-foreground">
      {/* Mobile header */}
      <div className="fixed top-0 left-0 right-0 z-30 flex items-center h-12 px-4 border-b border-border bg-background md:hidden">
        <button
          onClick={() => setMobileOpen(!mobileOpen)}
          className="p-1 mr-3 text-muted-foreground hover:text-foreground"
          aria-label="Toggle menu"
        >
          <svg width="20" height="20" viewBox="0 0 20 20" fill="currentColor">
            <rect y="3" width="20" height="2" rx="1" />
            <rect y="9" width="20" height="2" rx="1" />
            <rect y="15" width="20" height="2" rx="1" />
          </svg>
        </button>
        <span className="text-sm font-bold tracking-tight text-primary font-[var(--font-display)]">Carto</span>
        <div className="ml-auto">
          <ThemeToggle />
        </div>
      </div>

      {/* Mobile overlay */}
      {mobileOpen && (
        <div
          className="fixed inset-0 z-40 bg-black/50 md:hidden"
          onClick={() => setMobileOpen(false)}
        />
      )}

      {/* Mobile sidebar */}
      {mobileOpen && (
        <aside className="fixed inset-y-0 left-0 z-50 w-56 border-r border-border bg-sidebar flex flex-col md:hidden">
          <SidebarContent health={health} onNavClick={() => setMobileOpen(false)} />
        </aside>
      )}

      {/* Desktop sidebar — always expanded */}
      <aside className="fixed inset-y-0 left-0 z-50 w-56 border-r border-border bg-sidebar flex-col hidden md:flex">
        <SidebarContent health={health} />
      </aside>

      {/* Main content */}
      <main className="flex-1 overflow-y-auto p-4 pt-16 md:p-8 md:pt-8 md:ml-56">
        <Outlet />
      </main>
    </div>
  )
}
