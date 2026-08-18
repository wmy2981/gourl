package com.wmy2981.gourl;

import android.os.Bundle;
import android.webkit.WebView;

import com.getcapacitor.BridgeActivity;

public class MainActivity extends BridgeActivity {
    @Override
    public void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);
        // Hide the system overlay scrollbars: the SPA themes its own
        // scrollbars (and suppresses them entirely in app mode via CSS).
        WebView wv = getBridge().getWebView();
        wv.setVerticalScrollBarEnabled(false);
        wv.setHorizontalScrollBarEnabled(false);
    }
}
