package reedsolomon

import (
	"bytes"
	"io"

	"github.com/OhanaFS/stitch/util"
)

// ReadSeeker implements the io.ReadSeeker interface for Reed-Solomon encoded
// shards.
type ReadSeeker struct {
	encoder *Encoder
	shards  []io.ReadSeeker
	outSize int64

	// currentOffset specifies the offset of the underlying file
	currentOffset int64
	// bytesToDiscard specifies how many bytes to skip when reading before
	// returning the data to the user, in order to deliver the requested offset.
	bytesToDiscard int64
}

// NewReadSeeker returns a new ReaderSeeker
func NewReadSeeker(encoder *Encoder, shards []io.ReadSeeker, outSize int64) io.ReadSeeker {
	return util.NewLimitReader(&ReadSeeker{
		encoder:       encoder,
		shards:        shards,
		outSize:       outSize,
		currentOffset: 0,
	}, outSize)
}

func (r *ReadSeeker) Read(p []byte) (int, error) {
	if _, err := r.Seek(0, io.SeekCurrent); err != nil {
		return 0, err
	}

	// Check if EOF
	if r.currentOffset >= r.outSize {
		return 0, io.EOF
	}

	// Create a buffer to read the data into
	size := len(p)
	if r.currentOffset+int64(size) > r.outSize {
		size = int(r.outSize - r.currentOffset)
	}
	buf := new(bytes.Buffer)
	buf.Grow(size + int(r.bytesToDiscard))

	// Set up readers
	readers := make([]io.Reader, len(r.shards))
	for i, shard := range r.shards {
		readers[i] = shard
	}

	// Read the data
	err := r.encoder.Join(buf, readers, int64(buf.Cap()))
	if err != nil {
		return 0, err
	}

	// Write the data to the output
	buf.Next(int(r.bytesToDiscard))
	n, err := buf.Read(p)

	// Update the current offset
	if _, err := r.Seek(int64(n), io.SeekCurrent); err != nil {
		return 0, err
	}

	return n, err
}

func (r *ReadSeeker) Seek(offset int64, whence int) (int64, error) {
	// Calculate offset from the start
	if whence == io.SeekCurrent {
		offset += r.currentOffset
	} else if whence == io.SeekEnd {
		offset = r.outSize + offset
	}

	// Calculate the offset for each shard
	blockSize := int64(r.encoder.BlockSize)
	dataShards := int64(r.encoder.DataShards)
	realBlockSize := blockSize + int64(BlockOverhead)
	block := offset / (blockSize * dataShards)
	shardOffset := block * realBlockSize

	r.currentOffset = offset
	r.bytesToDiscard = offset - (block * blockSize * dataShards)

	// Seek each shard
	for _, shard := range r.shards {
		if _, err := shard.Seek(shardOffset, io.SeekStart); err != nil {
			return 0, err
		}
	}

	return offset, nil
}
