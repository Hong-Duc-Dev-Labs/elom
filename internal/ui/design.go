package ui

import (
	"math"
	"strings"

	. "github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/chart"
)

// The window follows Mole's measured design system: base #15141b with the
// screen's tab wash over it (alpha 217 at the top, 107 from the middle down),
// cards cut from that wash and sunk toward the base, one type scale, and the
// session's planet drawn as a character globe. These values are measured from
// Mole's UI (TabWash.cs, HudTile.cs, TabWashes.cs) and are ours to use; no
// asset or glyph path is copied.

// Font stacks: the bundled Geist families first, then the system faces a
// machine without them still has (Windows 10/11 and Windows Terminal).
const (
	fontBody = "Geist, Segoe UI Variable Text, Segoe UI"
	fontMono = "Geist Mono, Cascadia Mono, Consolas"
)

// Base and accent, as Mole's Surface.cs.
var (
	bgBase     = Hex("#15141b")
	accentMint = Hex("#4fd6a9")
)

// Type scale and spacing, in DIPs.
const (
	sizeFootnote = 10.0
	sizeCaption  = 11.0
	sizeMeta     = 11.5
	sizeMicro    = 10.5
	sizeBody     = 13.5
	sizeValue    = 24.0

	radiusCard  = 12.0
	radiusRow   = 8.0
	radiusPill  = 999.0
	radiusTile  = 4.0
	padCardSide = 16.0
	padCardVert = 14.0
)

var appTheme = func() *Theme {
	t := DarkTheme()
	t.Background = bgBase
	t.Surface = RGBA(255, 255, 255, 0.055)
	t.SurfaceHover = RGBA(255, 255, 255, 0.10)
	t.SurfacePressed = RGBA(255, 255, 255, 0.14)
	t.Border = RGBA(237, 236, 238, 0.078)
	t.Text = Hex("#ededee")
	t.TextMuted = Hex("#9aa3b2")
	t.Accent = Hex("#f2eee4")
	t.AccentHover = Hex("#ffffff")
	t.AccentPressed = Hex("#ddd6c6")
	t.AccentText = Hex("#10151f")
	t.InactiveAccent = t.Accent
	t.Danger = Hex("#ff8a80")
	t.Warning = Hex("#f2c078")
	t.Success = accentMint
	t.Selection = RGBA(242, 238, 228, 0.22)
	t.Focus = RGBA(242, 238, 228, 0.55)
	t.Inverse = Hex("#f5f1e8")
	t.InverseText = Hex("#10151f")
	t.Scrollbar = RGBA(255, 255, 255, 0.22)
	t.ScrollbarWidth = 8
	t.Radius = radiusCard
	t.Spacing = 4
	t.FontSize = sizeBody
	return t
}()

// palette is one screen's share of the look: the wash that tints the window
// and its cards, the glow behind the content, and the character globe that
// stands for the screen.
type palette struct {
	wash Color
	glow Color
	tiles []Color
	// ink is the globe's three tones: faint, mid, lit.
	ink  [3]Color
	body planetKind
}

func paletteFor(id string) palette {
	switch id {
	case "software":
		return palette{
			wash: Hex("#47201d"), glow: Hex("#c0645a"),
			tiles: []Color{Hex("#8e5a4a"), Hex("#a7654a"), Hex("#7d4b3c")},
			ink:   [3]Color{Hex("#5e3a31"), Hex("#b4694f"), Hex("#f2c2a8")},
			body:  mars,
		}
	case "startup":
		return palette{
			wash: Hex("#123028"), glow: Hex("#3fae86"),
			tiles: []Color{Hex("#3d7460"), Hex("#4f8a6d"), Hex("#356b58")},
			ink:   [3]Color{Hex("#2f5a4c"), Hex("#7fb99f"), Hex("#d8f0e2")},
			body:  moon,
		}
	case "optimize":
		return palette{
			wash: Hex("#423d38"), glow: Hex("#a8a29c"),
			tiles: []Color{Hex("#6f6a63"), Hex("#7c776f"), Hex("#605c56")},
			ink:   [3]Color{Hex("#4c4c48"), Hex("#8f8f88"), Hex("#e3e3da")},
			body:  mercury,
		}
	case "analyze":
		return palette{
			wash: Hex("#5c3011"), glow: Hex("#d39a62"),
			tiles: []Color{Hex("#8e6650"), Hex("#9a6a3a"), Hex("#a7553a"), Hex("#8a5a3a"), Hex("#7e5a48"), Hex("#5e3d2a"), Hex("#8d6855"), Hex("#6e4a35")},
			ink:   [3]Color{Hex("#6a5340"), Hex("#c08e5a"), Hex("#f6dfb8")},
			body:  jupiter,
		}
	case "status":
		return palette{
			wash: Hex("#574312"), glow: Hex("#e0ae4c"),
			ink:  [3]Color{Hex("#7c4a1c"), Hex("#e0a13a"), Hex("#fff0b8")},
			body: sun,
		}
	case "settings":
		return palette{
			wash: Hex("#262c38"), glow: Hex("#5b6d8c"),
			ink:  [3]Color{Hex("#43433f"), Hex("#8f8f86"), Hex("#efece1")},
			body: moon,
		}
	default: // clean
		return palette{
			wash: Hex("#29385c"), glow: Hex("#4f7fc4"),
			tiles: []Color{Hex("#4e6f8a"), Hex("#5a7f7a"), Hex("#3f5f78")},
			ink:   [3]Color{Hex("#3f5f5a"), Hex("#8fbdb0"), Hex("#e9f7f0")},
			body:  earth,
		}
	}
}

// mixPalette blends two screens' washes for the tab cross-fade. Only the tint
// and the glow move; the rest snaps with the screen.
func mixPalette(a, b palette, t float32) palette {
	t = clampFrac(t)
	if t >= 1 {
		return b
	}
	out := b
	out.wash = a.wash.Mix(b.wash, t)
	out.glow = a.glow.Mix(b.glow, t)
	for i := range out.ink {
		out.ink[i] = a.ink[i].Mix(b.ink[i], t)
	}
	return out
}

// washAlphaAt is Mole's ramp: 217/255 at the top, 107/255 from the middle
// down, straight in between (TabWash.AlphaAt).
func washAlphaAt(f float32) float32 {
	f = clampFrac(f)
	if f >= 0.5 {
		return 107.0 / 255.0
	}
	return (217.0 - 110.0*(f/0.5)) / 255.0
}

// washOver composites the wash over bg at fraction f of the ramp.
func washOver(bg, wash Color, f float32) Color {
	a := float64(washAlphaAt(f))
	mix := func(u, o uint8) uint8 {
		return uint8(math.Round(float64(u) + float64(int(o)-int(u))*a))
	}
	return RGB(mix(bg.R, wash.R), mix(bg.G, wash.G), mix(bg.B, wash.B))
}

// sink moves c a fraction k of the way toward under, as a card sits below the
// wash it is cut from.
func sink(c, under Color, k float32) Color {
	f := func(v, u uint8) uint8 {
		return uint8(math.Round(float64(v) + float64(int(u)-int(v))*float64(k)))
	}
	return RGB(f(c.R, under.R), f(c.G, under.G), f(c.B, under.B))
}

// cardGround is the fill of every card on a screen: the wash composited over
// the base, then sunk 15% back toward the base (TabWash.CardGround).
func cardGround(wash Color) Color {
	return sink(washOver(bgBase, wash, 1), bgBase, 0.15)
}

// frameWash is the wash of the frame being built: View sets it before the
// screens, and card() reads it. The view is built on the UI thread only.
var frameWash = paletteFor("clean").wash

func frameCard() Color { return cardGround(frameWash) }

func fgPrimary() Color   { return Hex("#ededee") }
func fgSecondary() Color { return RGBA(237, 236, 238, 0.72) }
func fgMuted() Color     { return RGBA(237, 236, 238, 0.62) }
func fgFaint() Color     { return RGBA(237, 236, 238, 0.52) }
func hairline() Color    { return RGBA(237, 236, 238, 0.078) }
func faintFill() Color   { return RGBA(255, 255, 255, 0.055) }
func faintFill2() Color  { return RGBA(255, 255, 255, 0.09) }
func quietText() Color   { return fgMuted() }
func softText() Color    { return fgSecondary() }
func clampFrac(v float32) float32 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

// card is a translucent panel cut from the screen's wash: radius 12, 14/16
// padding, a 0.6 hairline border.
func card(c *Context) Element {
	return Box(c).FillWidth().
		Padding(padCardVert, padCardSide).
		Radius(radiusCard).
		Background(frameCard()).
		Border(0.6, hairline())
}

// cardWith is card with a column of children gap DIPs apart.
func cardWith(c *Context, gap float32, fn func()) Element {
	return card(c).Children(func() {
		Column(c).FillWidth().Gap(gap).Children(fn)
	})
}

// capsLabel is one voice for every small capital on the character grid:
// mono, 9.5, tracked. Screens that used to invent 10.5/600/1.0 or 9.5/600/1.1
// labels route through it so the data chrome matches the globe's cells.
func capsLabel(c *Context, label string, color Color) {
	Text(c, strings.ToUpper(label)).Font(fontMono).FontSize(9.5).FontWeight(500).
		LetterSpacing(1.1).TextColor(color).NoWrap()
}

// cardHead is a card's title row: a small colored dot, the label in tracked
// capitals, and room for a value or an action on the right.
func cardHead(c *Context, pal palette, label, meta string) {
	Row(c).FillWidth().AlignItems(Center).Gap(8).Children(func() {
		Box(c).Size(6, 6).Radius(3).Background(pal.glow)
		capsLabel(c, label, fgMuted())
		Spacer(c)
		if meta != "" {
			Text(c, meta).FontSize(sizeCaption).TextColor(fgMuted()).NoWrap()
		}
	})
}

// rule is the hairline between a card's title and its body.
func rule(c *Context) {
	Box(c).FillWidth().Height(1).Background(hairline())
}

// bigNumber is a screen's result: a large figure with its unit beside it.
func bigNumber(c *Context, value, unit string) {
	Row(c).AlignItems(End).Gap(4).Children(func() {
		// Values are data, so they take the character grid's voice: a mono
		// figure keeps every tile on the same digit rhythm as the globe.
		Text(c, value).Font(fontMono).FontSize(sizeValue).FontWeight(500).TextColor(fgPrimary()).NoWrap()
		if unit != "" {
			Text(c, unit).Font(fontMono).FontSize(12).TextColor(fgMuted()).Margin(0, 0, 3, 0)
		}
	})
}

func hintText(c *Context, s string) {
	Text(c, s).FontSize(sizeCaption).TextColor(fgMuted())
}

// primaryPill is the one action a screen offers, shaped as Mole's.
func primaryPill(c *Context, label string) Element {
	return PrimaryButton(c, label).Radius(radiusPill).Padding(9, 24)
}

// ghostPill is a secondary action beside primaryPill. Only the primary
// capsule stays fully round; a row of capsules out-shouted the 1 DIP cells
// of the globe.
func ghostPill(c *Context, label string) Element {
	return Button(c, label).Radius(radiusRow).Padding(8, 14)
}

// pillTag is a small neutral chip, as "needs administrator" on a task.
func pillTag(c *Context, label string, fg Color) Element {
	return Box(c).Padding(3, 9).Radius(radiusRow).Background(faintFill2()).
		Border(1, hairline()).Children(func() {
		Text(c, label).Font(fontMono).FontSize(sizeFootnote).TextColor(fg).NoWrap()
	})
}

// listRow is one row of a list: 44 DIPs high, 8 radius, 12 gap. It does not
// paint its hover state; the caller does when the row is interactive.
func listRow(c *Context) Element {
	return Row(c).FillWidth().Height(44).Padding(0, 8).Radius(radiusRow).
		AlignItems(Center).Gap(12)
}

// rowDivider is the hairline between two list rows, inset to line up with
// the text rather than the icons.
func rowDivider(c *Context) {
	Box(c).FillWidth().Height(1).Margin(0, 8).Background(hairline())
}

// bar is a track with a filled part, 0 to 1, rounded and tinted. The track
// is a row of 1 DIP characters rather than a solid wash, so a meter belongs
// to the same grid as the globe.
func bar(c *Context, fraction float32, color Color, height float32) Element {
	return Box(c).FillWidth().Height(height).Radius(height/2).
		Draw(func(p *Painter, r Rect) {
		for x := r.X; x < r.X+r.W; x += 4 {
			p.Fill(Rect{X: x, Y: r.Y, W: 1, H: r.H}, RGBA(255, 255, 255, 0.18), 0)
		}
		w := r.W * clampFrac(fraction)
		if w <= 0 {
			return
		}
		p.FillGradient(Rect{X: r.X, Y: r.Y, W: w, H: r.H}, LinearGradient{
			Angle: 90,
			From:  color.Alpha(0.55),
			To:    color.Alpha(0.95),
		}, r.H/2)
	})
}

// toggle is a switch whose track stays visible under the white knob.
func toggle(c *Context, on *bool, label string) Element {
	track := RGBA(255, 255, 255, 0.16)
	if *on {
		track = accentMint.Alpha(0.85)
	}
	return Switch(c, on).Background(track).Size(44, 24).Radius(12).Label(label)
}

// barsChart draws a series as a row of thin bars, as Mole's CPU reading.
func barsChart(c *Context, series []float64, maxValue float64, color Color, height float32) Element {
	return Box(c).FillWidth().Height(height).Draw(func(p *Painter, r Rect) {
		n := len(series)
		if n == 0 || maxValue <= 0 || r.W <= 0 {
			return
		}
		// The sampler keeps 60 samples, one a second: lay them out in those
		// slots and let a short series fill in from the right, so the chart
		// keeps its rhythm from the first second.
		slots := 60
		if n > slots {
			slots = n
		}
		slot := r.W / float32(slots)
		barW := min(6, max(1.5, slot-2))
		if barW <= 0 {
			return
		}
		for s := range slots {
			x := r.X + float32(s)*slot + (slot-barW)/2
			p.Fill(Rect{X: x, Y: r.Y + r.H - 2, W: barW, H: 2}, color.Alpha(0.18), 1)
		}
		start := slots - n
		for i, v := range series {
			f := clampFrac(float32(v / maxValue))
			h := r.H * (0.14 + 0.82*f)
			x := r.X + float32(start+i)*slot + (slot-barW)/2
			p.FillGradient(Rect{X: x, Y: r.Y + r.H - h, W: barW, H: h}, LinearGradient{
				Angle: 90,
				From:  color.Alpha(0.45),
				To:    color.Alpha(0.95),
			}, min(1.5, barW/2))
		}
	})
}

// perCoreBars is Mole's CPU reading: one bar per logical processor, the
// current share of its core.
func perCoreBars(c *Context, cores []float64, height float32) Element {
	return Box(c).FillWidth().Height(height).Draw(func(p *Painter, r Rect) {
		n := len(cores)
		if n == 0 || r.W <= 0 {
			return
		}
		slot := r.W / float32(n)
		w := min(9, max(2, slot-3))
		for i, v := range cores {
			f := clampFrac(float32(v / 100))
			h := max(1.5, r.H*f)
			x := r.X + float32(i)*slot + (slot-w)/2
			p.Fill(Rect{X: x, Y: r.Y + r.H - 1, W: w, H: 1}, RGBA(255, 255, 255, 0.12), 0)
			p.FillGradient(Rect{X: x, Y: r.Y + r.H - h, W: w, H: h}, LinearGradient{
				Angle: 90,
				From:  accentMint.Alpha(0.45),
				To:    accentMint.Alpha(0.95),
			}, min(1.5, w/2))
		}
	})
}

// chartField is the character-grid the vector series sit on: a baseline
// plus dotted quarter lines, so a curve has the same measured field as the
// globe's cells without turning the reading into a bar chart.
func chartField(p *Painter, r Rect) {
	if r.W <= 0 || r.H <= 0 {
		return
	}
	for _, f := range []float32{0.25, 0.5, 0.75} {
		y := r.Y + r.H*(1-f)
		for x := r.X; x < r.X+r.W; x += 8 {
			p.Fill(Rect{X: x, Y: y, W: 2, H: 1}, RGBA(255, 255, 255, 0.055), 0)
		}
	}
	p.Fill(Rect{X: r.X, Y: r.Y + r.H - 1, W: r.W, H: 1}, RGBA(255, 255, 255, 0.10), 0)
}

// areaChart draws a series as a soft filled curve, as Mole's network card.
func areaChart(c *Context, series []float64, maxValue float64, color Color, height float32) Element {
	return Box(c).FillWidth().Height(height).Draw(func(p *Painter, r Rect) {
		n := len(series)
		if n < 2 || maxValue <= 0 || r.W <= 0 {
			return
		}
		chartField(p, r)
		xs := make([]float32, n)
		ys := make([]float32, n)
		for i, v := range series {
			xs[i] = float32(i) / float32(n-1)
			ys[i] = clampFrac(float32(v / maxValue))
		}
		area := chart.Area(r, xs, nil, ys, chart.CurveMonotone)
		p.FillPathGradient(area, LinearGradient{
			Angle: 180,
			From:  color.Alpha(0.30),
			To:    color.Alpha(0.02),
		})
		p.StrokePath(chart.Line(r, xs, ys, chart.CurveMonotone), 1.4, color.Alpha(0.85))
	})
}

// netChart draws the network card's two series: what came down filled, what
// went up as a line, over one shared maximum.
func netChart(c *Context, down, up []float64, maxValue float64, height float32) Element {
	return Box(c).FillWidth().Height(height).Draw(func(p *Painter, r Rect) {
		n := max(len(down), len(up))
		if n < 2 || maxValue <= 0 || r.W <= 0 {
			chartField(p, r)
			return
		}
		chartField(p, r)
		series := func(v []float64) ([]float32, []float32) {
			xs := make([]float32, n)
			ys := make([]float32, n)
			off := n - len(v)
			for i := range xs {
				xs[i] = float32(i) / float32(n-1)
				if i >= off {
					ys[i] = clampFrac(float32(v[i-off] / maxValue))
				}
			}
			return xs, ys
		}
		dx, dy := series(down)
		ux, uy := series(up)
		p.FillPathGradient(chart.Area(r, dx, nil, dy, chart.CurveMonotone), LinearGradient{
			Angle: 180,
			From:  accentMint.Alpha(0.26),
			To:    accentMint.Alpha(0.02),
		})
		p.StrokePath(chart.Line(r, dx, dy, chart.CurveMonotone), 1.3, accentMint.Alpha(0.75))
		p.StrokePath(chart.Line(r, ux, uy, chart.CurveMonotone), 1.3, Hex("#8fb4ff").Alpha(0.75))
	})
}

// lineChart is areaChart without the fill, for a quiet series.
func lineChart(c *Context, series []float64, maxValue float64, color Color, height float32) Element {
	return Box(c).FillWidth().Height(height).Draw(func(p *Painter, r Rect) {
		n := len(series)
		if n < 2 || maxValue <= 0 || r.W <= 0 {
			return
		}
		chartField(p, r)
		xs := make([]float32, n)
		ys := make([]float32, n)
		for i, v := range series {
			xs[i] = float32(i) / float32(n-1)
			ys[i] = clampFrac(float32(v / maxValue))
		}
		p.StrokePath(chart.Line(r, xs, ys, chart.CurveMonotone), 1.4, color.Alpha(0.85))
	})
}

// readableOn picks the ink with the higher measured contrast on a filled
// tile, rather than guessing from the fill's brightness: a mid amber tile
// can sit between the two candidates.
func readableOn(fill Color) Color {
	dark, light := Hex("#1f1206"), Hex("#fffaf0")
	if contrastOf(dark, fill) >= contrastOf(light, fill) {
		return dark
	}
	return light
}

// colorLuminance is the sRGB relative luminance used for ink decisions.
func colorLuminance(c Color) float64 {
	lin := func(v uint8) float64 {
		f := float64(v) / 255
		if f <= 0.03928 {
			return f / 12.92
		}
		return math.Pow((f+0.055)/1.055, 2.4)
	}
	return 0.2126*lin(c.R) + 0.7152*lin(c.G) + 0.0722*lin(c.B)
}

func contrastOf(a, b Color) float64 {
	hi, lo := colorLuminance(a), colorLuminance(b)
	if hi < lo {
		hi, lo = lo, hi
	}
	return (hi + 0.05) / (lo + 0.05)
}

// percentOf is used by the cards that show a share of a whole.
func percentOf(part, whole float64) float64 {
	if whole <= 0 {
		return 0
	}
	return math.Min(100, part/whole*100)
}
