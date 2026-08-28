import { isApp } from './api'

// Share a URL through the system/browser share sheet. The route depends on
// the environment:
// - Android app: the WebView never exposes the Web Share API (even on the
//   secure https://localhost origin), so the GourlBridge opens the system
//   ACTION_SEND chooser instead.
// - Web: the Web Share API requires a secure context (HTTPS or localhost);
//   on plain http it is absent entirely, so callers get 'unsupported' and
//   show a hint.
// A user dismissing either sheet is not a failure.

export type ShareOutcome = 'shared' | 'unsupported' | 'cancelled' | 'failed'

export async function shareUrl(url: string): Promise<ShareOutcome> {
  if (isApp()) {
    const bridge = (window as Window & { GourlBridge?: { share?: (url: string) => boolean } }).GourlBridge
    if (bridge?.share) return bridge.share(url) ? 'shared' : 'failed'
    return 'unsupported' // pre-bridge APK
  }
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
