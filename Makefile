.PHONY: test build

bin/stitch: $(shell find . -name '*.go')
	mkdir -p bin
	go build -o bin/stitch -ldflags="-extldflags=-static" ./cmd/stitch

test:
	go test ./...
