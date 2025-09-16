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
	"github.com/xishang0128/payload-dumper-go/dumper"
)

var (
	listOut        string
	listPartitions string
	listJson       bool
	listSave       bool
)

func initListCmd() {
	listCmd := &cobra.Command{
		Use:   i18n.I18nMsg.List.Use,
		Short: i18n.I18nMsg.List.Short,
		Long:  i18n.I18nMsg.List.Long,
		Args:  cobra.ExactArgs(1),
		Run:   runList,
	}

	listCmd.Flags().StringVarP(&listOut, "out", "o", "output", i18n.I18nMsg.Common.FlagOut)
	listCmd.Flags().StringVarP(&listPartitions, "partitions", "p", "", i18n.I18nMsg.Extract.FlagPartitions)
	listCmd.Flags().BoolVarP(&listJson, "json", "j", false, i18n.I18nMsg.Common.FlagJSON)
	listCmd.Flags().BoolVarP(&listSave, "save", "s", false, i18n.I18nMsg.List.FlagSavePartitions)

	rootCmd.AddCommand(listCmd)
}

func runList(cmd *cobra.Command, args []string) {
	start := time.Now()
	defer func() {
		elapsed := time.Since(start)
		if !listJson {
			fmt.Printf(i18n.I18nMsg.Common.ElapsedTime+"\n", elapsed)
		}
	}()

	payloadFile := args[0]

	d, err := createDumper(payloadFile)
	if err != nil {
		log.Fatalf(i18n.I18nMsg.Common.ErrorFailedToCreateDumper, err)
	}

	partitionsInfo, err := d.ListPartitionsAsMap()
	if err != nil {
		log.Fatalf(i18n.I18nMsg.List.ErrorFailedToList, err)
	}

	if listPartitions != "" {
		req := strings.Split(listPartitions, ",")
		pi := make(map[string]dumper.PartitionInfo)

		for _, name := range req {
			if info, ok := partitionsInfo[name]; ok {
				pi[name] = info
			}
		}
		partitionsInfo = pi
	}

	if listJson {
		data, err := json.MarshalIndent(partitionsInfo, "", "    ")
		if err != nil {
			log.Fatalf(i18n.I18nMsg.Common.ErrorFailedToMarshalJSON, err)
		}
		fmt.Println(string(data))
	} else {
		fmt.Printf(i18n.I18nMsg.List.TotalPartitions+"\n", len(partitionsInfo))
		for _, info := range partitionsInfo {
			fmt.Printf("%s (%s)\n", info.PartitionName, info.SizeReadable)
		}
	}

	if listSave {
		if err := os.MkdirAll(listOut, 0755); err != nil {
			log.Fatalf(i18n.I18nMsg.Common.ErrorFailedToCreateDir, err)
		}

		var outFile string
		var saveData []byte
		var err error

		if listJson {
			outFile = filepath.Join(listOut, "partitions_info.json")
			saveData, err = json.MarshalIndent(partitionsInfo, "", "    ")
			if err != nil {
				log.Fatalf(i18n.I18nMsg.Common.ErrorFailedToMarshalJSON, err)
			}
		} else {
			outFile = filepath.Join(listOut, "partitions_info.txt")
			var content strings.Builder
			content.WriteString(fmt.Sprintf(i18n.I18nMsg.List.TotalPartitions+"\n", len(partitionsInfo)))
			for _, info := range partitionsInfo {
				content.WriteString(fmt.Sprintf("%s (%s)\n", info.PartitionName, info.SizeReadable))
			}
			saveData = []byte(content.String())
		}

		if err := os.WriteFile(outFile, saveData, 0644); err != nil {
			log.Fatalf(i18n.I18nMsg.Common.ErrorFailedToWriteFile, err)
		}

		fmt.Printf(i18n.I18nMsg.List.PartitionInfoSaved, outFile)
	}
}
