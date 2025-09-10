//go:build !cgo
// +build !cgo

package dumper

import (
	"bytes"
	"io"

	"github.com/ulikunitz/xz"
)

func decompressXZ(data []byte) ([]byte, error) {
	reader, err := xz.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	return io.ReadAll(reader)
}

func getXZImplementation() string {
	return "Pure Go"
}
