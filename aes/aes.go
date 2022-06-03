package aes

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"encoding/binary"
	"errors"
	"io"
)

var (
	ErrInvalidKeyLength = errors.New("Key must be 16, 24, or 32 bytes long")
)

// AESReader reads data from an io.Reader that was generated using AESWriter.
type AESReader struct {
	ds        io.Reader
	block     cipher.Block
	gcm       cipher.AEAD
	chunkSize int
}

// Assert that the AESReader struct satisfies the io.ReadSeeker interface
// var _ io.ReadSeeker = &AESReader{}

// AESWriter generates a ciphertext to an io.Writer that can be read back using
// AESReader
type AESWriter struct {
	ds        io.Writer
	block     cipher.Block
	gcm       cipher.AEAD
	chunkSize int

	buffer  bytes.Buffer
	written int
}

// Asert that the AESWriter struct satisfies the io.Writer interface
var _ io.WriteCloser = &AESWriter{}

// NewReader creates a new AESReader
func NewReader() (io.ReadSeeker, error) {
	return nil, nil
}

// NewWriter creates a new AESWriter
func NewWriter(ds io.Writer, key []byte, chunkSize int) (io.WriteCloser, error) {
	if len(key) != 16 && len(key) != 24 && len(key) != 32 {
		return nil, ErrInvalidKeyLength
	}

	// Create a new block cipher
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	return &AESWriter{ds: ds, block: block, gcm: gcm, chunkSize: chunkSize}, nil
}

// GetOffset returns the offset of the chunk specified by the index.
func GetOffset(chunkSize, overhead, index int) int {
	return index * (chunkSize + overhead)
}

// FromOffset returns the index of the chunk given an offset.
func FromOffset(chunkSize, overhead, offset int) int {
	return offset / (chunkSize + overhead)
}

func (w *AESWriter) Write(p []byte) (int, error) {
	// Append p to the buffer
	w.buffer.Write(p)

	// Process the buffer until there's not enough data to process
	chunk := make([]byte, w.chunkSize)
	for {
		if w.buffer.Len() < w.chunkSize {
			break
		}

		// Read up to `chunkSize` bytes
		n, err := w.buffer.Read(chunk)
		if err != nil {
			return n, err
		}

		index := FromOffset(w.chunkSize, w.gcm.Overhead(), w.written)
		nonce := make([]byte, w.gcm.NonceSize())
		binary.BigEndian.PutUint64(nonce, uint64(index))

		// Encrypt chunk
		ciphertext := w.gcm.Seal(nil, nonce, chunk, nil)

		// Write it out
		n, err = w.ds.Write(ciphertext)
		w.written += n

		if err != nil {
			return 0, err
		}
	}

	// Clean up the buffer
	b := bytes.Buffer{}
	b.Write(w.buffer.Bytes())
	w.buffer = b

	return len(p), nil
}

// Close finalizes the writes and flushes any remaining buffered data onto
// the writer.
func (w *AESWriter) Close() error {
	chunk := w.buffer.Bytes()
	index := FromOffset(w.chunkSize, w.gcm.Overhead(), w.written)
	nonce := make([]byte, w.gcm.NonceSize())
	binary.BigEndian.PutUint64(nonce, uint64(index))

	ciphertext := w.gcm.Seal(nil, nonce, chunk, nil)
	n, err := w.ds.Write(ciphertext)
	w.written += n

	if err != nil {
		return err
	}

	return nil
}
