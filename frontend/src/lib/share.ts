// Share a URL through the system/browser share sheet (Web Share API).
// Requires a secure context (HTTPS or localhost): on plain http the API is
// absent entirely, so callers get 'unsupported' and show a hint (the Android
// app loads from https://localhost and therefore always reaches the sheet).

export type ShareOutcome = 'shared' | 'unsupported' | 'cancelled' | 'failed'

export async function shareUrl(url: string): Promise<ShareOutcome> {
  if (!('share' in navigator)) return 'unsupported'
  try {
    await navigator.share({ url })
    return 'shared'
  } catch (err) {
    // The user dismissing the sheet is not a failure.
    if (err instanceof DOMException && err.name === 'AbortError') return 'cancelled'
    return 'failed'
  }
}
