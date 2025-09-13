package dumper

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"github.com/xishang0128/payload-dumper-go/common/file"
	"github.com/xishang0128/payload-dumper-go/common/i18n"
	"github.com/xishang0128/payload-dumper-go/common/metadata"
	"github.com/xishang0128/payload-dumper-go/common/ziputil"
)

// Close releases underlying resources held by the Dumper (e.g. the payload reader).
func (d *Dumper) Close() error {
	if d == nil || d.payloadFile == nil {
		return nil
	}
	return d.payloadFile.Close()
}

// New creates a new Dumper instance for extracting Android OTA payloads.
// The reader parameter should be an implementation of file.Reader (such as LocalFile or HTTPFile).
//
// Example:
//
//	reader, err := file.NewLocalFile("payload.bin")
//	if err != nil {
//	    return err
//	}
//	defer reader.Close()
//
//	dumper, err := dumper.New(reader)
func New(reader file.Reader) (*Dumper, error) {
	i18n.InitLanguage()
	dumper := &Dumper{
		payloadFile: reader,
	}

	if offset, _, err := ziputil.GetStoredEntryOffset(reader, "payload.bin"); err == nil {
		dumper.baseOffset = offset
	}

	if err := dumper.parseMetadata(); err != nil {
		return nil, err
	}

	return dumper, nil
}

// ExtractPartitions extracts specified partitions from the payload.
// If partitionNames is empty, all partitions will be extracted.
func (d *Dumper) ExtractPartitions(outputDir string, partitionNames []string, workers int) error {
	return d.ExtractPartitionsWithFullOptions(outputDir, partitionNames, workers, workers, nil)
}

// ExtractPartitionsWithFullOptions extracts specified partitions with full control over concurrency options.
// partitionWorkers controls how many partitions are processed concurrently.
// operationWorkers controls how many worker threads are used per partition for processing operations.
// If partitionNames is empty, all partitions will be extracted.
// The progressCallback will be called with progress updates during extraction.
func (d *Dumper) ExtractPartitionsWithFullOptions(outputDir string, partitionNames []string, partitionWorkers, operationWorkers int, progressCallback ProgressCallback) error {
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf(i18n.I18nMsg.Common.ErrorFailedToCreateDir, err)
	}

	var partitions []*metadata.PartitionUpdate

	if len(partitionNames) == 0 {
		partitions = d.manifest.Partitions
	} else {
		for _, imageName := range partitionNames {
			imageName = strings.TrimSpace(imageName)
			found := false
			for _, partition := range d.manifest.Partitions {
				if partition.GetPartitionName() == imageName {
					partitions = append(partitions, partition)
					found = true
					break
				}
			}
			if !found {
				fmt.Printf(i18n.I18nMsg.Dumper.PartitionNotFound+"\n", imageName)
			}
		}
	}

	if len(partitions) == 0 {
		fmt.Println(i18n.I18nMsg.Dumper.NotOperatingOnPartitions)
		return nil
	}

	partitionsWithOps := make([]PartitionWithOps, 0, len(partitions))
	for _, partition := range partitions {
		operations := make([]Operation, 0, len(partition.Operations))
		for _, operation := range partition.Operations {
			operations = append(operations, Operation{
				Operation: operation,
				Offset:    d.dataOffset + int64(operation.GetDataOffset()),
				Length:    operation.GetDataLength(),
			})
		}
		partitionsWithOps = append(partitionsWithOps, PartitionWithOps{
			Partition:  partition,
			Operations: operations,
		})
	}

	return d.multiprocessPartitions(partitionsWithOps, outputDir, partitionWorkers, operationWorkers, false, "", progressCallback)
}

// ExtractPartitionsDiff extracts partitions using differential OTA.
func (d *Dumper) ExtractPartitionsDiff(outputDir string, oldDir string, partitionNames []string, workers int) error {
	return d.ExtractPartitionsDiffWithProgress(outputDir, oldDir, partitionNames, workers, nil)
}

// ExtractPartitionsDiffWithProgress extracts partitions using differential OTA with progress reporting.
func (d *Dumper) ExtractPartitionsDiffWithProgress(outputDir string, oldDir string, partitionNames []string, workers int, progressCallback ProgressCallback) error {
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf(i18n.I18nMsg.Common.ErrorFailedToCreateDir, err)
	}

	var partitions []*metadata.PartitionUpdate

	if len(partitionNames) == 0 {
		partitions = d.manifest.Partitions
	} else {
		for _, imageName := range partitionNames {
			imageName = strings.TrimSpace(imageName)
			found := false
			for _, partition := range d.manifest.Partitions {
				if partition.GetPartitionName() == imageName {
					partitions = append(partitions, partition)
					found = true
					break
				}
			}
			if !found {
				fmt.Printf(i18n.I18nMsg.Dumper.PartitionNotFound+"\n", imageName)
			}
		}
	}

	if len(partitions) == 0 {
		fmt.Println(i18n.I18nMsg.Dumper.NotOperatingOnPartitions)
		return nil
	}

	partitionsWithOps := make([]PartitionWithOps, 0, len(partitions))
	for _, partition := range partitions {
		operations := make([]Operation, 0, len(partition.Operations))
		for _, operation := range partition.Operations {
			operations = append(operations, Operation{
				Operation: operation,
				Offset:    d.dataOffset + int64(operation.GetDataOffset()),
				Length:    operation.GetDataLength(),
			})
		}
		partitionsWithOps = append(partitionsWithOps, PartitionWithOps{
			Partition:  partition,
			Operations: operations,
		})
	}

	return d.multiprocessPartitions(partitionsWithOps, outputDir, workers, workers, true, oldDir, progressCallback)
}

// ListPartitions returns partition information from the payload.
func (d *Dumper) ListPartitions() ([]PartitionInfo, error) {
	var partitionsInfo []PartitionInfo

	for _, partition := range d.manifest.GetPartitions() {
		var sizeInBlocks uint64
		for _, operation := range partition.GetOperations() {
			for _, extent := range operation.GetDstExtents() {
				sizeInBlocks += extent.GetNumBlocks()
			}
		}

		sizeInBytes := sizeInBlocks * uint64(d.blockSize)
		sizeReadable := formatSize(sizeInBytes)

		hash := ""
		if partition.GetNewPartitionInfo() != nil && partition.GetNewPartitionInfo().GetHash() != nil {
			hash = fmt.Sprintf("%x", partition.GetNewPartitionInfo().GetHash())
		}

		partitionsInfo = append(partitionsInfo, PartitionInfo{
			PartitionName: partition.GetPartitionName(),
			SizeInBlocks:  sizeInBlocks,
			SizeInBytes:   sizeInBytes,
			SizeReadable:  sizeReadable,
			Hash:          hash,
		})
	}

	return partitionsInfo, nil
}

// ListPartitionsAsMap returns partition information as a map with partition names as keys.
// This allows for efficient lookup of partition information by name.
func (d *Dumper) ListPartitionsAsMap() (map[string]PartitionInfo, error) {
	partitionsInfo := make(map[string]PartitionInfo)

	for _, partition := range d.manifest.GetPartitions() {
		var sizeInBlocks uint64
		for _, operation := range partition.GetOperations() {
			for _, extent := range operation.GetDstExtents() {
				sizeInBlocks += extent.GetNumBlocks()
			}
		}

		sizeInBytes := sizeInBlocks * uint64(d.blockSize)
		sizeReadable := formatSize(sizeInBytes)

		hash := ""
		if partition.GetNewPartitionInfo() != nil && partition.GetNewPartitionInfo().GetHash() != nil {
			hash = fmt.Sprintf("%x", partition.GetNewPartitionInfo().GetHash())
		}

		partitionName := partition.GetPartitionName()
		partitionsInfo[partitionName] = PartitionInfo{
			PartitionName: partitionName,
			SizeInBlocks:  sizeInBlocks,
			SizeInBytes:   sizeInBytes,
			SizeReadable:  sizeReadable,
			Hash:          hash,
		}
	}

	return partitionsInfo, nil
}

// getPartitionSizeInBytes calculates the total size of a partition in bytes
func (d *Dumper) getPartitionSizeInBytes(partition *metadata.PartitionUpdate) uint64 {
	var sizeInBlocks uint64
	for _, operation := range partition.GetOperations() {
		for _, extent := range operation.GetDstExtents() {
			sizeInBlocks += extent.GetNumBlocks()
		}
	}
	return sizeInBlocks * uint64(d.blockSize)
}

// shouldUseMultithread determines if a partition should be processed with multiple threads
// based on its size compared to MultithreadThreshold
func (d *Dumper) shouldUseMultithread(partition *metadata.PartitionUpdate) bool {
	sizeInBytes := d.getPartitionSizeInBytes(partition)
	return sizeInBytes > MultithreadThreshold
}

// ShouldUseMultithread determines if a partition should be processed with multiple threads
// based on its size compared to MultithreadThreshold
func (d *Dumper) ShouldUseMultithread(partition *metadata.PartitionUpdate) bool {
	return d.shouldUseMultithread(partition)
}

// MultiprocessPartitions processes partitions with specified concurrency settings
func (d *Dumper) MultiprocessPartitions(partitions []*PartitionWithOps, outputDir string, partitionWorkers, operationWorkers int, isDiff bool, oldDir string, progressCallback ProgressCallback) error {
	// Convert from pointer slice to value slice for internal use
	partitionsWithOps := make([]PartitionWithOps, len(partitions))
	for i, p := range partitions {
		partitionsWithOps[i] = *p
	}
	return d.multiprocessPartitions(partitionsWithOps, outputDir, partitionWorkers, operationWorkers, isDiff, oldDir, progressCallback)
}

// GetAllPartitionsWithOps returns all partitions with their operations
func (d *Dumper) GetAllPartitionsWithOps() ([]*PartitionWithOps, error) {
	partitions := make([]*PartitionWithOps, 0, len(d.manifest.Partitions))
	for _, partition := range d.manifest.Partitions {
		operations := make([]Operation, 0, len(partition.Operations))
		for _, operation := range partition.Operations {
			operations = append(operations, Operation{
				Operation: operation,
				Offset:    d.dataOffset + int64(operation.GetDataOffset()),
				Length:    operation.GetDataLength(),
			})
		}
		partitions = append(partitions, &PartitionWithOps{
			Partition:  partition,
			Operations: operations,
		})
	}
	return partitions, nil
}

// GetPartitionsWithOps returns specified partitions with their operations
func (d *Dumper) GetPartitionsWithOps(partitionNames []string) ([]*PartitionWithOps, error) {
	partitions := make([]*PartitionWithOps, 0, len(partitionNames))

	for _, imageName := range partitionNames {
		imageName = strings.TrimSpace(imageName)
		found := false
		for _, partition := range d.manifest.Partitions {
			if partition.GetPartitionName() == imageName {
				operations := make([]Operation, 0, len(partition.Operations))
				for _, operation := range partition.Operations {
					operations = append(operations, Operation{
						Operation: operation,
						Offset:    d.dataOffset + int64(operation.GetDataOffset()),
						Length:    operation.GetDataLength(),
					})
				}
				partitions = append(partitions, &PartitionWithOps{
					Partition:  partition,
					Operations: operations,
				})
				found = true
				break
			}
		}
		if !found {
			fmt.Printf(i18n.I18nMsg.Dumper.PartitionNotFound+"\n", imageName)
		}
	}

	return partitions, nil
}

func (d *Dumper) multiprocessPartitions(partitions []PartitionWithOps, outputDir string, partitionWorkers, operationWorkers int, isDiff bool, oldDir string, progressCallback ProgressCallback) error {
	if partitionWorkers <= 0 {
		partitionWorkers = runtime.NumCPU()
	}
	if operationWorkers <= 0 {
		operationWorkers = runtime.NumCPU()
	}

	var wg sync.WaitGroup
	var completedPartitions []string
	var mu sync.Mutex

	// Separate partitions by size
	largePartitions := make([]PartitionWithOps, 0)
	smallPartitions := make([]PartitionWithOps, 0)

	for _, part := range partitions {
		if d.shouldUseMultithread(part.Partition) {
			largePartitions = append(largePartitions, part)
		} else {
			smallPartitions = append(smallPartitions, part)
		}
	}

	// Process large partitions with partition-level concurrency
	largeSemaphore := make(chan struct{}, partitionWorkers)
	for _, part := range largePartitions {
		wg.Add(1)
		go func(p PartitionWithOps) {
			defer wg.Done()
			largeSemaphore <- struct{}{}
			defer func() { <-largeSemaphore }()

			partitionName := p.Partition.GetPartitionName()

			if err := d.processPartition(p, outputDir, operationWorkers, isDiff, oldDir, progressCallback); err != nil {
				log.Printf(i18n.I18nMsg.Dumper.ErrorProcessingPartition, partitionName, err)
			} else {
				mu.Lock()
				completedPartitions = append(completedPartitions, partitionName)
				mu.Unlock()
			}
		}(part)
	}

	// Process small partitions with limited concurrency (single-threaded)
	smallSemaphore := make(chan struct{}, 1) // Only 1 small partition at a time
	for _, part := range smallPartitions {
		wg.Add(1)
		go func(p PartitionWithOps) {
			defer wg.Done()
			smallSemaphore <- struct{}{}
			defer func() { <-smallSemaphore }()

			partitionName := p.Partition.GetPartitionName()

			if err := d.processPartitionSingleThreaded(p, outputDir, isDiff, oldDir, progressCallback); err != nil {
				log.Printf(i18n.I18nMsg.Dumper.ErrorProcessingPartition, partitionName, err)
			} else {
				mu.Lock()
				completedPartitions = append(completedPartitions, partitionName)
				mu.Unlock()
			}
		}(part)
	}

	wg.Wait()

	return nil
}

// ExtractPartitionsWithStrategy extracts partitions using the specified strategy and CPU count
func (d *Dumper) ExtractPartitionsWithStrategy(outputDir string, partitionNames []string, strategy ExtractionStrategy, cpuCount int, progressCallback ProgressCallback) error {
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf(i18n.I18nMsg.Common.ErrorFailedToCreateDir, err)
	}

	// Get all partitions or specified ones
	var allPartitions []*PartitionWithOps
	if len(partitionNames) == 0 {
		partitions, err := d.GetAllPartitionsWithOps()
		if err != nil {
			return fmt.Errorf(i18n.I18nMsg.Dumper.ErrorFailedToGetAllPartitions, err)
		}
		allPartitions = partitions
	} else {
		partitions, err := d.GetPartitionsWithOps(partitionNames)
		if err != nil {
			return fmt.Errorf(i18n.I18nMsg.Dumper.ErrorFailedToGetSpecifiedPartitions, err)
		}
		allPartitions = partitions
	}

	if len(allPartitions) == 0 {
		fmt.Println(i18n.I18nMsg.Dumper.NotOperatingOnPartitions)
		return nil
	}

	// Special optimization for single partition extraction
	if len(allPartitions) == 1 {
		return d.extractSinglePartitionOptimized(allPartitions[0], outputDir, cpuCount, progressCallback)
	}

	switch strategy {
	case StrategySequential:
		return d.extractPartitionsSequential(allPartitions, outputDir, cpuCount, progressCallback)
	case StrategyAdaptive:
		return d.extractPartitionsWithWorkStealing(allPartitions, outputDir, cpuCount, progressCallback)
	default:
		return fmt.Errorf(i18n.I18nMsg.Dumper.ErrorUnknownExtractionStrategy, strategy)
	}
}

func (d *Dumper) processPartition(part PartitionWithOps, outputDir string, operationWorkers int, isDiff bool, oldDir string, progressCallback ProgressCallback) error {
	partitionName := part.Partition.GetPartitionName()

	outputPath := filepath.Join(outputDir, partitionName+".img")
	outFile, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf(i18n.I18nMsg.Dumper.ErrorFailedToCreateOutputFile, err)
	}
	defer outFile.Close()

	var oldFile *os.File
	if isDiff {
		oldPath := filepath.Join(oldDir, partitionName+".img")
		oldFile, err = os.Open(oldPath)
		if err != nil {
			return fmt.Errorf(i18n.I18nMsg.Dumper.ErrorFailedToOpenOldFile, err)
		}
		defer oldFile.Close()
	}

	// compute readable size for the partition to include in progress reports
	var sizeInBlocks uint64
	for _, operation := range part.Partition.GetOperations() {
		for _, extent := range operation.GetDstExtents() {
			sizeInBlocks += extent.GetNumBlocks()
		}
	}
	sizeInBytes := sizeInBlocks * uint64(d.blockSize)
	sizeReadable := formatSize(sizeInBytes)

	var wrappedCallback ProgressCallback
	if progressCallback != nil {
		wrappedCallback = func(progress ProgressInfo) {
			progress.PartitionName = partitionName
			progress.SizeReadable = sizeReadable
			progressCallback(progress)
		}
	}

	return d.processOperationsOptimized(part.Operations, outFile, oldFile, operationWorkers, isDiff, wrappedCallback)
}

func (d *Dumper) processPartitionSingleThreaded(part PartitionWithOps, outputDir string, isDiff bool, oldDir string, progressCallback ProgressCallback) error {
	partitionName := part.Partition.GetPartitionName()

	outputPath := filepath.Join(outputDir, partitionName+".img")
	outFile, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf(i18n.I18nMsg.Dumper.ErrorFailedToCreateOutputFile, err)
	}
	defer outFile.Close()

	var oldFile *os.File
	if isDiff {
		oldPath := filepath.Join(oldDir, partitionName+".img")
		oldFile, err = os.Open(oldPath)
		if err != nil {
			return fmt.Errorf(i18n.I18nMsg.Dumper.ErrorFailedToOpenOldFile, err)
		}
		defer oldFile.Close()
	}

	// compute readable size for the partition to include in progress reports
	var sizeInBlocks uint64
	for _, operation := range part.Partition.GetOperations() {
		for _, extent := range operation.GetDstExtents() {
			sizeInBlocks += extent.GetNumBlocks()
		}
	}
	sizeInBytes := sizeInBlocks * uint64(d.blockSize)
	sizeReadable := formatSize(sizeInBytes)

	var wrappedCallback ProgressCallback
	if progressCallback != nil {
		wrappedCallback = func(progress ProgressInfo) {
			progress.PartitionName = partitionName
			progress.SizeReadable = sizeReadable
			progressCallback(progress)
		}
	}

	// Process operations sequentially for small partitions
	return d.processOperationsSingleThreaded(part.Operations, outFile, oldFile, isDiff, wrappedCallback)
}
