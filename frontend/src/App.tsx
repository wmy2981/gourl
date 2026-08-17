import { useEffect } from 'react'
import { useQuery } from '@tanstack/react-query'
import { Navigate, Route, Routes, useLocation } from 'react-router-dom'
import { api } from './lib/api'
import Layout from './components/Layout'
import { ToastProvider } from './components/ui'
import Dashboard from './pages/Dashboard'
import Links from './pages/Links'
import Login from './pages/Login'
import Logs from './pages/Logs'
import Settings from './pages/Settings'
import Setup from './pages/Setup'

// Apply the persisted theme before first paint.
const savedTheme = localStorage.getItem('gourl-theme')
const prefersDark = window.matchMedia('(prefers-color-scheme: dark)').matches
if (savedTheme === 'dark' || (!savedTheme && prefersDark)) {
  document.documentElement.classList.add('dark')
}

export default function App() {
  const location = useLocation()
  const { data: cfg } = useQuery({ queryKey: ['config'], queryFn: api.getConfig })
  // Unconfigured servers route the SPA to the one-time setup page; once a
  // password exists the setup page is unreachable again.
  const { data: authStatus } = useQuery({ queryKey: ['authStatus'], queryFn: api.authStatus })
  // Public identity for the auth-page tab title: the config API refuses
  // requests while no admin password exists, so the health endpoint carries
  // the service name there.
  const { data: health } = useQuery({ queryKey: ['health'], queryFn: api.health })

  useEffect(() => {
    // The login/setup tab shows the service name, the console the site title;
    // the favicon follows the uploaded custom icon (the query string
    // cache-busts so the tab updates at once).
    const isAuthPage = location.pathname === '/admin/login' || location.pathname === '/admin/setup'
    document.title = isAuthPage ? health?.name || 'gourl' : cfg?.site.title || 'gourl'
    const link = document.querySelector<HTMLLinkElement>('link[rel="icon"]')
    if (link) link.href = `/favicon.svg?t=${cfg?.icon ? 'custom' : 'default'}`
  }, [cfg, health, location.pathname])

  return (
    <ToastProvider>
      <Routes>
        <Route
          path="/admin/login"
          element={authStatus && !authStatus.configured ? <Navigate to="/admin/setup" replace /> : <Login />}
        />
        <Route
          path="/admin/setup"
          element={authStatus && authStatus.configured ? <Navigate to="/admin/login" replace /> : <Setup />}
        />
        <Route
          path="/admin"
          element={
            authStatus && !authStatus.configured ? (
              <Navigate to="/admin/setup" replace />
            ) : (
              <Layout />
            )
          }
        >
          <Route index element={<Dashboard />} />
          <Route path="links" element={<Links />} />
          <Route path="logs" element={<Logs />} />
          <Route path="settings" element={<Settings />} />
        </Route>
        <Route path="*" element={<Navigate to="/admin" replace />} />
      </Routes>
    </ToastProvider>
  )
}
