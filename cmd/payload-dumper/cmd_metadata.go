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
	"github.com/xishang/payload-dumper-go/common/file"
	"github.com/xishang/payload-dumper-go/common/i18n"
	"github.com/xishang/payload-dumper-go/dumper"
)

var metadataCmd *cobra.Command

// MetadataInfo represents metadata information in JSON format
type MetadataInfo struct {
	Properties  map[string]string `json:"properties"`
	Size        int               `json:"size"`
	PayloadFile string            `json:"payload_file"`
	Raw         string            `json:"raw,omitempty"`
}

var (
	metadataOut  string
	metadataSave bool
	metadataJson bool
	metadataRaw  bool
)

func initMetadataCmd() {
	// Initialize metadata command with localized strings
	metadataCmd = &cobra.Command{
		Use:   i18n.I18nMsg.Metadata.Use,
		Short: i18n.I18nMsg.Metadata.Short,
		Long:  i18n.I18nMsg.Metadata.Long,
		Args:  cobra.ExactArgs(1),
		Run:   runMetadata,
	}

	metadataCmd.Flags().StringVarP(&metadataOut, "out", "o", "output", i18n.I18nMsg.Common.FlagOut)
	metadataCmd.Flags().BoolVarP(&metadataSave, "save", "s", false, i18n.I18nMsg.Common.FlagSave)
	metadataCmd.Flags().BoolVarP(&metadataJson, "json", "j", false, i18n.I18nMsg.Common.FlagJSON)
	metadataCmd.Flags().BoolVarP(&metadataRaw, "raw", "r", false, i18n.I18nMsg.Metadata.FlagRaw)

	rootCmd.AddCommand(metadataCmd)
}

// parseMetadata parses the metadata content into a map of properties
func parseMetadata(metadata []byte) map[string]string {
	properties := make(map[string]string)
	content := string(metadata)

	// Split by lines and parse key=value pairs
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Find the first = to split key and value
		if idx := strings.Index(line, "="); idx != -1 {
			key := strings.TrimSpace(line[:idx])
			value := strings.TrimSpace(line[idx+1:])
			properties[key] = value
		}
	}

	return properties
}

func runMetadata(cmd *cobra.Command, args []string) {
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

	// Extract metadata
	metadata, err := d.ExtractMetadata()
	if err != nil {
		log.Fatalf(i18n.I18nMsg.Metadata.ErrorFailedToGetMetadata, err)
	}

	// Output results
	if metadataJson {
		// Output as JSON
		properties := parseMetadata(metadata)
		metadataInfo := MetadataInfo{
			Properties:  properties,
			Size:        len(metadata),
			PayloadFile: payloadFile,
		}

		// Include raw content only if --raw flag is specified
		if metadataRaw {
			metadataInfo.Raw = string(metadata)
		}

		jsonData, err := json.MarshalIndent(metadataInfo, "", "    ")
		if err != nil {
			log.Fatalf(i18n.I18nMsg.Common.ErrorFailedToMarshalJSON, err)
		}
		fmt.Println(string(jsonData))
	} else {
		// Output as plain text
		fmt.Printf("%s", string(metadata))
	}

	// Save to file if requested
	if metadataSave {
		if err := os.MkdirAll(metadataOut, 0755); err != nil {
			log.Fatalf(i18n.I18nMsg.Common.ErrorFailedToCreateDir, err)
		}

		var outputFile string
		var dataToSave []byte
		var err error

		if metadataJson {
			// Save as JSON file
			outputFile = filepath.Join(metadataOut, "metadata_info.json")
			properties := parseMetadata(metadata)
			metadataInfo := MetadataInfo{
				Properties:  properties,
				Size:        len(metadata),
				PayloadFile: payloadFile,
			}

			// Include raw content only if --raw flag is specified
			if metadataRaw {
				metadataInfo.Raw = string(metadata)
			}

			dataToSave, err = json.MarshalIndent(metadataInfo, "", "    ")
			if err != nil {
				log.Fatalf(i18n.I18nMsg.Common.ErrorFailedToMarshalJSON, err)
			}
		} else {
			// Save as plain text file
			outputFile = filepath.Join(metadataOut, "metadata")
			dataToSave = metadata
		}

		if err := os.WriteFile(outputFile, dataToSave, 0644); err != nil {
			log.Fatalf(i18n.I18nMsg.Common.ErrorFailedToWriteFile, err)
		}

		fmt.Printf(i18n.I18nMsg.Metadata.MetadataSaved, outputFile)
	}
}
