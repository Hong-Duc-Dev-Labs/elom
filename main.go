package main

import (
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

type mainApp struct {
	app *ui.App
	win *mygo.Window
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
	m.app.StartStatus()

	// Elom is a window, not a system utility: closing it is how the user
	// quits, and no tray icon, flyout, shortcut or sampler outlives it.
}

func (m *mainApp) show() {
	if m.win == nil {
		return
	}
	m.win.Show()
	m.win.Focus()
}
