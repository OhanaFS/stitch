.PHONY: test build doc

bin/stitch: $(shell find . -name '*.go')
	mkdir -p bin
	go build -o bin/stitch -ldflags="-extldflags=-static" ./cmd/stitch

test:
	go test ./...

doc:
	go install golang.org/x/tools/cmd/godoc@latest
	`go env GOPATH`/bin/godoc -http=:6060 -index
