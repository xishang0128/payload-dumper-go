package dumper

import (
	"runtime"
	"sync"
	"time"
)

// WorkloadBalancer controls the smooth distribution of work
type WorkloadBalancer struct {
	maxCon   int
	current  int
	mu       sync.Mutex
	lastAdj  time.Time
	cpuSamps []float64
	memSamps []uint64
}

func newBalancer() *WorkloadBalancer {
	return &WorkloadBalancer{
		maxCon:   runtime.NumCPU(),
		lastAdj:  time.Now(),
		cpuSamps: make([]float64, 0, 10),
		memSamps: make([]uint64, 0, 10),
	}
}

func (wb *WorkloadBalancer) acquire() bool {
	wb.mu.Lock()
	defer wb.mu.Unlock()

	if wb.current < wb.maxCon {
		wb.current++
		return true
	}
	return false
}

func (wb *WorkloadBalancer) release() {
	wb.mu.Lock()
	defer wb.mu.Unlock()

	if wb.current > 0 {
		wb.current--
	}
}

func (wb *WorkloadBalancer) workers() int {
	wb.mu.Lock()
	defer wb.mu.Unlock()

	now := time.Now()
	if now.Sub(wb.lastAdj) > time.Second {
		wb.adjust()
		wb.lastAdj = now
	}

	return wb.maxCon
}

func (wb *WorkloadBalancer) largeWorkers(sizeBytes int64) int {
	wb.mu.Lock()
	defer wb.mu.Unlock()

	base := wb.maxCon

	if sizeBytes > 1024*1024*1024 {
		mul := 1.5
		if sizeBytes > 2*1024*1024*1024 {
			mul = 2.0
		}

		opt := int(float64(base) * mul)
		return min(opt, runtime.NumCPU()*2)
	}

	return base
}

func (wb *WorkloadBalancer) adjust() {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	if len(wb.memSamps) >= 10 {
		wb.memSamps = wb.memSamps[1:]
	}
	wb.memSamps = append(wb.memSamps, m.Alloc)

	if len(wb.memSamps) >= 5 {
		rec := wb.memSamps[len(wb.memSamps)-3:]
		var grow bool
		for i := 1; i < len(rec); i++ {
			if rec[i] > rec[i-1]*11/10 {
				grow = true
				break
			}
		}

		if grow && wb.maxCon > runtime.NumCPU()/2 {
			wb.maxCon--
		} else if !grow && wb.maxCon < runtime.NumCPU()*2 {
			wb.maxCon++
		}
	}
}

var globalBalancer = newBalancer()

// GetGlobalBalancer returns the global workload balancer
func GetGlobalBalancer() *WorkloadBalancer {
	return globalBalancer
}
