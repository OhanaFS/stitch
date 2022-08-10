package stitch_test

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"testing"

	"github.com/OhanaFS/stitch"
	"github.com/OhanaFS/stitch/util"
	"github.com/OhanaFS/stitch/util/debug"
	"github.com/stretchr/testify/assert"
)

// A simple example to demonstrate how to use the Encoder and ReadSeeker.
func Example() {
	// Create a new encoder.
	encoder := stitch.NewEncoder(&stitch.EncoderOptions{
		DataShards:   2,
		ParityShards: 1,
		KeyThreshold: 2,
	})

	// Open the input file.
	input, _ := os.Open("input.txt")
	defer input.Close()

	// Open the output files.
	out1, _ := os.Create("output.shard1")
	defer out1.Close()
	out2, _ := os.Create("output.shard2")
	defer out2.Close()

	// Use a dummy key and IV.
	key := []byte("00000000000000000000000000000000")
	iv := []byte("000000000000")

	// Encode the data.
	result, _ := encoder.Encode(input, []io.Writer{out1, out2}, key, iv)
	fmt.Printf("File size: %d\n", result.FileSize)
	fmt.Printf("File hash: %x\n", result.FileHash)

	// Decode the data.
	reader, _ := encoder.NewReadSeeker([]io.ReadSeeker{out1, out2}, key, iv)
	io.Copy(os.Stdout, reader)
}

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

		debug.Hexdump(shards[0].Bytes(), "shard0")

		// Decode the data
		reader, err := encoder.NewReadSeeker(shardReaders, key, iv)
		assert.NoError(err)

		// Read the data.
		output := util.NewMembuf()
		n, err := io.Copy(output, reader)
		assert.NoError(err)
		assert.Equal(int64(len(input)), n)

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
