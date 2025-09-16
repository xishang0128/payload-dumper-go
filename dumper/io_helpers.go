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

var globalZeroBuf = make([]byte, 64*1024)

func (d *Dumper) procOps(operations []Operation, outFile *os.File, oldFile *os.File, opWorkers int, isDiff bool, progressCallback ProgressCallback) error {
	if opWorkers <= 0 {
		opWorkers = runtime.NumCPU()
	}

	balancer := GetGlobalBalancer()

	var dataSize int64
	for _, op := range operations {
		dataSize += int64(op.Length)
	}

	var workers int
	if dataSize > 512*1024*1024 {
		workers = min(opWorkers, balancer.largeWorkers(dataSize))
	} else {
		workers = min(opWorkers, balancer.workers())
	}

	poolSize := min(workers, 4)
	bufPool := make(chan []byte, poolSize)
	for range poolSize {
		bufPool <- nil
	}

	workerCount := min(workers, len(operations))

	type workItem struct {
		index int
		op    Operation
	}

	chanBuf := max(1, min(workers/2, 4))
	workChan := make(chan workItem, chanBuf)
	resultChan := make(chan error, len(operations))

	var wg sync.WaitGroup
	wg.Add(workerCount)

	totalOps := len(operations)
	var completed int64
	var progMutex sync.Mutex

	for range workerCount {
		go func() {
			defer wg.Done()
			for item := range workChan {
				err := d.procOp(item.op, outFile, oldFile, isDiff, bufPool)
				resultChan <- err

				if progressCallback != nil {
					progMutex.Lock()
					completed++
					current := int(completed)
					percent := float64(current) / float64(totalOps) * 100
					progMutex.Unlock()

					info := ProgressInfo{
						PartitionName:   "",
						TotalOperations: totalOps,
						CompletedOps:    current,
						ProgressPercent: percent,
					}
					progressCallback(info)
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

func (d *Dumper) procOpDirect(op Operation, outFile *os.File, oldFile *os.File, isDiff bool) error {
	operation := op.Operation
	var data []byte
	var err error

	if op.Length > 0 {
		data, err = d.payloadFile.Read(op.Offset, int(op.Length))
		if err != nil {
			return fmt.Errorf(i18n.I18nMsg.Dumper.ErrorFailedToReadOperationData, err)
		}
	}

	return d.executeOperation(operation, data, outFile, oldFile, isDiff)
}

func (d *Dumper) procOp(op Operation, outFile *os.File, oldFile *os.File, isDiff bool, bufPool chan []byte) error {
	operation := op.Operation

	var data []byte
	var err error

	if op.Length > 0 {
		if op.Length < 32*1024 {
			return d.procOpDirect(op, outFile, oldFile, isDiff)
		}

		buf := <-bufPool
		defer func() {
			if buf == nil {
				bufPool <- nil
				return
			}
			if cap(buf) > 16*1024*1024 { // Don't pool very large buffers
				pool := GetGlobalMemoryPool()
				pool.Put(buf)
				bufPool <- nil
			} else {
				buf = buf[:0]
				bufPool <- buf
			}
		}()

		need := int(op.Length)
		if buf == nil || cap(buf) < need {
			pool := GetGlobalMemoryPool()
			alloc := max(need, 64*1024)
			if int64(alloc) > MaxBufferSize/2 {
				alloc = int(MaxBufferSize / 2)
			}
			if alloc < need {
				alloc = need
			}

			// Try to reuse from pool first
			if buf != nil && cap(buf) > 0 {
				pool.Put(buf)
			}
			buf = pool.Get(alloc)
		}
		buf = buf[:need]

		data, err = d.payloadFile.Read(op.Offset, int(op.Length))
		if err != nil {
			return fmt.Errorf(i18n.I18nMsg.Dumper.ErrorFailedToReadOperationData, err)
		}
	}

	return d.executeOperation(operation, data, outFile, oldFile, isDiff)
}

func (d *Dumper) executeOperation(operation *metadata.InstallOperation, data []byte, outFile *os.File, oldFile *os.File, isDiff bool) error {

	switch operation.GetType() {
	case metadata.InstallOperation_REPLACE:
		return d.writeExtents(outFile, operation.GetDstExtents(), data)

	case metadata.InstallOperation_REPLACE_BZ:
		reader := bzip2.NewReader(bytes.NewReader(data))
		decompressed, err := io.ReadAll(reader)
		if err != nil {
			return fmt.Errorf(i18n.I18nMsg.Dumper.ErrorFailedToDecompressBzip2, err)
		}
		return d.writeExtents(outFile, operation.GetDstExtents(), decompressed)

	case metadata.InstallOperation_REPLACE_XZ:
		decompressed, err := decompressXZ(data)
		if err != nil {
			return fmt.Errorf(i18n.I18nMsg.Dumper.ErrorFailedToDecompressXZ, err)
		}
		return d.writeExtents(outFile, operation.GetDstExtents(), decompressed)

	case metadata.InstallOperation_SOURCE_COPY:
		if !isDiff {
			return fmt.Errorf(i18n.I18nMsg.Dumper.ErrorSourceCopyOnlyForDiff)
		}
		return d.srcCopy(outFile, oldFile, operation)

	case metadata.InstallOperation_ZERO:
		return d.writeZeros(outFile, operation.GetDstExtents())

	default:
		return fmt.Errorf(i18n.I18nMsg.Dumper.ErrorUnsupportedOperationType, operation.GetType())
	}
}

func (d *Dumper) writeExtents(outFile *os.File, extents []*metadata.Extent, data []byte) error {
	offset := 0
	for _, extent := range extents {
		blkOffset := int64(extent.GetStartBlock()) * int64(d.blockSize)
		blkSize := int64(extent.GetNumBlocks()) * int64(d.blockSize)

		if offset+int(blkSize) > len(data) {
			blkSize = int64(len(data) - offset)
		}

		if blkSize <= 0 {
			break
		}

		if _, err := outFile.WriteAt(data[offset:offset+int(blkSize)], blkOffset); err != nil {
			return fmt.Errorf(i18n.I18nMsg.Dumper.ErrorFailedToWriteToFile, err)
		}

		offset += int(blkSize)
	}
	return nil
}

func (d *Dumper) srcCopy(outFile *os.File, oldFile *os.File, operation *metadata.InstallOperation) error {
	for _, srcExtent := range operation.GetSrcExtents() {
		blkOffset := int64(srcExtent.GetStartBlock()) * int64(d.blockSize)
		blkSize := int64(srcExtent.GetNumBlocks()) * int64(d.blockSize)

		data := make([]byte, blkSize)
		if _, err := oldFile.ReadAt(data, blkOffset); err != nil {
			return fmt.Errorf(i18n.I18nMsg.Dumper.ErrorFailedToReadFromOldFile, err)
		}

		dstOffset := int64(operation.GetDstExtents()[0].GetStartBlock()) * int64(d.blockSize)
		if _, err := outFile.WriteAt(data, dstOffset); err != nil {
			return fmt.Errorf(i18n.I18nMsg.Dumper.ErrorFailedToWriteToFile, err)
		}
	}
	return nil
}

func (d *Dumper) writeZeros(outFile *os.File, extents []*metadata.Extent) error {
	const maxChunk = 64 * 1024

	for _, extent := range extents {
		blkOffset := int64(extent.GetStartBlock()) * int64(d.blockSize)
		blkSize := int64(extent.GetNumBlocks()) * int64(d.blockSize)

		rem := blkSize
		pos := blkOffset

		for rem > 0 {
			chunk := min(rem, maxChunk)

			if _, err := outFile.WriteAt(globalZeroBuf[:chunk], pos); err != nil {
				return fmt.Errorf(i18n.I18nMsg.Dumper.ErrorFailedToWriteZeros, err)
			}

			rem -= chunk
			pos += chunk
		}
	}
	return nil
}
