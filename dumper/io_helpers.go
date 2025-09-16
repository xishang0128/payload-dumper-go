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

func (d *Dumper) procOps(operations []Operation, outFile *os.File, oldFile *os.File, opWorkers int, isDiff bool, progressCallback ProgressCallback) error {
	if opWorkers <= 0 {
		opWorkers = runtime.NumCPU()
	}

	bufPool := make(chan []byte, opWorkers)
	for i := 0; i < opWorkers; i++ {
		bufPool <- nil
	}

	type workItem struct {
		index int
		op    Operation
	}

	workChan := make(chan workItem, opWorkers*2)
	resultChan := make(chan error, len(operations))

	workerCount := min(opWorkers, len(operations))

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

func (d *Dumper) procOp(op Operation, outFile *os.File, oldFile *os.File, isDiff bool, bufPool chan []byte) error {
	operation := op.Operation

	var data []byte
	var err error

	if op.Length > 0 {
		buf := <-bufPool
		defer func() {
			if buf == nil {
				bufPool <- nil
				return
			}
			buf = buf[:0]
			bufPool <- buf
		}()

		need := int(op.Length)
		if buf == nil || cap(buf) < need {
			alloc := max(need, 64*1024)
			if int64(alloc) > MaxBufferSize {
				alloc = int(MaxBufferSize)
			}
			if alloc < need {
				alloc = need
			}
			buf = make([]byte, alloc)
		}
		buf = buf[:need]

		data, err = d.payloadFile.Read(op.Offset, int(op.Length))
		if err != nil {
			return fmt.Errorf(i18n.I18nMsg.Dumper.ErrorFailedToReadOperationData, err)
		}
	}

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
	const maxChunk = 1024 * 1024
	var zeroBuf []byte

	for _, extent := range extents {
		blkOffset := int64(extent.GetStartBlock()) * int64(d.blockSize)
		blkSize := int64(extent.GetNumBlocks()) * int64(d.blockSize)

		rem := blkSize
		pos := blkOffset

		for rem > 0 {
			chunk := min(rem, maxChunk)

			if len(zeroBuf) < int(chunk) {
				zeroBuf = make([]byte, chunk)
			} else {
				zeroBuf = zeroBuf[:chunk]
				for i := range zeroBuf {
					zeroBuf[i] = 0
				}
			}

			if _, err := outFile.WriteAt(zeroBuf, pos); err != nil {
				return fmt.Errorf(i18n.I18nMsg.Dumper.ErrorFailedToWriteZeros, err)
			}

			rem -= chunk
			pos += chunk
		}
	}
	return nil
}
