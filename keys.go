package stitch

import (
	"fmt"
	"io"

	"github.com/OhanaFS/stitch/header"
)

// RotateKeys reads the header from the supplied shards, reconstructs the file
// key, and then decrypts it with the supplied key and iv. It will then
// re-encrypt it with the new key and iv, and split them with Shamir's Secret
// Sharing Scheme. The resulting key splits are then returned.
//
// The caller must then use the UpdateShardKey() function to update each shard's
// header to use the new key splits.
func (e *Encoder) RotateKeys(shards []io.ReadSeeker,
	previousKey, previousIv, newKey, newIv []byte) ([][]byte, error) {
	totalShards := int(e.opts.DataShards + e.opts.ParityShards)

	// Check if there are sufficient input shards
	if len(shards) < int(e.opts.DataShards) {
		return nil, ErrNotEnoughShards
	}

	// Try to read the shard headers.
	_, headers, _, err := readHeader(shards, totalShards)
	if err != nil {
		return nil, fmt.Errorf("failed to read header: %v", err)
	}

	// Combine the header keys to get the encrypted file key.
	fileKey, err := combineHeaderKeys(headers, previousKey, previousIv)
	if err != nil {
		return nil, fmt.Errorf("failed to combine header keys: %v", err)
	}

	// Split the file key with the new key.
	keySplits, err := splitFileKey(fileKey, newKey, newIv,
		totalShards, int(e.opts.KeyThreshold))
	if err != nil {
		return nil, fmt.Errorf("failed to split file key: %v", err)
	}

	return keySplits, nil
}

// UpdateShardKey updates the header of the supplied shard with the new key
// split. The header is then written to the shard. To obtain a new key split,
// use the RotateKeys() function.
func (*Encoder) UpdateShardKey(shard io.ReadWriteSeeker, newKeySplit []byte) error {
	// Seek to the beginning of the shard.
	if _, err := shard.Seek(0, io.SeekStart); err != nil {
		return fmt.Errorf("failed to seek to beginning of shard: %v", err)
	}

	// Read the header.
	buf := make([]byte, header.HeaderSize)
	if _, err := shard.Read(buf); err != nil {
		return fmt.Errorf("failed to read header: %v", err)
	}

	// Parse the header
	hdr := header.NewHeader()
	if err := hdr.Decode(buf); err != nil {
		return fmt.Errorf("failed to decode header: %v", err)
	}

	// Update the header with the new key split.
	hdr.FileKey = newKeySplit
	newHeader, err := hdr.Encode()
	if err != nil {
		return fmt.Errorf("failed to encode header: %v", err)
	}

	// Seek to the beginning of the shard.
	if _, err := shard.Seek(0, io.SeekStart); err != nil {
		return fmt.Errorf("failed to seek to beginning of shard: %v", err)
	}

	// Write the new header.
	if _, err := shard.Write(newHeader); err != nil {
		return fmt.Errorf("failed to write header: %v", err)
	}

	return nil
}
