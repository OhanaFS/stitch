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

	runBench := func(dataShards, parityShards int) (time.Duration, error) {
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
			return 0, err
		}
		if _, err := rand.Read(iv); err != nil {
			return 0, err
		}

		// Set up the reader and writer
		startTime := time.Now()
		if _, err := encoder.Encode(input, shardWriters, key, iv); err != nil {
			return 0, err
		}
		for _, shard := range shards {
			encoder.FinalizeHeader(shard)
		}
		r, err := encoder.NewReadSeeker(shardReadSeekers, key, iv)
		if err != nil {
			return 0, err
		}
		n, err := io.Copy(io.Discard, r)
		if err != nil {
			return 0, err
		}
		if n != int64(*bInputSize) {
			return 0, err
		}
		return time.Since(startTime), nil
	}

	// Run the benchmark for each thread
	var durations []time.Duration
	var lock sync.Mutex
	var wg sync.WaitGroup
	for i := 0; i < *bThreads; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			duration, err := runBench(*bDataShards, *bParityShards)
			if err != nil {
				log.Printf("Error running benchmark: %v", err)
				return
			}
			lock.Lock()
			durations = append(durations, duration)
			lock.Unlock()
		}()
	}

	// Wait for all the threads to finish
	wg.Wait()

	// Report the results
	var totalDuration time.Duration
	for _, duration := range durations {
		totalDuration += duration
	}
	var averageDuration time.Duration = totalDuration / time.Duration(*bThreads)

	// Calculate speed
	var speed = int64(float64(*bInputSize) * float64(*bThreads) / averageDuration.Seconds())
	log.Printf("Average duration: %v, speed: %s/s", averageDuration, util.FormatSize(speed))

	return 0
}
