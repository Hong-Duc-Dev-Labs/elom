package metrics

// Ring is a fixed-size history of float64 samples. Push overwrites the
// oldest sample once full; values returns them oldest first.
type Ring struct {
	buf  []float64
	next int
	full bool
}

// NewRing returns a ring holding capacity samples.
func NewRing(capacity int) *Ring {
	if capacity <= 0 {
		capacity = 1
	}
	return &Ring{buf: make([]float64, capacity)}
}

// Push adds one sample.
func (r *Ring) Push(v float64) {
	r.buf[r.next] = v
	r.next++
	if r.next == len(r.buf) {
		r.next = 0
		r.full = true
	}
}

// Len returns how many samples the ring holds.
func (r *Ring) Len() int {
	if r.full {
		return len(r.buf)
	}
	return r.next
}

// values writes the samples into dst (reused when it has room) and returns
// it, oldest first.
func (r *Ring) values(dst []float64) []float64 {
	n := r.Len()
	if cap(dst) < n {
		dst = make([]float64, n)
	}
	dst = dst[:n]
	if !r.full {
		copy(dst, r.buf[:n])
		return dst
	}
	copy(dst, r.buf[r.next:])
	copy(dst[len(r.buf)-r.next:], r.buf[:r.next])
	return dst
}

// Values returns the samples, oldest first.
func (r *Ring) Values() []float64 { return r.values(nil) }
