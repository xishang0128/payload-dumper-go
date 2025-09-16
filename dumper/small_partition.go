package dumper

import (
	"fmt"
	"os"

	"github.com/xishang0128/payload-dumper-go/common/i18n"
)

const smallThreshold = 16 * 1024 * 1024

func (d *Dumper) procOpsSmall(operations []Operation, outFile *os.File, oldFile *os.File, isDiff bool, progressCallback ProgressCallback) error {
	totalOps := len(operations)

	for i, op := range operations {
		err := d.procOpDirect(op, outFile, oldFile, isDiff)
		if err != nil {
			return err
		}

		if progressCallback != nil {
			cur := i + 1
			pct := float64(cur) / float64(totalOps) * 100

			info := ProgressInfo{
				PartitionName:   "",
				TotalOperations: totalOps,
				CompletedOps:    cur,
				ProgressPercent: pct,
			}
			progressCallback(info)
		}
	}

	return nil
}

func (d *Dumper) isSmall(part *PartitionWithOps) bool {
	sizeBytes := d.size(part.Partition)
	numOps := len(part.Operations)

	return sizeBytes <= smallThreshold || numOps <= 10
}

func (d *Dumper) procSmall(part *PartitionWithOps, outputDir string, isDiff bool, oldDir string, progressCallback ProgressCallback) error {
	name := part.Partition.GetPartitionName() + ".img"
	path := fmt.Sprintf("%s/%s", outputDir, name)

	outFile, err := os.Create(path)
	if err != nil {
		return fmt.Errorf(i18n.I18nMsg.Dumper.ErrorFailedToCreateOutputFile, name, err)
	}
	defer outFile.Close()

	var oldFile *os.File
	if isDiff && oldDir != "" {
		oldPath := fmt.Sprintf("%s/%s", oldDir, name)
		oldFile, err = os.Open(oldPath)
		if err != nil {
			return fmt.Errorf(i18n.I18nMsg.Dumper.ErrorFailedToOpenOldFile, name, err)
		}
		defer oldFile.Close()
	}

	return d.procOpsSmall(part.Operations, outFile, oldFile, isDiff, progressCallback)
}
