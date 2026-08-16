import { useEffect } from 'react'
import { useQuery } from '@tanstack/react-query'
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
  const { data: cfg } = useQuery({ queryKey: ['config'], queryFn: api.getConfig })

  useEffect(() => {
    // The site title drives the browser tab; the favicon follows the uploaded
    // custom icon (the query string cache-busts so the tab updates at once).
    document.title = cfg?.site.title || 'gourl'
    const link = document.querySelector<HTMLLinkElement>('link[rel="icon"]')
    if (link) link.href = `/favicon.svg?t=${cfg?.icon ? 'custom' : 'default'}`
  }, [cfg])

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
