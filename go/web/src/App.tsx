import { BrowserRouter, Routes, Route } from 'react-router-dom'
import { Toaster } from 'sonner'
import { ThemeProvider } from './components/ThemeProvider'
import { Layout } from './components/Layout'
import Dashboard from './pages/Dashboard'
import IndexRun from './pages/IndexRun'
import Query from './pages/Query'
import Settings from './pages/Settings'
import ProjectDetail from './pages/ProjectDetail'

function App() {
  return (
    <ThemeProvider>
      <BrowserRouter>
        <Routes>
          <Route element={<Layout />}>
            <Route path="/" element={<Dashboard />} />
            <Route path="/index" element={<IndexRun />} />
            <Route path="/query" element={<Query />} />
            <Route path="/settings" element={<Settings />} />
            <Route path="/projects/:name" element={<ProjectDetail />} />
          </Route>
        </Routes>
      </BrowserRouter>
      <Toaster richColors position="bottom-right" />
    </ThemeProvider>
  )
}

export default App
