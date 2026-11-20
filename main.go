package main

import (
	"fyne.io/fyne/v2/app"

	"github.com/Alexey71/opera-proxy/core"
	"github.com/Alexey71/opera-proxy/ui"
)

func main() {
	// "<exe> --tun-helper ..." is the short-lived elevated process that owns
	// the TUN device (Linux/Windows); it never reaches the GUI code below.
	core.MaybeRunHelper()

	a := app.NewWithID("com.opera.vpn.gui")
	a.Settings().SetTheme(ui.NewAppTheme())

	controller := core.NewController(core.NewModel())
	ui.New(a, controller).Show()
}
