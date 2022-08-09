// Stitch is a tool to compress, encrypt, and split any data into a set of
// shards.
package stitch

import "errors"

const (
	// rsBlockSize is the size of a Reed-Solomon block.
	rsBlockSize = 4096
	// aesBlockSize is the size of a chunk of data that is encrypted with AES-GCM.
	aesBlockSize = 1024
)

var (
	ErrShardCountMismatch = errors.New("shard count mismatch")
	ErrNonSeekableWriter  = errors.New("shards must support seeking")
	ErrNotEnoughKeyShards = errors.New("not enough shards to reconstruct the file key")
	ErrNotEnoughShards    = errors.New("not enough shards to reconstruct the file")
)

// EncoderOptions specifies options for the Encoder.
type EncoderOptions struct {
	// DataShards is the total number of shards to split data into.
	DataShards uint8
	// ParityShards is the total number of parity shards to create. This also
	// determines the maximum number of shards that can be lost before the data
	// cannot be recovered.
	ParityShards uint8
	// KeyThreshold is the minimum number of shards required to reconstruct the
	// key used to encrypt the data.
	KeyThreshold uint8
}

// Encoder takes in a stream of data and shards it into a specified number of
// data and parity shards. It includes compression using zstd, encryption using
// AES-GCM, and splitting the data into equal-sized shards using Reed-Solomon.
//
// It follows this process to encode the data into multiple shards:
//
//   1. Generate a random key Kr
//   2. Generate N output streams So_n
//   3. Generate a file header
//   4. Encrypt Kr with user-supplied key Ku, and embed it into the file header
//   5. Write the header to So_n
//   6. Take a byte stream of user-supplied data Sd and pipe it to the
//      compressor C
//   7. Pipe the output of C into a streaming symmetric encryption method E,
//      which uses Kr to encrypt
//   8. Pipe the output of E into Reed-Solomon encoder to get N output streams
//      RS_n
//   9. Pipe the output of RS_n to So_n
type Encoder struct {
	opts *EncoderOptions
}

type EncodingResult struct {
	// FileSize is the size of the input file in bytes.
	FileSize uint64
	// FileHash is the SHA256 hash of the input file.
	FileHash []byte
}

func NewEncoder(opts *EncoderOptions) *Encoder {
	return &Encoder{opts}
}
