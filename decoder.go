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

// readHeader reads the header from the shards. It returns the index of any
// complete header, a slice of the headers, and a slice of correctly-positioned
// readers.
//
// The slice of headers is not necessarily in the same order as the shards.
func readHeader(shards []io.ReadSeeker, totalShards int) (
	okIdx int, headers []header.Header, shardReaders []io.ReadSeeker, err error,
) {
	// Allocate a buffer to read the header into.
	headerBuf := make([]byte, header.HeaderSize)
	// Create a slice to hold the headers.
	headers = make([]header.Header, totalShards)
	// Create a slice to hold the correctly-indexed shard readers.
	shardReaders = make([]io.ReadSeeker, totalShards)
	// okIdx is the index of any shard that has a valid header.
	okIdx = -1

	// Seek to the beginning of each shard.
	for i, shard := range shards {
		if _, e := shard.Seek(0, io.SeekStart); e != nil {
			err = fmt.Errorf("failed to seek to beginning of shard %d: %v", i, e)
			return
		}
	}

	for i, shard := range shards {
		// Try to read the shard
		if _, err := shard.Read(headerBuf); err != nil {
			continue
		}

		// Try to parse the header.
		if err := headers[i].Decode(headerBuf); err != nil {
			continue
		}

		// If the header is valid, set the okIdx and append the shard to the shard
		// readers slice, according to the index in the header.
		if headers[i].IsComplete && headers[i].ShardIndex < totalShards {
			shardReaders[headers[i].ShardIndex] = shard
			okIdx = i
		}
	}

	// Return an error if no valid header was found.
	if okIdx == -1 {
		err = ErrNoCompleteHeader
	}

	return
}

// combineHeaderKeys combines the keys from the header and decrypts it with the
// supplied key and iv.
func combineHeaderKeys(headers []header.Header, key, iv []byte) ([]byte, error) {
	// Gather the key pieces into a slice of byte slices.
	var fileKeyPieces [][]byte
	for _, h := range headers {
		if !h.IsComplete {
			continue
		}
		fileKeyPieces = append(fileKeyPieces, h.FileKey)
	}

	// Combine the key pieces into a single encrypted key.
	ciphertext, err := shamir.Combine(fileKeyPieces)
	if err != nil {
		return nil, fmt.Errorf("failed to combine header keys: %v", err)
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
	fileKey, err := gcm.Open(nil, iv, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt file key: %v", err)
	}

	return fileKey, nil
}

// NewReadSeeker returns a new ReadSeeker that can be used to access the data
// contained within the shards.
func (e *Encoder) NewReadSeeker(shards []io.ReadSeeker, key []byte, iv []byte) (
	io.ReadSeeker, error,
) {
	totalShards := int(e.opts.DataShards + e.opts.ParityShards)

	// Check if there are sufficient input shards
	if len(shards) < int(e.opts.DataShards) {
		return nil, ErrNotEnoughShards
	}

	// Try to read the shard headers.
	okIdx, headers, shardReaders, err := readHeader(shards, totalShards)
	if err != nil {
		return nil, fmt.Errorf("failed to read header: %v", err)
	}
	hdr := headers[okIdx]

	// Pad nil readers
	for i, reader := range shardReaders {
		if reader == nil {
			log.Printf("[WARN] Missing shard %d", i)
			shardReaders[i] = &util.ZeroReadSeeker{Size: int64(hdr.EncryptedSize)}
		}
	}

	// Reconstruct and decrypt the encrypted file key from the headers.
	fileKey, err := combineHeaderKeys(headers, key, iv)
	if err != nil {
		return nil, fmt.Errorf("failed to combine file key pieces: %v", err)
	}

	// Seek shards to beginning of data.
	for i, reader := range shardReaders {
		if _, err := reader.Seek(header.HeaderSize, io.SeekStart); err != nil {
			return nil, fmt.Errorf("failed to seek to beginning of data in shard %d: %v", i, err)
		}
	}

	// Prepare offset reader for shards
	shardData := make([]io.ReadSeeker, totalShards)
	for i, reader := range shardReaders {
		shardData[i] = util.NewOffsetReader(reader, header.HeaderSize)
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
