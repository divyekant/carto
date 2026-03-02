import { Component, type ReactNode } from 'react'
import { Button } from '@/components/ui/button'

interface Props {
  children: ReactNode
  /** Optional custom fallback UI. Receives the caught error. */
  fallback?: (error: Error, retry: () => void) => ReactNode
}

interface State {
  error: Error | null
}

/**
 * ErrorBoundary catches JavaScript errors in the React component tree below it
 * and renders a recovery UI instead of crashing the whole page.
 *
 * Wrap page-level components or the <Outlet /> in Layout to prevent unhandled
 * errors from leaving users with a blank screen.
 *
 * Usage:
 *   <ErrorBoundary>
 *     <SomeComponent />
 *   </ErrorBoundary>
 */
export class ErrorBoundary extends Component<Props, State> {
  state: State = { error: null }

  static getDerivedStateFromError(error: Error): State {
    return { error }
  }

  componentDidCatch(error: Error, info: React.ErrorInfo) {
    // Log error details for debugging. In production this could be sent to an
    // error-tracking service such as Sentry.
    console.error('[ErrorBoundary] Caught error:', error, info.componentStack)
  }

  handleRetry = () => {
    this.setState({ error: null })
  }

  render() {
    const { error } = this.state
    const { children, fallback } = this.props

    if (error) {
      if (fallback) {
        return fallback(error, this.handleRetry)
      }

      return (
        <div className="flex items-center justify-center h-full min-h-[200px] p-8">
          <div className="text-center space-y-3 max-w-md">
            <p className="text-sm font-semibold text-destructive">
              Something went wrong
            </p>
            <p className="text-xs text-muted-foreground font-mono break-all">
              {error.message}
            </p>
            <Button
              size="sm"
              variant="outline"
              onClick={this.handleRetry}
            >
              Retry
            </Button>
          </div>
        </div>
      )
    }

    return children
  }
}
