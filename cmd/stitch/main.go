package main

import (
	"encoding/binary"
	"flag"
	"io"
	"log"
	"os"
	"strconv"

	"github.com/mitchellh/ioprogress"

	"github.com/OhanaFS/stitch/reedsolomon"
)

var (
	reedsolomonCmd = flag.NewFlagSet("reedsolomon", flag.ExitOnError)
	rsInputFile    = reedsolomonCmd.String("input", "", "path to the input file")
	rsOutputFile   = reedsolomonCmd.String("output", "", "path to the output file in bytes")
	rsBlockSize    = reedsolomonCmd.Int("block-size", 4*1024*1024, "block size")
	rsDataShards   = reedsolomonCmd.Int("data-shards", 2, "number of data shards")
	rsParityShards = reedsolomonCmd.Int("parity-shards", 1, "number of parity shards")
)

var subcommands = map[string]*flag.FlagSet{
	reedsolomonCmd.Name(): reedsolomonCmd,
}

func runReedSolomonCmd() int {
	// Make sure the user specifies either the input or output file.
	isInput := *rsInputFile != ""
	isOutput := *rsOutputFile != ""
	if isInput == isOutput {
		log.Fatalln("You must specify either -input or -output.")
	}

	// Grab the filename
	fileName := *rsInputFile
	if isOutput {
		fileName = *rsOutputFile
	}

	// Generate a list of shard names
	totalShards := *rsDataShards + *rsParityShards
	shardNames := make([]string, totalShards)
	for i := 0; i < totalShards; i++ {
		shardNames[i] = fileName + ".shard" + strconv.Itoa(i)
	}

	// Create a new reed solomon encoder
	enc, err := reedsolomon.NewEncoder(*rsDataShards, *rsParityShards, *rsBlockSize)
	if err != nil {
		log.Fatalln("Failed to create encoder:", err)
	}

	if isInput {
		// Open a file for reading
		file, err := os.Open(fileName)
		if err != nil {
			log.Fatalln("Failed to open file:", err)
		}
		defer file.Close()

		// Open the output files
		shards := make([]io.Writer, totalShards)
		for i := 0; i < totalShards; i++ {
			shardFile, err := os.Create(shardNames[i])
			if err != nil {
				log.Fatalf("Failed to create shard %d: %s\n", i, err)
			}
			defer shardFile.Close()
			shards[i] = shardFile
		}

		// Set up progress bar
		stat, err := file.Stat()
		if err != nil {
			log.Fatalln("Failed to stat file:", err)
		}
		progressReader := &ioprogress.Reader{
			Reader: file,
			Size:   stat.Size(),
		}

		// Write file size to all of the shards
		fsize := make([]byte, 8)
		binary.BigEndian.PutUint64(fsize, uint64(stat.Size()))
		for _, shard := range shards {
			if _, err := shard.Write(fsize); err != nil {
				log.Fatalln("Failed to write file size:", err)
			}
		}

		// Encode the file
		log.Println("Encoding file...")
		if err = enc.Split(progressReader, shards); err != nil {
			log.Fatalln("Failed to split file:", err)
		}
	} else {
		// Open output file for writing
		outputFile, err := os.Create(fileName)
		if err != nil {
			log.Fatalln("Failed to open output file:", err)
		}
		defer outputFile.Close()

		// Open the input shards
		shards := make([]io.Reader, totalShards)
		for i := 0; i < totalShards; i++ {
			shardFile, err := os.Open(shardNames[i])
			if err != nil {
				log.Fatalf("Failed to open shard %d: %s\n", i, err)
			}
			defer shardFile.Close()
			shards[i] = shardFile
		}

		// Read the file size from the shards
		fsizes := make(map[uint64]int)
		for _, shard := range shards {
			d := make([]byte, 8)
			if _, err := shard.Read(d); err != nil {
				log.Fatalln("Failed to read file size:", err)
			}
			shardFsize := binary.BigEndian.Uint64(d)
			fsizes[shardFsize] += 1
		}

		// Pick the most common file size
		fsize := int64(0)
		n := 0
		for k, v := range fsizes {
			if v > n {
				fsize = int64(k)
				n = v
			}
		}
		if n < totalShards {
			log.Printf("Warn: File size is not consistent. This may indicate shard corruption.\n")
			log.Printf("Warn: Assuming the most common file size of %d.\n", fsize)
		}

		// Decode the file
		log.Println("Decoding file...")
		if err := enc.Join(outputFile, shards, int64(fsize)); err != nil {
			if e, ok := err.(reedsolomon.ErrCorruptionDetected); ok {
				log.Printf("Warn: %s\n", e)
			} else {
				log.Fatalln("Failed to join file:", err)
			}
		}
	}

	log.Println("Done.")
	return 0
}

func run() int {
	cmd := subcommands[os.Args[1]]
	if cmd == nil {
		subcommandNames := []string{}
		for name := range subcommands {
			subcommandNames = append(subcommandNames, name)
		}

		log.Fatalf("unknown subcommand '%s'. Available commands are: %v", os.Args[1], subcommandNames)
	}

	cmd.Parse(os.Args[2:])

	switch cmd.Name() {
	case "reedsolomon":
		return runReedSolomonCmd()
	}

	return 0
}

func main() {
	os.Exit(run())
}
