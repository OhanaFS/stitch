package header

import (
	"encoding"
	"encoding/binary"
	"errors"
)

// Header describes the header of a shard.
type Header struct {
	// Magic is an arbitrary constant that identifies the file as a shard.
	Magic [4]byte
	// Version is the version of the shard format.
	Version [2]byte
	// ShardIndex is the index of the shard.
	ShardIndex uint8
	// FileHash is the SHA256 hash of the whole file plaintext.
	FileHash [32]byte
	// FileKey is one shard of the AES key used to encrypt the file plaintext.
	FileKey [32]byte
	// FileSize is the size of the file plaintext.
	FileSize uint64
	// reserved is reserved for future use. It should be 49 bytes of zero to pad
	// the header to 128 bytes.
	reserved [49]byte
}

// HeaderSize is the size of the header in bytes.
const HeaderSize = 4 + 2 + 1 + 32 + 32 + 8 + 49

var _ encoding.BinaryMarshaler = (*Header)(nil)
var _ encoding.BinaryUnmarshaler = (*Header)(nil)

var (
	MagicBytes     = [4]byte{'S', 'T', 'C', 'H'}
	CurrentVersion = [2]byte{0x00, 0x01}

	ErrInvalidHeaderSize = errors.New("invalid header size")
	ErrUnrecognizedMagic = errors.New("unrecognized magic bytes")
	ErrVersionMismatch   = errors.New("version mismatch")
)

func NewHeader() *Header {
	return &Header{
		Magic:   MagicBytes,
		Version: CurrentVersion,
	}
}

// MarshalBinary implements the encoding.BinaryMarshaler interface.
func (h *Header) MarshalBinary() ([]byte, error) {
	header := make([]byte, HeaderSize)
	copy(header[:4], h.Magic[:])
	copy(header[4:6], h.Version[:])
	header[6] = h.ShardIndex
	copy(header[7:39], h.FileHash[:])
	copy(header[39:71], h.FileKey[:])
	binary.LittleEndian.PutUint64(header[71:], h.FileSize)
	return header, nil
}

// UnmarshalBinary implements the encoding.BinaryUnmarshaler interface.
func (h *Header) UnmarshalBinary(data []byte) error {
	// Make sure the header is the correct size.
	if len(data) < HeaderSize {
		return ErrInvalidHeaderSize
	}

	// Check the magic bytes.
	for i, b := range data[:4] {
		if b != h.Magic[i] {
			return ErrUnrecognizedMagic
		}
	}

	// Check the version.
	for i, b := range data[4:6] {
		if b != h.Version[i] {
			return ErrVersionMismatch
		}
	}

	// Copy the header data.
	copy(h.Magic[:], data[:4])
	copy(h.Version[:], data[4:6])
	h.ShardIndex = data[6]
	copy(h.FileHash[:], data[7:39])
	copy(h.FileKey[:], data[39:71])
	h.FileSize = binary.LittleEndian.Uint64(data[71:])
	return nil
}
