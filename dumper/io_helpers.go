package dumper

import (
	"bytes"
	"compress/bzip2"
	"fmt"
	"io"
	"os"
	"runtime"
	"sync"

	"github.com/xishang0128/payload-dumper-go/common/i18n"
	"github.com/xishang0128/payload-dumper-go/common/metadata"
)

func (d *Dumper) processOperationsOptimized(operations []Operation, outFile *os.File, oldFile *os.File, operationWorkers int, isDiff bool, progressCallback ProgressCallback) error {
	if operationWorkers <= 0 {
		operationWorkers = runtime.NumCPU()
	}

	// Use nil placeholders to avoid preallocating large buffers. Allocate on demand in workers.
	bufferPool := make(chan []byte, operationWorkers)
	for i := 0; i < operationWorkers; i++ {
		bufferPool <- nil
	}

	type workItem struct {
		index int
		op    Operation
	}

	workChan := make(chan workItem, operationWorkers*2)
	resultChan := make(chan error, len(operations))

	workerCount := min(operationWorkers, len(operations))

	var wg sync.WaitGroup
	wg.Add(workerCount)

	totalOps := len(operations)
	var completedOps int64
	var progressMutex sync.Mutex

	for range workerCount {
		go func() {
			defer wg.Done()
			for item := range workChan {
				err := d.processOperationOptimized(item.op, outFile, oldFile, isDiff, bufferPool)
				resultChan <- err

				if progressCallback != nil {
					progressMutex.Lock()
					completedOps++
					current := int(completedOps)
					progressPercent := float64(current) / float64(totalOps) * 100
					progressMutex.Unlock()

					progressInfo := ProgressInfo{
						PartitionName:   "",
						TotalOperations: totalOps,
						CompletedOps:    current,
						ProgressPercent: progressPercent,
					}
					progressCallback(progressInfo)
				}
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

// processOperationsSingleThreaded processes operations sequentially without multi-threading
func (d *Dumper) processOperationsSingleThreaded(operations []Operation, outFile *os.File, oldFile *os.File, isDiff bool, progressCallback ProgressCallback) error {
	totalOps := len(operations)

	for i, op := range operations {
		err := d.processOperationSingleThreaded(op, outFile, oldFile, isDiff)
		if err != nil {
			return err
		}

		// Report progress
		if progressCallback != nil {
			progressPercent := float64(i+1) / float64(totalOps) * 100
			progressInfo := ProgressInfo{
				PartitionName:   "",
				TotalOperations: totalOps,
				CompletedOps:    i + 1,
				ProgressPercent: progressPercent,
			}
			progressCallback(progressInfo)
		}
	}

	return nil
}

// processOperationSingleThreaded processes a single operation without buffer pool optimization
func (d *Dumper) processOperationSingleThreaded(op Operation, outFile *os.File, oldFile *os.File, isDiff bool) error {
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

func (d *Dumper) processOperationOptimized(op Operation, outFile *os.File, oldFile *os.File, isDiff bool, bufferPool chan []byte) error {
	operation := op.Operation

	var data []byte
	var err error

	if op.Length > 0 {
		buffer := <-bufferPool
		defer func() {
			if buffer == nil {
				bufferPool <- nil
				return
			}
			buffer = buffer[:0]
			bufferPool <- buffer
		}()

		need := int(op.Length)
		if buffer == nil || cap(buffer) < need {
			alloc := max(need, 64*1024)
			if int64(alloc) > MaxBufferSize {
				alloc = int(MaxBufferSize)
			}
			if alloc < need {
				alloc = need
			}
			buffer = make([]byte, alloc)
		}
		buffer = buffer[:need]

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

// processOperationsToBytesWithProgress processes operations with progress reporting
func (d *Dumper) processOperationsToBytesWithProgress(operations []Operation, buffer []byte, progressCallback func(int, int)) error {
	total := len(operations)
	for i, op := range operations {
		if err := d.processOperationToBytes(op, buffer); err != nil {
			return err
		}

		// Report progress
		if progressCallback != nil {
			progressCallback(i+1, total)
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
			if workBuffer == nil {
				bufferPool <- nil
				return
			}
			workBuffer = workBuffer[:0]
			bufferPool <- workBuffer
		}()

		need := int(op.Length)
		if workBuffer == nil || cap(workBuffer) < need {
			alloc := max(need, 64*1024)
			if int64(alloc) > MaxBufferSize {
				alloc = int(MaxBufferSize)
			}
			if alloc < need {
				alloc = need
			}
			workBuffer = make([]byte, alloc)
		}
		workBuffer = workBuffer[:need]

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
