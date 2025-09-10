//go:build cgo
// +build cgo

package dumper

import (
	"bytes"
	"io"

	"github.com/spencercw/go-xz"
)

func decompressXZ(data []byte) ([]byte, error) {
	reader := xz.NewDecompressionReader(bytes.NewReader(data))
	defer reader.Close()
	return io.ReadAll(&reader)
}

func getXZImplementation() string {
	return "CGO"
}
