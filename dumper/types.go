package dumper

import (
	"fmt"

	"github.com/xishang0128/payload-dumper-go/common/file"
	"github.com/xishang0128/payload-dumper-go/common/metadata"
	"github.com/xishang0128/payload-dumper-go/compression"
)

// GetXZImplementation returns the current XZ decompression implementation in use.
func GetXZImplementation() string {
	manager := compression.NewDecompressorManager()
	decompressor, err := manager.GetDecompressor(compression.TypeXZ)
	if err != nil {
		return "Unknown"
	}
	impl := decompressor.Implementation()
	if impl == "CGO (spencercw/go-xz)" {
		return "CGO"
	}
	return "Pure Go"
}

// CompressionImplementation represents the implementation type for a compression algorithm
type CompressionImplementation struct {
	Algorithm string // Algorithm name
	IsCGO     bool   // true if using CGO, false if Pure Go
}

// GetCompressionImplementations returns the implementation information for all compression algorithms
func GetCompressionImplementations() map[string]CompressionImplementation {
	manager := compression.NewDecompressorManager()
	implementations := make(map[string]CompressionImplementation)

	// Define all compression types
	compressionTypes := map[compression.CompressionType]string{
		compression.TypeXZ:     "XZ",
		compression.TypeBzip2:  "bzip2",
		compression.TypeZSTD:   "ZSTD",
		compression.TypeBrotli: "Brotli",
	}

	for compType, name := range compressionTypes {
		decompressor, err := manager.GetDecompressor(compType)
		if err != nil {
			implementations[name] = CompressionImplementation{
				Algorithm: name,
				IsCGO:     false, // Default to Pure Go if unknown
			}
			continue
		}

		impl := decompressor.Implementation()
		isCGO := containsAny(impl, []string{"CGO", "cgo"})

		implementations[name] = CompressionImplementation{
			Algorithm: name,
			IsCGO:     isCGO,
		}
	}

	return implementations
}

// Helper function to check if string contains any of the substrings
func containsAny(s string, substrings []string) bool {
	for _, substr := range substrings {
		if len(s) >= len(substr) {
			for i := 0; i <= len(s)-len(substr); i++ {
				if s[i:i+len(substr)] == substr {
					return true
				}
			}
		}
	}
	return false
}

// Dumper handles the extraction of Android OTA payloads
type Dumper struct {
	payloadFile        file.Reader
	baseOffset         int64
	dataOffset         int64
	manifest           *metadata.DeltaArchiveManifest
	blockSize          uint32
	compressionManager *compression.DecompressorManager
}

var MaxBufferSize int64 = 64 * 1024 * 1024

var MultithreadThreshold uint64 = 128 * 1024 * 1024

func SetMultithreadThreshold(threshold uint64) {
	MultithreadThreshold = threshold
}

// PartitionInfo contains information about a partition
type PartitionInfo struct {
	PartitionName string `json:"partition_name"`
	SizeInBlocks  uint64 `json:"size_in_blocks"`
	SizeInBytes   uint64 `json:"size_in_bytes"`
	SizeReadable  string `json:"size_readable"`
	Hash          string `json:"hash"`
}

// ProgressInfo contains progress information for extraction operations
type ProgressInfo struct {
	PartitionName    string  `json:"partition_name"`
	TotalOperations  int     `json:"total_operations"`
	CompletedOps     int     `json:"completed_operations"`
	ProgressPercent  float64 `json:"progress_percent"`
	OperationsPerSec float64 `json:"operations_per_sec"`
	EstimatedTime    string  `json:"estimated_time"`
	SizeReadable     string  `json:"size_readable"`
}

// ProgressCallback is called during extraction to report progress
type ProgressCallback func(progress ProgressInfo)

// Operation represents a single update operation
type Operation struct {
	Operation *metadata.InstallOperation
	Offset    int64
	Length    uint64
}

// PartitionWithOps represents a partition with its operations
type PartitionWithOps struct {
	Partition  *metadata.PartitionUpdate
	Operations []Operation
}

func fmtSize(bytes uint64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
	)

	if bytes >= GB {
		return fmt.Sprintf("%.1fGB", float64(bytes)/GB)
	} else if bytes >= MB {
		return fmt.Sprintf("%.1fMB", float64(bytes)/MB)
	} else {
		return fmt.Sprintf("%.1fKB", float64(bytes)/KB)
	}
}
