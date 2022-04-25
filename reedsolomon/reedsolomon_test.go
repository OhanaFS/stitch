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

	rs, err := reedsolomon.New(5, 2, 1024*1024)
	assert.Nil(err)

	// Generate some data
	dataLen := 1024*1024*10 - 1024
	data := make([]byte, dataLen)
	_, err = rand.Read(data)
	assert.Nil(err)

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
		t.Logf("Shard %d: %d bytes", i, len(b))

		n, err = shards[i].Seek(0, io.SeekStart)
		assert.Nil(err)
		assert.Equal(int64(0), n)
		readers[i] = shards[i].BytesReader()
	}

	// Try to decode the data
	dest := &writerseeker.WriterSeeker{}
	err = rs.Join(dest, readers, int64(dataLen))
	assert.Nil(err)
}
