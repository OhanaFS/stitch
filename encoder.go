package stitch

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"time"

	aesgcm "github.com/OhanaFS/stitch/aes"
	"github.com/OhanaFS/stitch/header"
	"github.com/OhanaFS/stitch/reedsolomon"
	seekable "github.com/SaveTheRbtz/zstd-seekable-format-go"
	"github.com/hashicorp/vault/shamir"
	"github.com/klauspost/compress/zstd"
)

const (
	// rsBlockSize is the size of a Reed-Solomon block.
	rsBlockSize = 4096
	// aesBlockSize is the size of a chunk of data that is encrypted with AES-GCM.
	aesBlockSize = 4096
)

var (
	ErrShardCountMismatch = errors.New("shard count mismatch")
	ErrNonSeekableWriter  = errors.New("shards must support seeking")
	ErrNotEnoughKeyShards = errors.New("not enough shards to reconstruct the file key")
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

func (e *Encoder) Encode(data io.Reader, shards []io.Writer, key []byte, iv []byte) error {
	totalShards := int(e.opts.DataShards + e.opts.ParityShards)
	log.Printf("[DEBUG] Encoding %d shards", totalShards)

	// Check if the number of output writers matches the number of shards in the
	// encoder options.
	if len(shards) != totalShards {
		return ErrShardCountMismatch
	}

	// Prepare a 256-bit AES key to encrypt the data.
	fileKey := make([]byte, 32)
	if _, err := rand.Read(fileKey); err != nil {
		return fmt.Errorf("failed to generate file key: %v", err)
	}

	// Encrypt the file key with the user-supplied key and iv.
	block, err := aes.NewCipher(key)
	if err != nil {
		return fmt.Errorf("failed to create AES cipher: %v", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return fmt.Errorf("failed to create AES-GCM: %v", err)
	}
	fileKeyCiphertext := gcm.Seal(nil, iv, fileKey, nil)

	// Split the key into shards.
	fileKeySplit, err := shamir.Split(
		fileKeyCiphertext, totalShards, int(e.opts.KeyThreshold),
	)
	if err != nil {
		return fmt.Errorf("failed to split file key: %v", err)
	}

	// Prepare headers for each shard.
	headers := make([]header.Header, totalShards)
	for i := 0; i < totalShards; i++ {
		headers[i] = header.Header{
			ShardIndex:     i,
			ShardCount:     totalShards,
			FileKey:        fileKeySplit[i],
			FileHash:       make([]byte, 32),
			FileSize:       0,
			EncryptedSize:  0,
			CompressedSize: 0,
			RSBlockSize:    rsBlockSize,
			AESBlockSize:   aesBlockSize,
			IsComplete:     false,
		}

		// Write the header to the shard.
		b, err := headers[i].Encode()
		if err != nil {
			return fmt.Errorf("failed to encode header: %v", err)
		}
		if _, err := shards[i].Write(b); err != nil {
			return fmt.Errorf("failed to write header: %v", err)
		}
	}

	// Prepare the Reed-Solomon encoder.
	encRS, err := reedsolomon.NewEncoder(
		int(e.opts.DataShards), int(e.opts.ParityShards), rsBlockSize,
	)
	if err != nil {
		return fmt.Errorf("failed to create Reed-Solomon encoder: %v", err)
	}

	// Prepare the Reed-Solomon writer.
	wRS := encRS.NewWriter(shards)

	// Prepare the AES writer.
	wAES, err := aesgcm.NewWriter(wRS, fileKey, aesBlockSize)
	if err != nil {
		return fmt.Errorf("failed to create AES writer: %v", err)
	}

	// Prepare the zstd compressor.
	encZstd, err := zstd.NewWriter(nil)
	if err != nil {
		return fmt.Errorf("failed to create zstd writer: %v", err)
	}
	wZstd, err := seekable.NewWriter(wAES, encZstd)
	if err != nil {
		return fmt.Errorf("failed to create zstd writer: %v", err)
	}

	// Start encoding
	chunk := make([]byte, rsBlockSize)
	hash := sha256.New()
	fileSize := uint64(0)

	for {
		// Read a block of data
		n, err := data.Read(chunk)
		if err != nil {
			if err == io.EOF {
				break
			}
			return fmt.Errorf("failed to read data: %v", err)
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

	// Close the writers
	if err := wZstd.Close(); err != nil {
		return err
	}
	if err := encZstd.Close(); err != nil {
		return err
	}
	if err := wAES.Close(); err != nil {
		return err
	}
	if err := wRS.Close(); err != nil {
		return err
	}

	// TODO: rewrite the Reed-Solomon encoder to be a Writer interface without
	// io.Pipe. This time.Sleep is a quick workaround to allow for the encoder to
	// finish writing the last block.
	log.Println("Sleeping for 100ms")
	time.Sleep(time.Millisecond * 100)
	log.Printf("[DEBUG] Encoded %d bytes", fileSize)

	// Write the complete header to the end of the file.
	digest := hash.Sum(nil)
	for i := 0; i < totalShards; i++ {
		headers[i].FileHash = digest
		headers[i].FileSize = fileSize
		// TODO: make this less hacky
		headers[i].EncryptedSize = wAES.(*aesgcm.AESWriter).GetWritten()
		headers[i].CompressedSize = wAES.(*aesgcm.AESWriter).GetRead()
		headers[i].IsComplete = true

		log.Printf("[DEBUG] Header %d: %+v", i, headers[i])

		// Try to seek to the end of the file
		// TODO: this shouldn't be needed anymore after the Reed-Solomon encoder
		// is rewritten to be a Writer without io.Pipe
		if seeker, ok := shards[i].(io.WriteSeeker); ok {
			if _, err := seeker.Seek(0, io.SeekEnd); err != nil {
				return fmt.Errorf("failed to seek to end of file: %v", err)
			}
		}

		// Write the updated header to the end of the shard.
		b, err := headers[i].Encode()
		if err != nil {
			return err
		}
		n, err := shards[i].Write(b)
		if err != nil {
			return err
		}
		log.Printf("Wrote %d bytes footer to shard %d", n, i)
	}

	return nil
}

// FinalizeHeader rewrites the shard header with the one located at the end of
// the shard.
func (e *Encoder) FinalizeHeader(shard io.ReadWriteSeeker) error {
	// Seek to the start of the shard.
	if _, err := shard.Seek(0, io.SeekStart); err != nil {
		return fmt.Errorf("failed to seek to start of shard: %v", err)
	}

	// Try to read the header at the start
	headerBuf := make([]byte, header.HeaderSize)
	if _, err := shard.Read(headerBuf); err != nil {
		return fmt.Errorf("failed to read header at start of shard: %v", err)
	}

	// Parse the header at the start
	hdr := header.NewHeader()
	if err := hdr.Decode(headerBuf); err != nil {
		return fmt.Errorf("failed to decode header at start of shard: %v", err)
	}

	// Skip if the header is already complete
	if hdr.IsComplete {
		return nil
	}

	// Seek to the end of the shard
	hdrOffset, err := shard.Seek(-int64(header.HeaderSize), io.SeekEnd)
	if err != nil {
		return fmt.Errorf("failed to seek to end of shard: %v", err)
	}

	// Read the header at the end
	if _, err := shard.Read(headerBuf); err != nil {
		return fmt.Errorf("failed to read header at end of shard: %v", err)
	}

	// Parse the header at the end
	if err := hdr.Decode(headerBuf); err != nil {
		return fmt.Errorf("failed to decode header at end of shard: %v", err)
	}

	// Make sure the header is complete
	if !hdr.IsComplete {
		return header.ErrHeaderNotComplete
	}

	// Rewrite the header at the start
	if _, err := shard.Seek(0, io.SeekStart); err != nil {
		return fmt.Errorf("failed to seek to start of shard: %v", err)
	}
	if _, err := shard.Write(headerBuf); err != nil {
		return fmt.Errorf("failed to write header at start of shard: %v", err)
	}

	// Try to truncate the ending header
	if file, ok := shard.(*os.File); ok {
		if err := file.Truncate(hdrOffset); err != nil {
			return fmt.Errorf("failed to truncate shard: %v", err)
		}
	}

	return nil
}
