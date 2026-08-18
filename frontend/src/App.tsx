import { useEffect } from 'react'
import { useQuery } from '@tanstack/react-query'
import { Navigate, Route, Routes, useLocation, useNavigate } from 'react-router-dom'
import { api, getServerConfig, isApp } from './lib/api'
import Layout from './components/Layout'
import { ToastProvider } from './components/ui'
import ChangePassword from './pages/ChangePassword'
import Connect from './pages/Connect'
import Dashboard from './pages/Dashboard'
import Links from './pages/Links'
import Login from './pages/Login'
import Logs from './pages/Logs'
import Settings from './pages/Settings'
import Setup from './pages/Setup'

// Apply the persisted theme before first paint. 'system' (or no saved value
// at all) follows the OS; only an explicit 'light' forces light. The SPA
// keeps watching for OS changes while on the "system" mode.
const savedTheme = localStorage.getItem('gourl-theme')
const prefersDark = window.matchMedia('(prefers-color-scheme: dark)').matches
if (savedTheme === 'dark' || (savedTheme !== 'light' && prefersDark)) {
  document.documentElement.classList.add('dark')
}
// Capacitor app mode: gates the app-only CSS (hidden scrollbars, safe-area
// insets) that must never leak into the web console.
if (isApp()) {
  document.documentElement.classList.add('capacitor')
}

export default function App() {
  const location = useLocation()
  const navigate = useNavigate()
  // Token mode (Capacitor app): a stored server connection replaces the
  // session flow — login/setup pages never appear, an unconnected app lands
  // on the connect screen instead.
  const appMode = isApp()
  const server = getServerConfig()

  // Capacitor app only. Deep links (gourl://links …) navigate the SPA; the
  // Android back button closes the top dialog first (dispatching Escape, which
  // the Dialog component listens for on window), then exits the app.
  useEffect(() => {
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    const cap = (window as any).Capacitor
    if (!cap?.Plugins?.App) return
    const { App } = cap.Plugins
    const backHandler = App.addListener('backButton', () => {
      const dialog = document.querySelector<HTMLElement>('[role="dialog"]')
      if (dialog) {
        dialog.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }))
      } else {
        // Back sends the app to the launcher with its state intact; the
        // native bridge moves the task to the background (exitApp would kill
        // the process and lose the SPA state).
        // eslint-disable-next-line @typescript-eslint/no-explicit-any
        const bridge = (window as any).GourlBridge
        if (bridge?.moveToBackground) bridge.moveToBackground()
        else App.exitApp()
      }
    })
    const openHandler = App.addListener('appUrlOpen', (event: { url: string }) => {
      // gourl://links → /admin/links; unknown pages fall back to the dashboard.
      const page = new URL(event.url).host || 'dashboard'
      const targets: Record<string, string> = {
        dashboard: '/admin',
        links: '/admin/links',
        log: '/admin/logs',
        settings: '/admin/settings',
      }
      navigate(targets[page] ?? '/admin')
    })
    return () => {
      backHandler.remove()
      openHandler.remove()
    }
  }, [navigate])

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
          element={
            appMode ? (
              <Navigate to="/admin/connect" replace />
            ) : authStatus && !authStatus.configured ? (
              <Navigate to="/admin/setup" replace />
            ) : (
              <Login />
            )
          }
        />
        <Route
          path="/admin/setup"
          element={
            appMode ? (
              <Navigate to="/admin/connect" replace />
            ) : authStatus && authStatus.configured ? (
              <Navigate to="/admin/login" replace />
            ) : (
              <Setup />
            )
          }
        />
        <Route
          path="/admin/change-password"
          element={authStatus && !authStatus.configured ? <Navigate to="/admin/setup" replace /> : <ChangePassword />}
        />
        <Route path="/admin/connect" element={<Connect />} />
        <Route
          path="/admin"
          element={
            appMode && !server ? (
              <Navigate to="/admin/connect" replace />
            ) : authStatus && !authStatus.configured && !appMode ? (
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
