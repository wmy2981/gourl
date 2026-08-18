// Downloads that work in both the web console and the Capacitor app.
// Web: a classic <a download>. App: @capgo/capacitor-file-sharer's save()
// writes into the system Downloads directory (Android MediaStore,
// Download/gourl/ subfolder) — the caller shows the returned path in a toast.

import { FileSharer } from '@capgo/capacitor-file-sharer'
import { isApp } from './api'

/**
 * Saves a file. `data` is plain text unless `base64` is set (binary payloads
 * like the QR JPEG). In the app the file lands in Downloads/gourl/ and the
 * human-readable path is returned; on the web the browser download starts
 * and null is returned.
 */
export async function saveDownload(
  filename: string,
  data: string,
  base64 = false,
  mime = 'text/plain',
): Promise<string | null> {
  if (!isApp()) {
    const blob = base64
      ? new Blob([Uint8Array.from(atob(data), (c) => c.charCodeAt(0))], { type: mime })
      : new Blob([data], { type: mime })
    const a = document.createElement('a')
    a.href = URL.createObjectURL(blob)
    a.download = filename
    a.click()
    URL.revokeObjectURL(a.href)
    return null
  }
  await FileSharer.save({
    filename,
    contentType: mime,
    base64Data: base64 ? data : utf8ToBase64(data),
    android: { saveDirectory: 'downloads', relativePath: 'Download/gourl' },
  })
  return `Download/gourl/${filename}`
}

/** Blob → base64 (the app-side payload for binary downloads). */
export function blobToBase64(blob: Blob): Promise<string> {
  return new Promise((resolve, reject) => {
    const fr = new FileReader()
    fr.onload = () => resolve(String(fr.result).split(',')[1] ?? '')
    fr.onerror = () => reject(fr.error)
    fr.readAsDataURL(blob)
  })
}

// UTF-8 → base64 without the btoa single-byte trap (exported JSON can carry
// Chinese text). Chunked so large exports never hit the call-stack limit.
function utf8ToBase64(s: string): string {
  const bytes = new TextEncoder().encode(s)
  let bin = ''
  const chunk = 0x8000
  for (let i = 0; i < bytes.length; i += chunk) {
    bin += String.fromCharCode(...bytes.subarray(i, i + chunk))
  }
  return btoa(bin)
}
