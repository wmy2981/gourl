import { useRef, useState } from 'react'
import { CalendarDays, X } from 'lucide-react'

// Parses yyyy/MM/dd or yyyy-MM-dd (month/day may be 1-2 digits) into ISO
// yyyy-mm-dd; returns null when the string is not a real calendar date.
function parseDate(raw: string): string | null {
  const m = raw.trim().match(/^(\d{4})[/-](\d{1,2})[/-](\d{1,2})$/)
  if (!m) return null
  const [y, mo, d] = [Number(m[1]), Number(m[2]), Number(m[3])]
  if (mo < 1 || mo > 12 || d < 1 || d > 31) return null
  const dt = new Date(y, mo - 1, d)
  if (dt.getFullYear() !== y || dt.getMonth() !== mo - 1 || dt.getDate() !== d) return null
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${y}-${pad(mo)}-${pad(d)}`
}

// ISO yyyy-mm-dd → display yyyy/MM/dd.
function toSlash(iso: string): string {
  return iso.replace(/^(\d{4})-(\d{2})-(\d{2})$/, '$1/$2/$3')
}

const fieldClass =
  'w-full rounded-xl border border-hairline bg-white/70 dark:bg-white/[0.07] px-3.5 py-2 text-sm outline-none transition focus:border-accent focus:ring-2 focus:ring-accent/30 placeholder:text-muted/70'

// Expiry date field rendered as a fixed-format yyyy/MM/dd text input (the
// native date input's rendering follows the browser locale, which produced an
// inconsistent "yyyy/mm/日" mix). Typing accepts yyyy/MM/dd or yyyy-MM-dd;
// the calendar button opens the native picker via showPicker(). Empty value
// means "never expires". onChange reports the ISO date, or null while the
// current text is not a valid date (empty string = cleared / never).
export default function DateInput({
  value = '',
  onChange,
  id,
  ariaLabel,
  placeholder = 'yyyy/MM/dd',
  className = '',
}: {
  /** Initial ISO yyyy-mm-dd; '' = never expires. */
  value?: string
  /** ISO yyyy-mm-dd, or null while the text is not a valid date. */
  onChange: (iso: string | null) => void
  id?: string
  ariaLabel?: string
  placeholder?: string
  className?: string
}) {
  const [text, setText] = useState(toSlash(value))
  const [invalid, setInvalid] = useState(false)
  const pickerRef = useRef<HTMLInputElement>(null)

  const handle = (raw: string) => {
    setText(raw)
    setInvalid(false)
    const iso = raw.trim() === '' ? '' : parseDate(raw)
    onChange(iso)
  }

  const openPicker = () => {
    try {
      pickerRef.current?.showPicker()
    } catch {
      // showPicker is unavailable (e.g. Safari) — the text field still works.
    }
  }

  return (
    <div className={`relative ${className}`}>
      <input
        type="text"
        id={id}
        aria-label={ariaLabel}
        value={text}
        onChange={(e) => handle(e.target.value)}
        onBlur={() => {
          if (text.trim() !== '' && parseDate(text) === null) setInvalid(true)
        }}
        onFocus={() => setInvalid(false)}
        placeholder={placeholder}
        inputMode="numeric"
        className={`${fieldClass} pr-16 ${invalid ? 'border-danger focus:border-danger focus:ring-danger/30' : ''}`}
      />
      {text !== '' && (
        <button
          type="button"
          onClick={() => handle('')}
          aria-label="Clear"
          className="absolute right-9 top-1/2 -translate-y-1/2 rounded-md p-1 text-muted/70 transition-colors hover:text-ink dark:hover:text-ink-dark"
        >
          <X size={14} />
        </button>
      )}
      <button
        type="button"
        onClick={openPicker}
        aria-label="Pick a date"
        className="absolute right-2 top-1/2 -translate-y-1/2 rounded-md p-1 text-muted/70 transition-colors hover:text-ink dark:hover:text-ink-dark"
      >
        <CalendarDays size={16} />
      </button>
      {/* Hidden native input backing the calendar picker (rendered, not
          display:none, so showPicker() is allowed). */}
      <input
        ref={pickerRef}
        type="date"
        tabIndex={-1}
        aria-hidden="true"
        value={parseDate(text) ?? ''}
        onChange={(e) => handle(e.target.value)}
        className="pointer-events-none absolute h-0 w-0 opacity-0"
      />
    </div>
  )
}
