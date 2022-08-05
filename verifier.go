package stitch

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"io"

	"github.com/OhanaFS/stitch/header"
)

type VerificationResult struct {
	// TotalShards is the total number of shards.
	TotalShards int
	// AllGood specifies whether all chunks of all shards are readable and has no
	// issues.
	AllGood bool
	// ByShard contains a breakdown of issues per shard.
	ByShard []ShardVerificationResult
}

type ShardVerificationResult struct {
	// IsAvailable specifies whether the shard is readable at all.
	IsAvailable bool
	// IsHeaderComplete specifies whether the header in the shard is marked as
	// complete. An incomplete header indicates either a corrupt header, or a
	// shard that hasn't been finalized.
	IsHeaderComplete bool
	// ShardIndex is the index of the shard as specified in the header.
	ShardIndex int
	// BlocksCount is the number of blocks that are supposed to be in the file as
	// calculated by the shard's header
	BlocksCount int
	// BlocksFound is the total number of blocks actually found in the shard.
	BlocksFound int
	// BrokenBlocks is a slice of block indices that are corrupted, starting from
	// zero.
	BrokenBlocks []int
}

// Rounds a number up to the next multiple
func roundUpMult(num, multiple int) int {
	if multiple == 0 {
		return num
	}

	remainder := num % multiple
	if remainder == 0 {
		return num
	}

	return num + multiple - remainder
}

// VerifyShardIntegrity tries to read through an entire shard, and report back
// any issues. If the shard is unreadable, an error will be returned.
func VerifyShardIntegrity(shard io.Reader) (*ShardVerificationResult, error) {
	result := &ShardVerificationResult{
		BrokenBlocks: []int{},
	}

	// Check if shard isn't nil
	if shard == nil {
		return nil, fmt.Errorf("shard is nil")
	}

	// Read the header
	headerBuf := make([]byte, header.HeaderSize)
	hdr := &header.Header{}
	if _, err := shard.Read(headerBuf); err != nil {
		return nil, fmt.Errorf("failed to read header: %w", err)
	}
	result.IsAvailable = true

	if err := hdr.Decode(headerBuf); err != nil {
		return nil, fmt.Errorf("failed to decode header: %w", err)
	}
	result.IsHeaderComplete = true

	result.ShardIndex = hdr.ShardIndex
	totalBlocksAcrossAllShards := 1 + int(hdr.EncryptedSize/uint64(hdr.RSBlockSize))
	result.BlocksCount = roundUpMult(
		totalBlocksAcrossAllShards/hdr.ShardCount,
		hdr.ShardCount,
	)

	// Read each chunk
	block := make([]byte, hdr.RSBlockSize)
	hash := make([]byte, sha256.Size)
	iBlk := 0
	for {
		// Read block and hash
		if _, err := shard.Read(block); err != nil {
			if err == io.EOF {
				break
			}
			return nil, fmt.Errorf("failed to read block: %w", err)
		}
		if _, err := shard.Read(hash); err != nil {
			if err == io.EOF {
				break
			}
			return nil, fmt.Errorf("failed to read block hash: %w", err)
		}

		// Verify the hash
		computedHash := sha256.Sum256(block)
		if !bytes.Equal(hash, computedHash[:]) {
			// Mark the block as broken
			result.BrokenBlocks = append(result.BrokenBlocks, iBlk)
		}

		// Update the count of blocks found
		iBlk += 1
		result.BlocksFound = iBlk
	}

	return result, nil
}

// VerifyIntegrity tries to read and verify the integrity of all the provided
// shards. An error is returned if it is not possible to recover the original
// file.
func (e *Encoder) VerifyIntegrity(shards []io.ReadSeeker) (*VerificationResult, error) {
	totalShards := int(e.opts.DataShards + e.opts.ParityShards)
	result := &VerificationResult{
		TotalShards: totalShards,
		ByShard:     make([]ShardVerificationResult, totalShards),
		AllGood:     true,
	}

	// Check if there are sufficient input shards
	if len(shards) < int(e.opts.DataShards) {
		return nil, ErrNotEnoughShards
	}

	missingCount := 0
	shardResults := make([]*ShardVerificationResult, totalShards)
	for i, shard := range shards {
		// Seek to the beginning of each shard.
		if _, err := shard.Seek(0, io.SeekStart); err != nil {
			return nil, fmt.Errorf("failed to seek to beginning of shard %d: %v", i, err)
		}

		// Verify each shard individually
		res, err := VerifyShardIntegrity(shard)
		if err != nil {
			missingCount++
			result.AllGood = false
		} else {
			shardResults[i] = res
		}
	}

	// Check if there are sufficient shards
	if missingCount > int(e.opts.ParityShards) {
		return nil, ErrNotEnoughShards
	}

	// Check if the shards have any issues
	for _, res := range shardResults {
		if res == nil {
			continue
		}

		if res.BlocksCount != res.BlocksFound || len(res.BrokenBlocks) > 0 {
			result.AllGood = false
		}
	}

	return result, nil
}
