package reedsolomon

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"io"

	rs "github.com/klauspost/reedsolomon"
)

const (
	// BlockOverhead specifies the number of extra bytes required to encode a
	// block of data.
	BlockOverhead = sha256.Size
)

type ErrCorruptionDetected struct {
	BlockCount int
}

var _ error = ErrCorruptionDetected{}

func (e ErrCorruptionDetected) Error() string {
	return fmt.Sprintf("detected corruption in %d blocks", e.BlockCount)
}

// Encoder encodes Reed-Solomon parity sets from a stream of unknown length.
type Encoder struct {
	DataShards   int
	ParityShards int
	BlockSize    int
	encoder      rs.Encoder
}

func NewEncoder(dataShards, parityShards, blockSize int) (*Encoder, error) {
	enc, err := rs.New(dataShards, parityShards)
	if err != nil {
		return nil, err
	}

	return &Encoder{
		DataShards:   dataShards,
		ParityShards: parityShards,
		BlockSize:    blockSize,
		encoder:      enc,
	}, nil
}

type Writer struct {
	dst []io.Writer
	enc *Encoder

	buffer  bytes.Buffer
	read    uint64
	written uint64
}

// Assert that Writer implements the io.WriteCloser interface.
var _ io.WriteCloser = &Writer{}

// NewWriter creates a new Writer.
func NewWriter(dst []io.Writer, enc *Encoder) *Writer {
	return &Writer{
		dst: dst,
		enc: enc,
	}
}

// Write splits the data into the number of shards given to it, and writes the
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
func (w *Writer) Write(p []byte) (n int, err error) {
	// Append p to the buffer.
	n, err = w.buffer.Write(p)
	w.read += uint64(n)
	if err != nil {
		return n, err
	}

	// Process the buffer until there's not enough data to process
	readSize := w.enc.BlockSize * w.enc.DataShards
	chunk := make([]byte, readSize)
	for {
		if w.buffer.Len() < readSize {
			break
		}

		// Read a chunk
		n, err = w.buffer.Read(chunk)
		if err != nil {
			return n, err
		}

		// Split the block into shards.
		shards, err := w.enc.encoder.Split(chunk[:n])
		if err != nil {
			return n, err
		}

		// Encode parity.
		if err = w.enc.encoder.Encode(shards); err != nil {
			return n, err
		}

		// Write the shards to the destination.
		for i, shard := range shards {
			if w.dst[i] != nil {
				// Calculate the hash of the shard.
				hash := sha256.Sum256(shard)

				// Write the shards and the hash to the destination.
				n, err := w.dst[i].Write(shard)
				if err != nil {
					return n, err
				}
				w.written += uint64(n)

				n, err = w.dst[i].Write(hash[:])
				if err != nil {
					return n, err
				}
				w.written += uint64(n)
			}
		}
	}

	// Clean up the buffer.
	b := bytes.Buffer{}
	b.Write(w.buffer.Bytes())
	w.buffer = b

	return len(p), nil
}

// Close implements io.WriteCloser
func (w *Writer) Close() error {
	chunk := w.buffer.Bytes()

	// Do nothing if there's no data to process.
	if len(chunk) == 0 {
		return nil
	}

	// Pad the chunk to the block size.
	readSize := w.enc.BlockSize * w.enc.DataShards
	if len(chunk) < readSize {
		padding := make([]byte, readSize-len(chunk))
		if _, err := rand.Read(padding); err != nil {
			return err
		}
		chunk = append(chunk, padding...)
	}

	// Split the block into shards.
	shards, err := w.enc.encoder.Split(chunk)
	if err != nil {
		return err
	}

	// Encode parity.
	if err = w.enc.encoder.Encode(shards); err != nil {
		return err
	}

	// Write the shards to the destination.
	for i, shard := range shards {
		if w.dst[i] != nil {
			// Calculate the hash of the shard.
			hash := sha256.Sum256(shard)

			// Write the shards and the hash to the destination.
			if _, err := w.dst[i].Write(shard); err != nil {
				return err
			}
			if _, err := w.dst[i].Write(hash[:]); err != nil {
				return err
			}
		}
	}

	return nil
}

// Join reconstructs the data from the shards given to it. If it detects that
// some of the shards are corrupted, but is able to correct them, it should return
// ErrCorruptionDetected.
func (e *Encoder) Join(dst io.Writer, shards []io.Reader, outSize int64) error {
	totalShards := e.DataShards + e.ParityShards
	if len(shards) != totalShards {
		return fmt.Errorf("expected %d shards, got %d", totalShards, len(shards))
	}

	// Allocate buffers for the data shards.
	bufs := make([][]byte, len(shards))
	for i := range bufs {
		bufs[i] = make([]byte, e.BlockSize)
	}

	hashes := make([][]byte, len(shards))
	for i := range hashes {
		hashes[i] = make([]byte, sha256.Size)
	}

	// Initialize the Reed-Solomon decoder.
	enc, err := rs.New(e.DataShards, e.ParityShards)
	if err != nil {
		return err
	}

	// Keep track of the number of bytes left to be written to the output.
	bytesLeft := outSize
	// Keep track of the number of blocks that are corrupted.
	brokenBlocks := 0
	currentBlock := -1

	for {
		currentBlock += 1

		// Read shard blocks.
		for i, shard := range shards {
			if shard == nil {
				continue
			}

			if _, err := shard.Read(bufs[i]); err != nil {
				return fmt.Errorf("failed to read from shard %d, block %d: %w", i, currentBlock, err)
			}

			if _, err := shard.Read(hashes[i]); err != nil {
				return fmt.Errorf("failed to read hash from shard %d, block %d: %w", i, currentBlock, err)
			}

			// Verify the hash.
			hash := sha256.Sum256(bufs[i])
			if !bytes.Equal(hashes[i], hash[:]) {
				// If hashes don't match, truncate the shard so that `enc.Reconstruct`
				// will regenerate it.
				bufs[i] = []byte{}
				brokenBlocks++
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
		blockSize := int64(e.BlockSize) * int64(e.DataShards)
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
			bufs[i] = make([]byte, e.BlockSize)
		}
	}

	return nil
}

// NewReader wraps the Join method and returns a new io.ReadCloser.
func (e *Encoder) NewReader(shards []io.Reader, outSize int64) io.ReadCloser {
	r, w := io.Pipe()
	go func() {
		if err := e.Join(w, shards, outSize); err != nil {
			w.CloseWithError(err)
		} else {
			w.Close()
		}
	}()
	return r
}
