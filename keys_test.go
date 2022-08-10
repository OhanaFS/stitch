package stitch_test

import (
	"io"
	"testing"

	"github.com/OhanaFS/stitch"
	"github.com/OhanaFS/stitch/util"
	"github.com/stretchr/testify/assert"
)

func TestRotateKeys(t *testing.T) {
	assert := assert.New(t)

	// Create a new encoder.
	encoder := stitch.NewEncoder(&stitch.EncoderOptions{
		DataShards:   2,
		ParityShards: 1,
		KeyThreshold: 2,
	})

	// Create a dummy input reader.
	input := util.NewLimitReader(&util.ZeroReadSeeker{}, 1024)

	// Create the output files.
	out1 := util.NewMembuf()
	out2 := util.NewMembuf()
	out3 := util.NewMembuf()

	// Use a dummy key and IV.
	key1 := []byte("00000000000000000000000000000000")
	iv1 := []byte("000000000000")
	key2 := []byte("11111111111111111111111111111111")
	iv2 := []byte("111111111111")

	// Encode the data.
	_, err := encoder.Encode(
		input, []io.Writer{out1, out2, out3}, key1, iv1,
	)
	assert.NoError(err)

	// Rotate the keys before headers are finalized.
	_, err = encoder.RotateKeys(
		[]io.ReadSeeker{out1, out2, out3},
		key1, iv1, key2, iv2,
	)
	assert.Error(err)

	// Finalize the output files.
	assert.NoError(encoder.FinalizeHeader(out1))
	assert.NoError(encoder.FinalizeHeader(out2))
	assert.NoError(encoder.FinalizeHeader(out3))

	// Rotate the keys.
	newKeySplits, err := encoder.RotateKeys(
		[]io.ReadSeeker{out1, out2, out3},
		key1, iv1, key2, iv2,
	)
	assert.NoError(err)

	// Update the shard headers with the new key.
	assert.NoError(encoder.UpdateShardKey(out1, newKeySplits[0]))
	assert.NoError(encoder.UpdateShardKey(out2, newKeySplits[1]))
	assert.NoError(encoder.UpdateShardKey(out3, newKeySplits[2]))

	// Ensure the new key is used for decoding.
	_, err = encoder.RotateKeys(
		[]io.ReadSeeker{out1, out2, out3},
		key2, iv2, key1, iv1,
	)
	assert.NoError(err)
}
