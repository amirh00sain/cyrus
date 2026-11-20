package com.opera.vpn;

import android.app.Activity;
import android.content.Context;
import android.content.Intent;

/**
 * Static facade between the Go code (called through JNI, see core/tun_android.go)
 * and the Android VpnService. All state is process-wide because the Go runtime
 * and these classes live in the same process.
 */
public final class VpnBridge {
    public static final int IDLE = 0;
    public static final int WAITING = 1;   // consent dialog showing
    public static final int RUNNING = 2;   // TUN established, fd() is valid
    public static final int DENIED = 3;    // user refused the VPN permission
    public static final int FAILED = 4;

    static volatile int state = IDLE;
    static volatile int tunFd = -1;
    static volatile OperaVpnService service;

    private VpnBridge() {}

    /** Opens the (transparent) permission activity; connect=false only asks for consent. */
    public static void request(Context ctx, boolean connect) {
        state = WAITING;
        Intent i = new Intent(ctx, VpnPermissionActivity.class);
        i.putExtra(VpnPermissionActivity.EXTRA_CONNECT, connect);
        i.addFlags(Intent.FLAG_ACTIVITY_NEW_TASK);
        if (!(ctx instanceof Activity)) {
            i.addFlags(Intent.FLAG_ACTIVITY_NO_ANIMATION);
        }
        ctx.startActivity(i);
    }

    public static int state() { return state; }

    public static int fd() { return tunFd; }

    public static void reset() {
        state = IDLE;
        tunFd = -1;
    }

    public static void stop(Context ctx) {
        OperaVpnService s = service;
        if (s != null) {
            s.shutdown();
        }
        tunFd = -1;
        state = IDLE;
    }
}
