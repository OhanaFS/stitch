package reedsolomon_test

import (
	"bytes"
	"crypto/rand"
	"io"
	"io/ioutil"
	"testing"

	"github.com/orcaman/writerseeker"
	"github.com/stretchr/testify/assert"

	"github.com/OhanaFS/stitch/reedsolomon"
)

func TestReedSolomon(t *testing.T) {
	assert := assert.New(t)

	blockSize := 32
	rs, err := reedsolomon.New(5, 2, blockSize)
	assert.Nil(err)

	// Generate some data
	data := make([]byte, blockSize*10)
	for i := 0; i < len(data); i++ {
		data[i] = byte(i)
	}
	t.Logf("Using %d bytes of data", len(data))

	// Create buffers to hold the shards
	shards := make([]writerseeker.WriterSeeker, 7)
	writers := make([]io.Writer, 7)
	for i := 0; i < 7; i++ {
		writers[i] = &shards[i]
	}

	// Encode the data
	err = rs.Split(bytes.NewReader(data), writers)
	assert.Nil(err)

	readers := make([]io.Reader, 7)
	for i := 0; i < 7; i++ {
		// Seek to the beginning of the buffer
		n, err := shards[i].Seek(0, io.SeekStart)
		assert.Nil(err)
		assert.Equal(int64(0), n)

		// Try to read the data
		b, err := ioutil.ReadAll(shards[i].BytesReader())
		assert.Nil(err)
		assert.Greater(len(b), 1)

		n, err = shards[i].Seek(0, io.SeekStart)
		assert.Nil(err)
		assert.Equal(int64(0), n)
		readers[i] = shards[i].BytesReader()
	}

	// Try to decode the data
	dest := &writerseeker.WriterSeeker{}
	err = rs.Join(dest, readers, int64(len(data)))
	assert.Nil(err)

	// Check that the data is correct
	b, err := ioutil.ReadAll(dest.BytesReader())
	assert.Nil(err)
	assert.Equal(data, b)

	// Corrupt one of the shards
	readers = make([]io.Reader, 7)
	for i := 0; i < 7; i++ {
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
	data := make([]byte, blockSize*10)
	_, err = rand.Read(data)
	assert.NoError(err)

	// Create buffers to hold the shards
	shards := make([]writerseeker.WriterSeeker, totalShards)
	writers := make([]io.Writer, totalShards)
	for i := 0; i < totalShards; i++ {
		writers[i] = &shards[i]
	}

	// Encode the data
	err = rs.Split(bytes.NewReader(data), writers)
	assert.Nil(err)

	readers := make([]io.Reader, totalShards)
	for i := 0; i < totalShards; i++ {
		// Seek to the beginning of the buffer
		n, err := shards[i].Seek(0, io.SeekStart)
		assert.Nil(err)
		assert.Equal(int64(0), n)

		// Try to read the data
		b, err := ioutil.ReadAll(shards[i].BytesReader())
		assert.Nil(err)
		assert.Greater(len(b), 1)

		n, err = shards[i].Seek(0, io.SeekStart)
		assert.Nil(err)
		assert.Equal(int64(0), n)
		readers[i] = shards[i].BytesReader()
	}

	// Try to decode the data
	dest := &writerseeker.WriterSeeker{}
	err = rs.Join(dest, readers, int64(len(data)))
	assert.Nil(err)

	// Check that the data is correct
	b, err := ioutil.ReadAll(dest.BytesReader())
	assert.Nil(err)
	assert.Equal(data, b)
}
