import { createContext, useCallback, useContext, useEffect, useState } from 'react'
import type { ButtonHTMLAttributes, InputHTMLAttributes, ReactNode, TextareaHTMLAttributes } from 'react'
import { X } from 'lucide-react'

/* ---------- Button ---------- */

type ButtonVariant = 'primary' | 'ghost' | 'danger' | 'outline'

const buttonStyles: Record<ButtonVariant, string> = {
  primary:
    'bg-accent text-white hover:bg-accent-deep shadow-[0_4px_14px_rgba(245,158,11,0.35)]',
  ghost:
    'text-ink dark:text-ink-dark hover:bg-black/5 dark:hover:bg-white/10',
  danger:
    'bg-danger/10 text-danger hover:bg-danger/20',
  outline:
    'border border-hairline hover:bg-black/5 dark:hover:bg-white/10',
}

export function Button({
  variant = 'primary',
  className = '',
  ...props
}: ButtonHTMLAttributes<HTMLButtonElement> & { variant?: ButtonVariant }) {
  return (
    <button
      className={`inline-flex items-center justify-center gap-1.5 rounded-xl px-3.5 py-2 text-sm font-medium transition-colors disabled:opacity-50 disabled:pointer-events-none ${buttonStyles[variant]} ${className}`}
      {...props}
    />
  )
}

/* ---------- Inputs ---------- */

const fieldClass =
  'w-full rounded-xl border border-hairline bg-white/70 dark:bg-white/[0.07] px-3.5 py-2 text-sm outline-none transition focus:border-accent focus:ring-2 focus:ring-accent/30 placeholder:text-muted/70'

export function Input({ className = '', ...props }: InputHTMLAttributes<HTMLInputElement>) {
  return <input className={`${fieldClass} ${className}`} {...props} />
}

export function Textarea({ className = '', ...props }: TextareaHTMLAttributes<HTMLTextAreaElement>) {
  return <textarea className={`${fieldClass} ${className}`} {...props} />
}

export function Label({ children, htmlFor }: { children: ReactNode; htmlFor?: string }) {
  return (
    <label htmlFor={htmlFor} className="mb-1.5 block text-xs font-medium text-muted">
      {children}
    </label>
  )
}

/* ---------- Card ---------- */

export function Card({ children, className = '' }: { children: ReactNode; className?: string }) {
  return <div className={`glass ${className}`}>{children}</div>
}

/* ---------- Dialog ---------- */

export function Dialog({
  open,
  onClose,
  title,
  children,
  wide = false,
}: {
  open: boolean
  onClose: () => void
  title: string
  children: ReactNode
  wide?: boolean
}) {
  // Exit animation: keep the dialog mounted for a beat after close so the
  // pop-out/backdrop-out play, then unmount. Escape/backdrop/close all funnel
  // through requestClose.
  const [closing, setClosing] = useState(false)
  const requestClose = () => {
    if (closing) return
    setClosing(true)
    setTimeout(() => {
      setClosing(false)
      onClose()
    }, 180)
  }

  useEffect(() => {
    if (!open) return
    const onKey = (e: KeyboardEvent) => e.key === 'Escape' && requestClose()
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open, closing])

  if (!open) return null
  return (
    // Outer scroller keeps tall dialogs reachable at the top (max-h centering
    // would clip the header off-screen); the panel itself is opaque so page
    // content never bleeds through.
    <div className="fixed inset-0 z-50 overflow-y-auto">
      <div className="flex min-h-full items-center justify-center p-4">
        <div
          className={`fixed inset-0 ${closing ? 'animate-backdrop-out' : 'animate-backdrop-in'} bg-black/30 backdrop-blur-sm`}
          onClick={requestClose}
        />
        <div
          role="dialog"
          aria-modal="true"
          className={`relative w-full rounded-2xl border border-hairline bg-white p-6 shadow-[0_24px_80px_rgba(0,0,0,0.25)] dark:bg-[#1c1c1e] ${
            wide ? 'max-w-2xl' : 'max-w-md'
          } ${closing ? 'animate-pop-out' : 'animate-pop-in'}`}
        >
          <div className="mb-4 flex items-center justify-between">
            <h2 className="text-lg font-semibold">{title}</h2>
            <button onClick={requestClose} aria-label="Close" className="rounded-lg p-1 transition-colors hover:bg-black/5 dark:hover:bg-white/10">
              <X size={18} />
            </button>
          </div>
          {children}
        </div>
      </div>
    </div>
  )
}

/* ---------- Toasts ---------- */

interface Toast {
  id: number
  message: string
  kind: 'success' | 'error'
  closing?: boolean
}

const ToastContext = createContext<{ toast: (message: string, kind?: 'success' | 'error') => void }>({
  toast: () => {},
})

export function ToastProvider({ children }: { children: ReactNode }) {
  const [toasts, setToasts] = useState<Toast[]>([])

  // Fade out first (marks closing), then unmount after the exit animation.
  const dismiss = (id: number) => {
    setToasts((ts) => ts.map((t) => (t.id === id ? { ...t, closing: true } : t)))
    setTimeout(() => setToasts((ts) => ts.filter((t) => t.id !== id)), 240)
  }

  const toast = useCallback((message: string, kind: 'success' | 'error' = 'success') => {
    const id = Date.now() + Math.random()
    // Errors deserve more reading time; successes dismiss quickly.
    const ttl = kind === 'error' ? 6000 : 3200
    // Stack newest on top, cap at 5 visible at once.
    setToasts((ts) => [...ts.slice(-4), { id, message, kind }])
    setTimeout(() => dismiss(id), ttl)
  }, [])

  return (
    <ToastContext.Provider value={{ toast }}>
      {children}
      {/* Bottom-right stacked toasts: compact, out of the way, layered */}
      <div className="pointer-events-none fixed bottom-6 right-6 z-[70] flex max-w-sm flex-col items-end gap-2">
        {toasts.map((t) => (
          <div
            key={t.id}
            className={`pointer-events-auto flex max-w-full items-center gap-2 rounded-xl border border-hairline bg-white px-3.5 py-2.5 text-sm font-medium shadow-[0_12px_40px_rgba(0,0,0,0.18)] dark:bg-[#1c1c1e] ${
              t.kind === 'error' ? '!border-danger/25 !text-danger' : ''
            } ${t.closing ? 'animate-toast-out' : 'animate-toast-in'}`}
          >
            <span className="min-w-0 truncate">{t.message}</span>
            <button
              onClick={() => dismiss(t.id)}
              aria-label="Dismiss"
              className="shrink-0 rounded-md p-0.5 text-muted transition-colors hover:text-ink dark:hover:text-ink-dark"
            >
              <X size={14} />
            </button>
          </div>
        ))}
      </div>
    </ToastContext.Provider>
  )
}

export function useToast() {
  return useContext(ToastContext)
}
