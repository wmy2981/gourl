import { useEffect } from 'react'
import { Navigate, Route, Routes } from 'react-router-dom'
import { api } from './lib/api'
import Layout from './components/Layout'
import { ToastProvider } from './components/ui'
import Dashboard from './pages/Dashboard'
import Links from './pages/Links'
import Login from './pages/Login'
import Settings from './pages/Settings'

// Apply the persisted theme before first paint.
const savedTheme = localStorage.getItem('gourl-theme')
const prefersDark = window.matchMedia('(prefers-color-scheme: dark)').matches
if (savedTheme === 'dark' || (!savedTheme && prefersDark)) {
  document.documentElement.classList.add('dark')
}

export default function App() {
  useEffect(() => {
    // The site title from the admin config drives the browser tab; fall
    // back to the brand name when the API is unreachable.
    document.title = 'gourl'
    api
      .getConfig()
      .then((cfg) => {
        if (cfg.site.title) document.title = cfg.site.title
      })
      .catch(() => {})
  }, [])

  return (
    <ToastProvider>
      <Routes>
        <Route path="/admin/login" element={<Login />} />
        <Route path="/admin" element={<Layout />}>
          <Route index element={<Dashboard />} />
          <Route path="links" element={<Links />} />
          <Route path="settings" element={<Settings />} />
        </Route>
        <Route path="*" element={<Navigate to="/admin" replace />} />
      </Routes>
    </ToastProvider>
  )
}
