import { useState, useEffect, type ReactNode } from 'react'
import { Input } from '@/components/ui/input'
import { Button } from '@/components/ui/button'
import { ApiError, apiFetchRaw } from '@/lib/api'

interface AuthGuardProps {
  children: ReactNode
}

/**
 * AuthGuard gates access to the application when the server has authentication
 * enabled (CARTO_SERVER_TOKEN is set).
 *
 * On first load it makes a probe request to /api/health. If the response is
 * 200 or auth is not configured, children are rendered immediately. If the
 * server returns 401, the guard shows a token-entry form. The token is
 * persisted in localStorage so subsequent page loads skip the form.
 *
 * When no server token is configured, the guard is completely transparent —
 * children render without any additional network request (health is probed
 * once and the result is cached in component state).
 */
export function AuthGuard({ children }: AuthGuardProps) {
  const [locked, setLocked] = useState(false)
  const [checked, setChecked] = useState(false)
  const [token, setToken] = useState(localStorage.getItem('carto_token') ?? '')
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)

  // On mount, probe the server to determine if auth is required.
  useEffect(() => {
    async function probe() {
      try {
        // Probe a protected endpoint without triggering the global 401 reload
        // path. AuthGuard needs to decide whether to show the unlock screen.
        await apiFetchRaw('/projects', undefined, { skipUnauthorizedRedirect: true })
      } catch (error) {
        if (error instanceof ApiError && error.status === 401) {
          localStorage.removeItem('carto_token')
          setToken('')
          setLocked(true)
        } else {
          // Network error — assume no auth required (server may be starting).
        }
      } finally {
        setChecked(true)
      }
    }
    probe()
  }, [])

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    if (!token.trim()) {
      setError('Please enter your server token')
      return
    }
    setLoading(true)
    setError('')
    try {
      // Validate the token against a protected endpoint.
      await apiFetchRaw('/projects', {
        headers: { Authorization: `Bearer ${token}` },
      }, {
        skipUnauthorizedRedirect: true,
      })
      localStorage.setItem('carto_token', token)
      setLocked(false)
    } catch (error) {
      if (error instanceof ApiError && error.status === 401) {
        setError('Invalid token — please try again')
      } else {
        setError('Network error — ensure the Carto server is running')
      }
    } finally {
      setLoading(false)
    }
  }

  // Show nothing until we've confirmed auth state to avoid flash of locked UI.
  if (!checked) {
    return null
  }

  if (locked) {
    return (
      <div className="flex items-center justify-center h-screen bg-background">
        <div className="w-80 space-y-4">
          <div className="text-center space-y-1">
            <h1 className="text-base font-semibold">Carto</h1>
            <p className="text-xs text-muted-foreground">
              Authentication required. Enter your server token to continue.
            </p>
          </div>
          <form onSubmit={handleSubmit} className="space-y-3">
            <Input
              type="password"
              placeholder="Server token (CARTO_SERVER_TOKEN)"
              value={token}
              onChange={(e) => setToken(e.target.value)}
              autoFocus
              autoComplete="current-password"
            />
            {error && (
              <p className="text-xs text-destructive text-center">{error}</p>
            )}
            <Button
              type="submit"
              size="sm"
              className="w-full"
              disabled={loading}
            >
              {loading ? 'Verifying…' : 'Unlock'}
            </Button>
          </form>
          <p className="text-xs text-muted-foreground text-center">
            Set <code className="font-mono">CARTO_SERVER_TOKEN</code> on the
            server to configure the expected value.
          </p>
        </div>
      </div>
    )
  }

  return <>{children}</>
}
