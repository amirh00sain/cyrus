package core

// TunAdapter manages the TUN/VPN interface used by TUN mode.
//
//	desktop (Linux, Windows): a short-lived elevated helper process of this very
//	  executable creates the TUN device and the routes (tun_desktop.go), so the
//	  GUI itself never has to run as root/Administrator.
//	Android: VpnService (Java, android/java) hands a TUN file descriptor to Go
//	  after the system's VPN consent dialog (tun_android.go).
//
// In both cases tun2socks turns the packets into SOCKS5 requests for the local
// engine, which runs in SOCKS mode while TUN is selected.
type TunAdapter struct {
	address string
	active  bool
	// DirectIR keeps Iranian IP ranges outside the tunnel (desktop only).
	DirectIR bool
}

// NewTunAdapter creates an adapter that routes traffic to the local proxy.
func NewTunAdapter(address string) *TunAdapter {
	return &TunAdapter{address: address}
}

// Supported reports whether TUN mode can actually be enabled here.
func (t *TunAdapter) Supported() bool { return tunSupported() }

// Active reports whether the TUN interface is up.
func (t *TunAdapter) Active() bool { return t.active }

// Start brings the TUN interface up.
func (t *TunAdapter) Start() error {
	if err := startTun(t.address, t.DirectIR); err != nil {
		return err
	}
	t.active = true
	return nil
}

// Stop tears the TUN interface down.
func (t *TunAdapter) Stop() error {
	if !t.active {
		return nil
	}
	err := stopTun()
	t.active = false
	return err
}
