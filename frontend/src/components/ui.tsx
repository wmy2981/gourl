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
  useEffect(() => {
    if (!open) return
    const onKey = (e: KeyboardEvent) => e.key === 'Escape' && onClose()
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [open, onClose])

  if (!open) return null
  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center bg-black/30 p-4 backdrop-blur-sm"
      onMouseDown={(e) => e.target === e.currentTarget && onClose()}
    >
      <div
        className={`glass w-full ${wide ? 'max-w-2xl' : 'max-w-md'} max-h-[85vh] overflow-y-auto p-6`}
      >
        <div className="mb-4 flex items-center justify-between">
          <h2 className="text-lg font-semibold">{title}</h2>
          <button onClick={onClose} aria-label="Close" className="rounded-lg p-1 hover:bg-black/5 dark:hover:bg-white/10">
            <X size={18} />
          </button>
        </div>
        {children}
      </div>
    </div>
  )
}

/* ---------- Toasts ---------- */

interface Toast {
  id: number
  message: string
  kind: 'success' | 'error'
}

const ToastContext = createContext<{ toast: (message: string, kind?: 'success' | 'error') => void }>({
  toast: () => {},
})

export function ToastProvider({ children }: { children: ReactNode }) {
  const [toasts, setToasts] = useState<Toast[]>([])
  const toast = useCallback((message: string, kind: 'success' | 'error' = 'success') => {
    const id = Date.now() + Math.random()
    setToasts((ts) => [...ts, { id, message, kind }])
    setTimeout(() => setToasts((ts) => ts.filter((t) => t.id !== id)), 3200)
  }, [])
  return (
    <ToastContext.Provider value={{ toast }}>
      {children}
      <div className="fixed bottom-5 left-1/2 z-[60] flex -translate-x-1/2 flex-col items-center gap-2">
        {toasts.map((t) => (
          <div
            key={t.id}
            className={`glass px-4 py-2.5 text-sm font-medium ${
              t.kind === 'error' ? '!text-danger' : ''
            } animate-[fadeIn_.2s_ease]`}
          >
            {t.message}
          </div>
        ))}
      </div>
    </ToastContext.Provider>
  )
}

export function useToast() {
  return useContext(ToastContext)
}
