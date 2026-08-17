// Copy-to-clipboard with multi-tier fallback, ported from
// connection-checker's useClipboard: Clipboard API → hidden textarea +
// execCommand → false (caller shows a manual-copy hint).
//
// Lessons baked in:
// - The textarea must be renderable but off-viewport (opacity/display:none
//   makes execCommand return true while copying empty/old selection)
// - Mount it inside the closest [role=dialog] so focus traps don't clear
//   the selection (modal copies otherwise fail)

export async function copyText(text: string): Promise<boolean> {
  // Tier 1: Clipboard API (HTTPS or localhost; unavailable on plain http).
  if (navigator.clipboard && typeof navigator.clipboard.writeText === 'function') {
    try {
      await navigator.clipboard.writeText(text)
      return true
    } catch {
      // permission denied or non-secure context → fall through
    }
  }

  // Tier 2: hidden textarea + execCommand (works on http intranets).
  try {
    const ta = document.createElement('textarea')
    ta.value = text
    ta.setAttribute('readonly', '')
    ta.style.position = 'absolute'
    ta.style.left = '-9999px'
    ta.style.top = '0'
    const container =
      (document.activeElement?.closest('[role="dialog"]') as HTMLElement | null) ?? document.body
    container.appendChild(ta)
    const selection = document.getSelection()
    const prevRange = selection && selection.rangeCount > 0 ? selection.getRangeAt(0) : null
    ta.select()
    ta.setSelectionRange(0, text.length)
    const ok = document.execCommand('copy')
    ta.remove()
    if (prevRange && selection) {
      selection.removeAllRanges()
      selection.addRange(prevRange)
    }
    if (ok) return true
  } catch {
    // fall through
  }

  // Tier 3: nothing worked — let the caller prompt manual copy.
  return false
}
