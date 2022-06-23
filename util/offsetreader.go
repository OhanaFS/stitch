package util

import "io"

// OffsetReader wraps an io.ReadSeeker and adds an offset to the seek position.
type OffsetReader struct {
	reader io.ReadSeeker
	offset int64
}

// Assert that the OffsetReader struct satisfies the io.ReadSeeker interface.
var _ io.ReadSeeker = &OffsetReader{}

// NewOffsetReader creates a new OffsetReader.
func NewOffsetReader(reader io.ReadSeeker, offset int64) *OffsetReader {
	return &OffsetReader{reader, offset}
}

func (r *OffsetReader) Read(p []byte) (n int, err error) {
	return r.reader.Read(p)
}

func (r *OffsetReader) Seek(offset int64, whence int) (int64, error) {
	return r.reader.Seek(r.offset+offset, whence)
}
