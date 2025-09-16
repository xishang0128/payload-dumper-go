package dumper

import (
	"fmt"
	"sync"

	"github.com/xishang0128/payload-dumper-go/common/i18n"
)

type ExtractionStrategy int

const (
	StrategySequential ExtractionStrategy = iota
	StrategyAdaptive
)

func (d *Dumper) extractSeq(parts []*PartitionWithOps, outputDir string, cpuCount int, progressCallback ProgressCallback) error {
	for _, part := range parts {
		var opWorkers int
		if d.ShouldUseMultithread(part.Partition) {
			opWorkers = max(2, cpuCount*3/4)
		} else {
			opWorkers = max(1, cpuCount/2)
		}

		single := []*PartitionWithOps{part}
		err := d.MultiprocessPartitions(single, outputDir, 1, opWorkers, false, "", progressCallback)
		if err != nil {
			return fmt.Errorf(i18n.I18nMsg.Dumper.ErrorFailedToProcessPartition, part.Partition.GetPartitionName(), err)
		}
	}
	return nil
}

func (d *Dumper) extractAdaptive(parts []*PartitionWithOps, outputDir string, cpuCount int, progressCallback ProgressCallback) error {
	type partitionWork struct {
		partition  *PartitionWithOps
		complexity int
		priority   int
	}

	queue := make([]*partitionWork, 0, len(parts))
	totalComp := 0

	for _, part := range parts {
		comp := 1
		prio := 1

		if d.ShouldUseMultithread(part.Partition) {
			comp = 3
			prio = 3
		} else {
			comp = 1
			prio = 2
		}

		work := &partitionWork{
			partition:  part,
			complexity: comp,
			priority:   prio,
		}
		queue = append(queue, work)
		totalComp += comp
	}

	for i := 0; i < len(queue)-1; i++ {
		for j := i + 1; j < len(queue); j++ {
			if queue[j].priority > queue[i].priority ||
				(queue[j].priority == queue[i].priority && queue[j].complexity > queue[i].complexity) {
				queue[i], queue[j] = queue[j], queue[i]
			}
		}
	}

	optWorkers := d.calcWorkers(cpuCount, len(parts), totalComp)

	workChan := make(chan *partitionWork, len(queue))
	var wg sync.WaitGroup
	var errChan = make(chan error, optWorkers)
	var compMutex sync.Mutex
	var compCount int

	for _, work := range queue {
		workChan <- work
	}
	close(workChan)

	for i := 0; i < optWorkers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			for work := range workChan {
				opWorkers := d.calcOpWorkers(cpuCount, work.complexity, compCount, len(parts))

				err := d.processPartAdaptive(work.partition, outputDir, opWorkers, progressCallback)
				if err != nil {
					errChan <- fmt.Errorf(i18n.I18nMsg.Dumper.ErrorWorkerFailedToProcessPartition,
						workerID, work.partition.Partition.GetPartitionName(), err)
					return
				}

				compMutex.Lock()
				compCount++
				compMutex.Unlock()
			}
		}(i)
	}

	go func() {
		wg.Wait()
		close(errChan)
	}()

	for err := range errChan {
		if err != nil {
			return err
		}
	}

	return nil
}

func (d *Dumper) calcWorkers(numCPU, count, totalComp int) int {
	baseWorkers := min(count, numCPU)
	compFactor := float64(totalComp) / float64(count)
	if compFactor > 2.0 {
		baseWorkers = min(baseWorkers*2, numCPU*2)
	} else if compFactor < 1.5 {
		baseWorkers = max(baseWorkers/2, 1)
	}

	return max(1, min(baseWorkers, count))
}

func (d *Dumper) calcOpWorkers(numCPU, comp, done, total int) int {
	baseWorkers := max(1, numCPU/2)

	switch comp {
	case 3:
		baseWorkers = max(2, numCPU*3/4)
	case 2:
		baseWorkers = max(1, numCPU/2)
	case 1:
		baseWorkers = max(1, numCPU/4)
	}

	remRatio := float64(total-done) / float64(total)
	if remRatio < 0.3 && comp >= 2 {
		baseWorkers = min(numCPU, baseWorkers*2)
	}

	return max(1, baseWorkers)
}

func (d *Dumper) processPartAdaptive(withOps *PartitionWithOps, outputDir string, opWorkers int, progressCallback ProgressCallback) error {
	parts := []*PartitionWithOps{withOps}
	return d.MultiprocessPartitions(parts, outputDir, 1, opWorkers, false, "", progressCallback)
}

func (d *Dumper) extractSingle(part *PartitionWithOps, outputDir string, cpuCount int, progressCallback ProgressCallback) error {
	origBuf := MaxBufferSize
	defer func() {
		MaxBufferSize = origBuf
	}()

	var opWorkers int
	sizeInBytes := d.size(part.Partition)
	operationCount := len(part.Operations)

	if d.ShouldUseMultithread(part.Partition) {
		if operationCount > cpuCount*2 {
			opWorkers = cpuCount
		} else {
			opWorkers = max(2, cpuCount*3/4)
		}

		if sizeInBytes > 1024*1024*1024 {
			opWorkers = min(cpuCount+2, cpuCount*5/4)
			MaxBufferSize = min(256*1024*1024, MaxBufferSize*4)
		}
	} else {
		if operationCount > cpuCount {
			opWorkers = max(2, cpuCount*3/4)
		} else if operationCount > 2 {
			opWorkers = max(2, cpuCount/2)
		} else {
			opWorkers = max(1, min(operationCount, cpuCount/4))
		}
	}

	single := []*PartitionWithOps{part}
	return d.MultiprocessPartitions(single, outputDir, 1, opWorkers, false, "", progressCallback)
}
