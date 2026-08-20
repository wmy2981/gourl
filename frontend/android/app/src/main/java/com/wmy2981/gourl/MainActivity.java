package com.wmy2981.gourl;

import android.content.ContentValues;
import android.net.Uri;
import android.os.Build;
import android.os.Bundle;
import android.os.Environment;
import android.provider.MediaStore;
import android.util.Base64;
import android.view.HapticFeedbackConstants;
import android.webkit.JavascriptInterface;
import android.webkit.WebSettings;
import android.webkit.WebView;

import com.getcapacitor.BridgeActivity;

import java.io.OutputStream;

public class MainActivity extends BridgeActivity {
    @Override
    public void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);
        WebView wv = getBridge().getWebView();
        // Hide the system overlay scrollbars: the SPA themes its own
        // scrollbars (and suppresses them entirely in app mode via CSS).
        wv.setVerticalScrollBarEnabled(false);
        wv.setHorizontalScrollBarEnabled(false);
        // Identifiable user agent — "gourl/<version> <default UA>" — so the
        // servers the app talks to can recognize its requests. The version
        // mirrors the Docker VERSION_STR (plain on main, "VERSION (sha7)" on
        // dev) injected as the APK versionName by the workflow.
        wv.getSettings().setUserAgentString(
            "gourl/" + BuildConfig.VERSION_NAME + " " + WebSettings.getDefaultUserAgent(this));
        wv.addJavascriptInterface(new GourlBridge(), "GourlBridge");
    }

    /**
     * Native bridge for the SPA (window.GourlBridge), for the things the
     * Capacitor plugins cannot do correctly here:
     *
     * saveToDownloads — writes a file into the system Downloads/gourl
     * directory via MediaStore. The capgo file-sharer plugin reroutes any
     * image/* content type to the Pictures collection and then rejects a
     * Download/ relative path with ERR_PARAM_DATA_INVALID, so the QR JPEG
     * could never be saved to Downloads through it. Android 10+ needs no
     * permission for MediaStore.Downloads.
     *
     * moveToBackground — Android back returns the app to the launcher with
     * its state intact; App.exitApp() would kill the process.
     *
     * switchHaptic — the system's switch tick through the haptic feedback
     * engine. The Capacitor haptics plugins only drive the vibrator motor
     * (a buzzing hum, not a click); performHapticFeedback uses the touch
     * engine the system switches use, respects the user's haptic-feedback
     * setting, and needs no VIBRATE permission. TOGGLE_ON/OFF exist since
     * API 33; below that CLOCK_TICK is the closest light tick.
     *
     * buttonHaptic — the same engine's lightest standard feedback
     * (KEYBOARD_TAP, a soft virtual-key press) for button presses; it also
     * respects the user's haptic-feedback setting. KEYBOARD_TAP exists
     * since API 5, so no version branch is needed.
     */
    public class GourlBridge {
        @JavascriptInterface
        public String saveToDownloads(String filename, String base64, String mime) {
            try {
                byte[] data = Base64.decode(base64, Base64.DEFAULT);
                ContentValues values = new ContentValues();
                values.put(MediaStore.MediaColumns.DISPLAY_NAME, filename);
                values.put(MediaStore.MediaColumns.MIME_TYPE, mime);
                values.put(
                    MediaStore.MediaColumns.RELATIVE_PATH,
                    Environment.DIRECTORY_DOWNLOADS + "/gourl");
                Uri uri =
                    getContentResolver().insert(MediaStore.Downloads.EXTERNAL_CONTENT_URI, values);
                if (uri == null) {
                    return null;
                }
                try (OutputStream os = getContentResolver().openOutputStream(uri)) {
                    if (os == null) {
                        return null;
                    }
                    os.write(data);
                }
                return Environment.DIRECTORY_DOWNLOADS + "/gourl/" + filename;
            } catch (Exception e) {
                return null;
            }
        }

        @JavascriptInterface
        public void moveToBackground() {
            runOnUiThread(() -> moveTaskToBack(true));
        }

        @JavascriptInterface
        public void switchHaptic(boolean on) {
            runOnUiThread(() -> {
                int feedback =
                    Build.VERSION.SDK_INT >= Build.VERSION_CODES.TIRAMISU
                        ? (on ? HapticFeedbackConstants.TOGGLE_ON : HapticFeedbackConstants.TOGGLE_OFF)
                        : HapticFeedbackConstants.CLOCK_TICK;
                getBridge().getWebView().performHapticFeedback(feedback);
            });
        }

        @JavascriptInterface
        public void buttonHaptic() {
            runOnUiThread(() ->
                getBridge().getWebView().performHapticFeedback(HapticFeedbackConstants.KEYBOARD_TAP));
        }
    }
}
