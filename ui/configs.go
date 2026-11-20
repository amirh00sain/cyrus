package ui

import (
	"fmt"
	"sync"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

	"github.com/Alexey71/opera-proxy/core"
	"github.com/Alexey71/opera-proxy/servers"
)

type serverRow struct {
	srv    servers.Server
	bg     *canvas.Rectangle
	dot    *canvas.Circle
	ping   *canvas.Text
	root   fyne.CanvasObject
	active bool
	styled bool
}

func newServerRow(srv servers.Server, onTap func()) *serverRow {
	r := &serverRow{srv: srv}
	r.bg = panel(colorCard, colorBorder)
	r.dot = canvas.NewCircle(colorTransparent)
	r.dot.StrokeWidth = 2
	r.dot.StrokeColor = colorDim
	r.ping = mono("—", 13, colorMuted, false)

	sub := srv.Region
	if srv.Host != "" {
		sub = fmt.Sprintf("%s:%d", srv.Host, srv.Port)
	}
	// Long texts are cut with "…" so a row never grows wider than the screen.
	left := container.NewBorder(nil, nil,
		pad(icon("file", colorGreen, 26), 0, 0, 0, 6), nil,
		container.NewVBox(ellipsisLabel(srv.Name, false, true, false), ellipsisLabel(sub, true, false, true)))
	right := container.NewHBox(r.ping, pad(container.NewGridWrap(fyne.NewSize(18, 18), r.dot), 0, 0, 8, 0))
	row := container.NewBorder(nil, nil, nil, container.NewCenter(right), left)

	r.root = container.NewStack(r.bg, pad(row, 6, 6, 10, 10), newTapArea(onTap))
	return r
}

func (r *serverRow) setActive(on bool) {
	if r.styled && r.active == on {
		return
	}
	r.styled = true
	r.active = on
	if on {
		r.bg.StrokeColor = colorGreen
		r.dot.FillColor = colorGreen
		r.dot.StrokeColor = colorGreen
	} else {
		r.bg.StrokeColor = colorBorder
		r.dot.FillColor = colorTransparent
		r.dot.StrokeColor = colorDim
	}
	r.bg.Refresh()
	r.dot.Refresh()
}

func (r *serverRow) setLatency(ms int) {
	if ms > 0 {
		r.ping.Text = fmt.Sprintf("%dms", ms)
		r.ping.Color = colorAmber
	} else {
		r.ping.Text = "—"
		r.ping.Color = colorMuted
	}
	r.ping.Refresh()
}

type configsView struct {
	ui     *App
	root   fyne.CanvasObject
	rows   []*serverRow
	status *widget.Label
}

func newConfigsView(ui *App) *configsView {
	v := &configsView{ui: ui}

	v.status = widget.NewLabel("Latency is a starting estimate. Tap TEST ALL to measure.")
	v.status.Wrapping = fyne.TextWrapWord
	v.status.Importance = widget.LowImportance
	testAll := mono("TEST ALL", 13, colorGreen, true)

	list := container.NewVBox()
	add := func(label string, items []servers.Server) {
		list.Add(pad(header(label), 10, 2, 4, 0))
		for _, s := range items {
			srv := s
			row := newServerRow(srv, func() {
				ui.Controller().SetServer(srv)
				ui.Go("home")
			})
			row.setLatency(srv.BaseDelay)
			v.rows = append(v.rows, row)
			list.Add(row.root)
		}
	}
	add("AUTOMATIC", servers.Automatic())
	add("LOCATIONS", servers.Catalog())

	top := container.NewBorder(nil, nil, screenTitle("CONFIGS"),
		container.NewStack(pad(tapRow(testAll), 0, 0, 8, 4), newTapArea(v.pingAll)))
	body := container.NewVBox(top, v.status, list)
	v.root = container.NewVScroll(column(pad(body, 2, 12, 12, 12)))

	ui.Controller().Model().Subscribe(func(s core.Snapshot) { uiDo(func() { v.renderSelection(s) }) })
	v.renderSelection(ui.Controller().Model().Snapshot())
	return v
}

func (v *configsView) view() fyne.CanvasObject { return v.root }

func (v *configsView) renderSelection(s core.Snapshot) {
	for _, r := range v.rows {
		r.setActive(r.srv.Name == s.Server.Name)
	}
}

func (v *configsView) pingAll() {
	v.status.SetText("Testing real delay…")
	var wg sync.WaitGroup
	sem := make(chan struct{}, 8)
	for _, r := range v.rows {
		row := r
		wg.Add(1)
		core.SafeGo(func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			d, err := v.ui.Controller().Ping(row.srv)
			ms := -1
			if err == nil {
				ms = int(d.Milliseconds())
				if ms < 1 {
					ms = 1
				}
			}
			uiDo(func() { row.setLatency(ms) })
		})
	}
	core.SafeGo(func() {
		wg.Wait()
		uiDo(func() {
			v.status.SetText("Finished testing.")
		})
	})
}
