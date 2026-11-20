package com.opera.vpn;

import android.content.Intent;
import android.net.VpnService;
import android.os.ParcelFileDescriptor;

/**
 * Establishes the TUN interface and hands its file descriptor to Go, where
 * tun2socks turns the packets into SOCKS5 connections to the local engine.
 *
 * The app's own package is excluded from the VPN, so the engine's connections
 * to the Opera servers use the real network and never loop back into the tunnel.
 */
public class OperaVpnService extends VpnService {
    private static final int MTU = 1500;
    private ParcelFileDescriptor tun;

    @Override
    public int onStartCommand(Intent intent, int flags, int startId) {
        VpnBridge.service = this;
        try {
            Builder b = new Builder()
                    .setSession("Opera VPN")
                    .setMtu(MTU)
                    .addAddress("198.18.0.1", 32)
                    .addRoute("0.0.0.0", 0)
                    .addDnsServer("1.1.1.1")
                    .addDnsServer("8.8.8.8");
            try {
                // IPv6 would otherwise bypass the tunnel; it fails fast inside
                // it and apps fall back to IPv4.
                b.addAddress("fd66:6f70:6572:1::1", 128).addRoute("::", 0);
            } catch (Exception ignored) {
                // device without IPv6 support
            }
            try {
                b.addDisallowedApplication(getPackageName());
            } catch (Exception e) {
                VpnBridge.state = VpnBridge.FAILED;
                stopSelf();
                return START_NOT_STICKY;
            }
            tun = b.establish();
            if (tun == null) {
                // consent was revoked between prepare() and now
                VpnBridge.state = VpnBridge.DENIED;
                stopSelf();
                return START_NOT_STICKY;
            }
            // Go takes over (and closes) the descriptor.
            VpnBridge.tunFd = tun.detachFd();
            tun = null;
            VpnBridge.state = VpnBridge.RUNNING;
        } catch (Exception e) {
            VpnBridge.state = VpnBridge.FAILED;
            stopSelf();
        }
        return START_NOT_STICKY;
    }

    void shutdown() {
        stopSelf();
    }

    @Override
    public void onRevoke() {
        // The user turned the VPN off from the system UI or another VPN took over.
        VpnBridge.state = VpnBridge.IDLE;
        VpnBridge.tunFd = -1;
        super.onRevoke();
    }

    @Override
    public void onDestroy() {
        if (VpnBridge.service == this) {
            VpnBridge.service = null;
        }
        if (tun != null) {
            try { tun.close(); } catch (Exception ignored) { }
            tun = null;
        }
        super.onDestroy();
    }
}
