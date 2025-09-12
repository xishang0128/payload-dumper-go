package main

import (
	"fmt"
	"log"
	"net/http"
	_ "net/http/pprof"
	"os"
	"runtime"
	"runtime/pprof"
	"strconv"
	"strings"
	"time"

	"sync"

	"github.com/AlecAivazis/survey/v2"
	"github.com/spf13/cobra"
	"github.com/vbauerster/mpb/v8"
	"github.com/vbauerster/mpb/v8/decor"
	"github.com/xishang0128/payload-dumper-go/common/file"
	"github.com/xishang0128/payload-dumper-go/common/i18n"
	"github.com/xishang0128/payload-dumper-go/dumper"
)

var (
	extractOut           string
	extractPartitions    string
	extractWorkers       int
	extractUseBuffer     bool
	extractHTTPWorkers   int
	extractHTTPCacheSize string
	// New flags for profiling and memory tuning
	extractPprofAddr   string
	extractHeapProfile string
	extractMaxBufferMB int
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
	extractCmd.Flags().StringVar(&extractHTTPCacheSize, "http-cache-size", "", i18n.I18nMsg.Extract.FlagHTTPCacheSize)
	extractCmd.Flags().BoolVarP(&extractUseBuffer, "buffer", "b", false, i18n.I18nMsg.Common.FlagBuffer)
	extractCmd.Flags().StringVar(&extractPprofAddr, "pprof-addr", "", i18n.I18nMsg.Extract.FlagPprofAddr)
	extractCmd.Flags().StringVar(&extractHeapProfile, "heap-profile", "", i18n.I18nMsg.Extract.FlagHeapProfile)
	extractCmd.Flags().IntVar(&extractMaxBufferMB, "max-buffer-mb", 64, i18n.I18nMsg.Extract.FlagMaxBufferMB)

	rootCmd.AddCommand(extractCmd)
}

func runExtract(cmd *cobra.Command, args []string) {
	start := time.Now()
	defer func() {
		elapsed := time.Since(start)
		fmt.Printf(i18n.I18nMsg.Common.ElapsedTime+"\n", elapsed)
	}()
	payloadFile := args[0]

	// Apply MaxBufferSize from flag (convert MB to bytes)
	if extractMaxBufferMB <= 0 {
		extractMaxBufferMB = 64
	}
	// set dumper MaxBufferSize (int bytes)
	dumper.MaxBufferSize = int64(extractMaxBufferMB) * 1024 * 1024

	// Start pprof server if requested
	var pprofServer *http.Server
	if extractPprofAddr != "" {
		pprofServer = &http.Server{Addr: extractPprofAddr}
		go func() {
			// _ = http.ListenAndServe(extractPprofAddr, nil) // pprof already registered by import
			if err := pprofServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				log.Printf("pprof server error: %v", err)
			}
		}()
		log.Printf("pprof server started at %s", extractPprofAddr)
	}

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
	if extractHTTPCacheSize != "" {
		if v, err := parseSizeString(extractHTTPCacheSize); err == nil {
			file.SetHTTPReadCacheSize(v)
		} else {
			log.Fatalf(i18n.I18nMsg.Extract.ErrorInvalidHTTPCacheSize, err)
		}
	}
	// Create dumper
	d, err := createDumper(payloadFile)
	if err != nil {
		log.Fatalf(i18n.I18nMsg.Common.ErrorFailedToCreateDumper, err)
	}
	defer func() {
		if d != nil {
			_ = d.Close()
		}
	}()

	// Extract partitions with progress rendered in cmd layer
	progress := mpb.New(mpb.WithWidth(60))
	bars := make(map[string]*mpb.Bar)
	var barsMu sync.Mutex

	progressCallback := func(pi dumper.ProgressInfo) {
		barsMu.Lock()
		defer barsMu.Unlock()
		// create bar if not exists
		if _, ok := bars[pi.PartitionName]; !ok {
			partitionDesc := fmt.Sprintf("[%s](%s)", pi.PartitionName, pi.SizeReadable)
			bar := progress.AddBar(int64(pi.TotalOperations),
				mpb.PrependDecorators(
					decor.Name(partitionDesc, decor.WCSyncSpaceR),
				),
				mpb.AppendDecorators(
					decor.Percentage(decor.WC{W: 5}),
					decor.Counters(0, " | %d/%d"),
					decor.AverageETA(decor.ET_STYLE_GO, decor.WC{W: 6}, decor.WCSyncSpace),
					decor.AverageSpeed(0, fmt.Sprintf(" | %%.2f %s", i18n.I18nMsg.Dumper.OpsSuffix)),
				),
			)
			bars[pi.PartitionName] = bar
		}
		bar := bars[pi.PartitionName]
		// set bar current
		cur := int64(pi.CompletedOps)
		if cur > 0 {
			delta := cur - bar.Current()
			if delta > 0 {
				bar.IncrBy(int(delta))
			}
		}
	}

	if err := d.ExtractPartitionsWithOptionsAndProgress(extractOut, partitionNames, extractWorkers, extractUseBuffer, progressCallback); err != nil {
		log.Fatalf(i18n.I18nMsg.Extract.ErrorFailedToExtract, err)
	}

	progress.Wait()

	// Optionally write heap profile after extraction
	if extractHeapProfile != "" {
		f, err := os.Create(extractHeapProfile)
		if err != nil {
			log.Printf("failed to create heap profile file: %v", err)
		} else {
			runtime.GC() // get up-to-date statistics
			if err := pprof.WriteHeapProfile(f); err != nil {
				log.Printf("failed to write heap profile: %v", err)
			}
			f.Close()
			log.Printf("heap profile written to %s", extractHeapProfile)
		}
	}

	// Shutdown pprof server if it was started
	if pprofServer != nil {
		_ = pprofServer.Close()
	}

	fmt.Println(i18n.I18nMsg.Extract.ExtractionCompleted)
}

// selectPartitionsInteractively shows an interactive partition selector using survey
func selectPartitionsInteractively(p string) ([]string, error) {
	d, err := createDumper(p)
	if err != nil {
		log.Fatalf(i18n.I18nMsg.Common.ErrorFailedToCreateDumper, err)
	}
	defer func() {
		if d != nil {
			_ = d.Close()
		}
	}()
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

	return dumper.New(reader)
}

// parseSizeString parses a human-friendly size string like "4M", "256K", "1G" into bytes.
// Supports suffixes: K, M, G (case-insensitive). No suffix or empty string returns 0.
func parseSizeString(s string) (int64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, nil
	}

	last := s[len(s)-1]
	multiplier := int64(1)
	numStr := s

	switch last {
	case 'K', 'k':
		multiplier = 1 << 10
		numStr = s[:len(s)-1]
	case 'M', 'm':
		multiplier = 1 << 20
		numStr = s[:len(s)-1]
	case 'G', 'g':
		multiplier = 1 << 30
		numStr = s[:len(s)-1]
	}

	numStr = strings.TrimSpace(numStr)
	if numStr == "" {
		return 0, fmt.Errorf("invalid size")
	}

	v, err := strconv.ParseFloat(numStr, 64)
	if err != nil {
		return 0, err
	}

	bytes := int64(v * float64(multiplier))
	return bytes, nil
}
