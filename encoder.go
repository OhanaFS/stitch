package main

import (
	"crypto/rand"
	"errors"
	"io"
)

var (
	ErrNonSeekableWriter = errors.New("shards must support seeking")
)

type EncoderOptions struct {
	// DataShards is the total number of shards to split data into.
	DataShards uint
	// ParityShards is the total number of parity shards to create. This also
	// determines the maximum number of shards that can be lost before the data
	// cannot be recovered.
	ParityShards uint
	// KeyThreshold is the minimum number of shards required to reconstruct the
	// key used to encrypt the data.
	KeyThreshold uint
}

// Encoder takes in a stream of data and shards it into a specified number of
// data and parity shards. It includes compression using zstd, encryption using
// AES-GCM, and splitting the data into equal-sized shards using Reed-Solomon.
//
// It follows this process to encode the data into multiple shards:
// 1. Generate a random key Kr
// 2. Generate N output streams So_n
// 3. Generate a file header
// 4. Encrypt Kr with user-supplied key Ku, and embed it into the file header
// 5. Write the header to So_n
// 6. Take a byte stream of user-supplied data Sd and pipe it to the compressor C
// 7. Pipe the output of C into a streaming symmetric encryption method E, which
//    uses Kr to encrypt
// 8. Pipe the output of E into Reed-Solomon encoder to get N output streams RS_n
// 9. Pipe the output of RS_n to So_n
type Encoder struct {
	opts *EncoderOptions
}

func NewEncoder(opts *EncoderOptions) *Encoder {
	return &Encoder{
		opts: opts,
	}
}

func (e *Encoder) Encode(data io.Reader, shards []io.Writer, key []byte) error {
	// Check if the writers support seeking.
	for _, s := range shards {
		if _, ok := s.(io.Seeker); !ok {
			return ErrNonSeekableWriter
		}
	}

	// Prepare a 256-bit AES key for use in the data shards.
	fileKey := make([]byte, 32)
	if _, err := rand.Read(fileKey); err != nil {
		return err
	}

	// Encrypt the file key with the user-supplied key.
	// TODO

	return nil
}
