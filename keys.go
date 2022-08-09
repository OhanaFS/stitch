package stitch

import (
	"fmt"
	"io"
)

// RotateKeys reads the header from the supplied shards, reconstructs the file
// key, and then decrypts it with the supplied key and iv. It will then
// re-encrypt it with the new key and iv, and split them with Shamir's Secret
// Sharing Scheme. The resulting key splits are then returned.
//
// The caller must then update each shard's header to use the new key splits.
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
