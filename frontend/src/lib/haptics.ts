import { isApp } from './api'

// Light key-press feedback for interactive taps (GourlBridge.buttonHaptic):
// the system's softest haptic through the touch engine, not the vibrator
// motor, and it honors the system haptic-feedback setting. Web: no-op.
export function buttonHaptic() {
  if (!isApp()) return
  const bridge = (window as Window & { GourlBridge?: { buttonHaptic?: () => void } }).GourlBridge
  bridge?.buttonHaptic?.()
}

// Interactive elements outside the haptic path:
// - role="switch" keeps its dedicated toggle tick (GourlBridge.switchHaptic)
// - role="checkbox" rows toggle without a buzz, matching a plain list tap
// - external anchors (target=_blank/_system, or a different origin) only
//   hand off to another app/browser — no feedback
const SKIP_ROLES = new Set(['switch', 'checkbox'])

function isExternalAnchor(el: HTMLAnchorElement): boolean {
  if (el.target === '_blank' || el.target === '_system') return true
  try {
    return new URL(el.href, location.href).origin !== location.origin
  } catch {
    return true // unparsable hrefs (javascript:, mailto:) are not in-app links
  }
}

// Global tap-haptics delegate: a capture-phase click on <a> or <button>
// anywhere (current and future components alike) fires the button haptic
// unless the element is skipped above. Capturing also beats any React
// stopPropagation, and disabled controls never dispatch clicks.
export function handleInteractiveClick(e: MouseEvent) {
  if (!(e.target instanceof Element)) return
  const el = e.target.closest('a, button')
  if (!el) return
  const role = el instanceof HTMLButtonElement ? el.getAttribute('role') : null
  if (role && SKIP_ROLES.has(role)) return
  if (el instanceof HTMLAnchorElement && isExternalAnchor(el)) return
  buttonHaptic()
}
