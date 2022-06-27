package util

import (
	"fmt"
	"io"
)

// Membuf is an in-memory buffer that implements the ReadWriteSeeker interface.
type Membuf struct {
	buf    []byte
	pos    int
	length int
}

// Assert that the Membuf struct satisfies the io.ReadWriteSeeker interface.
var _ io.ReadWriteSeeker = &Membuf{}

// NewMembuf creates a new Membuf.
func NewMembuf() *Membuf {
	return &Membuf{buf: make([]byte, 1024)}
}

func (m *Membuf) Read(p []byte) (n int, err error) {
	if m.pos >= m.length {
		return 0, io.EOF
	}
	n = copy(p, m.buf[m.pos:m.length])
	m.pos += n
	return n, nil
}

func (m *Membuf) Write(p []byte) (n int, err error) {
	for m.pos+len(p) > len(m.buf) {
		// Allocate double the size of the buffer if the write would overflow.
		newbuf := make([]byte, len(m.buf)*2)
		copy(newbuf, m.buf)
		m.buf = newbuf
	}
	n = copy(m.buf[m.pos:], p)
	m.pos += n
	m.length = Max(m.length, m.pos)
	return len(p), nil
}

func (m *Membuf) Seek(offset int64, whence int) (int64, error) {
	switch whence {
	case io.SeekStart:
		m.pos = int(offset)
	case io.SeekCurrent:
		m.pos += int(offset)
	case io.SeekEnd:
		m.pos = m.length + int(offset)
	default:
		return 0, fmt.Errorf("invalid whence: %d", whence)
	}
	return int64(m.pos), nil
}

func (m *Membuf) Len() int {
	return m.length
}

func (m *Membuf) Bytes() []byte {
	return m.buf[:m.length]
}
