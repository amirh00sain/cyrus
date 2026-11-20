package ui

import (
	"net/url"
	"os"
	"runtime"
	"strconv"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

	"github.com/Alexey71/opera-proxy/core"
	"github.com/Alexey71/opera-proxy/servers"
)

type moreView struct{ root fyne.CanvasObject }

func kv(key, value string) fyne.CanvasObject {
	val := ellipsisLabel(value, true, true, false)
	val.Alignment = fyne.TextAlignTrailing
	return container.NewBorder(nil, nil, mono(key, 13, colorMuted, false), nil, val)
}

func note(text string) fyne.CanvasObject {
	rt := widget.NewRichTextWithText(text)
	rt.Wrapping = fyne.TextWrapWord
	return rt
}

func isHyprland() bool {
	return strings.Contains(strings.ToLower(os.Getenv("XDG_CURRENT_DESKTOP")), "hyprland") ||
		os.Getenv("HYPRLAND_INSTANCE_SIGNATURE") != ""
}

func routingNotes(c *core.Controller) string {
	mark := func(ok bool) string {
		if ok {
			return "✓ "
		}
		return "✗ "
	}
	var b strings.Builder
	switch runtime.GOOS {
	case "android":
		b.WriteString("Proxy: local proxy on 127.0.0.1:18080.\n")
		b.WriteString(mark(c.SupportsTun()) + "TUN: Android VpnService; the system asks for VPN permission once. " +
			"This app itself is excluded from the tunnel.")
	case "windows":
		b.WriteString(mark(c.SupportsSystemProxy()) + "System: Windows Internet Settings (browsers, most apps).\n")
		b.WriteString(mark(c.SupportsTun()) + "TUN: Wintun adapter; a UAC prompt appears on every connect.")
	default:
		b.WriteString(mark(c.SupportsSystemProxy()) + "System: GNOME/KDE settings plus proxy environment variables.\n")
		if isHyprland() {
			b.WriteString("Hyprland: running apps keep their old environment, only newly started ones use the proxy. " +
				"Use TUN to cover everything.\n")
		}
		b.WriteString(mark(c.SupportsTun()) + "TUN: needs polkit (pkexec) and an authentication agent" +
			" (Hyprland: hyprpolkitagent); a prompt appears on every connect.")
	}
	return b.String()
}

func newMoreView(ui *App) *moreView {
	v := &moreView{}
	c := ui.Controller()

	routing := card(container.NewVBox(header("ROUTING SUPPORT"), note(routingNotes(c))))

	bypass := widget.NewCheck("Connect .ir sites directly", nil)
	bypass.SetChecked(c.BypassIR())
	bypass.OnChanged = func(on bool) {
		c.SetBypassIR(on)
		ui.app.Preferences().SetBool(prefBypassIR, on)
	}
	iran := card(container.NewVBox(
		header("IRAN"),
		bypass,
		note("Websites under .ir skip the VPN, so they see your normal connection. "+
			"On Windows and Linux TUN, Iranian IP ranges skip the tunnel as well. Applies on the next connect."),
	))

	var permission fyne.CanvasObject
	if hasVPNPermission {
		btn := mono("GRANT VPN PERMISSION", 13, colorGreen, true)
		btn.Alignment = fyne.TextAlignCenter
		permission = card(container.NewVBox(
			header("ANDROID VPN"),
			note("TUN mode needs Android's VPN permission. It is also requested automatically on the first connect."),
			container.NewStack(tapRow(btn), newTapArea(func() {
				core.SafeGo(func() {
					if err := requestVPNPermission(); err != nil {
						uiDo(func() { dialog.ShowError(err, ui.Window()) })
					}
				})
			})),
		))
	}

	count := 0
	if rows, err := servers.ParseDiscoveredRows(); err == nil {
		count = len(rows)
	}
	gh, _ := url.Parse("https://github.com/amirh00sain")
	about := card(container.NewVBox(
		header("ABOUT"),
		kv("Version", appVersion),
		kv("Platform", runtimeInfo()),
		kv("Endpoints", strconv.Itoa(count)),
		kv("Engine", "opera-proxy (in-process)"),
		container.NewCenter(widget.NewHyperlink("github.com/amirh00sain", gh)),
	))

	items := []fyne.CanvasObject{screenTitle("MORE"), iran, routing}
	if permission != nil {
		items = append(items, permission)
	}
	items = append(items, about)
	v.root = container.NewVScroll(column(pad(container.NewVBox(items...), 2, 12, 12, 12)))
	return v
}

func (v *moreView) view() fyne.CanvasObject { return v.root }
