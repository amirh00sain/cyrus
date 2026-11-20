package ui

import (
	"fmt"
	"image/color"
	"strconv"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/Alexey71/opera-proxy/core"
)

type homeView struct {
	ui   *App
	root fyne.CanvasObject

	ring      *canvas.Circle
	powerImg  *canvas.Image
	pulse     *fyne.Animation
	pulsing   bool
	statusImg *canvas.Image
	statusBg  *canvas.Circle
	statusTtl *canvas.Text
	statusMsg *widget.RichText
	modeHint  *widget.RichText
	segments  map[core.Mode]*segment

	cfgName *canvas.Text
	cfgHost *widget.Label
	pingKey *canvas.Text
	pingVal *canvas.Text
	pingAt  *canvas.Text
	down    *canvas.Text
	up      *canvas.Text
}

func newHomeView(ui *App) *homeView {
	v := &homeView{ui: ui, segments: map[core.Mode]*segment{}}

	title := mono(spaced("OPERA")+" "+spaced("VPN"), 16, colorGreen, true)
	title.Alignment = fyne.TextAlignCenter

	// --- power button -----------------------------------------------------
	v.ring = canvas.NewCircle(colorCard)
	v.ring.StrokeWidth = 2
	v.ring.StrokeColor = colorDim
	v.powerImg = icon("power", colorMuted, 64)
	powerBox := container.NewGridWrap(fyne.NewSize(168, 168),
		container.NewStack(v.ring, container.NewCenter(container.NewGridWrap(fyne.NewSize(64, 64), v.powerImg)),
			newTapArea(v.onPower)))

	// --- status card --------------------------------------------------------
	v.statusBg = canvas.NewCircle(colorIconBg)
	v.statusImg = icon("cloud-off", colorMuted, 24)
	badge := container.NewGridWrap(fyne.NewSize(48, 48),
		container.NewStack(v.statusBg, container.NewCenter(container.NewGridWrap(fyne.NewSize(26, 26), v.statusImg))))
	v.statusTtl = mono("NOT CONNECTED", 17, colorMuted, true)
	v.statusMsg = widget.NewRichText(&widget.TextSegment{
		Text: "", Style: widget.RichTextStyle{ColorName: theme.ColorNamePlaceHolder, SizeName: theme.SizeNameText},
	})
	v.statusMsg.Wrapping = fyne.TextWrapWord
	statusCard := card(container.NewBorder(nil, nil, container.NewVBox(badge), nil,
		container.NewVBox(v.statusTtl, v.statusMsg)))

	// --- routing mode -------------------------------------------------------
	var segObjs []fyne.CanvasObject
	for _, m := range core.AvailableModes() {
		mode := m
		s := newSegment(modeName(mode), func() { v.chooseMode(mode) })
		v.segments[mode] = s
		segObjs = append(segObjs, s.root)
	}
	v.modeHint = widget.NewRichText(&widget.TextSegment{
		Text: "", Style: widget.RichTextStyle{ColorName: theme.ColorNamePlaceHolder, SizeName: theme.SizeNameCaptionText},
	})
	v.modeHint.Wrapping = fyne.TextWrapWord
	modeCard := card(container.NewVBox(
		header("ROUTING MODE"),
		container.NewGridWithColumns(len(segObjs), segObjs...),
		v.modeHint,
	))

	// --- active configuration card ------------------------------------------
	v.cfgName = sans("", 16, colorForeground, true)
	v.cfgHost = ellipsisLabel("", true, false, true)
	v.pingKey = mono("Ping", 14, colorAmber, true)
	v.pingVal = mono("—", 14, colorMuted, false)
	v.pingAt = sans("Not tested", 12, colorMuted, true)
	pingBtn := sans("Ping", 16, colorGreen, true)
	pingBtn.Alignment = fyne.TextAlignCenter

	titleRow := container.NewStack(
		container.NewBorder(nil, nil,
			container.NewVBox(pad(icon("file", colorGreen, 28), 4, 0, 0, 10)),
			container.NewVBox(pad(icon("chevron", colorGreen, 22), 6, 0, 0, 0)),
			container.NewVBox(v.cfgName, v.cfgHost)),
		newTapArea(func() { ui.Go("configs") }))

	pingRow := container.NewBorder(nil, nil,
		container.NewHBox(v.pingKey, v.pingVal),
		container.NewStack(pad(tapRow(pingBtn), 0, 0, 12, 0), newTapArea(ui.ctrl.TestPing)),
		nil)

	divider := canvas.NewRectangle(colorBorder)
	divider.SetMinSize(fyne.NewSize(1, 1))

	downHead := mono("DOWNLINK", 12, colorMuted, true)
	upHead := mono("UPLINK", 12, colorMuted, true)
	v.down = mono("—", 14, colorMuted, false)
	v.up = mono("—", 14, colorMuted, false)
	downBox := container.NewVBox(container.NewCenter(downHead),
		container.NewCenter(container.NewHBox(icon("chevs-down", colorMuted, 18), v.down)))
	upBox := container.NewVBox(container.NewCenter(upHead),
		container.NewCenter(container.NewHBox(icon("chevs-up", colorMuted, 18), v.up)))

	cfgBody := container.NewVBox(
		header("ACTIVE CONFIGURATION"),
		titleRow,
		pingRow,
		v.pingAt,
		divider,
		pad(container.NewGridWithColumns(2, downBox, upBox), 6, 0, 0, 0),
	)
	cfgCard := card(cfgBody)

	body := container.NewVBox(
		pad(title, 10, 4, 0, 0),
		container.NewCenter(powerBox),
		statusCard,
		modeCard,
		cfgCard,
	)
	v.root = container.NewVScroll(column(pad(body, 2, 12, 12, 12)))

	v.pulse = fyne.NewAnimation(900*time.Millisecond, func(p float32) {
		a := uint8(90 + p*165)
		v.ring.StrokeColor = color.NRGBA{R: colorAmber.R, G: colorAmber.G, B: colorAmber.B, A: a}
		v.ring.Refresh()
	})
	v.pulse.AutoReverse = true
	v.pulse.RepeatCount = fyne.AnimationRepeatForever
	v.pulse.Curve = fyne.AnimationEaseInOut

	ui.ctrl.Model().Subscribe(func(s core.Snapshot) { uiDo(func() { v.render(s) }) })
	v.render(ui.ctrl.Model().Snapshot())
	return v
}

func (v *homeView) view() fyne.CanvasObject { return v.root }

func modeName(m core.Mode) string {
	switch m {
	case core.ModeTun:
		return "TUN"
	case core.ModeSystemProxy:
		return "SYSTEM"
	default:
		return "PROXY"
	}
}

func (v *homeView) modeSupported(m core.Mode) bool {
	switch m {
	case core.ModeTun:
		return v.ui.Controller().SupportsTun()
	case core.ModeSystemProxy:
		return v.ui.Controller().SupportsSystemProxy()
	}
	return true
}

func (v *homeView) chooseMode(m core.Mode) {
	if !v.modeSupported(m) {
		dialog.ShowInformation(modeName(m), unsupportedText(m), v.ui.Window())
		return
	}
	v.ui.Controller().SetMode(m)
}

func unsupportedText(m core.Mode) string {
	if m == core.ModeTun {
		return "TUN needs administrator rights. On Linux install polkit (pkexec) with an authentication agent, " +
			"or start the app with sudo."
	}
	return "The system proxy cannot be changed on this platform."
}

func modeDescription(m core.Mode) string {
	switch m {
	case core.ModeTun:
		return "TUN: every app is routed through the VPN. Asks for administrator permission on desktop and for VPN permission on Android."
	case core.ModeSystemProxy:
		return "System: sets the operating-system proxy for browsers and apps that honour it."
	default:
		return "Proxy: only exposes a local proxy on 127.0.0.1:18080 for apps you configure."
	}
}

func (v *homeView) onPower() {
	core.SafeGo(func() {
		if err := v.ui.Controller().Toggle(); err != nil {
			uiDo(func() { dialog.ShowError(err, v.ui.Window()) })
		}
	})
}

func (v *homeView) setPulse(on bool) {
	if on == v.pulsing {
		return
	}
	v.pulsing = on
	if on {
		v.pulse.Start()
	} else {
		v.pulse.Stop()
	}
}

func setText(rt *widget.RichText, s string) {
	if len(rt.Segments) > 0 {
		if seg, ok := rt.Segments[0].(*widget.TextSegment); ok {
			if seg.Text == s {
				return
			}
			seg.Text = s
		}
	}
	rt.Refresh()
}

func (v *homeView) render(s core.Snapshot) {
	var ring, glyph, accent color.Color
	var ringFill color.Color = colorCard
	title, msg, statusIcon := "NOT CONNECTED", "Choose a configuration, then tap the power button to connect.", "cloud-off"
	switch s.Status {
	case core.StatusConnecting:
		ring, glyph, accent, statusIcon = colorAmber, colorAmber, colorAmber, "loader"
		title = "CONNECTING"
		msg = "Establishing the tunnel…"
		switch s.Mode {
		case core.ModeTun:
			msg = "Establishing the tunnel… approve the permission prompt if one appears."
		}
	case core.StatusConnected:
		ring, glyph, accent, statusIcon = colorGreen, colorGreen, colorGreen, "shield"
		ringFill = colorGreenFill
		title = "CONNECTED"
		msg = s.ConnectedTo
		if s.ConnectedIP != "" {
			msg += " · " + s.ConnectedIP
		}
	case core.StatusError:
		ring, glyph, accent, statusIcon = colorRed, colorRed, colorRed, "alert"
		title = "ERROR"
		msg = s.ErrorMessage
	default:
		ring, glyph, accent = colorDim, colorMuted, colorMuted
	}

	v.setPulse(s.Status == core.StatusConnecting)
	if s.Status != core.StatusConnecting {
		v.ring.StrokeColor = ring
	}
	v.ring.FillColor = ringFill
	v.ring.Refresh()
	setIcon(v.powerImg, "power", glyph)

	setIcon(v.statusImg, statusIcon, accent)
	setCanvasText(v.statusTtl, title, accent)
	setText(v.statusMsg, msg)

	for m, seg := range v.segments {
		seg.set(m == s.Mode, v.modeSupported(m))
	}
	setText(v.modeHint, modeDescription(s.Mode))

	// active configuration
	setCanvasText(v.cfgName, s.Server.Name, colorForeground)
	host := s.Server.Region
	if s.Server.Host != "" {
		host = fmt.Sprintf("%s:%s", s.Server.Host, strconv.Itoa(s.Server.Port))
	}
	if v.cfgHost.Text != host {
		v.cfgHost.SetText(host)
	}
	if s.PingMS > 0 {
		setCanvasText(v.pingVal, fmt.Sprintf(" %dms", s.PingMS), colorForeground)
	} else {
		setCanvasText(v.pingVal, " —", colorMuted)
	}
	if s.PingAt.IsZero() {
		setCanvasText(v.pingAt, "Not tested", colorMuted)
	} else {
		setCanvasText(v.pingAt, "Tested "+s.PingAt.Format("01-02 15:04"), colorMuted)
	}
	setCanvasText(v.down, s.Download, colorMuted)
	setCanvasText(v.up, s.Upload, colorMuted)
}
