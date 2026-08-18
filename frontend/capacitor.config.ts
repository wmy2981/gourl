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
  plugins: {
    App: {
      // The plugin's back-button interception suppresses the Android 13+
      // predictive back gesture (the two are mutually exclusive per the
      // Capacitor docs). With the handler disabled, the system gesture works
      // and back simply finishes the activity; dialogs close via their X.
      disableBackButtonHandler: true,
    },
  },
}

export default config
