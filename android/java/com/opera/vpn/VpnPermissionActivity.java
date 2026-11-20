package com.opera.vpn;

import android.app.Activity;
import android.content.Intent;
import android.net.VpnService;
import android.os.Bundle;

/**
 * Transparent activity that shows Android's "Connection request" VPN consent
 * dialog (VpnService.prepare) and, once granted, starts OperaVpnService.
 */
public class VpnPermissionActivity extends Activity {
    static final String EXTRA_CONNECT = "connect";
    private static final int REQ_VPN = 4711;

    private boolean connect = true;

    @Override
    protected void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);
        connect = getIntent().getBooleanExtra(EXTRA_CONNECT, true);
        Intent consent = VpnService.prepare(this);
        if (consent == null) {
            onGranted();   // permission already given earlier
        } else {
            startActivityForResult(consent, REQ_VPN);
        }
    }

    @Override
    protected void onActivityResult(int requestCode, int resultCode, Intent data) {
        super.onActivityResult(requestCode, resultCode, data);
        if (requestCode != REQ_VPN) {
            return;
        }
        if (resultCode == RESULT_OK) {
            onGranted();
        } else {
            VpnBridge.state = VpnBridge.DENIED;
            finish();
        }
    }

    private void onGranted() {
        if (connect) {
            startService(new Intent(this, OperaVpnService.class));
        } else {
            VpnBridge.state = VpnBridge.IDLE;
        }
        finish();
    }
}
