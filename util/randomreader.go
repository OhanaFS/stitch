package util

import (
	"io"
	"math/rand"
)

// RandomReader is a io.Reader that returns random bytes. This uses math/rand
// to generate random bytes and should not be used for security purposes.
type RandomReader struct {
	// Size is the size of the file.
	Size int64
}

// Assert that RandomReader implements the io.Reader interface.
var _ io.Reader = &RandomReader{}

// Read implements io.Reader
func (r *RandomReader) Read(p []byte) (n int, err error) {
	if r.Size == 0 {
		return 0, io.EOF
	}
	n = len(p)
	if r.Size < int64(n) {
		n = int(r.Size)
	}
	r.Size -= int64(n)
	return rand.Read(p[:n])
}
