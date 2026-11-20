package ui

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
)

// maxContentWidth keeps the phone-style column readable on wide desktop windows.
const maxContentWidth = 520

// minContentWidth is the narrowest column the layout will ask the window for.
const minContentWidth = 240

// minTapHeight is the minimum height of every tappable control (Android's
// recommended 48dp touch target, minus the surrounding padding).
const minTapHeight = 44

// maxWidthLayout centres its children in a column of at most `max` width.
type maxWidthLayout struct{ max float32 }

// MinSize reports a small fixed minimum width on purpose: the column always
// takes exactly the width of the screen (or maxContentWidth on wide windows)
// and never forces a horizontal scroll on a narrow Android phone.
func (l maxWidthLayout) MinSize(objs []fyne.CanvasObject) fyne.Size {
	s := fyne.NewSize(minContentWidth, 0)
	for _, o := range objs {
		if h := o.MinSize().Height; h > s.Height {
			s.Height = h
		}
	}
	return s
}

func (l maxWidthLayout) Layout(objs []fyne.CanvasObject, size fyne.Size) {
	w := size.Width
	if w > l.max {
		w = l.max
	}
	x := (size.Width - w) / 2
	for _, o := range objs {
		o.Resize(fyne.NewSize(w, size.Height))
		o.Move(fyne.NewPos(x, 0))
	}
}

func column(obj fyne.CanvasObject) fyne.CanvasObject {
	return container.New(maxWidthLayout{max: maxContentWidth}, obj)
}

func pad(obj fyne.CanvasObject, top, bottom, left, right float32) fyne.CanvasObject {
	return container.New(layout.NewCustomPaddedLayout(top, bottom, left, right), obj)
}

// tapArea is an invisible widget that turns whatever it is stacked on into a button.
type tapArea struct {
	widget.BaseWidget
	onTap func()
}

func newTapArea(fn func()) *tapArea {
	t := &tapArea{onTap: fn}
	t.ExtendBaseWidget(t)
	return t
}

func (t *tapArea) Tapped(*fyne.PointEvent) {
	if t.onTap != nil {
		t.onTap()
	}
}

func (t *tapArea) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(canvas.NewRectangle(colorTransparent))
}

func mono(text string, size float32, c color.Color, bold bool) *canvas.Text {
	t := canvas.NewText(text, c)
	t.TextSize = size
	t.TextStyle = fyne.TextStyle{Monospace: true, Bold: bold}
	return t
}

func sans(text string, size float32, c color.Color, bold bool) *canvas.Text {
	t := canvas.NewText(text, c)
	t.TextSize = size
	t.TextStyle = fyne.TextStyle{Bold: bold}
	return t
}

func icon(name string, c color.Color, size float32) *canvas.Image {
	img := canvas.NewImageFromResource(svgIcon(name, c))
	img.FillMode = canvas.ImageFillContain
	img.SetMinSize(fyne.NewSize(size, size))
	return img
}

// setIcon recolours an icon. It does nothing when the icon is already shown:
// re-creating and re-rasterising an SVG on every state update was one of the
// main sources of UI load.
func setIcon(img *canvas.Image, name string, c color.Color) {
	res := svgIcon(name, c)
	if img.Resource != nil && img.Resource.Name() == res.Name() {
		return
	}
	img.Resource = res
	img.Refresh()
}

// setCanvasText updates a canvas.Text and only refreshes it on a real change.
func setCanvasText(t *canvas.Text, s string, c color.Color) {
	if t.Text == s && t.Color == c {
		return
	}
	t.Text = s
	t.Color = c
	t.Refresh()
}

// ellipsisLabel is a single-line label that is cut with "…" instead of
// widening its parent when the text does not fit the screen.
func ellipsisLabel(text string, monospace, bold, muted bool) *widget.Label {
	l := widget.NewLabel(text)
	l.Truncation = fyne.TextTruncateEllipsis
	l.TextStyle = fyne.TextStyle{Monospace: monospace, Bold: bold}
	if muted {
		l.Importance = widget.LowImportance
	}
	return l
}

// tapRow gives a tappable row a comfortable minimum touch height.
func tapRow(content fyne.CanvasObject) fyne.CanvasObject {
	spacer := canvas.NewRectangle(colorTransparent)
	spacer.SetMinSize(fyne.NewSize(1, minTapHeight))
	return container.NewStack(spacer, container.NewCenter(content))
}

// panel draws the rounded, hairline-bordered surface used by every card.
func panel(fill, stroke color.Color) *canvas.Rectangle {
	r := canvas.NewRectangle(fill)
	r.CornerRadius = 16
	r.StrokeColor = stroke
	r.StrokeWidth = 1
	return r
}

func card(content fyne.CanvasObject) fyne.CanvasObject {
	return container.NewStack(panel(colorCard, colorBorder), pad(content, 16, 16, 16, 16))
}

// header is the small caps label above a block ("ACTIVE CONFIGURATION").
func header(text string) *canvas.Text {
	return mono(text, 12, colorMuted, true)
}

// spaced letter-spaces a short title the way the reference does.
func spaced(s string) string {
	out := make([]rune, 0, len(s)*2)
	for i, r := range []rune(s) {
		if i > 0 {
			out = append(out, ' ')
		}
		out = append(out, r)
	}
	return string(out)
}

func screenTitle(text string) fyne.CanvasObject {
	return pad(mono(spaced(text), 18, colorGreen, true), 14, 6, 4, 4)
}

// navButton is one entry of the bottom navigation bar.
type navButton struct {
	icon  string
	pill  *canvas.Rectangle
	img   *canvas.Image
	label *canvas.Text
	root  fyne.CanvasObject
}

func newNavButton(iconName, label string, onTap func()) *navButton {
	b := &navButton{icon: iconName}
	b.pill = canvas.NewRectangle(colorTransparent)
	b.pill.CornerRadius = 16
	b.img = icon(iconName, colorMuted, 22)
	b.label = mono(label, 12, colorMuted, true)
	b.label.Alignment = fyne.TextAlignCenter
	pillBox := container.NewStack(b.pill, pad(b.img, 5, 5, 20, 20))
	col := container.NewVBox(container.NewCenter(pillBox), b.label)
	b.root = container.NewStack(pad(col, 8, 6, 0, 0), newTapArea(onTap))
	return b
}

func (b *navButton) setSelected(sel bool) {
	if sel {
		b.pill.FillColor = colorPill
		b.label.Color = colorCyan
		setIcon(b.img, b.icon, colorCyan)
	} else {
		b.pill.FillColor = colorTransparent
		b.label.Color = colorMuted
		setIcon(b.img, b.icon, colorMuted)
	}
	b.pill.Refresh()
	b.label.Refresh()
}

// segmented is a row of pill-shaped options (used for the routing mode).
type segment struct {
	bg    *canvas.Rectangle
	label *canvas.Text
	root  fyne.CanvasObject
}

func newSegment(text string, onTap func()) *segment {
	s := &segment{bg: panel(colorTransparent, colorBorder), label: mono(text, 12, colorMuted, true)}
	s.bg.CornerRadius = 12
	s.label.Alignment = fyne.TextAlignCenter
	s.root = container.NewStack(s.bg, pad(container.NewCenter(s.label), 10, 10, 6, 6), newTapArea(onTap))
	return s
}

func (s *segment) set(selected, enabled bool) {
	switch {
	case selected:
		s.bg.FillColor = colorPill
		s.bg.StrokeColor = colorPill
		s.label.Color = colorCyan
	case enabled:
		s.bg.FillColor = colorTransparent
		s.bg.StrokeColor = colorBorder
		s.label.Color = colorMuted
	default:
		s.bg.FillColor = colorTransparent
		s.bg.StrokeColor = colorBorder
		s.label.Color = colorDim
	}
	s.bg.Refresh()
	s.label.Refresh()
}
