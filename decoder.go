package stitch

import (
	"crypto/aes"
	"crypto/cipher"
	"fmt"
	"io"
	"log"

	aesgcm "github.com/OhanaFS/stitch/aes"
	"github.com/OhanaFS/stitch/header"
	"github.com/OhanaFS/stitch/reedsolomon"
	"github.com/OhanaFS/stitch/util"
	seekable "github.com/SaveTheRbtz/zstd-seekable-format-go"
	"github.com/hashicorp/vault/shamir"
	"github.com/klauspost/compress/zstd"
)

func (e *Encoder) NewReadSeeker(shards []io.ReadSeeker, key []byte, iv []byte) (io.ReadSeeker, error) {
	totalShards := int(e.opts.DataShards + e.opts.ParityShards)

	// Check if the number of input readers matches the number of shards in the
	// encoder options.
	if len(shards) != totalShards {
		return nil, ErrShardCountMismatch
	}

	// Seek to the beginning of each shard.
	for i, shard := range shards {
		if _, err := shard.Seek(0, io.SeekStart); err != nil {
			return nil, fmt.Errorf("failed to seek to beginning of shard %d: %v", i, err)
		}
	}

	// Try to read the header from a shard.
	headerBuf := make([]byte, header.HeaderSize)
	headers := make([]header.Header, totalShards)
	hdr := header.Header{}
	for i, shard := range shards {
		if _, err := shard.Read(headerBuf); err != nil {
			log.Printf("Failed to read header from shard %d: %v", i, err)
			continue
		}
		if err := headers[i].Decode(headerBuf); err != nil {
			log.Printf("Failed to decode header from shard %d: %v", i, err)
			continue
		}
		// Sample a complete header
		if headers[i].IsComplete {
			hdr = headers[i]
		}
	}

	// Debug print the header as JSON
	log.Printf("Header: %+v", hdr)

	// Reconstruct the file key from the headers.
	var fileKeyPieces [][]byte
	for _, h := range headers {
		if !h.IsComplete {
			continue
		}
		fileKeyPieces = append(fileKeyPieces, h.FileKey)
	}
	if len(fileKeyPieces) < int(e.opts.KeyThreshold) {
		return nil, ErrNotEnoughKeyShards
	}

	// Combine the file key pieces.
	fileKeyCiphertext, err := shamir.Combine(fileKeyPieces)
	if err != nil {
		return nil, fmt.Errorf("failed to combine file key pieces: %v", err)
	}

	// Decrypt the file key with the user-supplied key.
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher for file key: %v", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create gcm cipher for file key: %v", err)
	}
	fileKey, err := gcm.Open(nil, iv, fileKeyCiphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt file key: %v", err)
	}

	// Seek shards to beginning of data.
	for i, shard := range shards {
		if _, err := shard.Seek(header.HeaderSize, io.SeekStart); err != nil {
			return nil, fmt.Errorf("failed to seek to beginning of data in shard %d: %v", i, err)
		}
	}

	// Prepare offset reader for shards
	shardData := make([]io.ReadSeeker, totalShards)
	for i, shard := range shards {
		shardData[i] = util.NewOffsetReader(shard, header.HeaderSize)
	}

	// Prepare the Reed-Solomon decoder.
	encRS, err := reedsolomon.NewEncoder(
		int(e.opts.DataShards), int(e.opts.ParityShards), hdr.RSBlockSize,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create Reed-Solomon encoder: %v", err)
	}
	rRS := reedsolomon.NewReadSeeker(encRS, shardData, int64(hdr.EncryptedSize))

	// Prepare the AES cipher to decrypt the data.
	rAES, err := aesgcm.NewReader(rRS, fileKey, hdr.AESBlockSize, hdr.CompressedSize)
	if err != nil {
		return nil, fmt.Errorf("failed to create AES reader: %v", err)
	}

	// Prepare the zstd decoder.
	decZstd, err := zstd.NewReader(nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create zstd decoder: %v", err)
	}
	rZstd, err := seekable.NewReader(rAES, decZstd)
	if err != nil {
		return nil, fmt.Errorf("failed to create zstd reader: %v", err)
	}

	// Limit the reader to the size of the plaintext.
	rLim := util.NewLimitReader(rZstd, int64(hdr.FileSize))

	// Return the reader.
	return rLim, nil
}