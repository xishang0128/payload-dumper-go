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
	"github.com/xishang0128/payload-dumper-go/compression"
)

func (d *Dumper) Close() error {
	if d == nil || d.payloadFile == nil {
		return nil
	}
	return d.payloadFile.Close()
}

// New creates a new Dumper instance for the given payload file
func New(reader file.Reader) (*Dumper, error) {
	i18n.InitLanguage()
	dumper := &Dumper{
		payloadFile:        reader,
		compressionManager: compression.NewDecompressorManager(),
	}

	if offset, _, err := ziputil.GetStoredEntryOffset(reader, "payload.bin"); err == nil {
		dumper.baseOffset = offset
	}

	if err := dumper.parseMetadata(); err != nil {
		return nil, err
	}

	return dumper, nil
}

// ExtractPartitions extracts specified partitions to the output directory
func (d *Dumper) ExtractPartitions(outputDir string, partitionNames []string, workers int) error {
	return d.ExtractPartitionsWithFullOptions(outputDir, partitionNames, workers, workers, nil)
}

// ExtractPartitionsWithFullOptions extracts partitions with full configuration options
func (d *Dumper) ExtractPartitionsWithFullOptions(outputDir string, partitionNames []string, partitionWorkers, operationWorkers int, progressCallback ProgressCallback) error {
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

	withOps := make([]PartitionWithOps, 0, len(partitions))
	for _, partition := range partitions {
		operations := make([]Operation, 0, len(partition.Operations))
		for _, operation := range partition.Operations {
			operations = append(operations, Operation{
				Operation: operation,
				Offset:    d.dataOffset + int64(operation.GetDataOffset()),
				Length:    operation.GetDataLength(),
			})
		}
		withOps = append(withOps, PartitionWithOps{
			Partition:  partition,
			Operations: operations,
		})
	}

	return d.procMulti(withOps, outputDir, partitionWorkers, operationWorkers, false, "", progressCallback)
}

// ExtractPartitionsDiff extracts partitions using differential update with old partition images
func (d *Dumper) ExtractPartitionsDiff(outputDir string, oldDir string, partitionNames []string, workers int) error {
	return d.ExtractPartitionsDiffWithProgress(outputDir, oldDir, partitionNames, workers, nil)
}

// ExtractPartitionsDiffWithProgress extracts partitions using differential update with progress reporting
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

	withOps := make([]PartitionWithOps, 0, len(partitions))
	for _, partition := range partitions {
		operations := make([]Operation, 0, len(partition.Operations))
		for _, operation := range partition.Operations {
			operations = append(operations, Operation{
				Operation: operation,
				Offset:    d.dataOffset + int64(operation.GetDataOffset()),
				Length:    operation.GetDataLength(),
			})
		}
		withOps = append(withOps, PartitionWithOps{
			Partition:  partition,
			Operations: operations,
		})
	}

	return d.procMulti(withOps, outputDir, workers, workers, true, oldDir, progressCallback)
}

// ListPartitions returns a list of all partitions in the payload
func (d *Dumper) ListPartitions() ([]PartitionInfo, error) {
	var info []PartitionInfo

	for _, partition := range d.manifest.GetPartitions() {
		var blocks uint64
		for _, operation := range partition.GetOperations() {
			for _, extent := range operation.GetDstExtents() {
				blocks += extent.GetNumBlocks()
			}
		}

		bytes := blocks * uint64(d.blockSize)
		readable := fmtSize(bytes)

		hash := ""
		if partition.GetNewPartitionInfo() != nil && partition.GetNewPartitionInfo().GetHash() != nil {
			hash = fmt.Sprintf("%x", partition.GetNewPartitionInfo().GetHash())
		}

		info = append(info, PartitionInfo{
			PartitionName: partition.GetPartitionName(),
			SizeInBlocks:  blocks,
			SizeInBytes:   bytes,
			SizeReadable:  readable,
			Hash:          hash,
		})
	}

	return info, nil
}

// ListPartitionsAsMap returns a map of partition name to partition info
func (d *Dumper) ListPartitionsAsMap() (map[string]PartitionInfo, error) {
	info := make(map[string]PartitionInfo)

	for _, partition := range d.manifest.GetPartitions() {
		var blocks uint64
		for _, operation := range partition.GetOperations() {
			for _, extent := range operation.GetDstExtents() {
				blocks += extent.GetNumBlocks()
			}
		}

		bytes := blocks * uint64(d.blockSize)
		readable := fmtSize(bytes)

		hash := ""
		if partition.GetNewPartitionInfo() != nil && partition.GetNewPartitionInfo().GetHash() != nil {
			hash = fmt.Sprintf("%x", partition.GetNewPartitionInfo().GetHash())
		}

		name := partition.GetPartitionName()
		info[name] = PartitionInfo{
			PartitionName: name,
			SizeInBlocks:  blocks,
			SizeInBytes:   bytes,
			SizeReadable:  readable,
			Hash:          hash,
		}
	}

	return info, nil
}

func (d *Dumper) size(partition *metadata.PartitionUpdate) uint64 {
	var blocks uint64
	for _, operation := range partition.GetOperations() {
		for _, extent := range operation.GetDstExtents() {
			blocks += extent.GetNumBlocks()
		}
	}
	return blocks * uint64(d.blockSize)
}

func (d *Dumper) multi(partition *metadata.PartitionUpdate) bool {
	sizeInBytes := d.size(partition)
	return sizeInBytes > MultithreadThreshold
}

func (d *Dumper) ShouldUseMultithread(partition *metadata.PartitionUpdate) bool {
	return d.multi(partition)
}

func (d *Dumper) MultiprocessPartitions(partitions []*PartitionWithOps, outputDir string, partitionWorkers, operationWorkers int, isDiff bool, oldDir string, progressCallback ProgressCallback) error {
	withOps := make([]PartitionWithOps, len(partitions))
	for i, p := range partitions {
		withOps[i] = *p
	}
	return d.procMulti(withOps, outputDir, partitionWorkers, operationWorkers, isDiff, oldDir, progressCallback)
}

func (d *Dumper) GetAllPartitionsWithOps() ([]*PartitionWithOps, error) {
	partitions := make([]*PartitionWithOps, 0, len(d.manifest.Partitions))
	for _, partition := range d.manifest.Partitions {
		operations := make([]Operation, 0, len(partition.Operations))
		for _, operation := range partition.Operations {
			operations = append(operations, Operation{
				Operation: operation,
				Offset:    d.dataOffset + int64(operation.GetDataOffset()),
				Length:    operation.GetDataLength(),
			})
		}
		partitions = append(partitions, &PartitionWithOps{
			Partition:  partition,
			Operations: operations,
		})
	}
	return partitions, nil
}

func (d *Dumper) GetPartitionsWithOps(partitionNames []string) ([]*PartitionWithOps, error) {
	partitions := make([]*PartitionWithOps, 0, len(partitionNames))

	for _, imageName := range partitionNames {
		imageName = strings.TrimSpace(imageName)
		found := false
		for _, partition := range d.manifest.Partitions {
			if partition.GetPartitionName() == imageName {
				operations := make([]Operation, 0, len(partition.Operations))
				for _, operation := range partition.Operations {
					operations = append(operations, Operation{
						Operation: operation,
						Offset:    d.dataOffset + int64(operation.GetDataOffset()),
						Length:    operation.GetDataLength(),
					})
				}
				partitions = append(partitions, &PartitionWithOps{
					Partition:  partition,
					Operations: operations,
				})
				found = true
				break
			}
		}
		if !found {
			fmt.Printf(i18n.I18nMsg.Dumper.PartitionNotFound+"\n", imageName)
		}
	}

	return partitions, nil
}

func (d *Dumper) procMulti(partitions []PartitionWithOps, outputDir string, partitionWorkers, operationWorkers int, isDiff bool, oldDir string, progressCallback ProgressCallback) error {
	if partitionWorkers <= 0 {
		partitionWorkers = runtime.NumCPU()
	}
	if operationWorkers <= 0 {
		operationWorkers = runtime.NumCPU()
	}

	var wg sync.WaitGroup
	var completed []string
	var mu sync.Mutex

	large := make([]PartitionWithOps, 0)
	small := make([]PartitionWithOps, 0)

	for _, part := range partitions {
		if d.multi(part.Partition) {
			large = append(large, part)
		} else {
			small = append(small, part)
		}
	}

	largeSem := make(chan struct{}, partitionWorkers)
	for _, part := range large {
		wg.Add(1)
		go func(p PartitionWithOps) {
			defer wg.Done()
			largeSem <- struct{}{}
			defer func() { <-largeSem }()

			name := p.Partition.GetPartitionName()

			if err := d.procPart(p, outputDir, operationWorkers, isDiff, oldDir, progressCallback); err != nil {
				log.Printf(i18n.I18nMsg.Dumper.ErrorProcessingPartition, name, err)
			} else {
				mu.Lock()
				completed = append(completed, name)
				mu.Unlock()
			}
		}(part)
	}

	smallSem := make(chan struct{}, 1)
	for _, part := range small {
		wg.Add(1)
		go func(p PartitionWithOps) {
			defer wg.Done()
			smallSem <- struct{}{}
			defer func() { <-smallSem }()

			name := p.Partition.GetPartitionName()

			if err := d.procPart(p, outputDir, 1, isDiff, oldDir, progressCallback); err != nil {
				log.Printf(i18n.I18nMsg.Dumper.ErrorProcessingPartition, name, err)
			} else {
				mu.Lock()
				completed = append(completed, name)
				mu.Unlock()
			}
		}(part)
	}

	wg.Wait()

	return nil
}

func (d *Dumper) ExtractPartitionsWithStrategy(outputDir string, partitionNames []string, strategy ExtractionStrategy, cpuCount int, progressCallback ProgressCallback) error {
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf(i18n.I18nMsg.Common.ErrorFailedToCreateDir, err)
	}

	var all []*PartitionWithOps
	if len(partitionNames) == 0 {
		partitions, err := d.GetAllPartitionsWithOps()
		if err != nil {
			return fmt.Errorf(i18n.I18nMsg.Dumper.ErrorFailedToGetAllPartitions, err)
		}
		all = partitions
	} else {
		partitions, err := d.GetPartitionsWithOps(partitionNames)
		if err != nil {
			return fmt.Errorf(i18n.I18nMsg.Dumper.ErrorFailedToGetSpecifiedPartitions, err)
		}
		all = partitions
	}

	if len(all) == 0 {
		fmt.Println(i18n.I18nMsg.Dumper.NotOperatingOnPartitions)
		return nil
	}

	if len(all) == 1 {
		return d.extractSingle(all[0], outputDir, cpuCount, progressCallback)
	}

	switch strategy {
	case StrategySequential:
		return d.extractSeq(all, outputDir, cpuCount, progressCallback)
	case StrategyAdaptive:
		return d.extractAdaptive(all, outputDir, cpuCount, progressCallback)
	default:
		return fmt.Errorf(i18n.I18nMsg.Dumper.ErrorUnknownExtractionStrategy, strategy)
	}
}

func (d *Dumper) procPart(part PartitionWithOps, outputDir string, operationWorkers int, isDiff bool, oldDir string, progressCallback ProgressCallback) error {
	name := part.Partition.GetPartitionName()

	outputPath := filepath.Join(outputDir, name+".img")
	outFile, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf(i18n.I18nMsg.Dumper.ErrorFailedToCreateOutputFile, err)
	}
	defer outFile.Close()

	var oldFile *os.File
	if isDiff {
		oldPath := filepath.Join(oldDir, name+".img")
		oldFile, err = os.Open(oldPath)
		if err != nil {
			return fmt.Errorf(i18n.I18nMsg.Dumper.ErrorFailedToOpenOldFile, err)
		}
		defer oldFile.Close()
	}

	var blocks uint64
	for _, operation := range part.Partition.GetOperations() {
		for _, extent := range operation.GetDstExtents() {
			blocks += extent.GetNumBlocks()
		}
	}
	bytes := blocks * uint64(d.blockSize)
	readable := fmtSize(bytes)

	var wrappedCallback ProgressCallback
	if progressCallback != nil {
		wrappedCallback = func(progress ProgressInfo) {
			progress.PartitionName = name
			progress.SizeReadable = readable
			progressCallback(progress)
		}
	}

	workers := min(operationWorkers, len(part.Operations))
	if len(part.Operations) <= 2 {
		workers = 1
	}
	return d.procOps(part.Operations, outFile, oldFile, workers, isDiff, wrappedCallback)
}
