package stitch_test

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"io"
	"testing"

	"github.com/OhanaFS/stitch"
	"github.com/OhanaFS/stitch/util"
	"github.com/stretchr/testify/assert"
)

func TestVerify(t *testing.T) {
	assert := assert.New(t)

	// Generate some input
	input := make([]byte, 16384)
	_, err := rand.Read(input)
	assert.NoError(err)

	inputBuffer := &bytes.Buffer{}
	inputBuffer.Write(input)
	shards := make([]*util.Membuf, 3)
	shardWriters := make([]io.Writer, 3)
	shardReaders := make([]io.ReadSeeker, 3)
	for i := 0; i < 3; i++ {
		shards[i] = util.NewMembuf()
		shardWriters[i] = shards[i]
		shardReaders[i] = shards[i]
	}

	// Create a new encoder.
	encoder := stitch.NewEncoder(&stitch.EncoderOptions{
		DataShards:   2,
		ParityShards: 1,
		KeyThreshold: 2,
	})

	key := []byte("11111111222222223333333344444444")
	iv := []byte("1234567890ab")

	// Hash the data
	hash := sha256.New()
	hash.Write(input)
	fileHash := hash.Sum(nil)

	// Encode the data.
	res, err := encoder.Encode(inputBuffer, shardWriters, key, iv)
	assert.NoError(err)
	assert.Equal(uint64(len(input)), res.FileSize)
	assert.Equal(fileHash, res.FileHash)

	// Finalize the file headers
	for _, shard := range shards {
		assert.NoError(encoder.FinalizeHeader(shard))
	}

	// Verify the shards
	for n, shard := range shards {
		shard.Seek(0, io.SeekStart)
		vres, err := stitch.VerifyShardIntegrity(shard)
		assert.NoError(err)
		assert.Equal(stitch.ShardVerificationResult{
			IsAvailable:      true,
			IsHeaderComplete: true,
			ShardIndex:       n,
			BlocksCount:      3,
			BlocksFound:      3,
			BrokenBlocks:     []int{},
		}, *vres)
	}

	// Damage the shards
	_, err = shards[1].Seek(1024, io.SeekStart) // Block 0
	assert.NoError(err)
	_, err = shards[1].Write([]byte("blah"))
	assert.NoError(err)
	_, err = shards[1].Seek(12345, io.SeekStart) // Block 2
	assert.NoError(err)
	_, err = shards[1].Write([]byte("asdf"))
	assert.NoError(err)

	shards[1].Seek(0, io.SeekStart)
	vres, err := stitch.VerifyShardIntegrity(shards[1])
	assert.NoError(err)
	assert.Equal(stitch.ShardVerificationResult{
		IsAvailable:      true,
		IsHeaderComplete: true,
		ShardIndex:       1,
		BlocksCount:      3,
		BlocksFound:      3,
		BrokenBlocks:     []int{0, 2},
	}, *vres)

	// Check all
	vires, err := encoder.VerifyIntegrity(shardReaders)
	assert.Nil(err)
	assert.Equal(3, vires.TotalShards)
	assert.Equal(false, vires.AllGood)
	assert.Equal(true, vires.FullyReadable)
	assert.Equal(3, len(vires.ByShard))
	assert.Equal(0, len(vires.IrrecoverableBlocks))

	// Damage another shard
	_, err = shards[2].Seek(1024, io.SeekStart) // Block 0
	assert.NoError(err)
	_, err = shards[2].Write([]byte("blah"))
	assert.NoError(err)
	_, err = shards[2].Seek(8192, io.SeekStart) // Block 1
	assert.NoError(err)
	_, err = shards[2].Write([]byte("blah"))
	assert.NoError(err)

	shards[2].Seek(0, io.SeekStart)
	vres, err = stitch.VerifyShardIntegrity(shards[2])
	assert.NoError(err)
	assert.Equal(stitch.ShardVerificationResult{
		IsAvailable:      true,
		IsHeaderComplete: true,
		ShardIndex:       2,
		BlocksCount:      3,
		BlocksFound:      3,
		BrokenBlocks:     []int{0, 1},
	}, *vres)

	// Block 0 should be irrecoverable
	vires, err = encoder.VerifyIntegrity(shardReaders)
	assert.Nil(err)
	assert.Equal(3, vires.TotalShards)
	assert.Equal(false, vires.AllGood)
	assert.Equal(false, vires.FullyReadable)
	assert.Equal(3, len(vires.ByShard))
	assert.Equal([]int{0}, vires.IrrecoverableBlocks)

	// Damage the header
	_, err = shards[1].Seek(0, io.SeekStart)
	assert.NoError(err)
	_, err = shards[1].Write([]byte("meow meow"))
	assert.NoError(err)

	// It should fail
	shards[1].Seek(0, io.SeekStart)
	vres, err = stitch.VerifyShardIntegrity(shards[1])
	assert.Nil(vres)
	assert.Error(err)

	// Overall should still be recoverable except block 0
	vires, err = encoder.VerifyIntegrity(shardReaders)
	assert.Nil(err)
	assert.Equal(3, vires.TotalShards)
	assert.Equal(false, vires.AllGood)
	assert.Equal(false, vires.FullyReadable)
	assert.Equal(3, len(vires.ByShard))
	assert.Equal([]int{0}, vires.IrrecoverableBlocks)
}
