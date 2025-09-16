//go:build (cgo && !force_pure_compression && !pure_brotli) || force_cgo_compression || cgo_brotli
// +build cgo,!force_pure_compression,!pure_brotli force_cgo_compression cgo_brotli

package compression

import (
	"bytes"
	"io"

	"github.com/google/brotli/go/cbrotli"
)

type BrotliDecompressor struct{}

// NewBrotliDecompressor creates a new Brotli decompressor
func NewBrotliDecompressor() Decompressor {
	return &BrotliDecompressor{}
}

func (d *BrotliDecompressor) Decompress(data []byte) ([]byte, error) {
	reader := cbrotli.NewReader(bytes.NewReader(data))
	return io.ReadAll(reader)
}

func (d *BrotliDecompressor) Type() CompressionType {
	return TypeBrotli
}

func (d *BrotliDecompressor) Implementation() string {
	return "CGO (google/brotli/go/cbrotli)"
}
