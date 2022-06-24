package util

import (
	"fmt"
	"io"
	"time"
)

// ProgressReader wraps an io.Reader and reports the number of bytes read.
type ProgressReader struct {
	reader       io.Reader
	pos          int64
	max          int64
	snapshotTime time.Time
	snapshotPos  int64
	speed        int64
}

// Assert that the ProgressReader struct satisfies the io.Reader interface.
var _ io.Reader = &ProgressReader{}

// NewProgressReader creates a new ProgressReader.
func NewProgressReader(reader io.Reader, max int64) *ProgressReader {
	return &ProgressReader{reader: reader, max: max, snapshotTime: time.Now()}
}

// Read implements io.Reader
func (r *ProgressReader) Read(p []byte) (n int, err error) {
	n, err = r.reader.Read(p)
	r.pos += int64(n)

	// Report progress
	fmt.Print(FormatSize(r.pos))
	if r.max > 0 {
		fmt.Printf(" / %s", FormatSize(r.max))
	}
	if r.speed > 0 {
		fmt.Printf(" (%s/s)", FormatSize(r.speed))
	}
	fmt.Print("    \r")

	// Update snapshot if it's been more than 1 second since the last one
	if time.Since(r.snapshotTime) > time.Second {
		r.speed = r.pos - r.snapshotPos
		r.snapshotPos = r.pos
		r.snapshotTime = time.Now()
	}

	return n, err
}
