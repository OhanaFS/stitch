package header

import (
	"encoding"
	"encoding/binary"
	"encoding/json"
	"errors"
)

// Header describes the header of a shard.
type Header struct {
	// ShardIndex is the index of the shard.
	ShardIndex uint8 `json:"i"`
	// ShardCount is the total number of shards.
	ShardCount uint8 `json:"c"`
	// FileHash is the SHA256 hash of the whole file plaintext.
	FileHash []byte `json:"h"`
	// FileKey is one shard of the AES key used to encrypt the file plaintext. It
	// is 33 bytes long to allow for the overhead of Shamir's Secret Sharing.
	FileKey []byte `json:"k"`
	// FileIV is the AES initialization vector used to encrypt the file plaintext.
	FileIV []byte `json:"n"`
	// FileSize is the size of the file plaintext.
	FileSize uint64 `json:"s"`
}

// HeaderSize is the fixed size allocated for the header.
const HeaderSize = 1024

var _ encoding.BinaryMarshaler = (*Header)(nil)
var _ encoding.BinaryUnmarshaler = (*Header)(nil)

var (
	MagicBytes = []byte("STITCHv1")

	ErrInvalidHeaderSize = errors.New("invalid header size")
	ErrUnrecognizedMagic = errors.New("unrecognized magic bytes")
)

func NewHeader() *Header {
	return &Header{}
}

// MarshalBinary implements the encoding.BinaryMarshaler interface.
func (h *Header) MarshalBinary() ([]byte, error) {
	// Allocate a buffer for the header.
	buf := make([]byte, HeaderSize)

	// Write the magic bytes.
	copy(buf[:8], MagicBytes)

	// Marshal the header data as JSON.
	data, err := json.Marshal(h)
	if err != nil {
		return nil, err
	}

	// Make sure the header data is not too large.
	if len(data) > HeaderSize-8 {
		return nil, ErrInvalidHeaderSize
	}

	// Write the length of the JSON data to the header.
	binary.LittleEndian.PutUint16(buf[8:10], uint16(len(data)))

	// Copy the JSON data to the header.
	copy(buf[10:], data)

	return buf, nil
}

// UnmarshalBinary implements the encoding.BinaryUnmarshaler interface.
func (h *Header) UnmarshalBinary(data []byte) error {
	// Check the magic bytes.
	for i, b := range MagicBytes {
		if b != data[i] {
			return ErrUnrecognizedMagic
		}
	}

	// Check the size of the header data.
	dataLen := binary.LittleEndian.Uint16(data[8:10])

	// Unmarshal the header data.
	if err := json.Unmarshal(data[10:10+dataLen], h); err != nil {
		return err
	}

	return nil
}
