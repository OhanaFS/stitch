package reedsolomon

import (
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
// Additionally, the caller should keep track of the hash of each shard to
// detect data corruption.
func (e *encoder) Split(data io.Reader, dst []io.Writer) error {
	totalShards := e.dataShards + e.parityShards
	if len(dst) != totalShards {
		return fmt.Errorf("expected %d shards, got %d", totalShards, len(dst))
	}

	buf := make([]byte, e.blockSize)
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

		// Split the block into shards.
		shards, err := enc.Split(buf[:n])
		if err != nil {
			return err
		}

		// Encode parity.
		if err = enc.Encode(shards); err != nil {
			return err
		}

		// Write the shards.
		for i, shard := range shards {
			if dst[i] != nil {
				n, err := dst[i].Write(shard)
				if err != nil {
					return err
				}
				fmt.Printf("Wrote %d bytes to shard %d\n", n, i)
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

	bufs := make([][]byte, len(shards))
	enc, err := rs.New(e.dataShards, e.parityShards)
	if err != nil {
		return err
	}

	bytesLeft := outSize

	for {
		// Read shard blocks.
		for i, shard := range shards {
			if shard == nil {
				continue
			}

			n, err := shard.Read(bufs[i])
			fmt.Printf("Read %d bytes from shard %d\n", n, i)
			if err != nil {
				return fmt.Errorf("failed to read from shard %d: %s", i, err)
			}
		}

		fmt.Printf("%v\n", bufs)

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
		blockSize := int64(e.blockSize)
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
	}

	return nil
}
