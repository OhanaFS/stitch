package util

import (
	"fmt"
	"io"
)

// Membuf is an in-memory buffer that implements the ReadWriteSeeker interface.
type Membuf struct {
	buf []byte
	pos int
}

// Assert that the Membuf struct satisfies the io.ReadWriteSeeker interface.
var _ io.ReadWriteSeeker = &Membuf{}

// NewMembuf creates a new Membuf.
func NewMembuf() *Membuf {
	return &Membuf{}
}

func (m *Membuf) Read(p []byte) (n int, err error) {
	if m.pos >= len(m.buf) {
		return 0, io.EOF
	}
	n = copy(p, m.buf[m.pos:])
	m.pos += n
	return n, nil
}

func (m *Membuf) Write(p []byte) (n int, err error) {
	if m.pos+len(p) > len(m.buf) {
		m.buf = append(m.buf[:m.pos], p...)
	} else {
		copy(m.buf[m.pos:], p)
	}
	m.pos += len(p)
	return len(p), nil
}

func (m *Membuf) Seek(offset int64, whence int) (int64, error) {
	switch whence {
	case io.SeekStart:
		m.pos = int(offset)
	case io.SeekCurrent:
		m.pos += int(offset)
	case io.SeekEnd:
		m.pos = len(m.buf) + int(offset)
	default:
		return 0, fmt.Errorf("invalid whence: %d", whence)
	}
	return int64(m.pos), nil
}

func (m *Membuf) Len() int {
	return len(m.buf)
}

func (m *Membuf) Bytes() []byte {
	return m.buf
}
