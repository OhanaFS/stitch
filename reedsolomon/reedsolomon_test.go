package reedsolomon_test

import (
	"fmt"
	"io"
	"log"
	"testing"

	"github.com/orcaman/writerseeker"
	"github.com/stretchr/testify/assert"

	"github.com/OhanaFS/stitch/reedsolomon"
	"github.com/OhanaFS/stitch/util/debug"
)

func TestReedSolomon(t *testing.T) {
	assert := assert.New(t)

	blockSize := 32
	dataShards := 5
	parityShards := 2

	totalShards := dataShards + parityShards
	data := makeData(blockSize * 10)
	shards, writers := makeShardBuffer(totalShards)

	rs, err := reedsolomon.NewEncoder(dataShards, parityShards, blockSize)
	assert.Nil(err)

	// Encode the data
	w := reedsolomon.NewWriter(writers, rs)
	n, err := w.Write(data)
	assert.Nil(err)
	assert.Equal(len(data), n)
	assert.Nil(w.Close())
	// err = rs.Split(bytes.NewReader(data), writers)
	// assert.Nil(err)

	// Try to decode the data
	readers := getReadersFromShards(t, blockSize, shards)
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
	data := makeData(blockSize * 10)
	shards, writers := makeShardBuffer(totalShards)

	rs, err := reedsolomon.NewEncoder(dataShards, parityShards, blockSize)
	assert.Nil(err)

	// Encode the data
	w := reedsolomon.NewWriter(writers, rs)
	n, err := w.Write(data)
	assert.Nil(err)
	assert.Equal(len(data), n)
	assert.Nil(w.Close())
	// err = rs.Split(bytes.NewReader(data), writers)
	// assert.Nil(err)

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

/*
func TestReaderWriter(t *testing.T) {
	assert := assert.New(t)

	blockSize := 32
	dataShards := 5
	parityShards := 2

	totalShards := dataShards + parityShards
	data := makeData(blockSize * 10)
	shards, writers := makeShardBuffer(totalShards)

	rs, err := reedsolomon.NewEncoder(dataShards, parityShards, blockSize)
	assert.Nil(err)

	// Write the data
	rsWriter := rs.NewWriter(writers)
	n, err := rsWriter.Write(data)
	assert.Nil(err)
	assert.Equal(len(data), n)

	// Close the writer
	err = rsWriter.Close()
	assert.Nil(err)

	// Wait for the shards to be written
	time.Sleep(time.Millisecond * 100)

	// Grab the reader
	readers := getReadersFromShards(t, blockSize, shards)
	rsReader := rs.NewReader(readers, int64(len(data)))

	// Read the data
	b, err := io.ReadAll(rsReader)
	assert.Nil(err)
	assert.Equal(data, b)

	// Close the reader
	err = rsReader.Close()
	assert.Nil(err)
}
*/

func makeData(size int) []byte {
	data := make([]byte, size)
	for i := 0; i < len(data); i++ {
		data[i] = byte(i / 16)
	}
	return data
}

func makeShardBuffer(count int) ([]*writerseeker.WriterSeeker, []io.Writer) {
	shards := make([]*writerseeker.WriterSeeker, count)
	writers := make([]io.Writer, count)
	for i := 0; i < count; i++ {
		ws := &writerseeker.WriterSeeker{}
		shards[i] = ws
		writers[i] = ws
	}
	return shards, writers
}

func getReadersFromShards(t *testing.T, blockSize int, shards []*writerseeker.WriterSeeker) []io.Reader {
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
		assert.Equal(0, len(b)%(blockSize+reedsolomon.BlockOverhead))
		log.Printf("shard %d: %d bytes", i, len(b))
		debug.Hexdump(b, fmt.Sprintf("shard %d", i))
		fmt.Println("")

		n, err = shards[i].Seek(0, io.SeekStart)
		assert.Nil(err)
		assert.Equal(int64(0), n)
		readers[i] = shards[i].BytesReader()
	}

	return readers
}
