//go:build (!cgo) || force_pure_compression || pure_xz
// +build !cgo force_pure_compression pure_xz

package compression

import (
	"bytes"
	"io"

	"github.com/ulikunitz/xz"
)

type XZDecompressor struct{}

// NewXZDecompressor creates a new XZ decompressor using pure Go implementation
func NewXZDecompressor() Decompressor {
	return &XZDecompressor{}
}

func (d *XZDecompressor) Decompress(data []byte) ([]byte, error) {
	reader, err := xz.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	return io.ReadAll(reader)
}

func (d *XZDecompressor) Type() CompressionType {
	return TypeXZ
}

func (d *XZDecompressor) Implementation() string {
	return "Pure Go (ulikunitz/xz)"
}
