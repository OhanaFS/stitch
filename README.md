# stitch 🩹

Compress, encrypt, and split files into pieces. Then stitch them together again.

```
# Building
go build ./cmd/stitch

# Testing
go test -test.v ./...
```

## Usage

Check the Go FAQ on
[how to use private modules](https://go.dev/doc/faq#git_https). Specifically,

1. Add the following two lines to your `~/.gitconfig` file:
   ```
   [url "ssh://git@github.com/"]
       insteadOf = https://github.com/
   ```
2. Grab the dependency:
   ```bash
   export GOPRIVATE=github.com/OhanaFS/stitch
   go mod download
   ```

```go
package main

import (
  "github.com/OhanaFS/stitch"
)

func main() {
  enc := stitch.NewEncoder(&stitch.EncoderOptions{
    DataShards: 5,
    ParityShards: 3,
    KeyThreshold: 3,
  })

  err := enc.Encode(

  )
  if err != nil {
    panic(err)
  }
}
```
