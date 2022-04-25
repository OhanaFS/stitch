package header_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/OhanaFS/stitch/header"
)

var (
	testHash = []byte{
		1, 2, 3, 4, 5, 6, 7, 8,
		9, 10, 11, 12, 13, 14, 15, 16,
		17, 18, 19, 20, 21, 22, 23, 24,
		25, 26, 27, 28, 29, 30, 31, 32,
	}
	testKey = []byte{
		64, 65, 66, 67, 68, 69, 70, 71,
		72, 73, 74, 75, 76, 77, 78, 79,
		80, 81, 82, 83, 84, 85, 86, 87,
		88, 89, 90, 91, 92, 93, 94, 95,
	}
	testIv = []byte{
		127, 128, 129, 130, 131, 132, 133, 134,
		135, 136, 137, 138, 139, 140, 141, 142,
		143, 144, 145, 146, 147, 148, 149, 150,
		151, 152, 153, 154, 155, 156, 157, 158,
	}
)

func TestMarshalUnmarshal(t *testing.T) {
	assert := assert.New(t)

	h := header.NewHeader()
	h.ShardIndex = 1
	h.FileHash = testHash
	h.FileKey = testKey
	h.FileIV = testIv
	h.FileSize = uint64(0x123456789abcdef0)

	b, err := h.MarshalBinary()
	assert.Nil(err)

	h2 := header.NewHeader()
	err = h2.UnmarshalBinary(b)
	assert.Nil(err)
	assert.Equal(h, h2)
}
