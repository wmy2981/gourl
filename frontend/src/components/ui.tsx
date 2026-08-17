import { createContext, useCallback, useContext, useEffect, useReducer, useRef, useState } from 'react'
import type { ButtonHTMLAttributes, InputHTMLAttributes, ReactNode, TextareaHTMLAttributes } from 'react'
import { useTranslation } from 'react-i18next'
import { AnimatePresence, motion } from 'motion/react'
import { AlertCircle, Check, CheckCircle2, ChevronDown, X } from 'lucide-react'

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

/* ---------- Checkbox ---------- */

// Drawn checkbox (native inputs render with the OS palette, which clashes
// with both themes): amber fill when checked, hairline border otherwise, a
// theme-consistent hover state.
export function Checkbox({
  checked,
  onChange,
  'aria-label': ariaLabel,
  className = '',
}: {
  checked: boolean
  onChange: (checked: boolean) => void
  'aria-label'?: string
  className?: string
}) {
  return (
    <button
      type="button"
      role="checkbox"
      aria-checked={checked}
      aria-label={ariaLabel}
      onClick={() => onChange(!checked)}
      className={`flex size-4 shrink-0 cursor-pointer items-center justify-center rounded-[5px] border transition-colors ${
        checked
          ? 'border-accent bg-accent text-white'
          : 'border-muted/45 bg-transparent hover:border-accent'
      } ${className}`}
    >
      {checked && <Check size={12} strokeWidth={3.5} />}
    </button>
  )
}

/* ---------- Select ---------- */

// Custom dropdown replacing the native <select>: same amber-on-graphite
// language as the rest of the UI, pop-in options panel, theme-aware.
export function Select({
  value,
  onChange,
  options,
  ariaLabel,
  className = '',
}: {
  value: string
  onChange: (value: string) => void
  options: { value: string; label: string }[]
  ariaLabel?: string
  className?: string
}) {
  const [open, setOpen] = useState(false)
  const ref = useRef<HTMLDivElement>(null)

  useEffect(() => {
    if (!open) return
    const onDown = (e: MouseEvent) => {
      if (ref.current && !ref.current.contains(e.target as Node)) setOpen(false)
    }
    document.addEventListener('mousedown', onDown)
    return () => document.removeEventListener('mousedown', onDown)
  }, [open])

  const current = options.find((o) => o.value === value)

  return (
    <div ref={ref} className={`relative ${className}`}>
      <button
        type="button"
        aria-label={ariaLabel}
        aria-expanded={open}
        onClick={() => setOpen((o) => !o)}
        className="flex h-9 w-full items-center justify-between gap-2 rounded-xl border border-hairline bg-white/70 px-3 text-sm text-ink transition focus:border-accent focus:ring-2 focus:ring-accent/30 dark:bg-white/[0.07] dark:text-ink-dark"
      >
        <span className="truncate">{current?.label}</span>
        <ChevronDown
          size={14}
          className={`shrink-0 text-muted transition-transform duration-200 ${open ? 'rotate-180' : ''}`}
        />
      </button>
      {open && (
        <div className="absolute z-50 mt-1 w-full overflow-hidden rounded-xl border border-hairline bg-white p-1 shadow-[0_12px_40px_rgba(0,0,0,0.18)] dark:bg-[#1c1c1e] animate-pop-in">
          {options.map((o) => (
            <button
              key={o.value}
              type="button"
              onClick={() => {
                onChange(o.value)
                setOpen(false)
              }}
              className={`flex w-full items-center justify-between gap-2 rounded-lg px-2.5 py-1.5 text-left text-sm transition-colors ${
                o.value === value
                  ? 'bg-accent-soft font-medium text-accent-deep dark:text-accent'
                  : 'text-ink hover:bg-black/5 dark:text-ink-dark dark:hover:bg-white/10'
              }`}
            >
              <span className="truncate">{o.label}</span>
              {o.value === value && <Check size={14} className="shrink-0 text-accent" />}
            </button>
          ))}
        </div>
      )}
    </div>
  )
}

/* ---------- Card ---------- */

export function Card({
  children,
  className = '',
  ref,
  ...rest
}: { children: ReactNode; className?: string; ref?: React.Ref<HTMLDivElement> } & React.HTMLAttributes<HTMLDivElement>) {
  return (
    <div ref={ref} className={`glass ${className}`} {...rest}>
      {children}
    </div>
  )
}

/* ---------- Dialog ---------- */

export function Dialog({
  open,
  onClose,
  title,
  children,
  wide = false,
  headerActions,
}: {
  open: boolean
  onClose: () => void
  title: string
  children: ReactNode
  wide?: boolean
  /** Extra buttons rendered next to the close button in the header. */
  headerActions?: ReactNode
}) {
  // Exit animation driven by the open prop: whatever triggers close (X,
  // Escape, backdrop, or a form's cancel button calling onClose), the panel
  // stays mounted for a beat to play pop-out/backdrop-out before unmounting.
  const { t } = useTranslation()
  const [rendered, setRendered] = useState(open)
  const [closing, setClosing] = useState(false)

  useEffect(() => {
    if (open) {
      setRendered(true)
      setClosing(false)
    } else if (rendered) {
      setClosing(true)
      const t = setTimeout(() => setRendered(false), 180)
      return () => clearTimeout(t)
    }
  }, [open, rendered])

  useEffect(() => {
    if (!open) return
    const onKey = (e: KeyboardEvent) => e.key === 'Escape' && onClose()
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [open, onClose])

  // Scroll lock: while a dialog is up (including its exit animation) the
  // page behind must not scroll — dialogs are focused surfaces.
  useEffect(() => {
    if (!rendered) return
    const prev = document.body.style.overflow
    document.body.style.overflow = 'hidden'
    return () => {
      document.body.style.overflow = prev
    }
  }, [rendered])

  if (!rendered) return null
  return (
    // Outer scroller keeps tall dialogs reachable at the top (max-h centering
    // would clip the header off-screen); the panel itself is opaque so page
    // content never bleeds through.
    <div className="fixed inset-0 z-50 overflow-y-auto">
      <div className="flex min-h-full items-center justify-center p-4">
        <div
          className={`fixed inset-0 ${closing ? 'animate-backdrop-out' : 'animate-backdrop-in'} bg-black/30 backdrop-blur-sm`}
          onClick={onClose}
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
            <div className="flex items-center gap-1">
              {headerActions}
              <button onClick={onClose} aria-label={t('common.close')} className="rounded-lg p-1 transition-colors hover:bg-black/5 dark:hover:bg-white/10">
                <X size={18} />
              </button>
            </div>
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
}

const ToastContext = createContext<{ toast: (message: string, kind?: 'success' | 'error') => void }>({
  toast: () => {},
})

// Cards stack as a pile: the newest is fully visible in front, older ones
// peek 12px above it. Hovering the stack expands every card. Cards size
// themselves to their content; the overlap offset uses the measured height.
const TOAST_FALLBACK_H = 64 // used until the first measured height arrives
const PEEK = 12 // visible edge of each collapsed rear card
const EXPAND_GAP = 8 // spacing between cards when expanded

export function ToastProvider({ children }: { children: ReactNode }) {
  const { t } = useTranslation()
  const [toasts, setToasts] = useState<Toast[]>([])
  const [expanded, setExpanded] = useState(false)
  // Measured heights per card: the pile overlap is (height - peek), and cards
  // are content-sized so this can't be hardcoded.
  const heights = useRef(new Map<number, number>())
  const [, force] = useReducer((x: number) => x + 1, 0)

  const dismiss = useCallback((id: number) => {
    setToasts((ts) => ts.filter((t) => t.id !== id))
  }, [])

  const toast = useCallback((message: string, kind: 'success' | 'error' = 'success') => {
    const id = Date.now() + Math.random()
    // Errors deserve more reading time; successes dismiss quickly.
    const ttl = kind === 'error' ? 6000 : 3200
    // Newest first: index 0 is the front card, older ones stack above it.
    setToasts((ts) => [{ id, message, kind }, ...ts].slice(0, 5))
    setTimeout(() => dismiss(id), ttl)
  }, [dismiss])

  return (
    <ToastContext.Provider value={{ toast }}>
      {children}
      <div
        className="pointer-events-none fixed bottom-6 right-6 z-[70]"
        onMouseEnter={() => setExpanded(true)}
        onMouseLeave={() => setExpanded(false)}
      >
        <div className="pointer-events-auto flex flex-col">
          <AnimatePresence>
            {[...toasts].reverse().map((toast, i, arr) => {
              // Newest renders last (bottom, front-most). Rear cards get a
              // negative bottom margin so they rise and the front card covers
              // their lower half — only a 12px top edge of each stays visible,
              // a pile like the reference implementation. Hover expands them.
              const isFront = i === arr.length - 1
              const cardH = heights.current.get(toast.id) ?? TOAST_FALLBACK_H
              const marginBottom = isFront ? 0 : expanded ? EXPAND_GAP : PEEK - cardH
              return (
                <motion.div
                  key={toast.id}
                  layout
                  ref={(el) => {
                    // Cards are content-sized; record the rendered height so
                    // the pile overlap tracks it (fires once per card).
                    if (el && heights.current.get(toast.id) !== el.offsetHeight) {
                      heights.current.set(toast.id, el.offsetHeight)
                      force()
                    }
                  }}
                  initial={{ opacity: 0, y: 28, scale: 0.95 }}
                  animate={{ opacity: 1, y: 0, scale: 1, marginBottom }}
                  exit={{ opacity: 0, y: -14, scale: 0.95 }}
                  transition={{ type: 'spring', stiffness: 380, damping: 32 }}
                  className={`flex w-fit max-w-80 select-none items-start gap-2.5 rounded-lg border px-3.5 py-3 text-sm font-medium shadow-[0_8px_30px_rgba(0,0,0,0.12)] ${
                    toast.kind === 'error'
                      ? 'border-danger/20 bg-[#fff7f6] text-danger dark:bg-[#2a1a1a] dark:text-red-300'
                      : 'border-accent/20 bg-[#fffaf0] text-accent-deep dark:bg-[#2a2015] dark:text-amber-300'
                  }`}
                >
                  {toast.kind === 'error' ? (
                    <AlertCircle size={16} className="mt-0.5 shrink-0 text-danger" fill="currentColor" stroke="white" />
                  ) : (
                    <CheckCircle2 size={16} className="mt-0.5 shrink-0 text-accent" />
                  )}
                  <span className="min-w-0 line-clamp-2">{toast.message}</span>
                  <button
                    onClick={() => dismiss(toast.id)}
                    aria-label={t('common.dismiss')}
                    className="shrink-0 rounded-md p-0.5 text-muted/70 transition-colors hover:text-ink dark:hover:text-ink-dark"
                  >
                    <X size={14} />
                  </button>
                </motion.div>
              )
            })}
          </AnimatePresence>
        </div>
      </div>
    </ToastContext.Provider>
  )
}

export function useToast() {
  return useContext(ToastContext)
}
