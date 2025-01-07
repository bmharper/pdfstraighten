# PDF Straightener

This is a little library/CLI tool for straightening scanned PDF documents.
We only check for angles between -2.5 and +2.5 degrees. This is an arbitrary constant, but
the algorithm is brute force and takes longer if you want to scan a wider range.

## CLI Usage

> go run cmd/straighten.go [PDF File]

## API Usage

See [cmd/straighten.go](./cmd/straighten.go) for an example of how to use the library.
