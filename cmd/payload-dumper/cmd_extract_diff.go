package main

import (
	"fmt"
	"log"
	"runtime"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/xishang0128/payload-dumper-go/common/file"
	"github.com/xishang0128/payload-dumper-go/common/i18n"
	"github.com/xishang0128/payload-dumper-go/dumper"
)

var (
	diffOut        string
	diffOld        string
	diffPartitions string
	diffWorkers    int
)

func initExtractDiffCmd() {
	// Initialize extract-diff command with localized strings
	extractDiffCmd := &cobra.Command{
		Use:   i18n.I18nMsg.ExtractDiff.Use,
		Short: i18n.I18nMsg.ExtractDiff.Short,
		Long:  i18n.I18nMsg.ExtractDiff.Long,
		Args:  cobra.ExactArgs(1),
		Run:   runExtractDiff,
	}

	extractDiffCmd.Flags().StringVarP(&diffOut, "out", "o", "output", i18n.I18nMsg.Common.FlagOut)
	extractDiffCmd.Flags().StringVarP(&diffOld, "old", "", i18n.I18nMsg.Extract.DefaultOld, i18n.I18nMsg.ExtractDiff.FlagOld)
	extractDiffCmd.Flags().StringVarP(&diffPartitions, "partitions", "p", "", i18n.I18nMsg.Extract.FlagPartitions)
	extractDiffCmd.Flags().IntVarP(&diffWorkers, "workers", "w", runtime.NumCPU(), i18n.I18nMsg.Extract.FlagWorkers)

	rootCmd.AddCommand(extractDiffCmd)
}

func runExtractDiff(cmd *cobra.Command, args []string) {
	start := time.Now()
	defer func() {
		elapsed := time.Since(start)
		fmt.Printf(i18n.I18nMsg.Common.ElapsedTime+"\n", elapsed)
	}()

	payloadFile := args[0]

	// Create file reader
	var reader file.Reader
	var err error

	if strings.HasPrefix(payloadFile, "http://") || strings.HasPrefix(payloadFile, "https://") {
		reader, err = file.NewHTTPFile(payloadFile)
	} else {
		reader, err = file.NewLocalFile(payloadFile)
	}

	if err != nil {
		log.Fatalf(i18n.I18nMsg.Common.ErrorFailedToOpen, err)
	}
	defer reader.Close()

	// Create dumper
	d, err := dumper.New(reader)
	if err != nil {
		log.Fatalf(i18n.I18nMsg.Common.ErrorFailedToCreateDumper, err)
	}

	// Parse partition names
	var partitionNames []string
	if diffPartitions != "" {
		partitionNames = strings.Split(diffPartitions, ",")
		for i, name := range partitionNames {
			partitionNames[i] = strings.TrimSpace(name)
		}
	}

	// Extract partitions with diff
	if err := d.ExtractPartitionsDiff(diffOut, diffOld, partitionNames, diffWorkers); err != nil {
		log.Fatalf(i18n.I18nMsg.ExtractDiff.ErrorFailedToExtractDiff, err)
	}

	fmt.Println(i18n.I18nMsg.ExtractDiff.DiffExtractionCompleted)
}
