package stitch

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"io"

	"github.com/OhanaFS/stitch/header"
	"github.com/OhanaFS/stitch/reedsolomon"
	seekable "github.com/SaveTheRbtz/zstd-seekable-format-go"
	"github.com/hashicorp/vault/shamir"
	"github.com/klauspost/compress/zstd"
)

const (
	// rsBlockSize is the size of a Reed-Solomon block.
	rsBlockSize = 4096
)

var (
	ErrShardCountMismatch = errors.New("shard count mismatch")
	ErrNonSeekableWriter  = errors.New("shards must support seeking")
)

type EncoderOptions struct {
	// DataShards is the total number of shards to split data into.
	DataShards uint8
	// ParityShards is the total number of parity shards to create. This also
	// determines the maximum number of shards that can be lost before the data
	// cannot be recovered.
	ParityShards uint8
	// KeyThreshold is the minimum number of shards required to reconstruct the
	// key used to encrypt the data.
	KeyThreshold uint8
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
	return &Encoder{opts}
}

func (e *Encoder) Encode(data io.Reader, shards []io.WriteSeeker, key []byte, iv []byte) error {
	totalShards := int(e.opts.DataShards + e.opts.ParityShards)

	// Check if the number of output writers matches the number of shards in the
	// encoder options.
	if len(shards) != totalShards {
		return ErrShardCountMismatch
	}

	// Prepare a 256-bit AES key to encrypt the data.
	fileKey := make([]byte, 32)
	if _, err := rand.Read(fileKey); err != nil {
		return err
	}

	// Prepare a random IV to use for the AES-GCM cipher.
	fileIV := make([]byte, 12)
	if _, err := rand.Read(fileIV); err != nil {
		return err
	}

	// Encrypt the file key with the user-supplied key and iv.
	block, err := aes.NewCipher(key)
	if err != nil {
		return err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return err
	}
	fileKeyCiphertext := make([]byte, gcm.Overhead()+len(fileKey))
	gcm.Seal(fileKeyCiphertext[:0], iv, fileKey, nil)

	// Split the key and IV into shards.
	fileKeySplit, err := shamir.Split(fileKeyCiphertext, totalShards, int(e.opts.KeyThreshold))
	if err != nil {
		return err
	}
	fileIVSplit, err := shamir.Split(fileIV, totalShards, int(e.opts.KeyThreshold))
	if err != nil {
		return err
	}

	// Prepare headers for each shard.
	headers := make([]header.Header, totalShards)
	headerOffsets := make([]int64, totalShards)
	for i := 0; i < totalShards; i++ {
		headers[i] = header.Header{
			ShardIndex:  i,
			ShardCount:  totalShards,
			FileKey:     fileKeySplit[i],
			FileIV:      fileIVSplit[i],
			FileHash:    make([]byte, 32),
			FileSize:    0,
			RSBlockSize: rsBlockSize,
		}

		// Get the current position of the writer.
		if headerOffsets[i], err = shards[i].Seek(0, io.SeekCurrent); err != nil {
			return err
		}

		// Write the header to the shard.
		b, err := headers[i].MarshalBinary()
		if err != nil {
			return err
		}
		if _, err := shards[i].Write(b); err != nil {
			return err
		}
	}

	// Prepare the Reed-Solomon encoder.
	encRS, err := reedsolomon.NewEncoder(
		int(e.opts.DataShards), int(e.opts.ParityShards), rsBlockSize,
	)
	if err != nil {
		return err
	}

	// Prepare the zstd compressor.
	encZstd, err := zstd.NewWriter(nil)
	if err != nil {
		return err
	}
	_, err = seekable.NewWriter(io.Discard, encZstd)
	if err != nil {
		return err
	}

	return nil
}
