// Device-local app settings (localStorage). Only the app writes these keys —
// the connection card owns the toggles, and App.tsx applies the effects at
// startup.

const NO_SELECT_KEY = 'gourl-no-select'
const HAPTICS_KEY = 'gourl-haptics'

/** Whether text selection is disabled across the app (default: off). */
export function noSelectEnabled(): boolean {
  return localStorage.getItem(NO_SELECT_KEY) === '1'
}

export function setNoSelect(enabled: boolean) {
  if (enabled) localStorage.setItem(NO_SELECT_KEY, '1')
  else localStorage.removeItem(NO_SELECT_KEY)
  applyNoSelect()
}

/** Syncs the html.no-select class with the stored setting. */
export function applyNoSelect() {
  document.documentElement.classList.toggle('no-select', noSelectEnabled())
}

/** Whether tap haptics are enabled across the app (default: on). */
export function hapticsEnabled(): boolean {
  return localStorage.getItem(HAPTICS_KEY) !== '0'
}

export function setHapticsEnabled(enabled: boolean) {
  if (enabled) localStorage.removeItem(HAPTICS_KEY)
  else localStorage.setItem(HAPTICS_KEY, '0')
}
