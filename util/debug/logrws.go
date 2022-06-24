package debug

import (
	"fmt"
	"io"
	"log"
)

// Logrws is a wrapper around a ReadWriteSeeker that logs all reads and writes.
type Logrws[T any] struct {
	rws    T
	prefix string
}

// Assert that the Logrws struct satisfies the io.ReadWriteSeeker interface.
var _ io.ReadWriteSeeker = &Logrws[io.ReadWriteSeeker]{}
var _ io.Closer = &Logrws[io.Closer]{}

func NewLogrws[T any](rws T, prefix string) *Logrws[T] {
	return &Logrws[T]{rws: rws, prefix: prefix}
}

func (l *Logrws[T]) Read(p []byte) (n int, err error) {
	// Try to cast the underlying ReadWriteSeeker to a io.Reader
	if r, ok := any(l.rws).(io.Reader); ok {
		n, err = r.Read(p)
		if err != nil {
			return n, err
		}
		log.Printf("%s.Read(%d) = %d", l.prefix, len(p), n)
		Hexdump(p[:n], l.prefix+":r")
		return n, nil
	}
	return 0, fmt.Errorf("%s is not a io.Reader", l.prefix)
}

func (l *Logrws[T]) Write(p []byte) (n int, err error) {
	// Try to cast the underlying ReadWriteSeeker to a io.Writer
	if w, ok := any(l.rws).(io.Writer); ok {
		n, err = w.Write(p)
		if err != nil {
			return n, err
		}
		log.Printf("%s.Write(%d) = %d", l.prefix, len(p), n)
		Hexdump(p[:n], l.prefix+":w")
		return n, nil
	}
	return 0, fmt.Errorf("%s is not a io.Writer", l.prefix)
}

func (l *Logrws[T]) Seek(offset int64, whence int) (int64, error) {
	// Try to cast the underlying ReadWriteSeeker to a io.Seeker
	if s, ok := any(l.rws).(io.Seeker); ok {
		log.Printf("%s.Seek(%d, %d)", l.prefix, offset, whence)
		return s.Seek(offset, whence)
	}
	return 0, fmt.Errorf("%s is not a io.Seeker", l.prefix)
}

func (l *Logrws[T]) Close() error {
	if c, ok := any(l.rws).(io.Closer); ok {
		log.Printf("%s.Close()", l.prefix)
		return c.Close()
	}
	return fmt.Errorf("%s is not a io.Closer", l.prefix)
}
