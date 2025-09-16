// Package compression provides unified decompression interfaces for Android OTA payloads.
package compression

import (
	"fmt"
	"runtime"
)

type CompressionType int

const (
	TypeNone CompressionType = iota
	TypeXZ
	TypeBzip2
	TypeZSTD
	TypeBrotli
)

// String returns the string representation of compression type
func (t CompressionType) String() string {
	switch t {
	case TypeXZ:
		return "XZ"
	case TypeBzip2:
		return "bzip2"
	case TypeZSTD:
		return "ZSTD"
	case TypeBrotli:
		return "Brotli"
	default:
		return "None"
	}
}

// Decompressor is the interface for decompression operations
type Decompressor interface {
	Decompress(data []byte) ([]byte, error)
	Type() CompressionType
	Implementation() string
}

type DecompressorManager struct {
	decompressors map[CompressionType]Decompressor
}

// NewDecompressorManager creates a new decompressor manager with all available decompressors
func NewDecompressorManager() *DecompressorManager {
	manager := &DecompressorManager{
		decompressors: make(map[CompressionType]Decompressor),
	}

	manager.decompressors[TypeXZ] = NewXZDecompressor()
	manager.decompressors[TypeBzip2] = NewBzip2Decompressor()
	manager.decompressors[TypeZSTD] = NewZSTDDecompressor()
	manager.decompressors[TypeBrotli] = NewBrotliDecompressor()

	return manager
}

// Decompress decompresses data using the specified compression type
func (m *DecompressorManager) Decompress(compType CompressionType, data []byte) ([]byte, error) {
	decompressor, exists := m.decompressors[compType]
	if !exists {
		return nil, fmt.Errorf("unsupported compression type: %s", compType.String())
	}

	return decompressor.Decompress(data)
}

// GetDecompressor returns the decompressor for the specified type
func (m *DecompressorManager) GetDecompressor(compType CompressionType) (Decompressor, error) {
	decompressor, exists := m.decompressors[compType]
	if !exists {
		return nil, fmt.Errorf("unsupported compression type: %s", compType.String())
	}
	return decompressor, nil
}

// GetSupportedTypes returns all supported compression types
func (m *DecompressorManager) GetSupportedTypes() []CompressionType {
	types := make([]CompressionType, 0, len(m.decompressors))
	for t := range m.decompressors {
		types = append(types, t)
	}
	return types
}

// GetImplementationInfo returns information about the implementation of each decompressor
func (m *DecompressorManager) GetImplementationInfo() map[CompressionType]string {
	info := make(map[CompressionType]string)
	for t, d := range m.decompressors {
		info[t] = d.Implementation()
	}
	return info
}

// GetBuildInfo returns build information about compression support
func GetBuildInfo() map[string]interface{} {
	info := map[string]interface{}{
		"go_version": runtime.Version(),
		"goos":       runtime.GOOS,
		"goarch":     runtime.GOARCH,
	}

	info["cgo_enabled"] = isCGOEnabled()

	return info
}

func isCGOEnabled() bool {
	return getCGOStatus()
}
