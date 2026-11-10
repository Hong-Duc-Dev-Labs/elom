package ui

import (
	"math"
	"sync"

	. "github.com/egoist/mygo/ui"
)

// The planets are character globes, not photographs: a grid of monospace
// cells, one glyph per cell chosen by the surface alone. The body is lit
// evenly — no terminator, no night side — so the whole disc stays readable
// while continents drift under it. The look follows the ASCII-generator idea
// (brightness ramp, edges, colour); the drawing, the bodies and the noise are
// ours.

type planetKind int

const (
	earth planetKind = iota
	mars
	mercury
	jupiter
	sun
	moon
)

// globeRamp is deliberately sparse: dense ramps turn a 64-column globe into
// a grey block. Index 0 is blank.
var globeRamp = []rune(" ·.:+*")
var sunRamp = []rune(" ·*#%@")

func (k planetKind) ramp() []rune {
	if k == sun {
		return sunRamp
	}
	return globeRamp
}

// speed is how fast the surface turns, in radians a second.
func (k planetKind) speed() float64 {
	switch k {
	case earth:
		return 0.10
	case mars:
		return 0.14
	case mercury:
		return 0.06
	case jupiter:
		return 0.18
	case sun:
		return 0.05
	default:
		return 0.08
	}
}

// globeCell is the fixed part of one cell: where it sits on the sphere. It
// does not depend on the angle.
type globeCell struct {
	u, v, z float32
	inside  bool
}

type globeKey struct{ cols, rows int }

var globeCache sync.Map // globeKey -> []globeCell

const globeTilt = 0.35

func globeGeometry(cols, rows int) []globeCell {
	key := globeKey{cols, rows}
	if v, ok := globeCache.Load(key); ok {
		return v.([]globeCell)
	}
	cells := make([]globeCell, cols*rows)
	for r := range rows {
		for c := range cols {
			u := (float32(c) + 0.5) / float32(cols) * 2 - 1
			v := (float32(r) + 0.5) / float32(rows) * 2 - 1
			rr := u*u + v*v
			cell := globeCell{u: u, v: v}
			if rr <= 1 {
				z := float32(math.Sqrt(float64(1 - rr)))
				cell.inside = true
				cell.z = z
			}
			cells[r*cols+c] = cell
		}
	}
	globeCache.Store(key, cells)
	return cells
}

// planet draws the screen's body: a rotating character globe, 160 DIPs or
// larger turning at 20 Hz; smaller placeholders stay still.
func planet(c *Context, size float32, pal palette) Element {
	return Box(c).Size(size, size).Draw(func(p *Painter, r Rect) {
		drawGlobe(p, r, pal)
	})
}

// appMark is the nav's small planet: the sphere alone, crisp at 26 DIPs.
func appMark(c *Context, size float32, pal palette) Element {
	return Box(c).Size(size, size).Draw(func(p *Painter, r Rect) {
		d := min(r.W, r.H)
		if d <= 0 {
			return
		}
		sphere := Rect{X: r.X, Y: r.Y, W: d, H: d}
		p.FillRadialGradient(sphere, RadialGradient{
			Center: Point{X: 0.34, Y: 0.28},
			Radius: d * 0.95,
			Stops: []GradientStop{
				{Color: pal.ink[2], At: 0},
				{Color: pal.ink[1], At: 0.5},
				{Color: pal.ink[0], At: 1},
			},
		}, d/2)
		p.Clip(sphere, d/2, func() {
			p.FillRadialGradient(sphere, RadialGradient{
				Center: Point{X: 0.82, Y: 0.86},
				Radius: d * 0.9,
				Stops: []GradientStop{
					{Color: Transparent, At: 0.02},
					{Color: pal.ink[0].Alpha(0.75), At: 1},
				},
			}, d/2)
		})
		p.Stroke(sphere, pal.ink[2].Alpha(0.30), d/2, 1)
	})
}

// drawGlobe paints the character globe into r.
func drawGlobe(p *Painter, r Rect, pal palette) {
	size := min(r.W, r.H)
	if size < 12 {
		return
	}
	kind := pal.body
	targetCols := 64
	switch {
	case size < 40:
		targetCols = 12
	case size < 80:
		targetCols = 20
	case size < 150:
		targetCols = 32
	case size < 240:
		targetCols = 48
	}
	fontSize := size / float32(targetCols) * 1.5
	advance, _ := p.MeasureText(0, Span{Text: "MMMMMMMMMM", Font: fontMono, Size: fontSize})
	if advance <= 0 {
		return
	}
	cellW := advance / 10
	cellH := cellW * 1.68
	cols := int(size / cellW)
	rows := int(size / cellH)
	if cols < 6 || rows < 3 {
		return
	}

	angle := 0.7
	if size >= 150 && !p.Preferences().ReduceMotion {
		p.AnimationFrameRate(20)
		angle = float64(p.Now().UnixNano()) / 1e9 * kind.speed()
	}

	geometry := globeGeometry(cols, rows)
	ramp := kind.ramp()
	// Two tones: the body's base and its lit surface. With no shadow there is
	// no night side for the faint tone to describe, and a disc in the faintest
	// ink is the thing that was hard to see.
	tones := [2][]rune{
		fillRunes(' ', cols),
		fillRunes(' ', cols),
	}
	used := [2]bool{}

	x0 := r.X + (r.W-float32(cols)*cellW)/2
	y0 := r.Y + (r.H-float32(rows)*cellH)/2

	for row := 0; row < rows; row++ {
		for t := range tones {
			clearRunes(tones[t])
			used[t] = false
		}
		for col := range cols {
			cell := geometry[row*cols+col]
			if !cell.inside {
				continue
			}
			b := globeBrightness(kind, cell, angle)
			idx := 1 + int(b*float32(len(ramp)-2))
			// Every cell inside the limb is inked: the body is shown whole,
			// not as a lit crescent over a field of dots.
			if idx >= len(ramp) {
				idx = len(ramp) - 1
			}
			tone := 0
			if b >= 0.78 {
				tone = 1
			}
			tones[tone][col] = ramp[idx]
			used[tone] = true
		}
		for t := range tones {
			if !used[t] {
				continue
			}
			p.RichText(x0, y0+float32(row)*cellH, 0, Span{
				Text:  string(tones[t]),
				Font:  fontMono,
				Size:  fontSize,
				Color: pal.ink[t+1],
			})
		}
	}
}

func fillRunes(r rune, n int) []rune {
	out := make([]rune, n)
	for i := range out {
		out[i] = r
	}
	return out
}

func clearRunes(rs []rune) {
	for i := range rs {
		rs[i] = ' '
	}
}

// globeBrightness is how textured one cell is, 0 to 1. The body is lit
// evenly: no terminator and no night side, so the whole disc reads the same
// way and only the surface changes the glyph.
func globeBrightness(kind planetKind, cell globeCell, angle float64) float32 {
	// Turn the point around Y: only the surface moves with the angle.
	sinA, cosA := float32(math.Sin(angle)), float32(math.Cos(angle))
	// Reverse the tilt, then rotate.
	y := cell.v*float32(math.Cos(globeTilt)) - cell.z*float32(math.Sin(globeTilt))
	z2 := cell.v*float32(math.Sin(globeTilt)) + cell.z*float32(math.Cos(globeTilt))
	x := cell.u*cosA + z2*sinA
	zz := -cell.u*sinA + z2*cosA
	lat := math.Asin(math.Max(-1, math.Min(1, float64(y))))
	lon := math.Atan2(float64(x), float64(zz))

	// A floor keeps the darkest surface above the blank end of the ramp: the
	// planet is shown whole, not as a lit crescent.
	b := 0.35 + 0.65*bodyTexture(kind, lon, lat)
	b = math.Pow(math.Max(0, math.Min(1, b)), 1.2)
	return float32(b)
}

// bodyTexture is the body's surface at a longitude and latitude, 0 to 1.
func bodyTexture(kind planetKind, lon, lat float64) float64 {
	switch kind {
	case earth:
		land := smoothstep(0.48, 0.56, fbm(lon*1.1+8, lat*1.1+3, 11, 4))
		ice := smoothstep(1.00, 1.16, math.Abs(lat))
		return 0.30 + 0.70*land*(1-ice) + ice
	case mars:
		cr := smoothstep(0.46, 0.54, fbm(lon*1.7+2, lat*1.7+6, 14, 4))
		ice := smoothstep(1.07, 1.20, math.Abs(lat))
		return 0.40 + 0.60*cr*(1-ice) + ice
	case mercury:
		return 0.45 + 0.55*smoothstep(0.45, 0.55, fbm(lon*2.0+1, lat*2.0+9, 16, 4))
	case moon:
		return 0.45 + 0.50*smoothstep(0.47, 0.53, fbm(lon*1.8+4, lat*1.8+1, 18, 4))
	case jupiter:
		band := 0.5 + 0.5*math.Sin(lat*8+0.5*math.Sin(lon*2.2))
		storm := math.Exp(-(math.Pow((lon-1.2)/0.35, 2) + math.Pow((lat+0.35)/0.22, 2)))
		return math.Min(1, 0.55+0.45*band+0.30*storm)
	default: // sun
		return 0.45 + 0.55*fbm(lon*2.6+2, lat*2.6+6, 21, 4)
	}
}

func smoothstep(e0, e1, x float64) float64 {
	t := (x - e0) / (e1 - e0)
	t = math.Max(0, math.Min(1, t))
	return t * t * (3 - 2*t)
}

// fbm is deterministic value noise, four octaves by default: the bodies are
// the same on every run and in every test.
func fbm(x, y float64, seed int64, octaves int) float64 {
	total, amp, freq, norm := 0.0, 1.0, 1.0, 0.0
	for range octaves {
		fx, fy := x*freq, y*freq
		ix, iy := int64(math.Floor(fx)), int64(math.Floor(fy))
		tx, ty := fx-float64(ix), fy-float64(iy)
		tx = tx * tx * (3 - 2*tx)
		ty = ty * ty * (3 - 2*ty)
		a := valueNoise(ix, iy, seed)
		b := valueNoise(ix+1, iy, seed)
		c := valueNoise(ix, iy+1, seed)
		d := valueNoise(ix+1, iy+1, seed)
		total += (a + (b-a)*tx + (c-a)*ty + (a-b-c+d)*tx*ty) * amp
		norm += amp
		amp *= 0.5
		freq *= 2.03
	}
	return total / norm
}

func valueNoise(ix, iy, seed int64) float64 {
	n := ix*374761393 + iy*668265263 + seed*1442695040888963407
	n = (n ^ (n >> 13)) * 1274126177
	n = n ^ (n >> 16)
	return float64(n&0xffff) / 0xffff
}
