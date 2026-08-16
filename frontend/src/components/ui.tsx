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
        role="dialog"
        aria-modal="true"
        className={`glass w-full animate-pop-in ${wide ? 'max-w-2xl' : 'max-w-md'} max-h-[85vh] overflow-y-auto p-6`}
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
    // Errors deserve more reading time; successes dismiss quickly.
    const ttl = kind === 'error' ? 6000 : 3200
    setToasts((ts) => [...ts.slice(-4), { id, message, kind }]) // stack, cap at 5
    setTimeout(() => setToasts((ts) => ts.filter((t) => t.id !== id)), ttl)
  }, [])
  return (
    <ToastContext.Provider value={{ toast }}>
      {children}
      {/* Bottom-right stacked toasts: compact, out of the way, layered */}
      <div className="pointer-events-none fixed bottom-5 right-5 z-[60] flex max-w-sm flex-col items-end gap-1.5">
        {toasts.map((t) => (
          <div
            key={t.id}
            className={`glass animate-toast-in pointer-events-auto max-w-full truncate px-3.5 py-2 text-sm font-medium shadow-lg ${
              t.kind === 'error'
                ? '!text-danger !bg-white/90 dark:!bg-[#2a1a1a]/90'
                : ''
            }`}
            title={t.message}
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
