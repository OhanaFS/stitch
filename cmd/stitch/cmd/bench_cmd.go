package cmd

import (
	"crypto/rand"
	"flag"
	"io"
	"log"
	"sync"
	"time"

	"github.com/OhanaFS/stitch"
	"github.com/OhanaFS/stitch/util"
)

var (
	BenchCmd      = flag.NewFlagSet("bench", flag.ExitOnError)
	bDataShards   = BenchCmd.Int("data-shards", 2, "number of data shards")
	bParityShards = BenchCmd.Int("parity-shards", 1, "number of parity shards")
	bThreads      = BenchCmd.Int("threads", 1, "number of threads")
	bInputSize    = BenchCmd.Int("input-size", 10*1024*1024, "size of input file")
)

func RunBenchCmd() int {
	log.Printf("Running benchmark with %d data shards, %d parity shards, and %d threads", *bDataShards, *bParityShards, *bThreads)

	runBench := func(dataShards, parityShards int) (time.Duration, time.Duration, error) {
		// Create the inputs and outputs
		input := &util.RandomReader{Size: int64(*bInputSize)}

		shards := make([]*util.Membuf, dataShards+parityShards)
		shardWriters := make([]io.Writer, dataShards+parityShards)
		shardReadSeekers := make([]io.ReadSeeker, dataShards+parityShards)
		for i := 0; i < dataShards+parityShards; i++ {
			shards[i] = util.NewMembuf()
			shardWriters[i] = shards[i]
			shardReadSeekers[i] = shards[i]
		}

		// Create the encoder
		encoder := stitch.NewEncoder(&stitch.EncoderOptions{
			DataShards:   uint8(dataShards),
			ParityShards: uint8(parityShards),
			KeyThreshold: uint8(dataShards),
		})

		// Generate a key and IV
		key := make([]byte, 32)
		iv := make([]byte, 12)
		if _, err := rand.Read(key); err != nil {
			return 0, 0, err
		}
		if _, err := rand.Read(iv); err != nil {
			return 0, 0, err
		}

		// Set up the reader and writer
		startTime := time.Now()
		if _, err := encoder.Encode(input, shardWriters, key, iv); err != nil {
			return 0, 0, err
		}
		for _, shard := range shards {
			encoder.FinalizeHeader(shard)
		}
		encodeTime := time.Since(startTime)

		startTime = time.Now()
		r, err := encoder.NewReadSeeker(shardReadSeekers, key, iv)
		if err != nil {
			return 0, 0, err
		}
		n, err := io.Copy(io.Discard, r)
		if err != nil {
			return 0, 0, err
		}
		if n != int64(*bInputSize) {
			return 0, 0, err
		}
		decodeTime := time.Since(startTime)

		return encodeTime, decodeTime, nil
	}

	// Run the benchmark for each thread
	var durationsEncode []time.Duration
	var durationsDecode []time.Duration
	var lock sync.Mutex
	var wg sync.WaitGroup
	for i := 0; i < *bThreads; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			durEnc, durDec, err := runBench(*bDataShards, *bParityShards)
			if err != nil {
				log.Printf("Error running benchmark: %v", err)
				return
			}
			lock.Lock()
			durationsEncode = append(durationsEncode, durEnc)
			durationsDecode = append(durationsDecode, durDec)
			lock.Unlock()
		}()
	}

	// Wait for all the threads to finish
	wg.Wait()

	// Report the results
	var totalDurationEnc time.Duration
	var totalDurationDec time.Duration
	for i := range durationsEncode {
		totalDurationEnc += durationsEncode[i]
		totalDurationDec += durationsDecode[i]
	}
	var averageEncode time.Duration = totalDurationEnc / time.Duration(*bThreads)
	var averageDecode time.Duration = totalDurationDec / time.Duration(*bThreads)

	// Calculate speedEncode
	var speedEncode = int64(float64(*bInputSize) * float64(*bThreads) / averageEncode.Seconds())
	var speedDecode = int64(float64(*bInputSize) * float64(*bThreads) / averageDecode.Seconds())
	log.Printf("Average encode: %v, speed: %s/s", averageEncode, util.FormatSize(speedEncode))
	log.Printf("Average decode: %v, speed: %s/s", averageDecode, util.FormatSize(speedDecode))

	return 0
}
