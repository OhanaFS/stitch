package header

import (
	"encoding/binary"
	"errors"

	"github.com/OhanaFS/stitch/crypto"
	"github.com/vmihailenco/msgpack/v5"
)

// Header describes the header of a shard. This struct only contains the actual
// data. The full header of each shard is composed of the following:
//
// | Description                   | Length |
// | ----------------------------- | ------ |
// | magic bytes `STITCHv1`        | 8      |
// | length of header data uint16  | 2 		  |
// | header data                   | -      |
// | padding to fill to 1024 bytes | -      |
type Header struct {
	// ShardIndex is the index of the shard.
	ShardIndex int `msgpack:"i"`
	// ShardCount is the total number of shards.
	ShardCount int `msgpack:"c"`
	// FileHash is the SHA256 hash of the whole file plaintext.
	FileHash []byte `msgpack:"h"`
	// FileKey is one shard of the AES key used to encrypt the file plaintext.
	FileKey []byte `msgpack:"k"`
	// FileSize is the size of the file plaintext.
	FileSize uint64 `msgpack:"s"`
	// RSBlockSize is the size of the Reed-Solomon block.
	RSBlockSize int `msgpack:"b"`
}

// HeaderSize is the fixed size allocated for the header.
const HeaderSize = 1024

var (
	MagicBytes = []byte("STITCHv1")

	ErrInvalidHeaderSize = errors.New("invalid header size")
	ErrUnrecognizedMagic = errors.New("unrecognized magic bytes")
)

func NewHeader() *Header {
	return &Header{}
}

func (h *Header) Encode() ([]byte, error) {
	// Allocate a buffer for the header.
	buf, err := crypto.RandomBytes(HeaderSize)
	if err != nil {
		return nil, err
	}

	// Write the magic bytes.
	copy(buf[:8], MagicBytes)

	// Marshal the header data as MsgPack.
	data, err := msgpack.Marshal(h)
	if err != nil {
		return nil, err
	}

	// Make sure the header data is not too large.
	if len(data) > HeaderSize-8 {
		return nil, ErrInvalidHeaderSize
	}

	// Write the length of the data to the header.
	binary.LittleEndian.PutUint16(buf[8:10], uint16(len(data)))

	// Copy the data to the header.
	copy(buf[10:], data)

	return buf, nil
}

// Decode implements the encoding.BinaryUnmarshaler interface.
func (h *Header) Decode(data []byte) error {
	// Check the magic bytes.
	for i, b := range MagicBytes {
		if b != data[i] {
			return ErrUnrecognizedMagic
		}
	}

	// Check the size of the header data.
	dataLen := binary.LittleEndian.Uint16(data[8:10])

	// Unmarshal the header data.
	if err := msgpack.Unmarshal(data[10:10+dataLen], h); err != nil {
		return err
	}

	return nil
}
