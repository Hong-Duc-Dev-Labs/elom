package ui

import (
	"math"
	"testing"

	. "github.com/egoist/mygo/ui"
)

// The wash math is Mole's (TabWash.cs), pinned here so a refactor cannot
// silently change the tint every screen sits on.
func TestWashAlpha(t *testing.T) {
	cases := []struct {
		f    float32
		want float32
	}{
		{-1, 217.0 / 255.0},
		{0, 217.0 / 255.0},
		{0.25, 162.0 / 255.0},
		{0.5, 107.0 / 255.0},
		{0.75, 107.0 / 255.0},
		{1, 107.0 / 255.0},
		{2, 107.0 / 255.0},
	}
	for _, c := range cases {
		if got := washAlphaAt(c.f); math.Abs(float64(got-c.want)) > 1e-6 {
			t.Errorf("washAlphaAt(%v) = %v, want %v", c.f, got, c.want)
		}
	}
}

func TestCardGround(t *testing.T) {
	// Base #15141b, Clean wash #29385c: composite at the rest alpha 107/255,
	// then sink 15% toward the base.
	got := cardGround(Hex("#29385c"))
	want := RGB(28, 33, 50)
	if got.R != want.R || got.G != want.G || got.B != want.B || got.A != 255 {
		t.Fatalf("cardGround = #%02x%02x%02x, want #%02x%02x%02x", got.R, got.G, got.B, want.R, want.G, want.B)
	}
}

func TestEveryScreenKeepsCardTextReadable(t *testing.T) {
	for _, id := range []string{"clean", "software", "startup", "optimize", "analyze", "status", "settings"} {
		ground := cardGround(paletteFor(id).wash)
		for _, tc := range []struct {
			name string
			fg   Color
			min  float64
		}{
			{"muted", fgMuted(), 4.5},
			{"secondary", fgSecondary(), 7.0},
			{"primary", fgPrimary(), 10.0},
		} {
			got := contrastRatio(over(tc.fg, ground), ground)
			if got < tc.min {
				t.Errorf("%s: %s text is %.2f:1 on its card, want at least %.1f:1", id, tc.name, got, tc.min)
			}
		}
	}
}

func TestMixPaletteEnds(t *testing.T) {
	a, b := paletteFor("clean"), paletteFor("status")
	if got := mixPalette(a, b, 0); got.wash != a.wash {
		t.Errorf("t=0 wash = %v, want the first", got.wash)
	}
	if got := mixPalette(a, b, 1); got.wash != b.wash {
		t.Errorf("t=1 wash = %v, want the second", got.wash)
	}
	mid := mixPalette(a, b, 0.5)
	if mid.wash == a.wash || mid.wash == b.wash {
		t.Errorf("t=0.5 wash = %v, want a blend", mid.wash)
	}
}

func over(fg, bg Color) Color {
	a := float64(fg.A) / 255.0
	mix := func(f, b uint8) uint8 {
		return uint8(math.Round(float64(b) + (float64(f)-float64(b))*a))
	}
	return RGB(mix(fg.R, bg.R), mix(fg.G, bg.G), mix(fg.B, bg.B))
}

func contrastRatio(a, b Color) float64 {
	la, lb := luminance(a), luminance(b)
	hi, lo := math.Max(la, lb), math.Min(la, lb)
	return (hi + 0.05) / (lo + 0.05)
}

func luminance(c Color) float64 {
	lin := func(v uint8) float64 {
		f := float64(v) / 255.0
		if f <= 0.03928 {
			return f / 12.92
		}
		return math.Pow((f+0.055)/1.055, 2.4)
	}
	return 0.2126*lin(c.R) + 0.7152*lin(c.G) + 0.0722*lin(c.B)
}
