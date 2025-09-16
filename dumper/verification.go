package dumper

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	"github.com/xishang0128/payload-dumper-go/common/i18n"
)

const (
	defaultVerifyBufSize = 1 * 1024 * 1024
	minVerifyBufSize     = 64 * 1024
	verifyJobsBuffer     = 64
)

// VerificationManager manages partition verification
type VerificationManager struct {
	enabled       bool
	jobs          chan string
	wg            sync.WaitGroup
	results       map[string]error
	resultsMu     sync.Mutex
	partitionsMap map[string]PartitionInfo
	outputDir     string
}

// NewVerificationManager creates a new verification manager
func NewVerificationManager(d *Dumper, outputDir string, enabled bool) *VerificationManager {
	vm := &VerificationManager{
		enabled:   enabled,
		results:   make(map[string]error),
		outputDir: outputDir,
	}

	if !enabled {
		return vm
	}

	pm, err := d.ListPartitionsAsMap()
	if err != nil {
		panic(fmt.Errorf("failed to list partitions: %v", err))
	}
	vm.partitionsMap = pm
	vm.jobs = make(chan string, verifyJobsBuffer)

	bufSize := vm.calcVerifyBufSize()
	workerCount := vm.calcVerifyWorkerCount()
	for range workerCount {
		go vm.verificationWorker(bufSize)
	}

	return vm
}

// AddVerificationJob adds a partition to the verification queue
func (vm *VerificationManager) AddVerificationJob(partitionName string) {
	if vm.enabled {
		vm.wg.Add(1)
		vm.jobs <- partitionName
	}
}

// WaitForCompletion waits for all verification jobs to complete
func (vm *VerificationManager) WaitForCompletion() {
	if vm.enabled {
		vm.wg.Wait()
		close(vm.jobs)
	}
}

// GetResults returns the verification results
func (vm *VerificationManager) GetResults() map[string]error {
	vm.resultsMu.Lock()
	defer vm.resultsMu.Unlock()

	results := make(map[string]error)
	for k, v := range vm.results {
		results[k] = v
	}
	return results
}

func (vm *VerificationManager) calcVerifyWorkerCount() int {
	cpuCount := runtime.NumCPU()

	if runtime.GOOS == "windows" {
		if cpuCount <= 2 {
			return 1
		}
		return cpuCount / 2
	}

	if cpuCount <= 4 {
		return cpuCount
	}
	return cpuCount - 1
}

func (vm *VerificationManager) calcVerifyBufSize() int {
	bufSize := defaultVerifyBufSize

	if MaxBufferSize > 0 {
		maxAllowed := MaxBufferSize / 4
		if maxAllowed > defaultVerifyBufSize {
			bufSize = int(maxAllowed)
		} else {
			if maxAllowed < minVerifyBufSize {
				bufSize = minVerifyBufSize
			} else {
				bufSize = int(maxAllowed)
			}
		}
	}

	return bufSize
}

func (vm *VerificationManager) verificationWorker(bufSize int) {
	buf := make([]byte, bufSize)
	for partName := range vm.jobs {
		err := vm.verifyPartitionWithRetry(partName, buf)
		vm.resultsMu.Lock()
		vm.results[partName] = err
		vm.resultsMu.Unlock()
		vm.wg.Done()
	}
}

func (vm *VerificationManager) verifyPartitionWithRetry(partName string, buf []byte) error {
	maxRetries := 1

	if runtime.GOOS == "windows" {
		maxRetries = 3
	}

	var lastErr error
	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			time.Sleep(time.Duration(attempt*100) * time.Millisecond)
		}

		if runtime.GOOS == "windows" {
			done := make(chan error, 1)
			go func() {
				done <- vm.verifyPartition(partName, buf)
			}()

			select {
			case err := <-done:
				if err == nil {
					return nil
				}
				lastErr = err
			case <-time.After(30 * time.Second):
				lastErr = fmt.Errorf("verification timeout for partition %s", partName)
			}
		} else {
			err := vm.verifyPartition(partName, buf)
			if err == nil {
				return nil
			}
			lastErr = err
		}
	}

	return lastErr
}

func (vm *VerificationManager) verifyPartition(partName string, buf []byte) error {
	expectedHex := ""
	if info, ok := vm.partitionsMap[partName]; ok {
		expectedHex = info.Hash
	}

	if expectedHex == "" {
		return fmt.Errorf(i18n.I18nMsg.Dumper.ErrorNoExpectedHashInManifest)
	}

	path := filepath.Join(vm.outputDir, partName+".img")
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.CopyBuffer(h, f, buf); err != nil {
		return err
	}

	sum := h.Sum(nil)
	gotHex := hex.EncodeToString(sum)
	if gotHex != expectedHex {
		expBytes, _ := hex.DecodeString(expectedHex)
		return fmt.Errorf(i18n.I18nMsg.Dumper.ErrorSha256Mismatch, expBytes, sum)
	}

	return nil
}
