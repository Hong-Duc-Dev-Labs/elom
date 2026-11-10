package ui

import (
	"embed"
	"log"
	"sync"

	. "github.com/egoist/mygo/ui"
)

// Elom carries its typeface: Geist and Geist Mono (SIL OFL, see
// fonts/OFL.txt) are embedded, so the window draws the same on a machine
// that has neither. Registration is once, before the first window; a
// failure is logged and the stacks fall back to the system fonts.
//
//go:embed fonts/*.ttf
var fontFiles embed.FS

var fontOnce sync.Once

func registerFonts() {
	fontOnce.Do(func() {
		for _, f := range []struct{ file, family string }{
			{"fonts/Geist-Regular.ttf", "Geist"},
			{"fonts/Geist-Medium.ttf", "Geist"},
			{"fonts/Geist-SemiBold.ttf", "Geist"},
			{"fonts/Geist-Bold.ttf", "Geist"},
			{"fonts/GeistMono-Regular.ttf", "Geist Mono"},
			{"fonts/GeistMono-Medium.ttf", "Geist Mono"},
			{"fonts/GeistMono-SemiBold.ttf", "Geist Mono"},
		} {
			data, err := fontFiles.ReadFile(f.file)
			if err != nil {
				log.Printf("font %s: %v", f.file, err)
				continue
			}
			if err := RegisterFont(data, f.family); err != nil {
				log.Printf("font %s: %v", f.file, err)
			}
		}
	})
}
