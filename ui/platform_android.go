//go:build android

package ui

import (
	"fyne.io/fyne/v2"

	"github.com/Alexey71/opera-proxy/core"
)

func platformWindowSetup(_ fyne.Window) {}

const hasVPNPermission = true

func requestVPNPermission() error { return core.RequestVPNPermission() }
