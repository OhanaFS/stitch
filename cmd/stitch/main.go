package main

import (
	"flag"
	"os"
)

func run() int {
	flag.Parse()
	return 0
}

func main() {
	os.Exit(run())
}
