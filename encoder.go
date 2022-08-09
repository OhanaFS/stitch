package stitch

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"io"
	"os"

	aesgcm "github.com/OhanaFS/stitch/aes"
	"github.com/OhanaFS/stitch/header"
	"github.com/OhanaFS/stitch/reedsolomon"
	seekable "github.com/SaveTheRbtz/zstd-seekable-format-go"
	"github.com/hashicorp/vault/shamir"
	"github.com/klauspost/compress/zstd"
)

// Encode takes in a reader, performs the transformations and then splits the
// data into multiple shards, writing them to the output writers. The output
// writers are not closed after the data is written.
//
// After the data has finished encoding, a header will be written to the end of
// each shard. At this point, the shards are not usable yet until the header is
// finalized using the FinalizeHeader() function.
func (e *Encoder) Encode(data io.Reader, shards []io.Writer, key []byte, iv []byte) (*EncodingResult, error) {
	totalShards := int(e.opts.DataShards + e.opts.ParityShards)

	// Check if the number of output writers matches the number of shards in the
	// encoder options.
	if len(shards) != totalShards {
		return nil, ErrShardCountMismatch
	}

	// Prepare a 256-bit AES key to encrypt the data.
	fileKey := make([]byte, 32)
	if _, err := rand.Read(fileKey); err != nil {
		return nil, fmt.Errorf("failed to generate file key: %v", err)
	}

	// Encrypt the file key with the user-supplied key and iv.
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create AES cipher: %v", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create AES-GCM: %v", err)
	}
	fileKeyCiphertext := gcm.Seal(nil, iv, fileKey, nil)

	// Split the key into shards.
	fileKeySplit, err := shamir.Split(
		fileKeyCiphertext, totalShards, int(e.opts.KeyThreshold),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to split file key: %v", err)
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
			return nil, fmt.Errorf("failed to encode header: %v", err)
		}
		if _, err := shards[i].Write(b); err != nil {
			return nil, fmt.Errorf("failed to write header: %v", err)
		}
	}

	// Prepare the Reed-Solomon encoder.
	encRS, err := reedsolomon.NewEncoder(
		int(e.opts.DataShards), int(e.opts.ParityShards), rsBlockSize,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create Reed-Solomon encoder: %v", err)
	}

	// Prepare the Reed-Solomon writer.
	wRS := reedsolomon.NewWriter(shards, encRS)

	// Prepare the AES writer.
	wAES, err := aesgcm.NewWriter(wRS, fileKey, aesBlockSize)
	if err != nil {
		return nil, fmt.Errorf("failed to create AES writer: %v", err)
	}

	// Prepare the zstd compressor.
	encZstd, err := zstd.NewWriter(nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create zstd writer: %v", err)
	}
	wZstd, err := seekable.NewWriter(wAES, encZstd)
	if err != nil {
		return nil, fmt.Errorf("failed to create zstd writer: %v", err)
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
			return nil, fmt.Errorf("failed to read data: %v", err)
		}
		fileSize += uint64(n)

		// Encode
		if _, err := wZstd.Write(chunk[:n]); err != nil {
			return nil, err
		}

		// Update the hash
		if _, err := hash.Write(chunk[:n]); err != nil {
			return nil, err
		}

		if n < rsBlockSize {
			break
		}
	}

	// Close the writers
	if err := wZstd.Close(); err != nil {
		return nil, err
	}
	if err := encZstd.Close(); err != nil {
		return nil, err
	}
	if err := wAES.Close(); err != nil {
		return nil, err
	}
	if err := wRS.Close(); err != nil {
		return nil, err
	}

	// Write the complete header to the end of the file.
	digest := hash.Sum(nil)
	for i := 0; i < totalShards; i++ {
		headers[i].FileHash = digest
		headers[i].FileSize = fileSize
		headers[i].EncryptedSize = wAES.(*aesgcm.AESWriter).GetWritten()
		headers[i].CompressedSize = wAES.(*aesgcm.AESWriter).GetRead()
		headers[i].IsComplete = true

		// Write the updated header to the end of the shard.
		b, err := headers[i].Encode()
		if err != nil {
			return nil, err
		}
		if _, err := shards[i].Write(b); err != nil {
			return nil, err
		}
	}

	return &EncodingResult{
		FileSize: fileSize,
		FileHash: digest,
	}, nil
}

// FinalizeHeader rewrites the shard header with the one located at the end of
// the shard. If the provided shard is an *os.File, the header at the end of the
// file will be truncated.
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
