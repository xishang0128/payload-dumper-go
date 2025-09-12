package main

import (
	"fmt"
	"log"
	"runtime"
	"strings"
	"time"

	"github.com/AlecAivazis/survey/v2"
	"github.com/spf13/cobra"
	"github.com/xishang0128/payload-dumper-go/common/file"
	"github.com/xishang0128/payload-dumper-go/common/i18n"
	"github.com/xishang0128/payload-dumper-go/dumper"
)

var (
	extractOut         string
	extractPartitions  string
	extractWorkers     int
	extractUseBuffer   bool
	extractHTTPWorkers int
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
	extractCmd.Flags().IntVar(&extractHTTPWorkers, "http-workers", 0, i18n.I18nMsg.Extract.FlagHTTPWorkers)
	extractCmd.Flags().BoolVarP(&extractUseBuffer, "buffer", "b", false, i18n.I18nMsg.Common.FlagBuffer)

	rootCmd.AddCommand(extractCmd)
}

func runExtract(cmd *cobra.Command, args []string) {
	start := time.Now()
	defer func() {
		elapsed := time.Since(start)
		fmt.Printf(i18n.I18nMsg.Common.ElapsedTime+"\n", elapsed)
	}()
	payloadFile := args[0]

	var (
		partitionNames []string
		err            error
	)
	if extractPartitions != "" {
		partitionNames = strings.Split(extractPartitions, ",")
		for i, name := range partitionNames {
			partitionNames[i] = strings.TrimSpace(name)
		}
	} else {
		partitionNames, err = selectPartitionsInteractively(payloadFile)
		if err != nil {
			log.Fatalf(i18n.I18nMsg.Extract.FailedToSelectPartitions, err)
		}
	}

	file.SetHTTPClientTimeout(300 * time.Second)
	// Apply HTTP concurrent request limit if provided (0 = unlimited)
	file.SetHTTPMaxConcurrentRequests(extractHTTPWorkers)
	// Create dumper
	d, err := createDumper(payloadFile)
	if err != nil {
		log.Fatalf(i18n.I18nMsg.Common.ErrorFailedToCreateDumper, err)
	}

	// Extract partitions
	if err := d.ExtractPartitionsWithOptions(extractOut, partitionNames, extractWorkers, extractUseBuffer); err != nil {
		log.Fatalf(i18n.I18nMsg.Extract.ErrorFailedToExtract, err)
	}

	fmt.Println(i18n.I18nMsg.Extract.ExtractionCompleted)
}

// selectPartitionsInteractively shows an interactive partition selector using survey
func selectPartitionsInteractively(p string) ([]string, error) {
	d, err := createDumper(p)
	if err != nil {
		log.Fatalf(i18n.I18nMsg.Common.ErrorFailedToCreateDumper, err)
	}
	partitions, err := d.ListPartitions()
	if err != nil {
		return nil, fmt.Errorf(i18n.I18nMsg.Extract.FailedToListPartitions, err)
	}

	if len(partitions) == 0 {
		return nil, fmt.Errorf(i18n.I18nMsg.Extract.NoPartitionsFound)
	}

	var options []string
	for i, partition := range partitions {
		options = append(options, fmt.Sprintf("%d. %s (%s)", i+1, partition.PartitionName, partition.SizeReadable))
	}

	prompt := &survey.MultiSelect{
		Message:  i18n.I18nMsg.Extract.InteractiveSelection,
		Options:  options,
		Default:  nil,
		PageSize: 15,
	}

	var result []string
	err = survey.AskOne(prompt, &result)
	if err != nil {
		return nil, fmt.Errorf(i18n.I18nMsg.Extract.SelectionCancelled, err)
	}

	var selectedPartitions []string
	for _, selection := range result {
		parts := strings.SplitN(selection, ". ", 2)
		if len(parts) >= 2 {
			nameAndSize := parts[1]
			nameParts := strings.Split(nameAndSize, " ")
			if len(nameParts) >= 1 {
				partitionName := nameParts[0]
				selectedPartitions = append(selectedPartitions, partitionName)
			}
		}
	}

	if len(selectedPartitions) == 0 {
		return nil, fmt.Errorf(i18n.I18nMsg.Extract.NoPartitionsSelected)
	}

	return selectedPartitions, nil
}

func createDumper(p string) (*dumper.Dumper, error) {
	var (
		reader file.Reader
		err    error
	)
	if strings.HasPrefix(p, "http://") || strings.HasPrefix(p, "https://") {
		reader, err = file.NewHTTPFile(p)
	} else {
		reader, err = file.NewLocalFile(p)
	}

	if err != nil {
		log.Fatalf(i18n.I18nMsg.Common.ErrorFailedToOpen, err)
	}
	defer reader.Close()

	return dumper.New(reader)
}
