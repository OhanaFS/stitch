package util

import "io"

// ZeroReadSeeker is a ReadSeeker that always returns null bytes.
type ZeroReadSeeker struct {
	// Size is the size of the file.
	Size int64
	// cursor is the current position in the file.
	cursor int64
}

// Assert that NopReadSeeker implements the ReadSeeker interface.
var _ io.ReadSeeker = &ZeroReadSeeker{}

// Read implements io.ReadSeeker
func (z *ZeroReadSeeker) Read(p []byte) (n int, err error) {
	if z.cursor >= z.Size {
		return 0, io.EOF
	}
	n = len(p)
	if z.cursor+int64(n) > z.Size {
		n = int(z.Size - z.cursor)
	}
	for i := 0; i < n; i++ {
		p[i] = 0
	}
	z.cursor += int64(n)
	return n, nil
}

// Seek implements io.ReadSeeker
func (z *ZeroReadSeeker) Seek(offset int64, whence int) (int64, error) {
	switch whence {
	case io.SeekStart:
		z.cursor = offset
	case io.SeekCurrent:
		z.cursor += offset
	case io.SeekEnd:
		z.cursor = z.Size + offset
	}
	if z.cursor < 0 {
		z.cursor = 0
	}
	if z.cursor > z.Size {
		z.cursor = z.Size
	}
	return z.cursor, nil
}
