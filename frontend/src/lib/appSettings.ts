// Device-local app settings (localStorage). Only the app writes these keys —
// the connection card owns the toggles, and App.tsx applies the effects at
// startup.

const NO_SELECT_KEY = 'gourl-no-select'

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
