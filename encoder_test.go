package stitch_test

import (
	"bytes"
	"crypto/rand"
	"io"
	"testing"

	"github.com/OhanaFS/stitch"
	"github.com/OhanaFS/stitch/util"
	"github.com/OhanaFS/stitch/util/debug"
	"github.com/stretchr/testify/assert"
)

func TestEncodeDecode(t *testing.T) {
	assert := assert.New(t)

	runTest := func(input []byte) {
		inputBuffer := &bytes.Buffer{}
		inputBuffer.Write(input)
		// inputBuffer := bytes.NewBuffer([]byte(input))
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

		// Encode the data.
		assert.NoError(encoder.Encode(inputBuffer, shardWriters, key, iv))

		// Finalize the file headers
		for _, shard := range shards {
			assert.NoError(encoder.FinalizeHeader(shard))
		}

		debug.Hexdump(shards[0].Bytes(), "shard0")

		// Decode the data
		reader, err := encoder.NewReadSeeker(shardReaders, key, iv)
		assert.NoError(err)

		// Read the data.
		output := &bytes.Buffer{}
		_, err = io.Copy(output, reader)
		assert.NoError(err)

		// Verify the data.
		assert.Equal(input, output.Bytes())
	}

	// runTest([]byte("hello, world!"))

	random := make([]byte, 3922)
	_, err := rand.Read(random)
	assert.NoError(err)
	runTest(random)

	// dddd := make([]byte, 8192)
	// for i := 0; i < len(dddd); i++ {
	// dddd[i] = 0xdd
	// }
	// runTest(dddd)
}
