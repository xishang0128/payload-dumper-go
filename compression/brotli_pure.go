//go:build (!cgo) || force_pure_compression || pure_brotli
// +build !cgo force_pure_compression pure_brotli

package compression

import (
	"bytes"
	"io"

	"github.com/andybalholm/brotli"
)

type BrotliDecompressor struct{}

// NewBrotliDecompressor creates a new Brotli decompressor using pure Go implementation
func NewBrotliDecompressor() Decompressor {
	return &BrotliDecompressor{}
}

func (d *BrotliDecompressor) Decompress(data []byte) ([]byte, error) {
	reader := brotli.NewReader(bytes.NewReader(data))
	return io.ReadAll(reader)
}

func (d *BrotliDecompressor) Type() CompressionType {
	return TypeBrotli
}

func (d *BrotliDecompressor) Implementation() string {
	return "Pure Go (andybalholm/brotli)"
}
