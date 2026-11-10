package ui

import (
	"math"
	"sync"

	. "github.com/egoist/mygo/ui"
)

// The planets are dot-matrix globes, not photographs: a Braille-style grid
// of dots, one dot per sample of the surface, at an even pitch. The body is
// lit evenly — no terminator, no night side — so the whole disc stays
// readable while continents drift under it. Dot density, not glyph choice,
// carries the tone, which is what the ASCII generator's Braille preset
// does; the drawing, the bodies and the noise are ours.

type planetKind int

const (
	earth planetKind = iota
	mars
	mercury
	jupiter
	sun
	moon
)

// A Braille cell is a 2x4 grid of dots, so drawing the dots directly at an
// even pitch gives the same picture without the glyph's line-box gaps. Every
// dot inside the disc is drawn; three ink/size tiers — dark, body, lit —
// carry the features, so the field stays even while maria, belts and spots
// stay readable.
func (k planetKind) tiers() (dark, lit float32) {
	switch k {
	case earth:
		return 0.42, 0.78
	case mars:
		return 0.50, 0.78
	case mercury:
		return 0.52, 0.76
	case moon:
		return 0.58, 0.82
	case jupiter:
		return 0.48, 0.80
	default: // sun
		return 0.54, 0.82
	}
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

// appMark is the nav's small planet: the same dot globe as the hero, on a
// shorter dot pitch so the mark is still a sphere at 28 DIP.
func appMark(c *Context, size float32, pal palette) Element {
	return Box(c).Size(size, size).Draw(func(p *Painter, r Rect) {
		drawDots(p, r, pal, 0.7)
	})
}

// drawGlobe paints the screen's body as a dot globe: a rotating surface at
// 20 Hz for hero sizes, a still frame below.
func drawGlobe(p *Painter, r Rect, pal palette) {
	size := min(r.W, r.H)
	if size < 12 {
		return
	}
	angle := 0.7
	if size >= 150 && !p.Preferences().ReduceMotion {
		p.AnimationFrameRate(20)
		angle = float64(p.Now().UnixNano()) / 1e9 * pal.body.speed()
	}
	drawDots(p, r, pal, angle)
}

// drawDots paints the sphere as a Braille-style dot matrix: one dot per
// sampled cell, at one pitch, about 64 dots across a hero. A Braille glyph
// is the same 2x4 dot picture, but its line box leaves horizontal gaps
// between rows; drawing the dots directly makes the surface evenly dense
// and the limb round. Brightness becomes dot density through an ordered
// threshold, so the tone changes without glyph steps.
func drawDots(p *Painter, r Rect, pal palette, angle float64) {
	size := min(r.W, r.H)
	if size < 8 {
		return
	}
	// A whole-DIP pitch keeps the dot grid even: a fractional pitch rounds
	// to alternating 3 and 4 DIP gaps, which reads as a wobbling grid.
	pitch := float32(int(math.Round(float64(size / 56))))
	if size < 150 {
		pitch = float32(int(math.Round(float64(size / 30))))
	}
	if pitch < 2 {
		pitch = 2
	}
	dotCols := int(size / pitch)
	if dotCols < 10 {
		dotCols = 10
		pitch = size / float32(dotCols)
	}
	dotRows := dotCols
	if dotRows < 6 {
		return
	}
	// Every dot is present; brightness also changes its size and ink, which
	// is what makes continents, belts and spots readable at this scale.
	dotBase := max(1, pitch-2)
	dotLit := min(pitch-0.5, dotBase+1)
	if size < 150 {
		// Small tiles cannot afford a whole extra pixel: the lit dots merge
		// into white blobs. A half-step keeps the feature without the blob.
		dotLit = min(pitch-0.5, dotBase+0.5)
	}
	dark := pal.ink[0]

	geometry := globeGeometry(dotCols, dotRows)
	// The floor is darker than the palette's mid ink so oceans read as
	// water instead of a flat field; every dot is still present.
	base := pal.ink[0].Mix(pal.ink[1], 0.35)
	if pal.body == sun {
		// The sun is emissive: its dots start at the amber mid tone, not the
		// dark end, or the disc reads as a brown ball.
		base = pal.ink[1]
	}
	lit := pal.ink[2]
	x0 := float32(math.Round(float64(r.X + (r.W-float32(dotCols)*pitch)/2)))
	y0 := float32(math.Round(float64(r.Y + (r.H-float32(dotRows)*pitch)/2)))

	for row := range dotRows {
		for col := range dotCols {
			cell := geometry[row*dotCols+col]
			if !cell.inside {
				continue
			}
			b := globeBrightness(pal.body, cell, angle) * (0.82 + 0.18*cell.z)
			darkT, litT := pal.body.tiers()
			if size < 40 {
				// A nav mark has no room for three tiers: they merge into a
				// white blob, so it stays a calm one-tone sphere.
				darkT, litT = 0.2, 1.1
			}
			ink, dot := base, dotBase
			switch {
			case b >= litT:
				ink, dot = lit, dotLit
			case b < darkT:
				ink = dark
			}
			p.Fill(Rect{
				X: x0 + float32(col)*pitch + (pitch-dot)/2,
				Y: y0 + float32(row)*pitch + (pitch-dot)/2,
				W: dot,
				H: dot,
			}, ink, 0)
		}
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
		// Two broad landmasses with a bright relief and white caps: at tile
		// size the silhouette and the caps are what make it Earth.
		land := smoothstep(0.46, 0.62, fbm(lon*1.3+8, lat*1.3+3, 11, 3))
		ice := float64(0)
		ocean := 0.22 + 0.10*fbm(lon*1.1+5, lat*1.1+2, 12, 2)
		relief := 0.80 + 0.20*fbm(lon*1.7+2, lat*1.7+7, 13, 2)
		return math.Min(1, (ocean+relief*land)*(1-ice)+ice)
	case mars:
		// Dark albedo seas over rust, with bright polar caps.
		albedo := smoothstep(0.42, 0.60, fbm(lon*1.2+2, lat*1.2+6, 14, 3))
		ice := smoothstep(1.40, 1.50, math.Abs(lat))
		base := 0.30 + 0.08*fbm(lon*1.3+7, lat*1.3+4, 15, 2)
		return math.Min(1, (base+0.58*albedo)*(1-ice)+ice)
	case mercury:
		// A grey body of broad darker basins and bright crater rays.
		basin := smoothstep(0.34, 0.58, fbm(lon*1.2+3, lat*1.2+8, 17, 3))
		rays := smoothstep(0.60, 0.78, fbm(lon*1.2+1, lat*1.2+9, 16, 2))
		return math.Min(1, 0.30+0.48*basin+0.26*rays)
	case moon:
		// Broad dark maria over bright highlands: the pattern is the only
		// way to tell the Moon from Mercury at 40 dots.
		maria := smoothstep(0.42, 0.62, fbm(lon*1.0+6, lat*1.0+5, 19, 3))
		high := 0.72 + 0.18*fbm(lon*1.5+4, lat*1.5+1, 18, 2)
		return math.Min(1, high*(1-0.62*maria)+0.10)
	case jupiter:
		// Strong belts and a storm oval, the one feature that reads at 40 dots.
		band := 0.5 + 0.45*math.Sin(lat*5.2+0.35*math.Sin(lon*1.4))
		storm := math.Exp(-(math.Pow((lon-1.2)/0.55, 2) + math.Pow((lat+0.30)/0.32, 2)))
		return math.Max(0.18, math.Min(1, 0.12+0.92*band-0.30*storm))
	default: // sun
		// Granulation with two sunspots: a uniform amber disc said nothing.
		gran := 0.55 + 0.45*fbm(lon*1.8+2, lat*1.8+6, 21, 3)
		spot := math.Exp(-(math.Pow((lon-0.6)/0.36, 2) + math.Pow((lat-0.2)/0.26, 2))) +
			0.8*math.Exp(-(math.Pow((lon+2.1)/0.40, 2) + math.Pow((lat+0.5)/0.30, 2)))
		return math.Max(0.18, gran-0.70*spot)
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
