package dumper

import (
	"bytes"
	"compress/bzip2"
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"github.com/vbauerster/mpb/v8"
	"github.com/vbauerster/mpb/v8/decor"
	"github.com/xishang0128/payload-dumper-go/common/file"
	"github.com/xishang0128/payload-dumper-go/common/i18n"
	"github.com/xishang0128/payload-dumper-go/common/metadata"
	"github.com/xishang0128/payload-dumper-go/common/ziputil"

	"google.golang.org/protobuf/proto"
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

// PartitionInfo represents information about a partition
type PartitionInfo struct {
	PartitionName string `json:"partition_name"`
	SizeInBlocks  uint64 `json:"size_in_blocks"`
	SizeInBytes   uint64 `json:"size_in_bytes"`
	SizeReadable  string `json:"size_readable"`
	Hash          string `json:"hash"`
}

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
	return d.ExtractPartitionsWithOptions(outputDir, partitionNames, workers, false)
}

// ExtractPartitionsWithOptions extracts specified partitions with configurable options.
// If partitionNames is empty, all partitions will be extracted.
// If useMemoryBuffer is true, partitions will be processed in memory before writing (uses more RAM but may be faster for small partitions).
// If useMemoryBuffer is false, partitions will be written directly to disk (more memory efficient for large partitions).
func (d *Dumper) ExtractPartitionsWithOptions(outputDir string, partitionNames []string, workers int, useMemoryBuffer bool) error {
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
		return d.extractPartitionsViaMemory(partitions, outputDir)
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

	return d.multiprocessPartitions(partitionsWithOps, outputDir, workers, false, "")
}

// extractPartitionsViaMemory extracts partitions using memory buffers before writing to files
func (d *Dumper) extractPartitionsViaMemory(partitions []*metadata.PartitionUpdate, outputDir string) error {
	progress := mpb.New()

	bars := make(map[string]*mpb.Bar)
	for _, partition := range partitions {
		partitionName := partition.GetPartitionName()
		var sizeInBlocks uint64
		for _, operation := range partition.GetOperations() {
			for _, extent := range operation.GetDstExtents() {
				sizeInBlocks += extent.GetNumBlocks()
			}
		}
		sizeInBytes := sizeInBlocks * uint64(d.blockSize)
		sizeReadable := formatSize(sizeInBytes)

		partitionDesc := fmt.Sprintf("[%s](%s)", partitionName, sizeReadable)
		bar := progress.AddBar(int64(len(partition.Operations)),
			mpb.PrependDecorators(
				decor.Name(partitionDesc, decor.WCSyncSpaceR),
			),
			mpb.AppendDecorators(
				decor.Percentage(decor.WC{W: 5}),
				decor.Counters(0, " | %d/%d"),
				decor.AverageETA(decor.ET_STYLE_GO, decor.WC{W: 6}, decor.WCSyncSpace),
				decor.AverageSpeed(0, fmt.Sprintf(" | %%.2f %s", i18n.I18nMsg.Dumper.OpsSuffix)),
			),
		)
		bars[partitionName] = bar
	}

	var completedPartitions []string
	var mu sync.Mutex

	var wg sync.WaitGroup
	semaphore := make(chan struct{}, runtime.NumCPU())

	for _, partition := range partitions {
		wg.Add(1)
		go func(p *metadata.PartitionUpdate) {
			defer wg.Done()
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			partitionName := p.GetPartitionName()
			bar := bars[partitionName]

			data, err := d.extractSinglePartitionToBytesOptimized(p, bar)
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
			mu.Unlock()
		}(partition)
	}

	wg.Wait()
	progress.Wait()

	if len(completedPartitions) > 0 {
		fmt.Printf("\n" + i18n.I18nMsg.Dumper.CompletedPartitions)
		for i, partition := range completedPartitions {
			if i > 0 {
				fmt.Printf(", ")
			}
			fmt.Printf("%s", partition)
		}
		fmt.Printf("\n")
	}

	return nil
}

// extractSinglePartitionToBytesOptimized extracts a single partition to bytes with optimized multi-threading
func (d *Dumper) extractSinglePartitionToBytesOptimized(partition *metadata.PartitionUpdate, bar *mpb.Bar) ([]byte, error) {
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

	const maxBufferSize = 64 * 1024 * 1024 // 64MB
	bufferPool := make(chan []byte, runtime.NumCPU())
	for i := 0; i < runtime.NumCPU(); i++ {
		bufferPool <- make([]byte, 0, maxBufferSize)
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
				bar.Increment()
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

// extractPartitionsToBytes is a helper function for internal use
func (d *Dumper) extractPartitionsToBytes(partitions []*metadata.PartitionUpdate) (map[string][]byte, error) {
	result := make(map[string][]byte)

	for _, partition := range partitions {
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

		if err := d.processOperationsToBytes(operations, partitionData); err != nil {
			return nil, fmt.Errorf(i18n.I18nMsg.Dumper.ErrorFailedToProcessPartition, partitionName, err)
		}

		result[partitionName] = partitionData
	}

	return result, nil
}

// ExtractPartitionsToBytes extracts specified partitions from the payload and returns their bytes.
// Returns a map where keys are partition names and values are the partition data as byte slices.
// If partitionNames is empty, all partitions will be extracted.
func (d *Dumper) ExtractPartitionsToBytes(partitionNames []string) (map[string][]byte, error) {
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

	return d.extractPartitionsToBytes(partitions)
}

// ExtractPartitionsDiff extracts partitions using differential OTA.
func (d *Dumper) ExtractPartitionsDiff(outputDir string, oldDir string, partitionNames []string, workers int) error {
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

	return d.multiprocessPartitions(partitionsWithOps, outputDir, workers, true, oldDir)
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

// ExtractMetadata extracts and returns the metadata from the payload.
func (d *Dumper) ExtractMetadata() ([]byte, error) {
	metadataPath := "META-INF/com/android/metadata"
	offset, size, err := ziputil.GetStoredEntryOffset(d.payloadFile, metadataPath)
	if err != nil {
		return nil, fmt.Errorf(i18n.I18nMsg.Dumper.ErrorFailedToExtractMetadata, metadataPath, err)
	}

	data, err := d.payloadFile.Read(offset, int(size))
	if err != nil {
		return nil, fmt.Errorf(i18n.I18nMsg.Dumper.ErrorFailedToReadMetadata, err)
	}

	return data, nil
}

func (d *Dumper) parseMetadata() error {
	headLen := 4 + 8 + 8 + 4
	buffer, err := d.payloadFile.Read(d.baseOffset, headLen)
	if err != nil {
		return err
	}

	if len(buffer) != headLen {
		return fmt.Errorf(i18n.I18nMsg.Dumper.ErrorInsufficientDataForHeader)
	}

	magic := string(buffer[:4])
	if magic != "CrAU" {
		return fmt.Errorf(i18n.I18nMsg.Dumper.ErrorInvalidMagic, magic)
	}

	fileFormatVersion := binary.BigEndian.Uint64(buffer[4:12])
	if fileFormatVersion != 2 {
		return fmt.Errorf(i18n.I18nMsg.Dumper.ErrorUnsupportedFileFormat, fileFormatVersion)
	}

	manifestSize := binary.BigEndian.Uint64(buffer[12:20])
	metadataSignatureSize := uint32(0)

	if fileFormatVersion > 1 {
		metadataSignatureSize = binary.BigEndian.Uint32(buffer[20:24])
	}

	fp := d.baseOffset + int64(headLen)

	manifestData, err := d.payloadFile.Read(fp, int(manifestSize))
	if err != nil {
		return err
	}
	fp += int64(manifestSize)

	fp += int64(metadataSignatureSize)

	d.dataOffset = fp
	d.manifest = &metadata.DeltaArchiveManifest{}

	if err := proto.Unmarshal(manifestData, d.manifest); err != nil {
		return fmt.Errorf(i18n.I18nMsg.Dumper.ErrorFailedToParseManifest, err)
	}

	d.blockSize = d.manifest.GetBlockSize()
	if d.blockSize == 0 {
		d.blockSize = 4096
	}

	return nil
}

func (d *Dumper) multiprocessPartitions(partitions []PartitionWithOps, outputDir string, workers int, isDiff bool, oldDir string) error {
	if workers <= 0 {
		workers = runtime.NumCPU()
	}

	progress := mpb.New()

	bars := make(map[string]*mpb.Bar)
	for _, part := range partitions {
		partitionName := part.Partition.GetPartitionName()
		var sizeInBlocks uint64
		for _, operation := range part.Partition.GetOperations() {
			for _, extent := range operation.GetDstExtents() {
				sizeInBlocks += extent.GetNumBlocks()
			}
		}
		sizeInBytes := sizeInBlocks * uint64(d.blockSize)
		sizeReadable := formatSize(sizeInBytes)

		partitionDesc := fmt.Sprintf("[%s](%s)", partitionName, sizeReadable)
		bar := progress.AddBar(int64(len(part.Operations)),
			mpb.PrependDecorators(
				decor.Name(partitionDesc, decor.WCSyncSpaceR),
			),
			mpb.AppendDecorators(
				decor.Percentage(decor.WC{W: 5}),
				decor.Counters(0, " | %d/%d"),
				decor.AverageETA(decor.ET_STYLE_GO, decor.WC{W: 6}, decor.WCSyncSpace),
				decor.AverageSpeed(0, fmt.Sprintf(" | %%.2f %s", i18n.I18nMsg.Dumper.OpsSuffix)),
			),
		)
		bars[partitionName] = bar
	}

	var wg sync.WaitGroup
	semaphore := make(chan struct{}, workers)
	var completedPartitions []string
	var mu sync.Mutex

	for _, part := range partitions {
		wg.Add(1)
		go func(p PartitionWithOps) {
			defer wg.Done()
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			partitionName := p.Partition.GetPartitionName()
			bar := bars[partitionName]

			if err := d.processPartitionWithBar(p, outputDir, isDiff, oldDir, bar); err != nil {
				log.Printf(i18n.I18nMsg.Dumper.ErrorProcessingPartition, partitionName, err)
			} else {
				mu.Lock()
				completedPartitions = append(completedPartitions, partitionName)
				mu.Unlock()
			}
		}(part)
	}

	wg.Wait()
	progress.Wait()

	if len(completedPartitions) > 0 {
		fmt.Printf("\n" + i18n.I18nMsg.Dumper.CompletedPartitions)
		for i, partition := range completedPartitions {
			if i > 0 {
				fmt.Printf(", ")
			}
			fmt.Printf("%s", partition)
		}
		fmt.Printf("\n")
	}

	return nil
}

func (d *Dumper) processPartitionWithBar(part PartitionWithOps, outputDir string, isDiff bool, oldDir string, bar *mpb.Bar) error {
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

	return d.processOperationsOptimized(part.Operations, outFile, oldFile, isDiff, bar)
}

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

func (d *Dumper) processOperationsOptimized(operations []Operation, outFile *os.File, oldFile *os.File, isDiff bool, bar *mpb.Bar) error {
	const maxBufferSize = 64 * 1024 * 1024 // 64MB
	bufferPool := make(chan []byte, runtime.NumCPU())
	for i := 0; i < runtime.NumCPU(); i++ {
		bufferPool <- make([]byte, 0, maxBufferSize)
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
				err := d.processOperationOptimized(item.op, outFile, oldFile, isDiff, bufferPool)
				resultChan <- err
				bar.Increment()
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
			return err
		}
	}

	return nil
}

func (d *Dumper) processOperationOptimized(op Operation, outFile *os.File, oldFile *os.File, isDiff bool, bufferPool chan []byte) error {
	operation := op.Operation

	var data []byte
	var err error

	if op.Length > 0 {
		buffer := <-bufferPool
		defer func() {
			buffer = buffer[:0]
			bufferPool <- buffer
		}()

		if cap(buffer) < int(op.Length) {
			buffer = make([]byte, op.Length)
		} else {
			buffer = buffer[:op.Length]
		}

		data, err = d.payloadFile.Read(op.Offset, int(op.Length))
		if err != nil {
			return fmt.Errorf(i18n.I18nMsg.Dumper.ErrorFailedToReadOperationData, err)
		}
	}

	switch operation.GetType() {
	case metadata.InstallOperation_REPLACE:
		return d.writeToExtentsThreadSafe(outFile, operation.GetDstExtents(), data)

	case metadata.InstallOperation_REPLACE_BZ:
		reader := bzip2.NewReader(bytes.NewReader(data))
		decompressed, err := io.ReadAll(reader)
		if err != nil {
			return fmt.Errorf(i18n.I18nMsg.Dumper.ErrorFailedToDecompressBzip2, err)
		}
		return d.writeToExtentsThreadSafe(outFile, operation.GetDstExtents(), decompressed)

	case metadata.InstallOperation_REPLACE_XZ:
		decompressed, err := decompressXZ(data)
		if err != nil {
			return fmt.Errorf(i18n.I18nMsg.Dumper.ErrorFailedToDecompressXZ, err)
		}
		return d.writeToExtentsThreadSafe(outFile, operation.GetDstExtents(), decompressed)

	case metadata.InstallOperation_SOURCE_COPY:
		if !isDiff {
			return fmt.Errorf(i18n.I18nMsg.Dumper.ErrorSourceCopyOnlyForDiff)
		}
		return d.sourceCopyThreadSafe(outFile, oldFile, operation)

	case metadata.InstallOperation_ZERO:
		return d.writeZerosThreadSafe(outFile, operation.GetDstExtents())

	default:
		return fmt.Errorf(i18n.I18nMsg.Dumper.ErrorUnsupportedOperationType, operation.GetType())
	}
}

func (d *Dumper) writeToExtentsThreadSafe(outFile *os.File, extents []*metadata.Extent, data []byte) error {
	dataOffset := 0
	for _, extent := range extents {
		blockOffset := int64(extent.GetStartBlock()) * int64(d.blockSize)
		blockSize := int64(extent.GetNumBlocks()) * int64(d.blockSize)

		if dataOffset+int(blockSize) > len(data) {
			blockSize = int64(len(data) - dataOffset)
		}

		if blockSize <= 0 {
			break
		}

		if _, err := outFile.WriteAt(data[dataOffset:dataOffset+int(blockSize)], blockOffset); err != nil {
			return fmt.Errorf(i18n.I18nMsg.Dumper.ErrorFailedToWriteToFile, err)
		}

		dataOffset += int(blockSize)
	}
	return nil
}

func (d *Dumper) sourceCopyThreadSafe(outFile *os.File, oldFile *os.File, operation *metadata.InstallOperation) error {
	for _, srcExtent := range operation.GetSrcExtents() {
		blockOffset := int64(srcExtent.GetStartBlock()) * int64(d.blockSize)
		blockSize := int64(srcExtent.GetNumBlocks()) * int64(d.blockSize)

		data := make([]byte, blockSize)
		if _, err := oldFile.ReadAt(data, blockOffset); err != nil {
			return fmt.Errorf(i18n.I18nMsg.Dumper.ErrorFailedToReadFromOldFile, err)
		}

		dstOffset := int64(operation.GetDstExtents()[0].GetStartBlock()) * int64(d.blockSize)
		if _, err := outFile.WriteAt(data, dstOffset); err != nil {
			return fmt.Errorf(i18n.I18nMsg.Dumper.ErrorFailedToWriteToFile, err)
		}
	}
	return nil
}

func (d *Dumper) writeZerosThreadSafe(outFile *os.File, extents []*metadata.Extent) error {
	const maxZeroChunk = 1024 * 1024
	var zeroBuffer []byte

	for _, extent := range extents {
		blockOffset := int64(extent.GetStartBlock()) * int64(d.blockSize)
		blockSize := int64(extent.GetNumBlocks()) * int64(d.blockSize)

		remaining := blockSize
		offset := blockOffset

		for remaining > 0 {
			chunkSize := min(remaining, maxZeroChunk)

			if len(zeroBuffer) < int(chunkSize) {
				zeroBuffer = make([]byte, chunkSize)
			} else {
				zeroBuffer = zeroBuffer[:chunkSize]
				for i := range zeroBuffer {
					zeroBuffer[i] = 0
				}
			}

			if _, err := outFile.WriteAt(zeroBuffer, offset); err != nil {
				return fmt.Errorf(i18n.I18nMsg.Dumper.ErrorFailedToWriteZeros, err)
			}

			remaining -= chunkSize
			offset += chunkSize
		}
	}
	return nil
}

// processOperationsToBytes processes operations and writes data to a byte slice instead of a file
func (d *Dumper) processOperationsToBytes(operations []Operation, buffer []byte) error {
	for _, op := range operations {
		if err := d.processOperationToBytes(op, buffer); err != nil {
			return err
		}
	}
	return nil
}

// processOperationToBytes processes a single operation and writes data to a byte slice
func (d *Dumper) processOperationToBytes(op Operation, buffer []byte) error {
	operation := op.Operation

	var data []byte
	var err error

	if op.Length > 0 {
		data, err = d.payloadFile.Read(op.Offset, int(op.Length))
		if err != nil {
			return fmt.Errorf(i18n.I18nMsg.Dumper.ErrorFailedToReadOperationData, err)
		}
	}

	switch operation.GetType() {
	case metadata.InstallOperation_REPLACE:
		return d.writeToExtentsBytes(buffer, operation.GetDstExtents(), data)

	case metadata.InstallOperation_REPLACE_BZ:
		reader := bzip2.NewReader(bytes.NewReader(data))
		decompressed, err := io.ReadAll(reader)
		if err != nil {
			return fmt.Errorf(i18n.I18nMsg.Dumper.ErrorFailedToDecompressBzip2, err)
		}
		return d.writeToExtentsBytes(buffer, operation.GetDstExtents(), decompressed)

	case metadata.InstallOperation_REPLACE_XZ:
		decompressed, err := decompressXZ(data)
		if err != nil {
			return fmt.Errorf(i18n.I18nMsg.Dumper.ErrorFailedToDecompressXZ, err)
		}
		return d.writeToExtentsBytes(buffer, operation.GetDstExtents(), decompressed)

	case metadata.InstallOperation_ZERO:
		return d.writeZerosBytes(buffer, operation.GetDstExtents())

	case metadata.InstallOperation_SOURCE_COPY:
		return fmt.Errorf(i18n.I18nMsg.Dumper.ErrorSourceCopyNotSupportedInBytes)

	default:
		return fmt.Errorf(i18n.I18nMsg.Dumper.ErrorUnsupportedOperationType, operation.GetType())
	}
}

func (d *Dumper) writeToExtentsBytes(buffer []byte, extents []*metadata.Extent, data []byte) error {
	dataOffset := 0
	for _, extent := range extents {
		blockOffset := int64(extent.GetStartBlock()) * int64(d.blockSize)
		blockSize := int64(extent.GetNumBlocks()) * int64(d.blockSize)

		if dataOffset+int(blockSize) > len(data) {
			blockSize = int64(len(data) - dataOffset)
		}

		if blockSize <= 0 {
			break
		}

		if blockOffset+blockSize > int64(len(buffer)) {
			return fmt.Errorf(i18n.I18nMsg.Dumper.ErrorWriteExceedBuffer)
		}

		copy(buffer[blockOffset:blockOffset+blockSize], data[dataOffset:dataOffset+int(blockSize)])
		dataOffset += int(blockSize)
	}
	return nil
}

func (d *Dumper) writeZerosBytes(buffer []byte, extents []*metadata.Extent) error {
	for _, extent := range extents {
		blockOffset := int64(extent.GetStartBlock()) * int64(d.blockSize)
		blockSize := int64(extent.GetNumBlocks()) * int64(d.blockSize)

		if blockOffset+blockSize > int64(len(buffer)) {
			return fmt.Errorf(i18n.I18nMsg.Dumper.ErrorZeroWriteExceedBuffer)
		}

		// Clear the bytes to zero
		for i := blockOffset; i < blockOffset+blockSize; i++ {
			buffer[i] = 0
		}
	}
	return nil
}

// processOperationToBytesOptimized processes a single operation with buffer pool optimization
func (d *Dumper) processOperationToBytesOptimized(op Operation, buffer []byte, bufferPool chan []byte) error {
	operation := op.Operation

	var data []byte
	var err error

	if op.Length > 0 {
		workBuffer := <-bufferPool
		defer func() {
			workBuffer = workBuffer[:0]
			bufferPool <- workBuffer
		}()

		if cap(workBuffer) < int(op.Length) {
			workBuffer = make([]byte, op.Length)
		} else {
			workBuffer = workBuffer[:op.Length]
		}

		data, err = d.payloadFile.Read(op.Offset, int(op.Length))
		if err != nil {
			return fmt.Errorf(i18n.I18nMsg.Dumper.ErrorFailedToReadOperationData, err)
		}
	}

	switch operation.GetType() {
	case metadata.InstallOperation_REPLACE:
		return d.writeToExtentsBytesThreadSafe(buffer, operation.GetDstExtents(), data)

	case metadata.InstallOperation_REPLACE_BZ:
		reader := bzip2.NewReader(bytes.NewReader(data))
		decompressed, err := io.ReadAll(reader)
		if err != nil {
			return fmt.Errorf(i18n.I18nMsg.Dumper.ErrorFailedToDecompressBzip2, err)
		}
		return d.writeToExtentsBytesThreadSafe(buffer, operation.GetDstExtents(), decompressed)

	case metadata.InstallOperation_REPLACE_XZ:
		decompressed, err := decompressXZ(data)
		if err != nil {
			return fmt.Errorf(i18n.I18nMsg.Dumper.ErrorFailedToDecompressXZ, err)
		}
		return d.writeToExtentsBytesThreadSafe(buffer, operation.GetDstExtents(), decompressed)

	case metadata.InstallOperation_ZERO:
		return d.writeZerosBytesThreadSafe(buffer, operation.GetDstExtents())

	case metadata.InstallOperation_SOURCE_COPY:
		return fmt.Errorf(i18n.I18nMsg.Dumper.ErrorSourceCopyNotSupportedInBytes)

	default:
		return fmt.Errorf(i18n.I18nMsg.Dumper.ErrorUnsupportedOperationType, operation.GetType())
	}
}

// writeToExtentsBytesThreadSafe writes data to specific extents in a byte slice with thread safety
func (d *Dumper) writeToExtentsBytesThreadSafe(buffer []byte, extents []*metadata.Extent, data []byte) error {
	dataOffset := 0
	for _, extent := range extents {
		blockOffset := int64(extent.GetStartBlock()) * int64(d.blockSize)
		blockSize := int64(extent.GetNumBlocks()) * int64(d.blockSize)

		if dataOffset+int(blockSize) > len(data) {
			blockSize = int64(len(data) - dataOffset)
		}

		if blockSize <= 0 {
			break
		}

		if blockOffset+blockSize > int64(len(buffer)) {
			return fmt.Errorf(i18n.I18nMsg.Dumper.ErrorWriteExceedBuffer)
		}

		copy(buffer[blockOffset:blockOffset+blockSize], data[dataOffset:dataOffset+int(blockSize)])
		dataOffset += int(blockSize)
	}
	return nil
}

// writeZerosBytesThreadSafe writes zeros to specific extents in a byte slice with thread safety
func (d *Dumper) writeZerosBytesThreadSafe(buffer []byte, extents []*metadata.Extent) error {
	for _, extent := range extents {
		blockOffset := int64(extent.GetStartBlock()) * int64(d.blockSize)
		blockSize := int64(extent.GetNumBlocks()) * int64(d.blockSize)

		if blockOffset+blockSize > int64(len(buffer)) {
			return fmt.Errorf(i18n.I18nMsg.Dumper.ErrorZeroWriteExceedBuffer)
		}

		for i := blockOffset; i < blockOffset+blockSize; i++ {
			buffer[i] = 0
		}
	}
	return nil
}
