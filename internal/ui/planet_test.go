package ui

import (
	"math"
	"testing"
)

func TestGlobeGeometryCoversTheDisc(t *testing.T) {
	for _, colsRows := range [][2]int{{12, 8}, {32, 18}, {64, 32}} {
		cols, rows := colsRows[0], colsRows[1]
		cells := globeGeometry(cols, rows)
		if len(cells) != cols*rows {
			t.Fatalf("%dx%d: %d cells, want %d", cols, rows, len(cells), cols*rows)
		}
		limbs := 0
		for i, cell := range cells {
			inside := cell.u*cell.u+cell.v*cell.v <= 1
			if cell.inside != inside {
				t.Fatalf("cell %d (%.2f,%.2f) inside = %v", i, cell.u, cell.v, cell.inside)
			}
			if cell.limb {
				limbs++
			}
			if cell.inside && (cell.z < 0 || cell.z > 1) {
				t.Fatalf("cell %d z = %v", i, cell.z)
			}
		}
		if limbs == 0 {
			t.Errorf("%dx%d: no limb cells", cols, rows)
		}
	}
}

func TestGlobeGeometryIsCached(t *testing.T) {
	a := globeGeometry(32, 18)
	b := globeGeometry(32, 18)
	if len(a) == 0 || &a[0] != &b[0] {
		t.Fatalf("geometry is not reused for the same grid")
	}
}

func TestGlobeBrightnessIsDeterministic(t *testing.T) {
	cells := globeGeometry(64, 32)
	var before, after []float32
	for _, cell := range cells {
		if cell.inside {
			before = append(before, globeBrightness(earth, cell, 0.7))
			after = append(after, globeBrightness(earth, cell, 0.7))
		}
	}
	for i := range before {
		if before[i] != after[i] {
			t.Fatalf("cell %d differs between two paints: %v vs %v", i, before[i], after[i])
		}
		if before[i] < 0 || before[i] > 1 || math.IsNaN(float64(before[i])) {
			t.Fatalf("cell %d brightness = %v", i, before[i])
		}
	}
}

func TestGlobeSurfaceTurns(t *testing.T) {
	cells := globeGeometry(64, 32)
	changed := 0
	for _, cell := range cells {
		if !cell.inside {
			continue
		}
		a := globeBrightness(earth, cell, 0)
		b := globeBrightness(earth, cell, 1.2)
		if math.Abs(float64(a-b)) > 0.02 {
			changed++
		}
	}
	if changed < 20 {
		t.Fatalf("only %d cells changed between two angles: the surface does not turn", changed)
	}
}

func TestGlobeRampsAreSparse(t *testing.T) {
	if len(globeRamp) > 8 || len(sunRamp) > 8 {
		t.Fatalf("ramps are too dense for a character globe: %d, %d", len(globeRamp), len(sunRamp))
	}
	if globeRamp[0] != ' ' {
		t.Fatalf("the ramp must start blank, got %q", globeRamp[0])
	}
}

func BenchmarkGlobeShading(b *testing.B) {
	cells := globeGeometry(64, 32)
	b.ReportAllocs()
	for b.Loop() {
		for _, cell := range cells {
			if cell.inside {
				_ = globeBrightness(earth, cell, 0.7)
			}
		}
	}
}
