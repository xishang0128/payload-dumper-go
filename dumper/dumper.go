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
	return d.ExtractPartitionsWithProgress(outputDir, partitionNames, workers, nil)
}

// ExtractPartitionsWithProgress extracts specified partitions with real-time progress reporting.
// If partitionNames is empty, all partitions will be extracted.
// The progressCallback will be called with progress updates during extraction.
func (d *Dumper) ExtractPartitionsWithProgress(outputDir string, partitionNames []string, workers int, progressCallback ProgressCallback) error {
	return d.ExtractPartitionsWithOptionsAndProgress(outputDir, partitionNames, workers, false, progressCallback)
}

// ExtractPartitionsWithOptions extracts specified partitions with configurable options.
// If partitionNames is empty, all partitions will be extracted.
// If useMemoryBuffer is true, partitions will be processed in memory before writing (uses more RAM but may be faster for small partitions).
// If useMemoryBuffer is false, partitions will be written directly to disk (more memory efficient for large partitions).
func (d *Dumper) ExtractPartitionsWithOptions(outputDir string, partitionNames []string, workers int, useMemoryBuffer bool) error {
	return d.ExtractPartitionsWithOptionsAndProgress(outputDir, partitionNames, workers, useMemoryBuffer, nil)
}

// ExtractPartitionsWithOptionsAndProgress extracts specified partitions with configurable options and progress reporting.
// If partitionNames is empty, all partitions will be extracted.
// If useMemoryBuffer is true, partitions will be processed in memory before writing (uses more RAM but may be faster for small partitions).
// If useMemoryBuffer is false, partitions will be written directly to disk (more memory efficient for large partitions).
// The progressCallback will be called with progress updates during extraction.
func (d *Dumper) ExtractPartitionsWithOptionsAndProgress(outputDir string, partitionNames []string, workers int, useMemoryBuffer bool, progressCallback ProgressCallback) error {
	return d.ExtractPartitionsWithFullOptions(outputDir, partitionNames, workers, workers, useMemoryBuffer, progressCallback)
}

// ExtractPartitionsWithFullOptions extracts specified partitions with full control over concurrency options.
// partitionWorkers controls how many partitions are processed concurrently.
// operationWorkers controls how many worker threads are used per partition for processing operations.
// If partitionNames is empty, all partitions will be extracted.
// If useMemoryBuffer is true, partitions will be processed in memory before writing.
// The progressCallback will be called with progress updates during extraction.
func (d *Dumper) ExtractPartitionsWithFullOptions(outputDir string, partitionNames []string, partitionWorkers, operationWorkers int, useMemoryBuffer bool, progressCallback ProgressCallback) error {
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

	if useMemoryBuffer {
		return d.extractPartitionsViaMemory(partitions, outputDir, progressCallback)
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

// extractPartitionsViaMemory extracts partitions using memory buffers with progress reporting
func (d *Dumper) extractPartitionsViaMemory(partitions []*metadata.PartitionUpdate, outputDir string, progressCallback ProgressCallback) error {
	// progress rendering moved to cmd layer; dumper will only report progress via callback

	var completedPartitions []string
	var mu sync.Mutex

	var wg sync.WaitGroup
	// Use separate processing for large and small partitions
	largePartitions := make([]*metadata.PartitionUpdate, 0)
	smallPartitions := make([]*metadata.PartitionUpdate, 0)

	// Separate partitions by size
	for _, partition := range partitions {
		if d.shouldUseMultithread(partition) {
			largePartitions = append(largePartitions, partition)
		} else {
			smallPartitions = append(smallPartitions, partition)
		}
	}

	// Process large partitions with multi-threading
	largeSemaphore := make(chan struct{}, runtime.NumCPU())
	for _, partition := range largePartitions {
		wg.Add(1)
		go func(p *metadata.PartitionUpdate) {
			defer wg.Done()
			largeSemaphore <- struct{}{}
			defer func() { <-largeSemaphore }()

			partitionName := p.GetPartitionName()

			data, err := d.extractSinglePartitionToBytesOptimized(p)
			if err != nil {
				log.Printf(i18n.I18nMsg.Dumper.ErrorProcessingPartition, partitionName, err)
				return
			}

			outputPath := filepath.Join(outputDir, partitionName+".img")
			if err := os.WriteFile(outputPath, data, 0644); err != nil {
				log.Printf(i18n.I18nMsg.Dumper.ErrorFailedToWritePartition, partitionName, err)
				return
			}

			mu.Lock()
			completedPartitions = append(completedPartitions, partitionName)
			completed := len(completedPartitions)
			total := len(partitions)
			mu.Unlock()

			// Call progress callback if provided
			if progressCallback != nil {
				progressPercent := float64(completed) / float64(total) * 100
				progressInfo := ProgressInfo{
					PartitionName:   partitionName,
					TotalOperations: len(p.Operations),
					CompletedOps:    len(p.Operations),
					ProgressPercent: progressPercent,
				}
				progressCallback(progressInfo)
			}
		}(partition)
	}

	// Process small partitions with single-threading (sequential)
	smallSemaphore := make(chan struct{}, 1) // Only allow 1 at a time for small partitions
	for _, partition := range smallPartitions {
		wg.Add(1)
		go func(p *metadata.PartitionUpdate) {
			defer wg.Done()
			smallSemaphore <- struct{}{}
			defer func() { <-smallSemaphore }()

			partitionName := p.GetPartitionName()

			data, err := d.extractSinglePartitionToBytesSingleThreaded(p)
			if err != nil {
				log.Printf(i18n.I18nMsg.Dumper.ErrorProcessingPartition, partitionName, err)
				return
			}

			outputPath := filepath.Join(outputDir, partitionName+".img")
			if err := os.WriteFile(outputPath, data, 0644); err != nil {
				log.Printf(i18n.I18nMsg.Dumper.ErrorFailedToWritePartition, partitionName, err)
				return
			}

			mu.Lock()
			completedPartitions = append(completedPartitions, partitionName)
			completed := len(completedPartitions)
			total := len(partitions)
			mu.Unlock()

			// Call progress callback if provided
			if progressCallback != nil {
				progressPercent := float64(completed) / float64(total) * 100
				progressInfo := ProgressInfo{
					PartitionName:   partitionName,
					TotalOperations: len(p.Operations),
					CompletedOps:    len(p.Operations),
					ProgressPercent: progressPercent,
				}
				progressCallback(progressInfo)
			}
		}(partition)
	}

	wg.Wait()

	return nil
}

// extractSinglePartitionToBytesOptimized extracts a single partition to bytes with optimized threading
// Uses multi-threading for large partitions (>128MB) and single-threading for small partitions
func (d *Dumper) extractSinglePartitionToBytesOptimized(partition *metadata.PartitionUpdate) ([]byte, error) {
	// Check if this partition should use multi-threading
	if !d.shouldUseMultithread(partition) {
		return d.extractSinglePartitionToBytesSingleThreaded(partition)
	}

	var sizeInBlocks uint64
	for _, operation := range partition.GetOperations() {
		for _, extent := range operation.GetDstExtents() {
			sizeInBlocks += extent.GetNumBlocks()
		}
	}
	sizeInBytes := sizeInBlocks * uint64(d.blockSize)

	partitionData := make([]byte, sizeInBytes)

	operations := make([]Operation, 0, len(partition.Operations))
	for _, operation := range partition.Operations {
		operations = append(operations, Operation{
			Operation: operation,
			Offset:    d.dataOffset + int64(operation.GetDataOffset()),
			Length:    operation.GetDataLength(),
		})
	}

	bufferPool := make(chan []byte, runtime.NumCPU())
	for i := 0; i < runtime.NumCPU(); i++ {
		bufferPool <- nil
	}

	type workItem struct {
		index int
		op    Operation
	}

	workChan := make(chan workItem, runtime.NumCPU()*2)
	resultChan := make(chan error, len(operations))

	workerCount := min(runtime.NumCPU(), len(operations))

	var wg sync.WaitGroup
	wg.Add(workerCount)

	for range workerCount {
		go func() {
			defer wg.Done()
			for item := range workChan {
				err := d.processOperationToBytesOptimized(item.op, partitionData, bufferPool)
				resultChan <- err
			}
		}()
	}

	go func() {
		defer close(workChan)
		for i, op := range operations {
			workChan <- workItem{index: i, op: op}
		}
	}()

	go func() {
		wg.Wait()
		close(resultChan)
	}()

	for err := range resultChan {
		if err != nil {
			return nil, err
		}
	}

	return partitionData, nil
}

// extractSinglePartitionToBytesSingleThreaded extracts a single partition to bytes using single-threaded processing
func (d *Dumper) extractSinglePartitionToBytesSingleThreaded(partition *metadata.PartitionUpdate) ([]byte, error) {
	var sizeInBlocks uint64
	for _, operation := range partition.GetOperations() {
		for _, extent := range operation.GetDstExtents() {
			sizeInBlocks += extent.GetNumBlocks()
		}
	}
	sizeInBytes := sizeInBlocks * uint64(d.blockSize)

	partitionData := make([]byte, sizeInBytes)

	operations := make([]Operation, 0, len(partition.Operations))
	for _, operation := range partition.Operations {
		operations = append(operations, Operation{
			Operation: operation,
			Offset:    d.dataOffset + int64(operation.GetDataOffset()),
			Length:    operation.GetDataLength(),
		})
	}

	// Process operations sequentially for small partitions
	for _, op := range operations {
		err := d.processOperationToBytesOptimized(op, partitionData, nil) // no buffer pool for single-threaded
		if err != nil {
			return nil, err
		}
	}

	return partitionData, nil
}

// extractPartitionsToBytesWithProgress is a helper function for internal use with progress reporting
func (d *Dumper) extractPartitionsToBytesWithProgress(partitions []*metadata.PartitionUpdate, progressCallback ProgressCallback) (map[string][]byte, error) {
	result := make(map[string][]byte)

	for i, partition := range partitions {
		partitionName := partition.GetPartitionName()

		var sizeInBlocks uint64
		for _, operation := range partition.GetOperations() {
			for _, extent := range operation.GetDstExtents() {
				sizeInBlocks += extent.GetNumBlocks()
			}
		}
		sizeInBytes := sizeInBlocks * uint64(d.blockSize)

		partitionData := make([]byte, sizeInBytes)

		operations := make([]Operation, 0, len(partition.Operations))
		for _, operation := range partition.Operations {
			operations = append(operations, Operation{
				Operation: operation,
				Offset:    d.dataOffset + int64(operation.GetDataOffset()),
				Length:    operation.GetDataLength(),
			})
		}

		var wrappedCallback func(int, int)
		if progressCallback != nil {
			wrappedCallback = func(completed, total int) {
				progressPercent := float64(completed) / float64(total) * 100
				progressInfo := ProgressInfo{
					PartitionName:   partitionName,
					TotalOperations: total,
					CompletedOps:    completed,
					ProgressPercent: progressPercent,
				}
				progressCallback(progressInfo)
			}
		}

		if err := d.processOperationsToBytesWithProgress(operations, partitionData, wrappedCallback); err != nil {
			return nil, fmt.Errorf(i18n.I18nMsg.Dumper.ErrorFailedToProcessPartition, partitionName, err)
		}

		result[partitionName] = partitionData

		if progressCallback != nil {
			overallProgress := float64(i+1) / float64(len(partitions)) * 100
			progressInfo := ProgressInfo{
				PartitionName:   partitionName,
				TotalOperations: len(operations),
				CompletedOps:    len(operations),
				ProgressPercent: overallProgress,
			}
			progressCallback(progressInfo)
		}
	}

	return result, nil
}

// ExtractPartitionsToBytes extracts specified partitions from the payload and returns their bytes.
// Returns a map where keys are partition names and values are the partition data as byte slices.
// If partitionNames is empty, all partitions will be extracted.
func (d *Dumper) ExtractPartitionsToBytes(partitionNames []string) (map[string][]byte, error) {
	return d.ExtractPartitionsToBytesWithProgress(partitionNames, nil)
}

// ExtractPartitionsToBytesWithProgress extracts specified partitions and returns their bytes with progress reporting.
// Returns a map where keys are partition names and values are the partition data as byte slices.
// If partitionNames is empty, all partitions will be extracted.
func (d *Dumper) ExtractPartitionsToBytesWithProgress(partitionNames []string, progressCallback ProgressCallback) (map[string][]byte, error) {
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
		return make(map[string][]byte), nil
	}

	return d.extractPartitionsToBytesWithProgress(partitions, progressCallback)
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

// GetPartitionInfo returns information for a specific partition by name.
// Returns nil and an error if the partition is not found.
func (d *Dumper) GetPartitionInfo(partitionName string) (*PartitionInfo, error) {
	partitionsMap, err := d.ListPartitionsAsMap()
	if err != nil {
		return nil, err
	}

	info, exists := partitionsMap[partitionName]
	if !exists {
		return nil, fmt.Errorf(i18n.I18nMsg.Dumper.PartitionNotFound, partitionName)
	}

	return &info, nil
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
