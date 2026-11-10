package ui

import (
	"math"
	"testing"

	. "github.com/egoist/mygo/ui"
)

func areaOf(t Tile) float64 { return float64(t.Rect.W) * float64(t.Rect.H) }

func TestSquarifyConservesArea(t *testing.T) {
	values := []float64{40, 30, 20, 10}
	r := Rect{X: 0, Y: 0, W: 400, H: 300}
	tiles := squarify(values, r)
	if len(tiles) != len(values) {
		t.Fatalf("got %d tiles, want %d", len(tiles), len(values))
	}
	var sum float64
	for _, tile := range tiles {
		sum += areaOf(tile)
		if !tile.Rect.Contains(tile.Rect.X+tile.Rect.W/2, tile.Rect.Y+tile.Rect.H/2) {
			t.Errorf("tile %d is not inside itself: %+v", tile.Index, tile.Rect)
		}
		if tile.Rect.X < r.X-0.01 || tile.Rect.Y < r.Y-0.01 ||
			tile.Rect.X+tile.Rect.W > r.X+r.W+0.01 || tile.Rect.Y+tile.Rect.H > r.Y+r.H+0.01 {
			t.Errorf("tile %d escapes the rectangle: %+v", tile.Index, tile.Rect)
		}
	}
	want := float64(r.W) * float64(r.H)
	if math.Abs(sum-want) > want*0.001 {
		t.Errorf("tile areas sum to %.2f, want %.2f", sum, want)
	}
}

func TestSquarifyProportions(t *testing.T) {
	values := []float64{75, 25}
	r := Rect{W: 100, H: 100}
	tiles := squarify(values, r)
	var big, small float64
	for _, tile := range tiles {
		if tile.Index == 0 {
			big = areaOf(tile)
		} else {
			small = areaOf(tile)
		}
	}
	if math.Abs(big/small-3) > 0.05 {
		t.Errorf("areas %.1f and %.1f are not 3:1", big, small)
	}
}

func TestSquarifySingleAndEmpty(t *testing.T) {
	r := Rect{W: 200, H: 100}
	tiles := squarify([]float64{5}, r)
	if len(tiles) != 1 || tiles[0].Rect != r {
		t.Fatalf("single item = %+v, want the whole rect", tiles)
	}
	if got := squarify(nil, r); got != nil {
		t.Errorf("empty values = %+v", got)
	}
	if got := squarify([]float64{1}, Rect{}); got != nil {
		t.Errorf("empty rect = %+v", got)
	}
}

func TestSquarifySkipsZero(t *testing.T) {
	tiles := squarify([]float64{10, 0, 10}, Rect{W: 100, H: 100})
	if len(tiles) != 2 {
		t.Fatalf("got %d tiles, want 2", len(tiles))
	}
	for _, tile := range tiles {
		if tile.Index == 1 {
			t.Error("zero value produced a tile")
		}
	}
}

func TestSquarifyManyItems(t *testing.T) {
	values := make([]float64, 500)
	for i := range values {
		values[i] = float64(i + 1)
	}
	tiles := squarify(values, Rect{W: 800, H: 600})
	if len(tiles) != len(values) {
		t.Fatalf("got %d tiles", len(tiles))
	}
	for _, tile := range tiles {
		if tile.Rect.W < 0 || tile.Rect.H < 0 {
			t.Fatalf("negative tile %+v", tile.Rect)
		}
	}
}
