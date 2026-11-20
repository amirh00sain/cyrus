package ui

import (
	"fmt"
	"runtime"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"

	"github.com/Alexey71/opera-proxy/core"
)

const appVersion = "1.0.0"

// prefBypassIR is the preference key of the ".ir sites bypass the VPN" switch.
const prefBypassIR = "bypass_ir"

// App is the single-column application: one screen at a time with a bottom
// navigation bar (Home, Configs, Logs, More) on every platform.
type App struct {
	app     fyne.App
	window  fyne.Window
	ctrl    *core.Controller
	nav     map[string]*navButton
	content *fyne.Container

	home    *homeView
	configs *configsView
	logs    *logsView
	more    *moreView

	current string
}

// New creates the application UI.
func New(a fyne.App, ctrl *core.Controller) *App {
	ui := &App{app: a, ctrl: ctrl, nav: make(map[string]*navButton)}
	ui.window = a.NewWindow("Opera VPN")
	platformWindowSetup(ui.window)

	ctrl.SetBypassIR(a.Preferences().BoolWithFallback(prefBypassIR, true))

	ui.home = newHomeView(ui)
	ui.configs = newConfigsView(ui)
	ui.logs = newLogsView(ui)
	ui.more = newMoreView(ui)

	ui.content = container.NewStack()
	bg := canvas.NewRectangle(colorBackground)
	ui.window.SetContent(container.NewStack(bg,
		container.NewBorder(nil, ui.buildNav(), nil, nil, ui.content)))

	ui.setCurrent("home")
	ui.window.SetOnClosed(func() { _ = ctrl.Disconnect() })
	return ui
}

func (ui *App) buildNav() fyne.CanvasObject {
	entries := []struct{ id, icon, label string }{
		{"home", "home", "HOME"},
		{"configs", "cloud", "CONFIGS"},
		{"logs", "info", "LOGS"},
		{"more", "more", "MORE"},
	}
	var objs []fyne.CanvasObject
	for _, e := range entries {
		id := e.id
		b := newNavButton(e.icon, e.label, func() { ui.setCurrent(id) })
		ui.nav[id] = b
		objs = append(objs, b.root)
	}
	line := canvas.NewRectangle(colorBorder)
	line.SetMinSize(fyne.NewSize(1, 1))
	bar := container.NewStack(canvas.NewRectangle(colorNav),
		pad(column(container.NewGridWithColumns(len(objs), objs...)), 0, 6, 8, 8))
	return container.NewBorder(line, nil, nil, nil, bar)
}

func (ui *App) setCurrent(id string) {
	if ui.current == id {
		return
	}
	ui.current = id
	ui.logs.setShown(id == "logs")
	for key, b := range ui.nav {
		b.setSelected(key == id)
	}
	var view fyne.CanvasObject
	switch id {
	case "configs":
		view = ui.configs.view()
	case "logs":
		view = ui.logs.view()
	case "more":
		view = ui.more.view()
	default:
		view = ui.home.view()
	}
	ui.content.Objects = []fyne.CanvasObject{view}
	ui.content.Refresh()
}

// Show presents the window and starts the event loop.
func (ui *App) Show() { ui.window.ShowAndRun() }

// Controller exposes the shared controller to sub-views.
func (ui *App) Controller() *core.Controller { return ui.ctrl }

// Window exposes the window to sub-views (dialogs).
func (ui *App) Window() fyne.Window { return ui.window }

// Go navigates to a screen by id.
func (ui *App) Go(id string) { ui.setCurrent(id) }

func runtimeInfo() string { return fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH) }
