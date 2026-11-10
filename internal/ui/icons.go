package ui

import . "github.com/egoist/mygo/ui"

// Line icons drawn in the text color, 24-unit grid, 2-unit strokes: the
// quiet glyphs Mole puts beside labels and on its icon buttons.

func lineIcon(body string) *SVG {
	return MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">` + body + `</svg>`))
}

var (
	iconSettings = lineIcon(`<circle cx="12" cy="12" r="3"/><path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 1 1-2.83 2.83l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 1 1-4 0v-.09a1.65 1.65 0 0 0-1.08-1.51 1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 1 1-2.83-2.83l.06-.06a1.65 1.65 0 0 0 .33-1.82 1.65 1.65 0 0 0-1.51-1H3a2 2 0 1 1 0-4h.09a1.65 1.65 0 0 0 1.51-1.08 1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 1 1 2.83-2.83l.06.06a1.65 1.65 0 0 0 1.82.33H9a1.65 1.65 0 0 0 1-1.51V3a2 2 0 1 1 4 0v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 1 1 2.83 2.83l-.06.06a1.65 1.65 0 0 0-.33 1.82V9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 1 1 0 4h-.09a1.65 1.65 0 0 0-1.51 1z"/>`)
	iconRefresh  = lineIcon(`<path d="M21 12a9 9 0 1 1-2.64-6.36"/><path d="M21 3v6h-6"/>`)
	iconFolder   = lineIcon(`<path d="M4 20h16a2 2 0 0 0 2-2V8a2 2 0 0 0-2-2h-7.9a2 2 0 0 1-1.69-.9l-.81-1.2A2 2 0 0 0 7.93 3H4a2 2 0 0 0-2 2v13a2 2 0 0 0 2 2z"/>`)
	iconFile     = lineIcon(`<path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"/><path d="M14 2v6h6"/>`)
	iconChevron  = lineIcon(`<path d="m9 18 6-6-6-6"/>`)
	iconBack     = lineIcon(`<path d="m15 18-6-6 6-6"/>`)
	iconSearch   = lineIcon(`<circle cx="11" cy="11" r="7"/><path d="m20 20-3.5-3.5"/>`)
	iconHeart    = lineIcon(`<path d="M12 21s-7.5-4.6-9.5-9.2C1.2 8.6 3.2 5 6.6 5c2 0 3.4 1 4.4 2.5C12 6 13.4 5 15.4 5c3.4 0 5.4 3.6 4.1 6.8C19.5 16.4 12 21 12 21z"/>`)
	iconCPU      = lineIcon(`<rect x="5" y="5" width="14" height="14" rx="2"/><rect x="9" y="9" width="6" height="6" rx="1"/><path d="M9 2v3M15 2v3M9 19v3M15 19v3M2 9h3M2 15h3M19 9h3M19 15h3"/>`)
	iconMemory   = lineIcon(`<rect x="2" y="7" width="20" height="10" rx="2"/><path d="M6 11v2M10 11v2M14 11v2M18 11v2"/>`)
	iconDrive    = lineIcon(`<path d="M22 12H2"/><path d="M5.45 5.11 2 12v6a2 2 0 0 0 2 2h16a2 2 0 0 0 2-2v-6l-3.45-6.89A2 2 0 0 0 16.76 4H7.24a2 2 0 0 0-1.79 1.11z"/><path d="M6 16h.01M10 16h.01"/>`)
	iconNetwork  = lineIcon(`<circle cx="12" cy="12" r="10"/><path d="M2 12h20"/><path d="M12 2a15 15 0 0 1 4 10 15 15 0 0 1-4 10 15 15 0 0 1-4-10 15 15 0 0 1 4-10z"/>`)
	iconActivity = lineIcon(`<path d="M22 12h-4l-3 9L9 3l-3 9H2"/>`)
	iconBattery  = lineIcon(`<rect x="2" y="7" width="17" height="10" rx="2"/><path d="M22 11v2"/><path d="M6 10v4M10 10v4"/>`)
	iconSun      = lineIcon(`<circle cx="12" cy="12" r="4"/><path d="M12 2v2M12 20v2M4.93 4.93l1.41 1.41M17.66 17.66l1.41 1.41M2 12h2M20 12h2M6.34 17.66l-1.41 1.41M19.07 4.93l-1.41 1.41"/>`)
	iconMore     = lineIcon(`<circle cx="5" cy="12" r="1"/><circle cx="12" cy="12" r="1"/><circle cx="19" cy="12" r="1"/>`)
	iconClose    = lineIcon(`<path d="M6 6l12 12M18 6 6 18"/>`)
	iconStartup  = lineIcon(`<path d="M12 3v9"/><path d="M6.6 7a8 8 0 1 0 10.8 0"/>`)
	iconCheck    = lineIcon(`<path d="M20 6 9 17l-5-5"/>`)
)

// iconButton is a round, quiet button showing only an icon; label names it
// for the screen reader and for tests.
func iconButton(c *Context, icon *SVG, label string) Element {
	b := ButtonBase(c).Size(30, 30).Radius(999).AlignItems(Center).Justify(Center).Label(label)
	if b.Hovered() {
		b.Background(RGBA(255, 255, 255, 0.10))
	}
	b.Children(func() {
		Icon(c, icon).FontSize(15).TextColor(softText())
	})
	return b
}
