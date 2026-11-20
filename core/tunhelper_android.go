//go:build android

package core

// MaybeRunHelper is a no-op on Android (TUN goes through VpnService).
func MaybeRunHelper() {}
