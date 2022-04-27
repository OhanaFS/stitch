package main

import (
	"flag"
	"os"
)

var (
	reedsolomonCmd = flag.NewFlagSet("reedsolomon", flag.ExitOnError)
)

func run() int {
	flag.Parse()
	return 0
}

func main() {
	os.Exit(run())
}
