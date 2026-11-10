package main

import (
	_ "embed"
	"fmt"
	"log"
	"os"

	"github.com/egoist/mygo"
	. "github.com/egoist/mygo/ui"

	"elom/internal/core/maint"
	"elom/internal/core/oplog"
	"elom/internal/core/settings"
	"elom/internal/ui"
)

//go:embed resources/tray.png
var trayPNG []byte

type mainApp struct {
	app      *ui.App
	win      *mygo.Window
	tray     *mygo.Tray
	flyout   *mygo.Window
	quitting bool
}

func main() {
	// The elevated invocation runs one job and exits; it never opens a window
	// and never takes the single-instance lock.
	if jobPath := elevatedJobArg(os.Args); jobPath != "" {
		if err := maint.RunJob(jobPath); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}
	if !mygo.App.RequestSingleInstanceLock() {
		return
	}
	m := &mainApp{}
	mygo.App.OnSecondInstance(func([]string, string) { m.show() })
	mygo.App.OnBeforeQuit(func(*mygo.QuitEvent) {
		m.quitting = true
		if m.app != nil {
			m.app.Stop()
		}
	})
	mygo.App.WhenReady(m.ready)
	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}

// elevatedJobArg returns the job path when the process was started by the
// elevation helper.
func elevatedJobArg(args []string) string {
	for i, arg := range args {
		if arg == "--elevated-job" && i+1 < len(args) {
			return args[i+1]
		}
	}
	return ""
}

func (m *mainApp) ready() {
	// Elom's window is dark whatever the desktop says; this also darkens the
	// title bar and the menus around the native view.
	mygo.Theme.SetSource(mygo.ThemeDark)
	dataDir, err := mygo.App.Path(mygo.PathUserData)
	if err != nil {
		log.Fatal(err)
	}
	store, err := settings.Open(dataDir)
	if err != nil {
		log.Fatal(err)
	}
	logFile, err := oplog.Open(dataDir)
	if err != nil {
		log.Fatal(err)
	}
	m.app = ui.New(dataDir, store, logFile)

	m.win = mygo.NewWindow(mygo.WindowOptions{
		Title:           "Elom",
		Width:           1180,
		Height:          760,
		MinWidth:        960,
		MinHeight:       620,
		StateKey:        "main",
		BackgroundColor: "#1c2336",
		// The view draws the title bar: the tab pill sits in it, as Mole's,
		// under Windows' own minimize, maximize and close buttons.
		TitleBarStyle: mygo.TitleBarHidden,
		Content:         View(m.app.View),
	})
	m.app.Win = m.win
	m.app.ShowMain = m.show
	m.app.StartStatus()

	m.win.OnClose(func(e *mygo.CloseEvent) {
		if m.quitting {
			return
		}
		// Closing the window keeps the app in the tray, as a system utility
		// does; the tray panel's Quit is the way out.
		e.PreventDefault()
		m.win.Hide()
	})

	tray, err := mygo.NewTray(mygo.TrayOptions{
		ID:      "main",
		Icon:    trayPNG,
		ToolTip: "Elom",
	})
	if err != nil {
		// A missing tray icon is not fatal: the window still works.
		log.Println("tray:", err)
		return
	}
	m.tray = tray

	m.flyout = mygo.NewFlyout(mygo.FlyoutOptions{
		Tray:         tray,
		Width:        360,
		Height:       600,
		Focusable:    true,
		Vibrancy:     mygo.VibrancyMenu,
		CornerRadius: 12,
		Content:      View(m.app.PanelView),
	})
	tray.OnClick(m.togglePanel)
	tray.OnRightClick(func() {
		menu := mygo.NewMenu([]*mygo.MenuItem{
			{Label: "Open Elom", Click: func(*mygo.MenuItem, *mygo.Window) { m.show() }},
			mygo.Separator(),
			{Role: mygo.RoleQuit},
		})
		tray.SetMenu(menu)
		tray.PopUpMenu()
		tray.SetMenu(nil)
	})

	if err := mygo.GlobalShortcut.Register("Ctrl+Alt+E", m.togglePanel); err != nil {
		log.Println("global shortcut:", err)
	}
}

func (m *mainApp) togglePanel() {
	if m.flyout == nil {
		return
	}
	if m.flyout.IsVisible() {
		m.flyout.Hide()
		return
	}
	m.flyout.Show()
}

func (m *mainApp) show() {
	if m.win == nil {
		return
	}
	m.win.Show()
	m.win.Focus()
}
