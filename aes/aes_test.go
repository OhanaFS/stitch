package aes_test

import (
	"bytes"
	"testing"

	"github.com/OhanaFS/stitch/aes"
	"github.com/stretchr/testify/assert"
)

func TestAES(t *testing.T) {
	assert := assert.New(t)

	overhead := 16
	key := []byte("11111111aaaaaaaa")
	buf := &bytes.Buffer{}

	// Test writing small data
	w, err := aes.NewWriter(buf, key, 64)
	assert.NoError(err)

	datatext := "hello, world"
	n, err := w.Write([]byte(datatext))
	assert.NoError(err)
	assert.Equal(12, n)
	// Should be buffered
	assert.Equal(0, buf.Len())
	w.Close()
	// Should be flushed
	assert.Equal(len(datatext)+overhead, buf.Len())

	// Test writing data longer than chunk size
	buf.Reset()
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
	assert.Equal(2*(8+overhead)+(4+overhead), buf.Len())
}
