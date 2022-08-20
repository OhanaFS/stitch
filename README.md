# stitch 🩹

[![GoDoc](https://godoc.org/github.com/OhanaFS/stitch?status.svg)](https://godoc.org/github.com/OhanaFS/stitch)
[![GitHub Workflow Status](https://img.shields.io/github/workflow/status/OhanaFS/stitch/test?label=tests)](https://github.com/OhanaFS/stitch/actions)

Compress, encrypt, and split files into pieces. Then stitch them together again.

```bash
# Building
make

# Testing
make test

# View documentation
make doc
```

Once you've executed `make doc`, you can view the documentation with your
browser at http://localhost:6060/pkg/github.com/OhanaFS/stitch/

## How it works

![pipeline](./.github/images/pipeline.png)

## Use the command-line interface

Currently there is a basic CLI for the pipeline encoder:

```bash
go run ./cmd/stitch pipeline --help
```

To encode files, use the `-input` flag:

```bash
go run ./cmd/stitch pipeline -input file.bin
```

The command will create `file.bin.shardX` files in the same directory. To
decode, use the `-output` flag:

```bash
go run ./cmd/stitch pipeline -output file.bin
```

The command will look for `file.bin.shardX` files and use it to reconstruct
`file.bin`.
