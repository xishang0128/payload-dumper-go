package main

import (
	"fmt"
	"log"
	"runtime"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/xishang0128/payload-dumper-go/common/i18n"
)

var (
	diffOut        string
	diffOld        string
	diffPartitions string
	diffWorkers    int
)

func initExtractDiffCmd() {
	extractDiffCmd := &cobra.Command{
		Use:   i18n.I18nMsg.ExtractDiff.Use,
		Short: i18n.I18nMsg.ExtractDiff.Short,
		Long:  i18n.I18nMsg.ExtractDiff.Long,
		Args:  cobra.ExactArgs(1),
		Run:   runExtractDiff,
	}

	extractDiffCmd.Flags().StringVarP(&diffOut, "out", "o", "output", i18n.I18nMsg.Common.FlagOut)
	extractDiffCmd.Flags().StringVarP(&diffOld, "old", "", "old", i18n.I18nMsg.ExtractDiff.FlagOld)
	extractDiffCmd.Flags().StringVarP(&diffPartitions, "partitions", "p", "", i18n.I18nMsg.Extract.FlagPartitions)
	extractDiffCmd.Flags().IntVarP(&diffWorkers, "workers", "w", runtime.NumCPU(), i18n.I18nMsg.ExtractDiff.FlagWorkers)

	rootCmd.AddCommand(extractDiffCmd)
}

func runExtractDiff(cmd *cobra.Command, args []string) {
	start := time.Now()
	defer func() {
		elapsed := time.Since(start)
		fmt.Printf(i18n.I18nMsg.Common.ElapsedTime+"\n", elapsed)
	}()

	d, err := createDumper(args[0])
	if err != nil {
		log.Fatalf(i18n.I18nMsg.Common.ErrorFailedToCreateDumper, err)
	}
	defer closeDumper(d)

	var partitionNames []string
	if diffPartitions != "" {
		partitionNames = strings.Split(diffPartitions, ",")
		for i, name := range partitionNames {
			partitionNames[i] = strings.TrimSpace(name)
		}
	}

	if err := d.ExtractPartitionsDiff(diffOut, diffOld, partitionNames, diffWorkers); err != nil {
		log.Fatalf(i18n.I18nMsg.ExtractDiff.ErrorFailedToExtractDiff, err)
	}

	fmt.Println(i18n.I18nMsg.ExtractDiff.DiffExtractionCompleted)
}
