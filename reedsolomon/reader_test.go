package reedsolomon_test

import (
	"io"
	"testing"

	"github.com/OhanaFS/stitch/reedsolomon"
	"github.com/stretchr/testify/assert"
)

func testReadSeekerParam(t *testing.T, blockSize, dataShards, parityShards, dataSize, seekOffset int) {
	assert := assert.New(t)

	totalShards := dataShards + parityShards
	data := makeData(dataSize)

	shards, writers := makeShardBuffer(totalShards)

	t.Logf("Encoding %d bytes of data", len(data))
	t.Logf("Block size: %d", blockSize)
	t.Logf("Data shards: %d", dataShards)
	t.Logf("Parity shards: %d", parityShards)

	rs, err := reedsolomon.NewEncoder(dataShards, parityShards, blockSize)
	assert.Nil(err)

	// Encode the data
	w := reedsolomon.NewWriter(writers, rs)
	n, err := w.Write(data)
	assert.Nil(err)
	assert.Equal(len(data), n)
	assert.Nil(w.Close())

	readers := make([]io.ReadSeeker, len(shards))
	for i, shard := range shards {
		readers[i] = shard.BytesReader()
		n, err := shard.Seek(0, io.SeekEnd)
		assert.Nil(err)
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

	// Seek to some seekOffset
	n2, err := readSeeker.Seek(int64(seekOffset), io.SeekStart)
	assert.Nil(err)
	assert.Equal(int64(seekOffset), n2)

	// Read the data back in
	b, err = io.ReadAll(readSeeker)
	assert.Nil(err)
	assert.Equal(data[seekOffset:], b)

	// Seek from the end
	n2, err = readSeeker.Seek(int64(-seekOffset), io.SeekEnd)
	assert.Nil(err)
	assert.Equal(int64(len(data)-seekOffset), n2)

	// Read the data back in
	b, err = io.ReadAll(readSeeker)
	assert.Nil(err)
	assert.Equal(data[len(data)-seekOffset:], b)
}

func TestReadSeeker(t *testing.T) {
	testReadSeekerParam(t, 32, 4, 1, 1024, 0)
	testReadSeekerParam(t, 48, 3, 2, 2048, 512)
	// testReadSeekerParam(t, 4096, 2, 1, 4096, 0)
	// testReadSeekerParam(t, 4096, 17, 3, 1024*1024, 1234)
	// testReadSeekerParam(t, 2047, 13, 7, 1024*1024-3, 7777)
}
