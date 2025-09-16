//go:build (cgo && !force_pure_compression && !pure_xz) || force_cgo_compression || cgo_xz
// +build cgo,!force_pure_compression,!pure_xz force_cgo_compression cgo_xz

package compression

import (
	"bytes"
	"io"

	"github.com/spencercw/go-xz"
)

type XZDecompressor struct{}

// NewXZDecompressor creates a new XZ decompressor using CGO implementation
func NewXZDecompressor() Decompressor {
	return &XZDecompressor{}
}

func (d *XZDecompressor) Decompress(data []byte) ([]byte, error) {
	reader := xz.NewDecompressionReader(bytes.NewReader(data))
	defer reader.Close()
	return io.ReadAll(&reader)
}

func (d *XZDecompressor) Type() CompressionType {
	return TypeXZ
}

func (d *XZDecompressor) Implementation() string {
	return "CGO (spencercw/go-xz)"
}
