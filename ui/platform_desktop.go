//go:build !android

package ui

import "fyne.io/fyne/v2"

// platformWindowSetup gives the desktop window the proportions of a phone
// screen; it can still be resized freely.
func platformWindowSetup(w fyne.Window) {
	w.Resize(fyne.NewSize(440, 860))
	w.CenterOnScreen()
}

// The VPN permission button only exists on Android.
const hasVPNPermission = false

func requestVPNPermission() error { return nil }
