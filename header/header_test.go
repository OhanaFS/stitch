package header_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/OhanaFS/stitch/header"
)

var (
	testHash = [32]byte{
		1, 2, 3, 4, 5, 6, 7, 8,
		9, 10, 11, 12, 13, 14, 15, 16,
		17, 18, 19, 20, 21, 22, 23, 24,
		25, 26, 27, 28, 29, 30, 31, 32,
	}
	testKey = [32]byte{
		64, 65, 66, 67, 68, 69, 70, 71,
		72, 73, 74, 75, 76, 77, 78, 79,
		80, 81, 82, 83, 84, 85, 86, 87,
		88, 89, 90, 91, 92, 93, 94, 95,
	}
)

func TestMarshalUnmarshal(t *testing.T) {
	h := header.NewHeader()
	h.ShardIndex = 1
	h.FileHash = testHash
	h.FileKey = testKey
	h.FileSize = uint64(0x123456789abcdef0)

	var b []byte
	var err error

	t.Run("Marshal", func(t *testing.T) {
		assert := assert.New(t)

		b, err = h.MarshalBinary()
		assert.Nil(err)
		assert.Equal(header.HeaderSize, len(b))
		assert.Equal(header.MagicBytes[:], b[:4])
		assert.Equal(header.CurrentVersion[:], b[4:6])
		assert.Equal(byte(1), b[6])
		assert.Equal(testHash[:], b[7:39])
		assert.Equal(testKey[:], b[39:71])
		assert.Equal([]byte{0xf0, 0xde, 0xbc, 0x9a, 0x78, 0x56, 0x34, 0x12}[:], b[71:79])
	})

	t.Run("Unmarshal", func(t *testing.T) {
		assert := assert.New(t)

		h2 := header.NewHeader()
		err = h2.UnmarshalBinary(b)
		assert.Nil(err)
		assert.Equal(h, h2)
	})
}
