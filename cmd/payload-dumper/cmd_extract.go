package main

import (
	"fmt"
	"log"
	"runtime"
	"strings"

	"github.com/spf13/cobra"
	"github.com/xishang/payload-dumper-go/common/file"
	"github.com/xishang/payload-dumper-go/common/i18n"
	"github.com/xishang/payload-dumper-go/dumper"
)

var (
	extractOut        string
	extractPartitions string
	extractWorkers    int
)

func initExtractCmd() {
	// Initialize extract command with localized strings
	extractCmd := &cobra.Command{
		Use:   i18n.I18nMsg.Extract.Use,
		Short: i18n.I18nMsg.Extract.Short,
		Long:  i18n.I18nMsg.Extract.Long,
		Args:  cobra.ExactArgs(1),
		Run:   runExtract,
	}

	extractCmd.Flags().StringVarP(&extractOut, "out", "o", "output", i18n.I18nMsg.Common.FlagOut)
	extractCmd.Flags().StringVarP(&extractPartitions, "partitions", "p", "", i18n.I18nMsg.Extract.FlagPartitions)
	extractCmd.Flags().IntVarP(&extractWorkers, "workers", "w", runtime.NumCPU(), i18n.I18nMsg.Extract.FlagWorkers)

	rootCmd.AddCommand(extractCmd)
}

func runExtract(cmd *cobra.Command, args []string) {
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
	if extractPartitions != "" {
		partitionNames = strings.Split(extractPartitions, ",")
		for i, name := range partitionNames {
			partitionNames[i] = strings.TrimSpace(name)
		}
	}

	// Extract partitions
	if err := d.ExtractPartitions(extractOut, partitionNames, extractWorkers); err != nil {
		log.Fatalf(i18n.I18nMsg.Extract.ErrorFailedToExtract, err)
	}

	fmt.Println(i18n.I18nMsg.Extract.ExtractionCompleted)
}
