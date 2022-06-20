package header_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/OhanaFS/stitch/header"
)

var (
	testHash = []byte("TEST HASH TEST HASH TEST HASH256")
	testKey  = []byte("-KEYKEYKEYKEYKEYKEYKEYKEYKEYKEY-")
	testIv   = []byte("_IVIVIVIVIVIVIVIVIVIVIVIVIVIVIV_")
)

func TestMarshalUnmarshal(t *testing.T) {
	assert := assert.New(t)

	h := header.NewHeader()
	h.ShardIndex = 1
	h.FileHash = testHash
	h.FileKey = testKey
	h.FileSize = uint64(0x123456789abcdef0)

	b, err := h.Encode()
	assert.Nil(err)
	t.Logf("Encoded header: %v", b)

	h2 := header.NewHeader()
	err = h2.Decode(b)
	assert.Nil(err)
	assert.Equal(h, h2)
}
