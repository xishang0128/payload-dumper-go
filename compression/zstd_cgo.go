//go:build (cgo && !force_pure_compression && !pure_zstd) || force_cgo_compression || cgo_zstd
// +build cgo,!force_pure_compression,!pure_zstd force_cgo_compression cgo_zstd

package compression

import (
	"github.com/valyala/gozstd"
)

type ZSTDDecompressor struct{}

// NewZSTDDecompressor creates a new ZSTD decompressor using CGO implementation
func NewZSTDDecompressor() Decompressor {
	return &ZSTDDecompressor{}
}

func (d *ZSTDDecompressor) Decompress(data []byte) ([]byte, error) {
	return gozstd.Decompress(nil, data)
}

func (d *ZSTDDecompressor) Type() CompressionType {
	return TypeZSTD
}

func (d *ZSTDDecompressor) Implementation() string {
	return "CGO (valyala/gozstd)"
}
