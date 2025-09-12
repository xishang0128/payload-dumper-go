package dumper

import (
	"fmt"

	"github.com/xishang0128/payload-dumper-go/common/file"
	"github.com/xishang0128/payload-dumper-go/common/metadata"
)

// GetXZImplementation returns the current XZ decompression implementation in use.
func GetXZImplementation() string {
	return getXZImplementation()
}

// Dumper handles the extraction of Android OTA payloads
type Dumper struct {
	payloadFile file.Reader
	baseOffset  int64
	dataOffset  int64
	manifest    *metadata.DeltaArchiveManifest
	blockSize   uint32
}

// MaxBufferSize controls the maximum temporary buffer size used while processing operations.
// Value is in bytes. Default set internally but can be overridden by callers (e.g., cmd layer).
var MaxBufferSize int64 = 64 * 1024 * 1024 // 64 MB

// PartitionInfo represents information about a partition
type PartitionInfo struct {
	PartitionName string `json:"partition_name"`
	SizeInBlocks  uint64 `json:"size_in_blocks"`
	SizeInBytes   uint64 `json:"size_in_bytes"`
	SizeReadable  string `json:"size_readable"`
	Hash          string `json:"hash"`
}

// ProgressInfo represents progress information for extraction
type ProgressInfo struct {
	PartitionName    string  `json:"partition_name"`
	TotalOperations  int     `json:"total_operations"`
	CompletedOps     int     `json:"completed_operations"`
	ProgressPercent  float64 `json:"progress_percent"`
	OperationsPerSec float64 `json:"operations_per_sec"`
	EstimatedTime    string  `json:"estimated_time"`
	SizeReadable     string  `json:"size_readable"`
}

// ProgressCallback is a function type for receiving progress updates
type ProgressCallback func(progress ProgressInfo)

// Operation represents an operation to be performed
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

// formatSize converts bytes into a human-readable string.
func formatSize(bytes uint64) string {
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
