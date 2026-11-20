package core

import "fmt"

// SystemProxy sets or restores the OS-wide HTTP(S) proxy. The concrete
// implementation is selected by build tags (Windows registry + WinINET,
// Linux desktop settings + session environment, no-op elsewhere).
type SystemProxy struct {
	address string
	active  bool
	// BypassIR keeps .ir hosts out of the proxy.
	BypassIR bool
}

// sysProxyLog receives human readable notes about what was (not) configured.
// The controller points it at the Logs tab.
var sysProxyLog = func(string) {}

func logProxy(format string, a ...any) { sysProxyLog(fmt.Sprintf("system-proxy: "+format, a...)) }

// NewSystemProxy creates a helper for the given host:port address.
func NewSystemProxy(address string) *SystemProxy {
	return &SystemProxy{address: address}
}

// Active reports whether this process currently owns the system proxy.
func (s *SystemProxy) Active() bool { return s.active }

// Enabled reports whether the platform supports changing the system proxy.
func (s *SystemProxy) Enabled() bool { return platformSupportsSystemProxy() }

// Enable applies the system proxy.
func (s *SystemProxy) Enable() error {
	if err := enableSystemProxy(s.address, s.BypassIR); err != nil {
		return err
	}
	s.active = true
	return nil
}

// Disable restores the previous system proxy configuration.
func (s *SystemProxy) Disable() error {
	if !s.active {
		return nil
	}
	err := disableSystemProxy()
	if err == nil {
		s.active = false
	}
	return err
}

// RecoverSystemProxy undoes a system proxy left behind by a crashed or killed
// previous run. It is a no-op when nothing was left behind.
func RecoverSystemProxy() { recoverSystemProxy() }
