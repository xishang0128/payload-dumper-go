package dumper

import (
	"fmt"
	"sync"

	"github.com/xishang0128/payload-dumper-go/common/i18n"
)

// ExtractionStrategy defines the extraction strategy type
type ExtractionStrategy int

const (
	// StrategySequential processes partitions one by one to minimize resource usage
	StrategySequential ExtractionStrategy = iota
	// StrategyAdaptive uses work-stealing algorithm for optimal resource utilization
	StrategyAdaptive
)

// extractPartitionsSequential processes partitions one by one
func (d *Dumper) extractPartitionsSequential(partitions []*PartitionWithOps, outputDir string, cpuCount int, progressCallback ProgressCallback) error {
	for _, partition := range partitions {
		var operationWorkers int
		if d.ShouldUseMultithread(partition.Partition) {
			operationWorkers = max(2, cpuCount*3/4)
		} else {
			operationWorkers = max(1, cpuCount/2)
		}

		singlePartition := []*PartitionWithOps{partition}
		err := d.MultiprocessPartitions(singlePartition, outputDir, 1, operationWorkers, false, "", progressCallback)
		if err != nil {
			return fmt.Errorf(i18n.I18nMsg.Dumper.ErrorFailedToProcessPartition, partition.Partition.GetPartitionName(), err)
		}
	}
	return nil
}

// extractPartitionsWithWorkStealing implements an adaptive work-stealing algorithm
// that dynamically balances load and prevents resource idling
func (d *Dumper) extractPartitionsWithWorkStealing(partitions []*PartitionWithOps, outputDir string, cpuCount int, progressCallback ProgressCallback) error {
	// Classify partitions by processing complexity
	type partitionWork struct {
		partition  *PartitionWithOps
		complexity int
		priority   int
	}

	workQueue := make([]*partitionWork, 0, len(partitions))
	totalComplexity := 0

	// Analyze and prioritize partitions
	for _, partition := range partitions {
		complexity := 1
		priority := 1

		if d.ShouldUseMultithread(partition.Partition) {
			complexity = 3
			priority = 3 // Process large partitions first to maximize parallelism
		} else {
			// Small partitions can be processed more efficiently in parallel
			complexity = 1
			priority = 2
		}

		work := &partitionWork{
			partition:  partition,
			complexity: complexity,
			priority:   priority,
		}
		workQueue = append(workQueue, work)
		totalComplexity += complexity
	}

	// Sort by priority (high to low) then by complexity (high to low)
	for i := 0; i < len(workQueue)-1; i++ {
		for j := i + 1; j < len(workQueue); j++ {
			if workQueue[j].priority > workQueue[i].priority ||
				(workQueue[j].priority == workQueue[i].priority && workQueue[j].complexity > workQueue[i].complexity) {
				workQueue[i], workQueue[j] = workQueue[j], workQueue[i]
			}
		}
	}

	// Dynamic worker allocation based on workload
	optimalWorkers := d.calculateOptimalWorkers(cpuCount, len(partitions), totalComplexity)

	// Work-stealing implementation
	workChan := make(chan *partitionWork, len(workQueue))
	var wg sync.WaitGroup
	var errorsChan = make(chan error, optimalWorkers)
	var completedMutex sync.Mutex
	var completedCount int

	// Populate work queue
	for _, work := range workQueue {
		workChan <- work
	}
	close(workChan)

	// Start adaptive workers
	for i := 0; i < optimalWorkers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			for work := range workChan {
				// Dynamic operation worker allocation based on partition complexity
				operationWorkers := d.calculateOperationWorkers(cpuCount, work.complexity, completedCount, len(partitions))

				err := d.processPartitionAdaptive(work.partition, outputDir, operationWorkers, progressCallback)
				if err != nil {
					errorsChan <- fmt.Errorf(i18n.I18nMsg.Dumper.ErrorWorkerFailedToProcessPartition,
						workerID, work.partition.Partition.GetPartitionName(), err)
					return
				}

				// Update completion status
				completedMutex.Lock()
				completedCount++
				completedMutex.Unlock()
			}
		}(i)
	}

	go func() {
		wg.Wait()
		close(errorsChan)
	}()

	// Check for errors
	for err := range errorsChan {
		if err != nil {
			return err
		}
	}

	return nil
}

// calculateOptimalWorkers determines the optimal number of workers based on system resources and workload
func (d *Dumper) calculateOptimalWorkers(numCPU, partitionCount, totalComplexity int) int {
	baseWorkers := min(partitionCount, numCPU)

	complexityFactor := float64(totalComplexity) / float64(partitionCount)
	if complexityFactor > 2.0 {
		baseWorkers = min(baseWorkers*2, numCPU*2)
	} else if complexityFactor < 1.5 {
		baseWorkers = max(baseWorkers/2, 1)
	}

	return max(1, min(baseWorkers, partitionCount))
}

// calculateOperationWorkers dynamically calculates operation workers based on current state
func (d *Dumper) calculateOperationWorkers(numCPU, complexity, completed, total int) int {
	// Base allocation
	baseWorkers := max(1, numCPU/2)

	// Adjust based on complexity
	switch complexity {
	case 3: // Complex partition
		baseWorkers = max(2, numCPU*3/4)
	case 2: // Medium partition
		baseWorkers = max(1, numCPU/2)
	case 1: // Simple partition
		baseWorkers = max(1, numCPU/4)
	}

	// Boost workers for remaining partitions to utilize freed resources
	remainingRatio := float64(total-completed) / float64(total)
	if remainingRatio < 0.3 && complexity >= 2 {
		// Less than 30% remaining and dealing with complex partitions
		baseWorkers = min(numCPU, baseWorkers*2)
	}

	return max(1, baseWorkers)
}

// processPartitionAdaptive processes a single partition with adaptive resource allocation
func (d *Dumper) processPartitionAdaptive(partitionWithOps *PartitionWithOps, outputDir string, operationWorkers int, progressCallback ProgressCallback) error {
	// Create a slice containing just this partition for the MultiprocessPartitions call
	partitions := []*PartitionWithOps{partitionWithOps}

	// Use 1 partition worker since we're processing one partition at a time in this goroutine
	// The parallelism comes from multiple goroutines calling this function concurrently
	return d.MultiprocessPartitions(partitions, outputDir, 1, operationWorkers, false, "", progressCallback)
}

// extractSinglePartitionOptimized provides maximum CPU utilization for single partition extraction
func (d *Dumper) extractSinglePartitionOptimized(partition *PartitionWithOps, outputDir string, cpuCount int, progressCallback ProgressCallback) error {

	// For single partition, maximize operation-level parallelism and optimize I/O
	var operationWorkers int

	// Get partition size to determine optimal strategy
	sizeInBytes := d.getPartitionSizeInBytes(partition.Partition)
	operationCount := len(partition.Operations)

	// Temporarily increase buffer size for large single partition extraction
	originalBufferSize := MaxBufferSize
	defer func() {
		MaxBufferSize = originalBufferSize
	}()

	if d.ShouldUseMultithread(partition.Partition) {
		// Large partition: aggressive CPU utilization since it's the only task
		if operationCount > cpuCount*2 {
			// Many operations: use all available CPU cores
			operationWorkers = cpuCount
		} else {
			// Fewer operations: still use most cores but leave some for I/O
			operationWorkers = max(2, cpuCount*3/4)
		}

		// For very large partitions (>1GB), optimize I/O and buffer size
		if sizeInBytes > 1024*1024*1024 {
			// Boost worker count for I/O intensive workloads
			operationWorkers = min(cpuCount+2, cpuCount*5/4)

			// Increase buffer size for large partitions to reduce I/O overhead
			// Use larger buffers when we have lots of memory for single partition
			MaxBufferSize = min(256*1024*1024, MaxBufferSize*4) // Up to 256MB
		}
	} else {
		// Small partition: but it's the ONLY task, so use substantial CPU power
		// Even small partitions benefit from parallelism when they're the sole focus
		if operationCount > cpuCount {
			// Many operations in small partition: use most CPU cores
			operationWorkers = max(2, cpuCount*3/4)
		} else if operationCount > 2 {
			// Moderate operations: use half the CPU cores minimum
			operationWorkers = max(2, cpuCount/2)
		} else {
			// Very few operations: still use multiple threads for I/O overlap
			operationWorkers = max(1, min(operationCount, cpuCount/4))
		}
	}

	// Use single partition worker since we only have one partition
	singlePartition := []*PartitionWithOps{partition}
	return d.MultiprocessPartitions(singlePartition, outputDir, 1, operationWorkers, false, "", progressCallback)
}
