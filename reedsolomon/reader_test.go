package reedsolomon_test

import (
	"bytes"
	"io"
	"testing"

	"github.com/OhanaFS/stitch/reedsolomon"
	"github.com/stretchr/testify/assert"
)

func TestReadSeeker(t *testing.T) {
	assert := assert.New(t)

	blockSize := 256
	dataShards := 3
	parityShards := 1

	totalShards := dataShards + parityShards
	data := makeData(blockSize * 4)

	shards, writers := makeShardBuffer(totalShards)

	t.Logf("Encoding %d bytes of data", len(data))
	t.Logf("Block size: %d", blockSize)
	t.Logf("Data shards: %d", dataShards)
	t.Logf("Parity shards: %d", parityShards)

	rs, err := reedsolomon.NewEncoder(dataShards, parityShards, blockSize)
	assert.Nil(err)

	// Encode the data
	err = rs.Split(bytes.NewReader(data), writers)
	assert.Nil(err)

	readers := make([]io.ReadSeeker, len(shards))
	for i, shard := range shards {
		readers[i] = shard.BytesReader()
		n, err := shard.Seek(0, io.SeekEnd)
		assert.Nil(err)
		// assert.GreaterOrEqual(n, int64((blockSize+reedsolomon.BlockOverhead)*10/dataShards))
		t.Logf("Shard %d: %d bytes = %d blocks\n", i, n, n/int64(blockSize+reedsolomon.BlockOverhead))
		_, err = shard.Seek(0, io.SeekStart)
		assert.Nil(err)
	}

	// Now read the shards back in
	readSeeker := reedsolomon.NewReadSeeker(rs, readers, int64(len(data)))
	assert.NotNil(readSeeker)

	// Read the data back in
	b, err := io.ReadAll(readSeeker)
	assert.Nil(err)
	assert.Equal(data, b)
	// assert.Equal(len(data), len(b))

	// Seek to some offset
	offset := int64(blockSize + 32)
	n, err := readSeeker.Seek(offset, io.SeekStart)
	assert.Nil(err)
	assert.Equal(offset, n)
}
