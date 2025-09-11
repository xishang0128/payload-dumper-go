package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/xishang0128/payload-dumper-go/common/file"
	"github.com/xishang0128/payload-dumper-go/common/i18n"
	"github.com/xishang0128/payload-dumper-go/dumper"
)

var (
	listOut  string
	listJson bool
	listSave bool
)

func initListCmd() {
	// Initialize list command with localized strings
	listCmd := &cobra.Command{
		Use:   i18n.I18nMsg.List.Use,
		Short: i18n.I18nMsg.List.Short,
		Long:  i18n.I18nMsg.List.Long,
		Args:  cobra.ExactArgs(1),
		Run:   runList,
	}

	listCmd.Flags().StringVarP(&listOut, "out", "o", "output", i18n.I18nMsg.Common.FlagOut)
	listCmd.Flags().BoolVarP(&listJson, "json", "j", false, i18n.I18nMsg.Common.FlagJSON)
	listCmd.Flags().BoolVarP(&listSave, "save", "s", false, i18n.I18nMsg.List.FlagSavePartitions)

	rootCmd.AddCommand(listCmd)
}

func runList(cmd *cobra.Command, args []string) {
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

	// Get partition info
	partitionsInfo, err := d.ListPartitions()
	if err != nil {
		log.Fatalf(i18n.I18nMsg.List.ErrorFailedToList, err)
	}

	// Output results
	if listJson {
		// Output as JSON to stdout
		jsonData, err := json.MarshalIndent(partitionsInfo, "", "    ")
		if err != nil {
			log.Fatalf(i18n.I18nMsg.Common.ErrorFailedToMarshalJSON, err)
		}
		fmt.Println(string(jsonData))
	} else {
		// Output as human-readable format
		fmt.Printf(i18n.I18nMsg.List.TotalPartitions+"\n", len(partitionsInfo))
		for _, info := range partitionsInfo {
			fmt.Printf("%s (%s)\n", info.PartitionName, info.SizeReadable)
		}
	}

	// Save to file if requested
	if listSave {
		if err := os.MkdirAll(listOut, 0755); err != nil {
			log.Fatalf(i18n.I18nMsg.Common.ErrorFailedToCreateDir, err)
		}

		var outputFile string
		var dataToSave []byte
		var err error

		if listJson {
			// Save as JSON file
			outputFile = filepath.Join(listOut, "partitions_info.json")
			dataToSave, err = json.MarshalIndent(partitionsInfo, "", "    ")
			if err != nil {
				log.Fatalf(i18n.I18nMsg.Common.ErrorFailedToMarshalJSON, err)
			}
		} else {
			// Save as plain text file
			outputFile = filepath.Join(listOut, "partitions_info.txt")
			var content strings.Builder
			content.WriteString(fmt.Sprintf(i18n.I18nMsg.List.TotalPartitions+"\n", len(partitionsInfo)))
			for _, info := range partitionsInfo {
				content.WriteString(fmt.Sprintf("%s (%s)\n", info.PartitionName, info.SizeReadable))
			}
			dataToSave = []byte(content.String())
		}

		if err := os.WriteFile(outputFile, dataToSave, 0644); err != nil {
			log.Fatalf(i18n.I18nMsg.Common.ErrorFailedToWriteFile, err)
		}

		fmt.Printf(i18n.I18nMsg.List.PartitionInfoSaved, outputFile)
	}
}
