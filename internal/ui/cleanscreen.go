package ui

import (
	"time"

	"github.com/egoist/mygo"
	. "github.com/egoist/mygo/ui"

	"elom/locales"
)

// showCleanScreen opens a transparent-to-input solid cover over the display
// for wiping it. Escape closes it, and so does a click after two seconds so a
// touch on the screen cannot dismiss it by accident.
func (a *App) showCleanScreen() {
	if a.cleanScreen != nil {
		a.cleanScreen.Show()
		a.cleanOpenedAt = time.Now()
		return
	}
	a.cleanOpenedAt = time.Now()
	a.cleanScreen = mygo.NewWindow(mygo.WindowOptions{
		Title:          "Elom",
		Frameless:      true,
		FullScreen:     true,
		AlwaysOnTop:    true,
		DisableResize:  true,
		DisableMove:    true,
		BackgroundColor: "#0a0c10",
		Content:        View(a.cleanScreenView),
	})
}

// cleanScreenView draws the cover and its one instruction.
func (a *App) cleanScreenView(c *Context) {
	t := c.Theme()
	c.OnWindowShortcut(0, KeyEscape, func() {
		if a.cleanScreen != nil {
			a.cleanScreen.Hide()
		}
	})
	cover := Box(c).Fill().Background(t.Background)
	if time.Since(a.cleanOpenedAt) > 2*time.Second {
		cover = Box(c).Fill().Background(t.Background).OnClick(func() {
			if a.cleanScreen != nil {
				a.cleanScreen.Hide()
			}
		})
	}
	cover.Children(func() {
		Box(c).Fill().Center().Children(func() {
			Text(c, locales.CleanScreenHint()).FontSize(14).TextColor(t.TextMuted).Opacity(0.5)
		})
	})
}
