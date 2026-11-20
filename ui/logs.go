package ui

import (
	"strings"
	"sync/atomic"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

const (
	// logLinesShown is how many of the newest lines are drawn. The buffer
	// itself keeps more, but laying out hundreds of wrapped lines is slow.
	logLinesShown = 150
	// logRefreshEvery coalesces bursts of log lines (every proxied request
	// writes one) into at most a few redraws per second.
	logRefreshEvery = 500 * time.Millisecond
)

type logsView struct {
	root    fyne.CanvasObject
	text    *widget.Label
	scroll  *container.Scroll
	buf     interface{ Lines() []string }
	shown   bool  // only touched on the UI thread
	pending int32 // 1 while a redraw is already scheduled
}

func newLogsView(ui *App) *logsView {
	v := &logsView{}

	// A wrapping monospace label inside a vertical-only scroll: long log
	// lines wrap instead of being cut off at the screen edge.
	v.text = widget.NewLabel("")
	v.text.Wrapping = fyne.TextWrapBreak
	v.text.TextStyle = fyne.TextStyle{Monospace: true}
	v.scroll = container.NewVScroll(v.text)

	clear := mono("CLEAR", 13, colorGreen, true)
	clearTap := container.NewStack(pad(tapRow(clear), 0, 0, 8, 4), newTapArea(func() {
		ui.Controller().Model().Log().Clear()
		v.text.SetText("")
	}))
	top := container.NewBorder(nil, nil, screenTitle("LOGS"), clearTap)

	v.root = column(container.NewBorder(top, nil, nil, nil, pad(v.scroll, 0, 8, 12, 12)))

	buffer := ui.Controller().Model().Log()
	v.buf = buffer
	buffer.OnChange(func() {
		// Nothing to draw while another tab is open; the tab redraws itself
		// when it is shown again.
		if atomic.CompareAndSwapInt32(&v.pending, 0, 1) {
			time.AfterFunc(logRefreshEvery, func() {
				atomic.StoreInt32(&v.pending, 0)
				uiDo(v.render)
			})
		}
	})
	return v
}

// setShown tells the view whether its tab is on screen (UI thread).
func (v *logsView) setShown(on bool) {
	v.shown = on
	if on {
		v.render()
	}
}

func (v *logsView) render() {
	if !v.shown {
		return
	}
	lines := v.buf.Lines()
	if len(lines) > logLinesShown {
		lines = lines[len(lines)-logLinesShown:]
	}
	v.text.SetText(strings.Join(lines, "\n"))
	v.scroll.ScrollToBottom()
}

func (v *logsView) view() fyne.CanvasObject { return v.root }
