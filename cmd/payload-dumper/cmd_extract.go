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

	"crypto/sha256"
	"encoding/hex"
	"io"
	"path/filepath"
	"sync"

	"github.com/AlecAivazis/survey/v2"
	"github.com/spf13/cobra"
	"github.com/vbauerster/mpb/v8"
	"github.com/vbauerster/mpb/v8/decor"
	"github.com/xishang0128/payload-dumper-go/common/file"
	"github.com/xishang0128/payload-dumper-go/common/i18n"
	"github.com/xishang0128/payload-dumper-go/dumper"
)

const (
	defaultMaxBufferMB    = 64
	defaultVerifyBufSize  = 4 * 1024 * 1024
	minVerifyBufSize      = 64 * 1024
	defaultProgressWidth  = 60
	defaultPageSize       = 15
	httpTimeout           = 300 * time.Second
	verifyJobsBuffer      = 128
	maxPartitionNameLen   = 24
	partitionNameTruncate = 21
)

var (
	extractOut                      string
	extractPartitions               string
	extractAll                      bool // Extract all partitions
	extractHTTPWorkers              int
	extractHTTPCacheSize            string
	extractVerify                   bool
	extractPprofAddr                string
	extractHeapProfile              string
	extractMaxBufferMB              int
	extractPartitionSizeThresholdMB int    // Partition size threshold (in MB) to distinguish between large and small partitions
	extractStrategy                 string // Extraction strategy: "sequential" or "adaptive"
	extractCPUCount                 int    // Number of CPU cores to use (0 = auto-detect)
)

func initExtractCmd() {
	extractCmd := &cobra.Command{
		Use:   i18n.I18nMsg.Extract.Use,
		Short: i18n.I18nMsg.Extract.Short,
		Long:  i18n.I18nMsg.Extract.Long,
		Args:  cobra.ExactArgs(1),
		Run:   runExtract,
	}

	extractCmd.Flags().StringVarP(&extractOut, "out", "o", "output", i18n.I18nMsg.Common.FlagOut)
	extractCmd.Flags().StringVarP(&extractPartitions, "partitions", "p", "", i18n.I18nMsg.Extract.FlagPartitions)
	extractCmd.Flags().BoolVarP(&extractAll, "all", "a", false, i18n.I18nMsg.Extract.FlagAll)
	extractCmd.Flags().IntVar(&extractHTTPWorkers, "http-workers", 0, i18n.I18nMsg.Extract.FlagHTTPWorkers)
	extractCmd.Flags().StringVar(&extractHTTPCacheSize, "http-cache-size", "", i18n.I18nMsg.Extract.FlagHTTPCacheSize)
	extractCmd.Flags().IntVar(&extractMaxBufferMB, "max-buffer-mb", defaultMaxBufferMB, i18n.I18nMsg.Extract.FlagMaxBufferMB)
	extractCmd.Flags().IntVar(&extractPartitionSizeThresholdMB, "partition-size-threshold-mb", 128, i18n.I18nMsg.Extract.FlagPartitionSizeThresholdMB)
	extractCmd.Flags().StringVar(&extractStrategy, "strategy", "adaptive", i18n.I18nMsg.Extract.FlagStrategy)
	extractCmd.Flags().IntVar(&extractCPUCount, "cpu-count", runtime.NumCPU(), i18n.I18nMsg.Extract.FlagCPUCount)
	extractCmd.Flags().StringVar(&extractPprofAddr, "pprof-addr", "", i18n.I18nMsg.Extract.FlagPprofAddr)
	extractCmd.Flags().StringVar(&extractHeapProfile, "heap-profile", "", i18n.I18nMsg.Extract.FlagHeapProfile)
	extractCmd.Flags().BoolVar(&extractVerify, "verify", false, i18n.I18nMsg.Extract.FlagVerify)

	rootCmd.AddCommand(extractCmd)
}

func runExtract(cmd *cobra.Command, args []string) {
	start := time.Now()
	defer printElapsedTime(start)

	payloadFile := args[0]

	setupExtractionConfig()
	pprofServer := startPprofServer()
	defer stopPprofServer(pprofServer)

	configureHTTPClient()

	d, err := createDumper(payloadFile)
	if err != nil {
		log.Fatalf(i18n.I18nMsg.Common.ErrorFailedToCreateDumper, err)
	}
	defer closeDumper(d)

	partitionNames, err := getPartitionNames(payloadFile)
	if err != nil {
		log.Fatalf(i18n.I18nMsg.Extract.FailedToSelectPartitions, err)
	}

	progress := mpb.New(mpb.WithWidth(defaultProgressWidth))
	progressManager := newProgressManager(progress)
	verificationManager := setupVerificationManager(d)

	progressCallback := createProgressCallback(progressManager, verificationManager)

	// Determine extraction strategy
	strategy := getExtractionStrategy(extractStrategy)

	if err := d.ExtractPartitionsWithStrategy(extractOut, partitionNames, strategy, extractCPUCount, progressCallback); err != nil {
		log.Fatalf(i18n.I18nMsg.Extract.ErrorFailedToExtract, err)
	}

	progress.Wait()

	handleVerificationResults(verificationManager)
	writeHeapProfile()

	fmt.Println(i18n.I18nMsg.Extract.ExtractionCompleted)
}

func printElapsedTime(start time.Time) {
	elapsed := time.Since(start)
	fmt.Printf(i18n.I18nMsg.Common.ElapsedTime+"\n", elapsed)
}

func setupExtractionConfig() {
	if extractMaxBufferMB <= 0 {
		extractMaxBufferMB = defaultMaxBufferMB
	}
	dumper.MaxBufferSize = int64(extractMaxBufferMB) * 1024 * 1024

	// Set partition size threshold for distinguishing between large and small partitions
	if extractPartitionSizeThresholdMB < 0 {
		extractPartitionSizeThresholdMB = 128 // Default to 128MB
	}
	dumper.SetMultithreadThreshold(uint64(extractPartitionSizeThresholdMB) * 1024 * 1024)
}

// startPprofServer starts the pprof server if requested
func startPprofServer() *http.Server {
	if extractPprofAddr == "" {
		return nil
	}

	server := &http.Server{Addr: extractPprofAddr}
	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("pprof server error: %v", err)
		}
	}()
	log.Printf("pprof server started at %s", extractPprofAddr)
	return server
}

// stopPprofServer stops the pprof server if it was started
func stopPprofServer(server *http.Server) {
	if server != nil {
		_ = server.Close()
	}
}

// getPartitionNames gets partition names either from flags, all partitions, or interactively
func getPartitionNames(payloadFile string) ([]string, error) {
	if extractAll {
		return []string{}, nil
	}

	if extractPartitions != "" {
		if strings.ToLower(strings.TrimSpace(extractPartitions)) == "all" {
			return []string{}, nil
		}

		partitionNames := strings.Split(extractPartitions, ",")
		for i, name := range partitionNames {
			partitionNames[i] = strings.TrimSpace(name)
		}
		return partitionNames, nil
	}

	return selectPartitionsInteractively(payloadFile)
}

// configureHTTPClient sets up HTTP client parameters
func configureHTTPClient() {
	file.SetHTTPClientTimeout(httpTimeout)
	file.SetHTTPMaxConcurrentRequests(extractHTTPWorkers)

	if extractHTTPCacheSize != "" {
		if v, err := parseSizeString(extractHTTPCacheSize); err == nil {
			file.SetHTTPReadCacheSize(v)
		} else {
			log.Fatalf(i18n.I18nMsg.Extract.ErrorInvalidHTTPCacheSize, err)
		}
	}
}

// closeDumper safely closes the dumper
func closeDumper(d *dumper.Dumper) {
	if d != nil {
		_ = d.Close()
	}
}

type progressManager struct {
	progress            *mpb.Progress
	bars                map[string]*mpb.Bar
	completedBars       map[string]struct{}
	mu                  sync.Mutex
	totalBar            *mpb.Bar
	partitionTotals     map[string]int
	partitionCurrent    map[string]int
	totalOps            int64
	totalCompleted      int64
	activeOps           int64
	activeCompleted     int64
	totalStartTime      time.Time
	partitionStartTimes map[string]time.Time

	speedWindow     []speedSample
	speedWindowSize int
	lastUpdateTime  time.Time
}

type speedSample struct {
	timestamp time.Time
	ops       int64
}

func newProgressManager(progress *mpb.Progress) *progressManager {
	now := time.Now()
	pm := &progressManager{
		progress:            progress,
		bars:                make(map[string]*mpb.Bar),
		completedBars:       make(map[string]struct{}),
		partitionTotals:     make(map[string]int),
		partitionCurrent:    make(map[string]int),
		totalStartTime:      now,
		partitionStartTimes: make(map[string]time.Time),
		speedWindow:         make([]speedSample, 0, 20),
		speedWindowSize:     5,
		lastUpdateTime:      now,
	}

	pm.totalBar = progress.AddBar(0,
		mpb.BarRemoveOnComplete(),
		mpb.PrependDecorators(
			decor.Name(i18n.I18nMsg.Extract.TotalProgress, decor.WCSyncSpaceR),
		),
		mpb.AppendDecorators(
			decor.Percentage(decor.WC{W: 5}),
			decor.Counters(0, " | %d/%d"),
			decor.AverageETA(decor.ET_STYLE_GO, decor.WC{W: 6}, decor.WCSyncSpace),
			decor.Any(func(st decor.Statistics) string {
				currentSpeed := pm.getCurrentTotalSpeed()
				return fmt.Sprintf(" | %.2f %s", currentSpeed, i18n.I18nMsg.Dumper.OpsSuffix)
			}, decor.WC{W: 20}),
		),
	)

	return pm
}

func (pm *progressManager) getCurrentTotalSpeed() float64 {
	now := time.Now()

	cutoff := now.Add(-time.Duration(pm.speedWindowSize) * time.Second)
	i := 0
	for i < len(pm.speedWindow) && pm.speedWindow[i].timestamp.Before(cutoff) {
		i++
	}
	if i > 0 {
		pm.speedWindow = pm.speedWindow[i:]
	}

	if len(pm.speedWindow) < 2 {
		return 0.0
	}

	oldest := pm.speedWindow[0]
	newest := pm.speedWindow[len(pm.speedWindow)-1]

	timeDiff := newest.timestamp.Sub(oldest.timestamp)
	if timeDiff.Seconds() <= 0 {
		return 0.0
	}

	opsDiff := newest.ops - oldest.ops
	return float64(opsDiff) / timeDiff.Seconds()
}

func (pm *progressManager) updateSpeedWindow() {
	now := time.Now()

	if now.Sub(pm.lastUpdateTime) < 100*time.Millisecond {
		return
	}

	pm.lastUpdateTime = now

	sample := speedSample{
		timestamp: now,
		ops:       pm.totalCompleted,
	}
	pm.speedWindow = append(pm.speedWindow, sample)

	if len(pm.speedWindow) > 50 {
		pm.speedWindow = pm.speedWindow[len(pm.speedWindow)-50:]
	}
}

// verificationManager manages partition verification
type verificationManager struct {
	enabled       bool
	jobs          chan string
	wg            sync.WaitGroup
	results       map[string]error
	resultsMu     sync.Mutex
	partitionsMap map[string]dumper.PartitionInfo
}

// setupVerificationManager sets up verification if enabled
func setupVerificationManager(d *dumper.Dumper) *verificationManager {
	vm := &verificationManager{
		enabled: extractVerify,
		results: make(map[string]error),
	}

	if !extractVerify {
		return vm
	}

	// Get partition map for verification
	pm, err := d.ListPartitionsAsMap()
	if err != nil {
		log.Fatalf(i18n.I18nMsg.Extract.FailedToListPartitions+": %v", err)
	}
	vm.partitionsMap = pm
	vm.jobs = make(chan string, verifyJobsBuffer)

	// Start verification workers
	bufSize := calculateVerifyBufferSize()
	for i := 0; i < runtime.NumCPU(); i++ {
		go vm.verificationWorker(bufSize)
	}

	return vm
}

// calculateVerifyBufferSize determines the optimal buffer size for verification
func calculateVerifyBufferSize() int {
	bufSize := defaultVerifyBufSize
	if dumper.MaxBufferSize > 0 {
		if dumper.MaxBufferSize < int64(bufSize) {
			if dumper.MaxBufferSize < minVerifyBufSize {
				bufSize = minVerifyBufSize
			} else {
				bufSize = int(dumper.MaxBufferSize)
			}
		}
	}
	return bufSize
}

// verificationWorker processes verification jobs
func (vm *verificationManager) verificationWorker(bufSize int) {
	buf := make([]byte, bufSize)
	for partName := range vm.jobs {
		err := vm.verifyPartition(partName, buf)
		vm.resultsMu.Lock()
		vm.results[partName] = err
		vm.resultsMu.Unlock()
		vm.wg.Done()
	}
}

// verifyPartition verifies a single partition
func (vm *verificationManager) verifyPartition(partName string, buf []byte) error {
	expectedHex := ""
	if info, ok := vm.partitionsMap[partName]; ok {
		expectedHex = info.Hash
	}

	if expectedHex == "" {
		return fmt.Errorf(i18n.I18nMsg.Dumper.ErrorNoExpectedHashInManifest)
	}

	path := filepath.Join(extractOut, partName+".img")
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.CopyBuffer(h, f, buf); err != nil {
		return err
	}

	sum := h.Sum(nil)
	gotHex := hex.EncodeToString(sum)
	if gotHex != expectedHex {
		expBytes, _ := hex.DecodeString(expectedHex)
		return fmt.Errorf(i18n.I18nMsg.Dumper.ErrorSha256Mismatch, expBytes, sum)
	}

	return nil
}

// createProgressCallback creates the progress callback function
func createProgressCallback(pm *progressManager, vm *verificationManager) func(dumper.ProgressInfo) {
	return func(pi dumper.ProgressInfo) {
		pm.updateProgress(pi)

		// Enqueue verification if needed
		if vm.enabled && pi.CompletedOps > 0 && pi.CompletedOps == pi.TotalOperations {
			vm.wg.Add(1)
			vm.jobs <- pi.PartitionName
		}
	}
}

// updateProgress updates the progress bar for a partition
func (pm *progressManager) updateProgress(pi dumper.ProgressInfo) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	// Create bar if not exists
	if _, ok := pm.bars[pi.PartitionName]; !ok {
		pm.createProgressBar(pi)
	}

	bar := pm.bars[pi.PartitionName]

	// Update bar progress
	cur := int64(pi.CompletedOps)
	if cur > 0 {
		delta := cur - bar.Current()
		if delta > 0 {
			bar.IncrBy(int(delta))
		}
	}

	// Update total progress
	pm.updateTotalProgress(pi.PartitionName, pi.CompletedOps)

	// Remove completed bars
	if pi.CompletedOps > 0 && pi.CompletedOps == pi.TotalOperations {
		pm.removeCompletedBar(pi.PartitionName, bar)
	}
}

func (pm *progressManager) updateTotalProgress(partitionName string, completedOps int) {
	if pm.totalBar == nil || len(pm.partitionTotals) == 0 {
		return
	}

	prevCompleted := pm.partitionCurrent[partitionName]
	pm.partitionCurrent[partitionName] = completedOps

	// Update total completed operations
	delta := completedOps - prevCompleted
	if delta > 0 {
		pm.totalCompleted += int64(delta)

		// Only update active progress for non-completed partitions
		if _, completed := pm.completedBars[partitionName]; !completed {
			pm.activeCompleted += int64(delta)
		}
	}

	// Update speed window for real-time speed calculation
	pm.updateSpeedWindow()

	// Calculate actual completed operations across all partitions (weighted by size)
	var actualCompleted int64
	for partName, total := range pm.partitionTotals {
		if total > 0 {
			current := pm.partitionCurrent[partName]
			if current > total {
				current = total // Cap at maximum
			}
			actualCompleted += int64(current)
		}
	}

	// Update progress bar to reflect actual completion
	if actualCompleted > pm.totalBar.Current() {
		deltaProgress := actualCompleted - pm.totalBar.Current()
		pm.totalBar.IncrBy(int(deltaProgress))
	}
}

func (pm *progressManager) createProgressBar(pi dumper.ProgressInfo) {
	name := pi.PartitionName
	if len(name) > maxPartitionNameLen {
		name = name[:partitionNameTruncate] + "..."
	}

	partitionDesc := fmt.Sprintf("[%s](%s)", name, pi.SizeReadable)
	bar := pm.progress.AddBar(int64(pi.TotalOperations),
		mpb.BarRemoveOnComplete(),
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
	pm.bars[pi.PartitionName] = bar

	// Record start time for this partition
	pm.partitionStartTimes[pi.PartitionName] = time.Now()

	// Only add to total if this partition is new
	if _, exists := pm.partitionTotals[pi.PartitionName]; !exists {
		pm.partitionTotals[pi.PartitionName] = pi.TotalOperations
		pm.totalOps += int64(pi.TotalOperations)
		pm.activeOps += int64(pi.TotalOperations)

		if pm.totalBar != nil {
			pm.totalBar.SetTotal(pm.totalOps, false)
		}
	}
}

func (pm *progressManager) removeCompletedBar(partitionName string, bar *mpb.Bar) {
	if _, seen := pm.completedBars[partitionName]; !seen {
		pm.completedBars[partitionName] = struct{}{}
		if bar != nil {
			bar.Abort(true)
			delete(pm.bars, partitionName)
		}

		// Remove from active operations count
		if total, exists := pm.partitionTotals[partitionName]; exists {
			pm.activeOps -= int64(total)
			current := pm.partitionCurrent[partitionName]
			pm.activeCompleted -= int64(current)
		}

		// Check if all partitions are completed
		if len(pm.completedBars) == len(pm.partitionTotals) && pm.totalBar != nil {
			// Complete the total progress bar and trigger removal
			if pm.totalBar.Current() < pm.totalOps {
				remaining := pm.totalOps - pm.totalBar.Current()
				pm.totalBar.IncrBy(int(remaining))
			}
			// Force completion to trigger BarRemoveOnComplete
			pm.totalBar.SetTotal(pm.totalOps, true)
		}
	}
}

func handleVerificationResults(vm *verificationManager) {
	if !vm.enabled {
		return
	}

	close(vm.jobs)
	vm.wg.Wait()

	fmt.Println("\nVerification results:")
	vm.resultsMu.Lock()
	defer vm.resultsMu.Unlock()

	var failed []string
	for part, err := range vm.results {
		if err != nil {
			failed = append(failed, fmt.Sprintf("%s: %v", part, err))
		}
	}

	if len(failed) == 0 {
		fmt.Println(i18n.I18nMsg.Extract.VerificationAllOK)
	} else {
		fmt.Println(i18n.I18nMsg.Extract.VerificationFailed)
		for _, line := range failed {
			fmt.Println(line)
		}
	}
}

func writeHeapProfile() {
	if extractHeapProfile == "" {
		return
	}

	f, err := os.Create(extractHeapProfile)
	if err != nil {
		log.Printf("failed to create heap profile file: %v", err)
		return
	}
	defer f.Close()

	runtime.GC()
	if err := pprof.WriteHeapProfile(f); err != nil {
		log.Printf("failed to write heap profile: %v", err)
		return
	}

	log.Printf("heap profile written to %s", extractHeapProfile)
}

func selectPartitionsInteractively(payloadFile string) ([]string, error) {
	d, err := createDumper(payloadFile)
	if err != nil {
		return nil, fmt.Errorf(i18n.I18nMsg.Common.ErrorFailedToCreateDumper+": %v", err)
	}
	defer closeDumper(d)

	partitions, err := d.ListPartitions()
	if err != nil {
		return nil, fmt.Errorf(i18n.I18nMsg.Extract.FailedToListPartitions+": %v", err)
	}

	if len(partitions) == 0 {
		return nil, fmt.Errorf(i18n.I18nMsg.Extract.NoPartitionsFound)
	}

	options := createPartitionOptions(partitions)
	selectedOptions, err := showPartitionSelectionPrompt(options)
	if err != nil {
		return nil, err
	}

	selectedPartitions := parseSelectedPartitions(selectedOptions)

	if len(selectedPartitions) == 0 {
		return nil, fmt.Errorf(i18n.I18nMsg.Extract.NoPartitionsSelected)
	}

	return selectedPartitions, nil
}

func createPartitionOptions(partitions []dumper.PartitionInfo) []string {
	options := make([]string, 0, len(partitions))
	for i, partition := range partitions {
		option := fmt.Sprintf("%d. %s (%s)", i+1, partition.PartitionName, partition.SizeReadable)
		options = append(options, option)
	}
	return options
}

func showPartitionSelectionPrompt(options []string) ([]string, error) {
	prompt := &survey.MultiSelect{
		Message:  i18n.I18nMsg.Extract.InteractiveSelection,
		Options:  options,
		Default:  nil,
		PageSize: defaultPageSize,
	}

	var result []string
	err := survey.AskOne(prompt, &result)
	if err != nil {
		return nil, fmt.Errorf(i18n.I18nMsg.Extract.SelectionCancelled+": %v", err)
	}

	return result, nil
}

func parseSelectedPartitions(selectedOptions []string) []string {
	var partitions []string
	for _, selection := range selectedOptions {
		if partitionName := extractPartitionName(selection); partitionName != "" {
			partitions = append(partitions, partitionName)
		}
	}
	return partitions
}

func extractPartitionName(option string) string {
	parts := strings.SplitN(option, ". ", 2)
	if len(parts) < 2 {
		return ""
	}

	nameAndSize := parts[1]
	nameParts := strings.Split(nameAndSize, " ")
	if len(nameParts) == 0 {
		return ""
	}

	return nameParts[0]
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

// parseSizeString parses size strings like "4M", "256K", "1G" into bytes
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

// getExtractionStrategy converts string strategy to dumper.ExtractionStrategy
func getExtractionStrategy(strategy string) dumper.ExtractionStrategy {
	switch strings.ToLower(strings.TrimSpace(strategy)) {
	case "sequential", "seq":
		return dumper.StrategySequential
	case "adaptive", "auto", "":
		return dumper.StrategyAdaptive
	default:
		// Default to adaptive if unknown strategy
		log.Printf("Unknown extraction strategy '%s', using adaptive strategy", strategy)
		return dumper.StrategyAdaptive
	}
}
