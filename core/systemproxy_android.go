//go:build android

package core

// Android has no per-app system proxy: Proxy mode exposes the local proxy and
// TUN mode goes through VpnService, so System proxy is hidden in the UI.
func platformSupportsSystemProxy() bool { return false }

func enableSystemProxy(string, bool) error { return nil }

func disableSystemProxy() error { return nil }

func recoverSystemProxy() {}
