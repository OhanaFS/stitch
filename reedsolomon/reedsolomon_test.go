package reedsolomon_test

import (
	"bytes"
	"crypto/rand"
	"io"
	"testing"

	"github.com/orcaman/writerseeker"
	"github.com/stretchr/testify/assert"

	"github.com/OhanaFS/stitch/reedsolomon"
)

func TestReedSolomon(t *testing.T) {
	assert := assert.New(t)

	blockSize := 32
	dataShards := 5
	parityShards := 2
	totalShards := dataShards + parityShards

	rs, err := reedsolomon.New(dataShards, parityShards, blockSize)
	assert.Nil(err)

	// Generate some data
	data := makeRandomData(t, blockSize*10)

	// Create buffers to hold the shards
	shards, writers := makeShardBuffer(totalShards)

	// Encode the data
	err = rs.Split(bytes.NewReader(data), writers)
	assert.Nil(err)

	// Grab the reader
	readers := getReadersFromShards(t, blockSize, shards)

	// Try to decode the data
	dest := &writerseeker.WriterSeeker{}
	err = rs.Join(dest, readers, int64(len(data)))
	assert.Nil(err)

	// Check that the data is correct
	b, err := io.ReadAll(dest.BytesReader())
	assert.Nil(err)
	assert.Equal(data, b)

	// Corrupt one of the shards
	readers = make([]io.Reader, totalShards)
	for i := 0; i < totalShards; i++ {
		if i == 5 {
			// Seek to the beginning of the buffer
			n, err := shards[i].Seek(0, io.SeekStart)
			assert.Nil(err)
			assert.Equal(int64(0), n)

			// Corrupt the data
			shards[i].Write([]byte("never gonna give you up"))
		}

		n, err := shards[i].Seek(0, io.SeekStart)
		assert.Nil(err)
		assert.Equal(int64(0), n)
		readers[i] = shards[i].BytesReader()
	}

	// Try to decode the data
	dest = &writerseeker.WriterSeeker{}
	err = rs.Join(dest, readers, int64(len(data)))
	assert.Equal(reedsolomon.ErrCorruptionDetected{BlockCount: 1}, err)
}

func TestReedSolomonLarge(t *testing.T) {
	assert := assert.New(t)

	blockSize := 1024 * 1024
	dataShards := 17
	parityShards := 3
	totalShards := dataShards + parityShards

	rs, err := reedsolomon.New(dataShards, parityShards, blockSize)
	assert.Nil(err)

	// Generate some data
	data := makeRandomData(t, blockSize*10)

	// Create buffers to hold the shards
	shards, writers := makeShardBuffer(totalShards)

	// Encode the data
	err = rs.Split(bytes.NewReader(data), writers)
	assert.Nil(err)

	// Try to decode the data
	readers := getReadersFromShards(t, blockSize, shards)
	dest := &writerseeker.WriterSeeker{}
	err = rs.Join(dest, readers, int64(len(data)))
	assert.Nil(err)

	// Check that the data is correct
	b, err := io.ReadAll(dest.BytesReader())
	assert.Nil(err)
	assert.Equal(data, b)
}

func TestReaderWriter(t *testing.T) {
	assert := assert.New(t)

	blockSize := 1024
	dataShards := 3
	parityShards := 1
	totalShards := dataShards + parityShards

	rs, err := reedsolomon.New(dataShards, parityShards, blockSize)
	assert.Nil(err)

	// Generate some data
	data := makeRandomData(t, blockSize*10)

	// Create buffers to hold the shards
	shards, writers := makeShardBuffer(totalShards)

	// Grab the writer
	rsWriter := rs.NewWriter(writers)

	// Write the data
	n, err := rsWriter.Write(data)
	assert.Nil(err)
	assert.Equal(len(data), n)

	// Close the writer
	err = rsWriter.Close()
	assert.Nil(err)

	// Grab the reader
	readers := getReadersFromShards(t, blockSize, shards)
	rsReader := rs.NewReader(readers, int64(len(data)))

	// Read the data
	b, err := io.ReadAll(rsReader)
	assert.Nil(err)
	assert.Equal(len(data), len(b))

	// Close the reader
	err = rsReader.Close()
}

func makeRandomData(t *testing.T, size int) []byte {
	data := make([]byte, size)
	_, err := rand.Read(data)
	assert.NoError(t, err)
	return data
}

func makeShardBuffer(count int) ([]writerseeker.WriterSeeker, []io.Writer) {
	shards := make([]writerseeker.WriterSeeker, count)
	writers := make([]io.Writer, count)
	for i := 0; i < count; i++ {
		writers[i] = &shards[i]
	}
	return shards, writers
}

func getReadersFromShards(t *testing.T, blockSize int, shards []writerseeker.WriterSeeker) []io.Reader {
	assert := assert.New(t)
	readers := make([]io.Reader, len(shards))
	for i := 0; i < len(shards); i++ {
		// Seek to the beginning of the buffer
		n, err := shards[i].Seek(0, io.SeekStart)
		assert.Nil(err)
		assert.Equal(int64(0), n)

		// Try to read the data
		b, err := io.ReadAll(shards[i].BytesReader())
		assert.Nil(err)
		assert.Equal(0, len(b)%(blockSize+32))

		n, err = shards[i].Seek(0, io.SeekStart)
		assert.Nil(err)
		assert.Equal(int64(0), n)
		readers[i] = shards[i].BytesReader()
	}

	return readers
}
