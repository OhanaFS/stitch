package reedsolomon

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"io"

	rs "github.com/klauspost/reedsolomon"
)

// UnboundedStreamEncoder is an interface to encode Reed-Solomon parity sets
// from a stream of unknown length.
type UnboundedStreamEncoder interface {
	Split(data io.Reader, dst []io.Writer) error
	Join(dst io.Writer, shards []io.Reader, outSize int64) error
}

type encoder struct {
	dataShards   int
	parityShards int
	blockSize    int
	rsEncoder    rs.StreamEncoder
}

func New(dataShards, parityShards, blockSize int) (UnboundedStreamEncoder, error) {
	rsEncoder, err := rs.NewStream(dataShards, parityShards)
	if err != nil {
		return nil, err
	}

	return &encoder{
		dataShards,
		parityShards,
		blockSize,
		rsEncoder,
	}, nil
}

// Split splits the data into the number of shards given to it, and writes the
// shards to the writers. Note that the caller must keep track of the following
// metadata in order to correctly reconstruct the data:
//
// * The number of data shards
// * The number of parity shards
// * The block size
// * The size of the original data
// * The order of the shards
//
// This function also adds a sha256 hash every `blockSize` bytes.
func (e *encoder) Split(data io.Reader, dst []io.Writer) error {
	totalShards := e.dataShards + e.parityShards
	if len(dst) != totalShards {
		return fmt.Errorf("expected %d shards, got %d", totalShards, len(dst))
	}

	buf := make([]byte, e.blockSize*e.dataShards)
	enc, err := rs.New(e.dataShards, e.parityShards)
	if err != nil {
		return err
	}

	for {
		// Read a block.
		n, err := data.Read(buf)
		if err != nil {
			if err == io.EOF {
				break
			}
			return err
		}

		// If block is smaller than blockSize*dataShards, pad it.
		expected := e.blockSize * e.dataShards
		if n < expected {
			buf = append(buf[:n], make([]byte, expected-n)...)
		}

		// Split the block into shards.
		shards, err := enc.Split(buf)
		if err != nil {
			return err
		}

		// Encode parity.
		if err = enc.Encode(shards); err != nil {
			return err
		}

		for i, shard := range shards {
			if dst[i] != nil {
				// Calculate the hash of the shard.
				hash := sha256.Sum256(shard)

				// Write the shards and the hash to the destination.
				if _, err := dst[i].Write(shard); err != nil {
					return err
				}
				if _, err := dst[i].Write(hash[:]); err != nil {
					return err
				}
			}
		}
	}

	return nil
}

// Join reconstructs the data from the shards given to it.
func (e *encoder) Join(dst io.Writer, shards []io.Reader, outSize int64) error {
	totalShards := e.dataShards + e.parityShards
	if len(shards) != totalShards {
		return fmt.Errorf("expected %d shards, got %d", totalShards, len(shards))
	}

	// Allocate buffers for the data shards.
	bufs := make([][]byte, len(shards))
	for i := range bufs {
		bufs[i] = make([]byte, e.blockSize)
	}

	hashes := make([][]byte, len(shards))
	for i := range hashes {
		hashes[i] = make([]byte, sha256.Size)
	}

	// Initialize the Reed-Solomon decoder.
	enc, err := rs.New(e.dataShards, e.parityShards)
	if err != nil {
		return err
	}

	// Keep track of the number of bytes left to be written to the output.
	bytesLeft := outSize

	for {
		// Read shard blocks.
		for i, shard := range shards {
			if shard == nil {
				continue
			}

			if _, err := shard.Read(bufs[i]); err != nil {
				return fmt.Errorf("failed to read from shard %d: %s", i, err)
			}

			if _, err := shard.Read(hashes[i]); err != nil {
				return fmt.Errorf("failed to read hash from shard %d: %s", i, err)
			}

			// Verify the hash.
			hash := sha256.Sum256(bufs[i])
			if !bytes.Equal(hashes[i], hash[:]) {
				// If hashes don't match, truncate the shard so that `enc.Reconstruct`
				// will regenerate it.
				bufs[i] = []byte{}
			}
		}

		// Verify the shards.
		ok, err := enc.Verify(bufs)
		if !ok {
			// Try to reconstruct the data.
			if err = enc.Reconstruct(bufs); err != nil {
				return fmt.Errorf("reconstruct failed: %s", err)
			}

			// Re-verify the shards.
			if ok, err = enc.Verify(bufs); !ok {
				return fmt.Errorf("verify failed after reconstruct, data likely corrupted: %s", err)
			}
		}

		// Join the shards.
		blockSize := int64(e.blockSize) * int64(e.dataShards)
		if bytesLeft < blockSize {
			blockSize = bytesLeft
		}
		if err = enc.Join(dst, bufs, int(blockSize)); err != nil {
			return fmt.Errorf("join failed: %s", err)
		}

		// Update the number of bytes left.
		bytesLeft -= blockSize
		if bytesLeft <= 0 {
			break
		}

		// Reset the buffers.
		for i := range bufs {
			bufs[i] = make([]byte, e.blockSize)
		}
	}

	return nil
}
