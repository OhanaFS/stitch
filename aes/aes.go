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
	ds        io.ReadSeeker
	block     cipher.Block
	gcm       cipher.AEAD
	chunkSize int

	// bytesToDiscard is the number of bytes to discard after reading a chunk, to
	// ensure that the reader is at the correct position.
	bytesToDiscard uint64
}

// Assert that the AESReader struct satisfies the io.ReadSeeker interface
var _ io.ReadSeeker = &AESReader{}

// AESWriter generates a ciphertext to an io.Writer that can be read back using
// AESReader
type AESWriter struct {
	ds        io.Writer
	block     cipher.Block
	gcm       cipher.AEAD
	chunkSize int

	buffer  bytes.Buffer
	written uint64
}

// Asert that the AESWriter struct satisfies the io.Writer interface
var _ io.WriteCloser = &AESWriter{}

// GetOffset returns the offset of the chunk specified by the index.
func GetOffset(chunkSize, overhead, index int) uint64 {
	return uint64(index) * uint64(chunkSize+overhead)
}

// FromOffset returns the index of the chunk given an offset.
func FromOffset(chunkSize, overhead int, offset uint64) int {
	return int(offset / uint64(chunkSize+overhead))
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

// Write buffers p and encrypts the buffer in chunks of chunkSize.
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
		w.written += uint64(n)

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
	w.written += uint64(n)

	if err != nil {
		return err
	}

	return nil
}

// NewReader creates a new AESReader
func NewReader(ds io.ReadSeeker, key []byte, chunkSize int) (io.ReadSeeker, error) {
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

	return &AESReader{ds: ds, block: block, gcm: gcm, chunkSize: chunkSize}, nil
}

func (r *AESReader) Seek(offset int64, whence int) (int64, error) {
	// Calculate the offset from the start
	var startOffset uint64
	switch whence {
	case io.SeekStart:
		startOffset = uint64(offset)
		break
	case io.SeekCurrent:
		// Get the current position of the underlying reader
		pos, err := r.ds.Seek(0, io.SeekCurrent)
		if err != nil {
			return 0, err
		}

		startOffset = uint64(pos + offset)
		break
	case io.SeekEnd:
		// Disable seeking from the end
		return 0, errors.New("Seeking from the end is not supported")

		/*
			// Get the size of the underlying reader
			size, err := r.ds.Seek(0, io.SeekEnd)
			if err != nil {
				return 0, err
			}

			// Calculate the size of the plaintext
			// TODO: Assuming the plaintext is a nice multiple of the chunk size is not
			//       correct.
			chunks := size / int64(r.chunkSize+r.gcm.Overhead())
			plaintextSize := chunks * int64(r.chunkSize)

			startOffset = uint64(plaintextSize + offset)
			break
		*/
	default:
		return 0, errors.New("Invalid whence")
	}

	// Calculate the closest start block and its offset
	chunkSize := r.chunkSize
	overhead := r.gcm.Overhead()
	block := FromOffset(chunkSize, 0, startOffset)
	ciphertextOffset := GetOffset(chunkSize, overhead, block)
	r.bytesToDiscard = startOffset - uint64(block*chunkSize)

	// Seek to the correct offset
	if _, err := r.ds.Seek(int64(ciphertextOffset), io.SeekStart); err != nil {
		return 0, err
	}

	return int64(startOffset), nil
}

func (r *AESReader) Read(p []byte) (int, error) {
	// Get number of blocks to read
	blocks := (len(p) / r.chunkSize) + 1
	b := make([]byte, blocks*(r.chunkSize+r.gcm.Overhead()))

	// Read the data from the underlying reader
	n, err := r.ds.Read(b)
	if err != nil {
		return 0, err
	}

	// Get the index of the chunk
	currentOffset, err := r.ds.Seek(0, io.SeekCurrent)
	if err != nil {
		return 0, err
	}
	index := FromOffset(r.chunkSize, r.gcm.Overhead(), uint64(currentOffset))

	// Decrypt each chunk
	buf := bytes.NewBuffer(b)
	written := 0
	for i := 0; i < n; i += r.chunkSize + r.gcm.Overhead() {
		// Get the nonce
		nonce := make([]byte, r.gcm.NonceSize())
		binary.BigEndian.PutUint64(nonce, uint64(index))

		// Decrypt the chunk
		ciphertext := buf.Next(r.chunkSize + r.gcm.Overhead())
		plaintext, err := r.gcm.Open(nil, nonce, ciphertext, nil)
		if err != nil {
			return 0, err
		}

		// Discard the bytes if necessary
		if r.bytesToDiscard > 0 {
			plaintext = plaintext[r.bytesToDiscard:]
			r.bytesToDiscard = 0
		}
		if len(plaintext) > len(p[written:]) {
			plaintext = plaintext[:len(p[written:])]
		}

		// Write the decrypted chunk to the output buffer
		outidx := i / (r.chunkSize + r.gcm.Overhead()) * r.chunkSize
		copy(p[outidx:], plaintext)

		// Update the index and the written bytes
		index++
		written += len(plaintext)
	}

	return written, nil
}
