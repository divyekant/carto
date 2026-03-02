import { BrowserRouter, Routes, Route } from 'react-router-dom'
import { Toaster } from 'sonner'
import { ThemeProvider } from './components/ThemeProvider'
import { Layout } from './components/Layout'
import { AuthGuard } from './components/AuthGuard'
import { ErrorBoundary } from './components/ErrorBoundary'
import Dashboard from './pages/Dashboard'
import IndexRun from './pages/IndexRun'
import Query from './pages/Query'
import Settings from './pages/Settings'
import ProjectDetail from './pages/ProjectDetail'

function App() {
  return (
    <ThemeProvider>
      {/* AuthGuard gates the entire app when CARTO_SERVER_TOKEN is set on the server.
          When auth is not configured, it renders children immediately with no overhead. */}
      <AuthGuard>
        <BrowserRouter>
          {/* Top-level ErrorBoundary prevents unhandled React errors from
              leaving users with a blank white screen. */}
          <ErrorBoundary>
            <Routes>
              <Route element={<Layout />}>
                <Route path="/" element={<Dashboard />} />
                <Route path="/index" element={<IndexRun />} />
                <Route path="/query" element={<Query />} />
                <Route path="/settings" element={<Settings />} />
                <Route path="/projects/:name" element={<ProjectDetail />} />
              </Route>
            </Routes>
          </ErrorBoundary>
        </BrowserRouter>
      </AuthGuard>
      <Toaster richColors position="bottom-right" />
    </ThemeProvider>
  )
}

export default App
