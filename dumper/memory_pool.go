package dumper

import (
	"sync"
)

// MemoryPool manages reusable byte slices to reduce GC pressure
type MemoryPool struct {
	pools map[int]*sync.Pool
	mu    sync.RWMutex
}

func newPool() *MemoryPool {
	return &MemoryPool{
		pools: make(map[int]*sync.Pool),
	}
}

// Get retrieves a byte slice of the requested size
func (mp *MemoryPool) Get(size int) []byte {
	bkt := mp.bucket(size)

	mp.mu.RLock()
	pool, exists := mp.pools[bkt]
	mp.mu.RUnlock()

	if !exists {
		mp.mu.Lock()
		pool, exists = mp.pools[bkt]
		if !exists {
			pool = &sync.Pool{
				New: func() any {
					buf := make([]byte, bkt)
					return &buf
				},
			}
			mp.pools[bkt] = pool
		}
		mp.mu.Unlock()
	}

	bufPtr := pool.Get().(*[]byte)
	return (*bufPtr)[:size]
}

// Put returns a byte slice to the pool
func (mp *MemoryPool) Put(buf []byte) {
	if cap(buf) == 0 {
		return
	}

	bkt := mp.bucket(cap(buf))

	mp.mu.RLock()
	pool, exists := mp.pools[bkt]
	mp.mu.RUnlock()

	if exists && cap(buf) == bkt {
		pool.Put(&buf)
	}
}

func (mp *MemoryPool) bucket(size int) int {
	bkts := []int{4 * 1024, 16 * 1024, 64 * 1024, 256 * 1024, 1024 * 1024, 4 * 1024 * 1024, 16 * 1024 * 1024}

	for _, bkt := range bkts {
		if size <= bkt {
			return bkt
		}
	}

	mb := (size + 1024*1024 - 1) / (1024 * 1024)
	return mb * 1024 * 1024
}

var globalMemoryPool = newPool()

// GetGlobalMemoryPool returns the global memory pool
func GetGlobalMemoryPool() *MemoryPool {
	return globalMemoryPool
}
