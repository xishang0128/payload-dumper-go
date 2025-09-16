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
	"github.com/xishang0128/payload-dumper-go/common/i18n"
)

var metadataCmd *cobra.Command

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

func parseMeta(metadata []byte) map[string]string {
	props := make(map[string]string)
	content := string(metadata)
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		if idx := strings.Index(line, "="); idx != -1 {
			key := strings.TrimSpace(line[:idx])
			value := strings.TrimSpace(line[idx+1:])
			props[key] = value
		}
	}

	return props
}

func runMetadata(cmd *cobra.Command, args []string) {
	start := time.Now()
	defer func() {
		elapsed := time.Since(start)
		fmt.Printf(i18n.I18nMsg.Common.ElapsedTime+"\n", elapsed)
	}()
	payloadFile := args[0]

	d, err := createDumper(payloadFile)
	if err != nil {
		log.Fatalf(i18n.I18nMsg.Common.ErrorFailedToCreateDumper, err)
	}

	metadata, err := d.ExtractMetadata()
	if err != nil {
		log.Fatalf(i18n.I18nMsg.Metadata.ErrorFailedToGetMetadata, err)
	}

	if metadataJson {
		props := parseMeta(metadata)
		info := MetadataInfo{
			Properties:  props,
			Size:        len(metadata),
			PayloadFile: payloadFile,
		}

		if metadataRaw {
			info.Raw = string(metadata)
		}

		data, err := json.MarshalIndent(info, "", "    ")
		if err != nil {
			log.Fatalf(i18n.I18nMsg.Common.ErrorFailedToMarshalJSON, err)
		}
		fmt.Println(string(data))
	} else {
		fmt.Printf("%s", string(metadata))
	}

	if metadataSave {
		if err := os.MkdirAll(metadataOut, 0755); err != nil {
			log.Fatalf(i18n.I18nMsg.Common.ErrorFailedToCreateDir, err)
		}

		var outFile string
		var saveData []byte
		var err error

		if metadataJson {
			outFile = filepath.Join(metadataOut, "metadata_info.json")
			props := parseMeta(metadata)
			info := MetadataInfo{
				Properties:  props,
				Size:        len(metadata),
				PayloadFile: payloadFile,
			}

			if metadataRaw {
				info.Raw = string(metadata)
			}

			saveData, err = json.MarshalIndent(info, "", "    ")
			if err != nil {
				log.Fatalf(i18n.I18nMsg.Common.ErrorFailedToMarshalJSON, err)
			}
		} else {
			outFile = filepath.Join(metadataOut, "metadata")
			saveData = metadata
		}

		if err := os.WriteFile(outFile, saveData, 0644); err != nil {
			log.Fatalf(i18n.I18nMsg.Common.ErrorFailedToWriteFile, err)
		}

		fmt.Printf(i18n.I18nMsg.Metadata.MetadataSaved, outFile)
	}
}
