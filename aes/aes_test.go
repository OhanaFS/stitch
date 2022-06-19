package aes_test

import (
	"testing"

	"github.com/OhanaFS/stitch/aes"
	"github.com/orcaman/writerseeker"
	"github.com/stretchr/testify/assert"
)

func TestAES(t *testing.T) {
	assert := assert.New(t)

	overhead := 16
	key := []byte("11111111aaaaaaaa")
	buf := &writerseeker.WriterSeeker{}

	// Test writing small data
	w, err := aes.NewWriter(buf, key, 64)
	assert.NoError(err)

	datatext := "hello, world"
	n, err := w.Write([]byte(datatext))
	assert.NoError(err)
	assert.Equal(12, n)
	// Should be buffered
	assert.Equal(0, buf.BytesReader().Len())
	w.Close()
	// Should be flushed
	assert.Equal(len(datatext)+overhead, buf.BytesReader().Len())

	// Test writing data longer than chunk size
	buf = &writerseeker.WriterSeeker{}
	w, err = aes.NewWriter(buf, key, 8)
	assert.NoError(err)

	datatext = "test-1234-asdf-abcd-"
	n, err = w.Write([]byte(datatext))
	assert.NoError(err)
	assert.Equal(20, n)
	// Should be partially written
	assert.Equal(2*(8+overhead), buf.BytesReader().Len())
	w.Close()
	// Should be flushed
	assert.Equal(2*(8+overhead)+(4+overhead), buf.BytesReader().Len())

	// Test decryption
	reader := buf.BytesReader()
	r, err := aes.NewReader(reader, key, 8)
	assert.NoError(err)

	res := make([]byte, 20)
	n, err = r.Read(res)
	assert.NoError(err)
	assert.Equal(len(datatext), n)
	assert.Equal(datatext, string(res[:]))
}
