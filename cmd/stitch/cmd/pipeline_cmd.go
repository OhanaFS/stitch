package cmd

import (
	"encoding/hex"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"strconv"

	"github.com/OhanaFS/stitch"
	"github.com/OhanaFS/stitch/util"
)

var (
	PipelineCmd    = flag.NewFlagSet("pipeline", flag.ExitOnError)
	plInputFile    = PipelineCmd.String("input", "", "path to the input file")
	plOutputFile   = PipelineCmd.String("output", "", "path to the output file")
	plDataShards   = PipelineCmd.Int("data-shards", 2, "number of data shards")
	plParityShards = PipelineCmd.Int("parity-shards", 1, "number of parity shards")
	plFileKey      = PipelineCmd.String("file-key", "00000000000000000000000000000000", "file key")
	plFileKeySalt  = PipelineCmd.String("file-key-salt", "000000000000000000000000", "file key salt")
)

func RunPipelineCmd() int {
	// Make sure the user specifies either the input or output file.
	isInput := *plInputFile != ""
	isOutput := *plOutputFile != ""
	if isInput == isOutput {
		log.Fatalln("You must specify either -input or -output.")
	}

	// Grab the filename
	fileName := *plInputFile
	if isOutput {
		fileName = *plOutputFile
	}

	// Generate a list of shard names
	totalShards := *plDataShards + *plParityShards
	shardNames := make([]string, totalShards)
	for i := 0; i < totalShards; i++ {
		shardNames[i] = fileName + ".shard" + strconv.Itoa(i)
	}

	// Create the encoder
	encoder := stitch.NewEncoder(&stitch.EncoderOptions{
		DataShards:   uint8(*plDataShards),
		ParityShards: uint8(*plParityShards),
		KeyThreshold: uint8(*plDataShards),
	})

	// Get key and IV
	key, err := hex.DecodeString(*plFileKey)
	if err != nil {
		log.Fatalln("Invalid key:", err)
	}
	iv, err := hex.DecodeString(*plFileKeySalt)
	if err != nil {
		log.Fatalln("Invalid IV:", err)
	}

	if isInput {
		// Open a file for reading
		fmt.Printf("Opening file %s for reading\n", fileName)
		file, err := os.Open(fileName)
		if err != nil {
			log.Fatalln("Failed to open file:", err)
		}
		defer file.Close()

		// Open the output files
		shardWriters := make([]io.Writer, totalShards)
		shardFiles := make([]*os.File, totalShards)
		for i := 0; i < totalShards; i++ {
			shardFile, err := os.Create(shardNames[i])
			if err != nil {
				log.Fatalf("Failed to create shard %d: %s\n", i, err)
			}
			defer shardFile.Close()
			shardWriters[i] = shardFile
			shardFiles[i] = shardFile
		}

		// Set up progress bar
		stat, err := file.Stat()
		if err != nil {
			log.Fatalln("Failed to stat file:", err)
		}
		progressReader := util.NewProgressReader(file, stat.Size())

		// Encode the file
		log.Println("Encoding file...")
		if err = encoder.Encode(progressReader, shardWriters, key, iv); err != nil {
			log.Fatalln("Failed to encode file:", err)
		}
		fmt.Println("")

		// Finalize shard headers
		log.Println("Finalizing shard headers...")
		for i := 0; i < totalShards; i++ {
			if err = encoder.FinalizeHeader(shardFiles[i]); err != nil {
				log.Fatalf("Failed to finalize shard %d: %s\n", i, err)
			}
		}
	} else {
		// Open output file for writing
		fmt.Printf("Opening file %s for writing\n", fileName)
		outputFile, err := os.Create(fileName)
		if err != nil {
			log.Fatalln("Failed to open output file:", err)
		}
		defer outputFile.Close()

		// Open the input files
		shards := make([]io.ReadSeeker, totalShards)
		for i := 0; i < totalShards; i++ {
			shardFile, err := os.Open(shardNames[i])
			if err != nil {
				log.Fatalln("Failed to open shard:", err)
			}
			defer shardFile.Close()
			shards[i] = shardFile
		}

		// Decode the file
		log.Println("Decoding file...")
		reader, err := encoder.NewReadSeeker(shards, key, iv)
		if err != nil {
			log.Fatalln("Failed to create reader:", err)
		}
		n, err := io.Copy(outputFile, util.NewProgressReader(reader, 0))
		if err != nil {
			log.Fatalln("Failed to decode file:", err)
		}
		fmt.Println("")
		log.Printf("Decoded %d bytes\n", n)
	}

	log.Println("Done.")
	return 0
}
