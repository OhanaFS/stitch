package stitch

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"io"

	"github.com/OhanaFS/stitch/header"
	"github.com/OhanaFS/stitch/reedsolomon"
	seekable "github.com/SaveTheRbtz/zstd-seekable-format-go"
	"github.com/hashicorp/vault/shamir"
	"github.com/klauspost/compress/zstd"
	"github.com/nicholastoddsmith/aesrw"
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
	_, err := rand.Read(fileKey)
	if err != nil {
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
	fileKeySplit, err := shamir.Split(
		fileKeyCiphertext, totalShards, int(e.opts.KeyThreshold),
	)
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
			FileHash:    make([]byte, 32),
			FileSize:    0,
			RSBlockSize: rsBlockSize,
		}

		// Get the current position of the writer.
		if headerOffsets[i], err = shards[i].Seek(0, io.SeekCurrent); err != nil {
			return err
		}

		// Write the header to the shard.
		b, err := headers[i].Encode()
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

	// Prepare the Reed-Solomon writer.
	wShards := make([]io.Writer, totalShards)
	for i, shard := range shards {
		wShards[i] = shard
	}
	wRS := encRS.NewWriter(wShards)

	// Prepare the AES writer.
	wAES, err := aesrw.NewWriter(wRS, fileKey)
	if err != nil {
		return err
	}

	// Prepare the zstd compressor.
	encZstd, err := zstd.NewWriter(nil)
	if err != nil {
		return err
	}
	wZstd, err := seekable.NewWriter(wAES, encZstd)
	if err != nil {
		return err
	}

	// Start encoding
	chunk := make([]byte, rsBlockSize)
	hash := sha256.New()
	fileSize := uint64(0)

	for {
		// Read a block of data
		n, err := data.Read(chunk)
		if err != nil {
			return err
		}
		fileSize += uint64(n)

		// Encode
		if _, err := wZstd.Write(chunk); err != nil {
			return err
		}

		// Update the hash
		if _, err := hash.Write(chunk); err != nil {
			return err
		}

		if n < rsBlockSize {
			break
		}
	}

	// Finalize the header
	digest := hash.Sum(nil)
	for i := 0; i < totalShards; i++ {
		headers[i].FileHash = digest
		headers[i].FileSize = fileSize

		// Seek the writers to the starting positions
		if _, err := shards[i].Seek(headerOffsets[i], io.SeekStart); err != nil {
			return err
		}

		// Write the header to the shard.
		b, err := headers[i].Encode()
		if err != nil {
			return err
		}
		if _, err := shards[i].Write(b); err != nil {
			return err
		}
	}

	return nil
}
