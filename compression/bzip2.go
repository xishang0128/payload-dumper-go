package compression

import (
	"bytes"
	"compress/bzip2"
	"io"
)

type Bzip2Decompressor struct{}

// NewBzip2Decompressor creates a new bzip2 decompressor
func NewBzip2Decompressor() Decompressor {
	return &Bzip2Decompressor{}
}

func (d *Bzip2Decompressor) Decompress(data []byte) ([]byte, error) {
	reader := bzip2.NewReader(bytes.NewReader(data))
	return io.ReadAll(reader)
}

func (d *Bzip2Decompressor) Type() CompressionType {
	return TypeBzip2
}

func (d *Bzip2Decompressor) Implementation() string {
	return "Go Standard Library (compress/bzip2)"
}
