import type { CapacitorConfig } from '@capacitor/cli'

// Mobile wrapper config: the admin SPA ships inside a Capacitor WebView and
// talks to a remote gourl server through a bearer token (see lib/api.ts).
// The gourl:// scheme drives deep links; cleartext http is allowed because
// self-hosted servers are often plain http on a LAN.
const config: CapacitorConfig = {
  appId: 'com.wmy2981.gourl',
  appName: 'gourl',
  webDir: 'dist',
  android: {
    allowMixedContent: true,
  },
  server: {
    cleartext: true,
  },
  // No disableBackButtonHandler: the plugin's back-button event is enabled so
  // App.tsx can intercept it — back first closes the top dialog (Escape),
  // then exits the app. (Enabling the handler trades the Android 13+
  // predictive-back animation for that control, which the user prefers.)
  plugins: {
    // The WebView does not propagate status-bar insets to env() on many
    // devices; insetsHandling "css" makes the core SystemBars plugin inject
    // --safe-area-inset-* variables the CSS below falls back to.
    SystemBars: {
      insetsHandling: 'css',
    },
  },
}

export default config
