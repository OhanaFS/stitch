package aes_test

import (
	"io"
	"testing"

	"github.com/OhanaFS/stitch/aes"
	"github.com/OhanaFS/stitch/util"
	"github.com/stretchr/testify/assert"
)

func TestAES(t *testing.T) {
	assert := assert.New(t)

	overhead := 16
	key := []byte("11111111aaaaaaaa")
	buf := util.NewMembuf()

	// Test writing small data
	w, err := aes.NewWriter(buf, key, 32)
	assert.NoError(err)

	datatext := "hello, world"
	n, err := w.Write([]byte(datatext))
	assert.NoError(err)
	assert.Equal(12, n)
	// Should be buffered
	assert.Equal(0, buf.Len())
	w.Close()
	// Should be flushed
	assert.Equal(32+overhead, buf.Len())

	// Test writing data longer than chunk size
	buf = util.NewMembuf()
	w, err = aes.NewWriter(buf, key, 8)
	assert.NoError(err)

	datatext = "test-1234-asdf-abcd-"
	n, err = w.Write([]byte(datatext))
	assert.NoError(err)
	assert.Equal(20, n)
	// Should be partially written
	assert.Equal(2*(8+overhead), buf.Len())
	w.Close()
	// Should be flushed
	assert.Equal(3*(8+overhead), buf.Len())

	// Test decryption
	buf.Seek(0, io.SeekStart)
	r, err := aes.NewReader(buf, key, 8, uint64(len(datatext)))
	assert.NoError(err)

	res := make([]byte, 20)
	n, err = r.Read(res)
	assert.NoError(err)
	assert.Equal(len(datatext), n)
	assert.Equal(datatext, string(res[:]))

	// Seek to the middle of the data
	midpoint := int64(len(datatext) / 2)
	ns, err := r.Seek(midpoint, io.SeekStart)
	assert.NoError(err)
	assert.Equal(midpoint, ns)

	// Read the rest of the data
	res = make([]byte, 20)
	n, err = r.Read(res)
	assert.NoError(err)
	assert.Equal(len(datatext)-int(midpoint), n)
	assert.Equal(datatext[midpoint:], string(res[:midpoint]))
}
