//go:build (!cgo) || force_pure_compression || pure_zstd
// +build !cgo force_pure_compression pure_zstd

package compression

import (
	"github.com/klauspost/compress/zstd"
)

type ZSTDDecompressor struct {
	decoder *zstd.Decoder
}

// NewZSTDDecompressor creates a new ZSTD decompressor using pure Go implementation
func NewZSTDDecompressor() Decompressor {
	decoder, err := zstd.NewReader(nil, zstd.WithDecoderConcurrency(0))
	if err != nil {
		decoder, _ = zstd.NewReader(nil)
	}

	return &ZSTDDecompressor{
		decoder: decoder,
	}
}

func (d *ZSTDDecompressor) Decompress(data []byte) ([]byte, error) {
	return d.decoder.DecodeAll(data, nil)
}

func (d *ZSTDDecompressor) Type() CompressionType {
	return TypeZSTD
}

func (d *ZSTDDecompressor) Implementation() string {
	return "Pure Go (klauspost/compress/zstd)"
}
