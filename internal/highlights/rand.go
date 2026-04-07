package highlights

import (
	"crypto/rand"
	"io"
)

// newRand returns a crypto/rand-backed io.Reader for ULID entropy. ULID's
// monotonic reader handles same-millisecond increment internally.
func newRand() io.Reader {
	return rand.Reader
}
