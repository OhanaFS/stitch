package util

import "io"

// LimitReader wraps an io.ReadSeeker and limits the number of bytes that can
// be read.
type LimitReader struct {
	reader io.ReadSeeker
	limit  int64
	pos    int64
}

// Assert that the LimitReader struct satisfies the io.ReadSeeker interface.
var _ io.ReadSeeker = &LimitReader{}

// NewLimitReader creates a new LimitReader.
func NewLimitReader(reader io.ReadSeeker, limit int64) *LimitReader {
	return &LimitReader{reader: reader, limit: limit}
}

func (r *LimitReader) Read(p []byte) (n int, err error) {
	if r.pos >= r.limit {
		return 0, io.EOF
	}

	if int64(len(p)) > r.limit {
		p = p[:r.limit]
	}
	n, err = r.reader.Read(p)
	r.pos += int64(n)

	return n, err
}

func (r *LimitReader) Seek(offset int64, whence int) (int64, error) {
	if whence == io.SeekEnd {
		whence = io.SeekStart
		offset = r.limit + offset
	}

	n, err := r.reader.Seek(offset, whence)
	r.pos = n

	return r.pos, err
}

func Min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func Max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
