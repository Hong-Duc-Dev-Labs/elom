package ui

import (
	"math"
	"sort"

	. "github.com/egoist/mygo/ui"
)

// Tile is one rectangle of a treemap and the item it shows.
type Tile struct {
	Index int
	Rect  Rect
}

// squarify lays items out as a squarified treemap (Bruls, Huizing, van Wijk):
// rectangles whose areas are proportional to their values, kept as close to
// square as the row structure allows. Values are any non-negative unit; they
// are scaled to the rectangle's area. It returns one tile per non-zero value,
// in no particular order, or nil for an empty rectangle.
func squarify(values []float64, r Rect) []Tile {
	if r.W <= 0 || r.H <= 0 || len(values) == 0 {
		return nil
	}
	total := 0.0
	for _, v := range values {
		if v > 0 {
			total += v
		}
	}
	if total <= 0 {
		return nil
	}

	order := make([]int, 0, len(values))
	for i, v := range values {
		if v > 0 {
			order = append(order, i)
		}
	}
	sort.SliceStable(order, func(a, b int) bool { return values[order[a]] > values[order[b]] })

	scale := float64(r.W) * float64(r.H) / total
	area := func(i int) float64 { return values[i] * scale }

	tiles := make([]Tile, 0, len(order))
	cur := r
	i := 0
	for i < len(order) {
		side := math.Min(float64(cur.W), float64(cur.H))
		row := []int{order[i]}
		rowSum := area(order[i])
		i++
		for i < len(order) {
			candidate := append(row, order[i])
			nextSum := rowSum + area(order[i])
			if worst(candidate, nextSum, side, area) <= worst(row, rowSum, side, area) {
				row = candidate
				rowSum = nextSum
				i++
				continue
			}
			break
		}
		cur = placeRow(row, rowSum, cur, area, &tiles)
	}
	return tiles
}

// worst is the largest aspect ratio in a row of sum placed across side.
func worst(row []int, sum, side float64, area func(int) float64) float64 {
	if sum <= 0 || side <= 0 {
		return math.Inf(1)
	}
	mx, mn := 0.0, math.Inf(1)
	for _, idx := range row {
		a := area(idx)
		mx = math.Max(mx, a)
		mn = math.Min(mn, a)
	}
	if mn <= 0 {
		return math.Inf(1)
	}
	return math.Max(side*side*mx/(sum*sum), sum*sum/(side*side*mn))
}

// placeRow lays one row along the shorter side of r and returns what is left.
// A row is a column when the rectangle is taller than it is wide.
func placeRow(row []int, sum float64, r Rect, area func(int) float64, tiles *[]Tile) Rect {
	vertical := r.W >= r.H
	if vertical {
		width := float32(sum / float64(r.H))
		if width > r.W {
			width = r.W
		}
		y := r.Y
		for i, idx := range row {
			height := float32(float64(r.H) * area(idx) / sum)
			if i == len(row)-1 {
				height = r.Y + r.H - y
			}
			*tiles = append(*tiles, Tile{Index: idx, Rect: Rect{X: r.X, Y: y, W: width, H: height}})
			y += height
		}
		return Rect{X: r.X + width, Y: r.Y, W: r.W - width, H: r.H}
	}
	height := float32(sum / float64(r.W))
	if height > r.H {
		height = r.H
	}
	x := r.X
	for i, idx := range row {
		width := float32(float64(r.W) * area(idx) / sum)
		if i == len(row)-1 {
			width = r.X + r.W - x
		}
		*tiles = append(*tiles, Tile{Index: idx, Rect: Rect{X: x, Y: r.Y, W: width, H: height}})
		x += width
	}
	return Rect{X: r.X, Y: r.Y + height, W: r.W, H: r.H - height}
}
