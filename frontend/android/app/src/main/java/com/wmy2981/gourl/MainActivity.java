package com.wmy2981.gourl;

import android.os.Bundle;
import android.webkit.WebSettings;
import android.webkit.WebView;

import com.getcapacitor.BridgeActivity;

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
        // The transparent status bar overlays the WebView content, and the
        // WebView does not propagate the inset to CSS env()/safe-area
        // variables reliably on many devices. Pad the view natively by the
        // platform status-bar height — the single authoritative top offset
        // (the capacitor-mode CSS resets its own padding so nothing stacks).
        int statusBarHeight = 0;
        int resId = getResources().getIdentifier("status_bar_height", "dimen", "android");
        if (resId > 0) {
            statusBarHeight = getResources().getDimensionPixelSize(resId);
        }
        wv.setPadding(0, statusBarHeight, 0, 0);
    }
}
