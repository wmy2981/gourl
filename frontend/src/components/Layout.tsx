import { useEffect, useRef, useState } from 'react'
import { NavLink, Outlet, useLocation, useNavigate } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { LayoutDashboard, Link2, LogOut, Menu, Moon, ScrollText, Settings, Sun, X } from 'lucide-react'
import { api } from '../lib/api'
import { setLanguage } from '../lib/i18n'
// Single source of truth for the brand icon (also embedded server-side and
// referenced by the READMEs).
import iconUrl from '../assets/icon.svg'

function AppIcon({ size = 28 }: { size?: number }) {
  // The uploaded custom icon (served from /assets/) replaces the built-in
  // brand icon wherever the app shows it; falls back to the default.
  const { data: cfg } = useQuery({ queryKey: ['config'], queryFn: api.getConfig })
  const src = cfg?.icon ? `/assets/${cfg.icon}` : iconUrl
  return <img src={src} width={size} height={size} alt="" aria-hidden />
}

// Theme persisted on <html class="dark">.
function useTheme() {
  const [dark, setDark] = useState(() => document.documentElement.classList.contains('dark'))
  const toggle = () => {
    const next = !dark
    setDark(next)
    document.documentElement.classList.toggle('dark', next)
    localStorage.setItem('gourl-theme', next ? 'dark' : 'light')
  }
  return { dark, toggle }
}

export default function Layout() {
  const { t, i18n } = useTranslation()
  const navigate = useNavigate()
  const location = useLocation()
  const { dark, toggle } = useTheme()
  const [mobileOpen, setMobileOpen] = useState(false)
  const [mobileClosing, setMobileClosing] = useState(false)
  // The configured service name drives the sidebar, top bar and footer text;
  // falls back to the brand name while the config is still loading.
  const { data: cfg } = useQuery({ queryKey: ['config'], queryFn: api.getConfig })
  const siteName = cfg?.site.name || __APP_NAME__

  // Edge-swipe gesture: dragging the drawer by touch. dragX is the drawer's
  // left-edge position in px (0 = fully open, DRAWER_W = fully closed) while a
  // finger is down; settleTo is the target position during the release
  // animation. Non-gesture open/close also runs through the same transform,
  // so the drawer always slides — no class-animation jump on release.
  const DRAWER_W = 256 // w-64
  const [dragX, setDragX] = useState<number | null>(null)
  const [settleTo, setSettleTo] = useState<number | null>(null)
  const dragStartRef = useRef<number | null>(null)
  const settleTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null)
  useEffect(() => () => {
    if (settleTimerRef.current) clearTimeout(settleTimerRef.current)
  }, [])

  const onDrawerTouchStart = (e: React.TouchEvent) => {
    const touch = e.touches[0]
    // Trigger zone: the right 24px of the viewport, phones only (the drawer
    // exists only below md).
    if (!touch || window.innerWidth >= 768 || touch.clientX < window.innerWidth - 24) return
    if (settleTimerRef.current) {
      clearTimeout(settleTimerRef.current)
      settleTimerRef.current = null
    }
    setSettleTo(null)
    dragStartRef.current = touch.clientX
    setDragX(mobileOpen ? 0 : DRAWER_W)
  }

  const onDrawerTouchMove = (e: React.TouchEvent) => {
    if (dragX === null || dragStartRef.current === null) return
    const start = mobileOpen ? 0 : DRAWER_W
    const dx = dragStartRef.current - e.touches[0].clientX // >0 = dragging left
    setDragX(Math.max(0, Math.min(DRAWER_W, start + dx)))
  }

  const finishDrawerGesture = () => {
    if (dragX === null) return
    dragStartRef.current = null
    const open = dragX < DRAWER_W / 2
    setSettleTo(open ? 0 : DRAWER_W)
    setDragX(null)
    if (open) {
      setMobileOpen(true)
      settleTimerRef.current = setTimeout(() => setSettleTo(null), 320)
    } else {
      // Close: keep the drawer mounted while the slide-out plays, exactly
      // like the X button path.
      setMobileClosing(true)
      setTimeout(() => {
        setMobileClosing(false)
        setMobileOpen(false)
        setSettleTo(null)
      }, 320)
    }
  }

  // Close slides the drawer out (transform target) before unmounting it.
  const closeMobile = () => {
    if (mobileClosing) return
    setMobileClosing(true)
    setSettleTo(DRAWER_W)
    setTimeout(() => {
      setMobileClosing(false)
      setMobileOpen(false)
      setSettleTo(null)
    }, 320)
  }

  const lang = (i18n.language ?? 'en').startsWith('zh') ? 'zh' : 'en'
  const switchLang = () => setLanguage(lang === 'zh' ? 'en' : 'zh')

  const logout = async () => {
    try {
      await api.logout()
    } finally {
      navigate('/admin/login', { replace: true })
    }
  }

  const nav = (
    <nav className="flex flex-col gap-1">
      <NavItem to="/admin" icon={<LayoutDashboard size={18} />} label={t('nav.dashboard')} onClick={closeMobile} />
      <NavItem to="/admin/links" icon={<Link2 size={18} />} label={t('nav.links')} onClick={closeMobile} />
      <NavItem to="/admin/logs" icon={<ScrollText size={18} />} label={t('nav.logs')} onClick={closeMobile} />
      <NavItem to="/admin/settings" icon={<Settings size={18} />} label={t('nav.settings')} onClick={closeMobile} />
    </nav>
  )

  const sidebarContent = (
    <>
      <div className="mb-8 flex items-center gap-2.5">
        <AppIcon />
        <div>
          <div className="font-semibold leading-tight">{siteName}</div>
          <div className="text-xs text-muted">{t('app.tagline')}</div>
        </div>
      </div>
      {nav}
      {/* Bottom padding clears the fixed footer that spans the viewport. */}
      <div className="mt-auto flex flex-col gap-1 pb-12">
        {/* Buttons show the CURRENT mode/language, not the toggle target. */}
        <button
          onClick={toggle}
          className="flex items-center gap-2.5 rounded-xl px-3 py-2 text-sm text-muted transition-colors hover:bg-black/5 dark:hover:bg-white/10"
        >
          {dark ? <Moon size={18} /> : <Sun size={18} />}
          {dark ? t('app.dark') : t('app.light')}
        </button>
        <button
          onClick={switchLang}
          className="flex items-center gap-2.5 rounded-xl px-3 py-2 text-sm text-muted transition-colors hover:bg-black/5 dark:hover:bg-white/10"
        >
          <span className="w-[18px] text-center text-xs font-semibold">{lang === 'zh' ? '中' : 'EN'}</span>
          {lang === 'zh' ? '中文' : 'English'}
        </button>
        <button
          onClick={logout}
          className="flex items-center gap-2.5 rounded-xl px-3 py-2 text-sm text-muted transition-colors hover:bg-danger/10 hover:text-danger"
        >
          <LogOut size={18} />
          {t('nav.logout')}
        </button>
      </div>
    </>
  )

  return (
    <div
      className="flex min-h-screen"
      onTouchStart={onDrawerTouchStart}
      onTouchMove={onDrawerTouchMove}
      onTouchEnd={finishDrawerGesture}
      onTouchCancel={finishDrawerGesture}
    >
      {/* Desktop sidebar */}
      <aside className="sticky top-0 hidden h-screen w-60 shrink-0 flex-col overflow-y-auto border-r border-hairline bg-white/50 p-5 backdrop-blur-xl dark:bg-white/[0.04] md:flex">
        {sidebarContent}
      </aside>

      {/* Mobile top bar + drawer */}
      <div className="fixed inset-x-0 top-0 z-40 flex items-center justify-between border-b border-hairline bg-canvas/80 px-4 py-3 backdrop-blur-xl dark:bg-canvas-dark/80 md:hidden">
        <div className="flex items-center gap-2">
          <AppIcon size={22} />
          <span className="font-semibold">{siteName}</span>
        </div>
        <button onClick={() => setMobileOpen(true)} aria-label={t('app.menu')} className="rounded-lg p-1.5">
          <Menu size={20} />
        </button>
      </div>
      {(mobileOpen || mobileClosing) && (
        <div className="fixed inset-0 z-50 md:hidden">
          <div
            className={`absolute inset-0 bg-black/30 backdrop-blur-sm ${mobileClosing ? 'animate-backdrop-out' : 'animate-backdrop-in'}`}
            onClick={closeMobile}
          />
          <div
            className="absolute inset-y-0 right-0 flex w-64 flex-col overflow-y-auto bg-canvas p-5 shadow-2xl dark:bg-canvas-dark"
            style={{
              transform: `translateX(${settleTo ?? dragX ?? (mobileOpen ? 0 : DRAWER_W)}px)`,
              transition: dragX !== null ? 'none' : 'transform 0.3s cubic-bezier(0.32, 0.72, 0, 1)',
            }}
          >
            <button onClick={closeMobile} className="mb-4 self-end rounded-lg p-1.5" aria-label={t('common.close')}>
              <X size={20} />
            </button>
            {sidebarContent}
          </div>
        </div>
      )}

      <main className="min-w-0 flex-1 px-4 pb-16 pt-16 md:px-8 md:pt-8">
        {/* key remounts the page on navigation, replaying the fade-up */}
        <div key={location.pathname} className="mx-auto max-w-5xl animate-fade-up">
          <Outlet />
        </div>
      </main>

      {/* Footer: project name, GitHub link, version */}
      <footer className="fixed inset-x-0 bottom-0 z-30 border-t border-hairline bg-canvas/80 py-2.5 text-center text-xs text-muted backdrop-blur-xl dark:bg-canvas-dark/80">
        <span className="font-medium text-ink/80 dark:text-ink-dark/80">{siteName}</span>
        <span className="mx-2 opacity-40">·</span>
        <a href={__APP_REPO__} target="_blank" rel="noreferrer" className="transition-colors hover:text-accent">
          {t('footer.openSource')}
        </a>
        <span className="mx-2 opacity-40">·</span>
        <span className="short-code">v{__APP_VERSION__}</span>
      </footer>
    </div>
  )
}

function NavItem({
  to,
  icon,
  label,
  onClick,
}: {
  to: string
  icon: React.ReactNode
  label: string
  onClick?: () => void
}) {
  return (
    <NavLink
      to={to}
      end={to === '/admin'}
      onClick={onClick}
      className={({ isActive }) =>
        `flex items-center gap-2.5 rounded-xl px-3 py-2 text-sm transition-colors ${
          isActive
            ? 'bg-accent-soft font-medium text-accent-deep dark:text-accent'
            : 'text-muted hover:bg-black/5 dark:hover:bg-white/10'
        }`
      }
    >
      {icon}
      {label}
    </NavLink>
  )
}
