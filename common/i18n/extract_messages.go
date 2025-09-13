package i18n

// ExtractMessages holds extract command translatable strings
type ExtractMessages struct {
	Use   string
	Short string
	Long  string

	FlagPartitions             string
	FlagWorkers                string
	FlagPartitionWorkers       string
	FlagMultithreadThresholdMB string
	FlagHTTPWorkers            string
	FlagHTTPCacheSize          string
	ErrorInvalidHTTPCacheSize  string
	FlagPprofAddr              string
	FlagHeapProfile            string
	FlagMaxBufferMB            string
	FlagVerify                 string

	ErrorFailedToExtract string
	ExtractionCompleted  string

	// Progress related messages
	ProcessingPartition string
	CompletedPartition  string
	OperationProgress   string

	// Partition selection messages
	SelectPartitions         string
	AvailablePartitions      string
	SelectAll                string
	ConfirmSelection         string
	InvalidSelection         string
	NoPartitionsSelected     string
	InteractiveSelection     string
	FailedToSelectPartitions string
	FailedToListPartitions   string
	NoPartitionsFound        string
	SelectionCancelled       string

	// Total progress messages
	TotalProgress string

	// Verification results
	VerificationAllOK  string
	VerificationFailed string
}

// English extract messages
var EnglishExtractMessages = ExtractMessages{
	Use:   "extract [payload_file URL/path]",
	Short: "Extract partitions from payload file",
	Long:  `Extract all or specified partitions from Android OTA payload file.`,

	FlagPartitions:             "comma separated list of partitions to extract",
	FlagWorkers:                "number of worker threads per partition (for processing operations within a single partition)",
	FlagPartitionWorkers:       "number of partitions to process concurrently",
	FlagMultithreadThresholdMB: "minimum partition size (in MB) to enable multi-threading (0 to always use multi-threading)",
	FlagHTTPWorkers:            "max concurrent HTTP range requests (0 = unlimited)",
	FlagHTTPCacheSize:          "size of HTTP read cache (supports suffix K/M/G, e.g. 4M; 0 = default 1 MiB)",
	ErrorInvalidHTTPCacheSize:  "invalid http-cache-size: %v",
	FlagPprofAddr:              "address to start pprof HTTP server (e.g. localhost:6060)",
	FlagHeapProfile:            "write heap profile to file after extraction",
	FlagMaxBufferMB:            "maximum per-worker buffer size in MB (default 64)",
	FlagVerify:                 "verify partition sha256 after extraction",

	ErrorFailedToExtract: "Failed to extract partitions: %v",
	ExtractionCompleted:  "Extraction completed successfully!",

	// Progress related messages
	ProcessingPartition: "Processing partition: %s (size: %s, %d operations)",
	CompletedPartition:  "Completed partition: %s",
	OperationProgress:   "Processing operations...",

	// Partition selection messages
	SelectPartitions:         "Please select partitions to extract (enter numbers separated by commas, 'a' for all, or press Enter to select all):",
	AvailablePartitions:      "Available partitions:",
	SelectAll:                "Select all",
	ConfirmSelection:         "Selected partitions: %s. Continue? (y/N): ",
	InvalidSelection:         "Invalid selection: %s",
	NoPartitionsSelected:     "No partitions selected for extraction.",
	InteractiveSelection:     "Please select partitions to extract (space to select/deselect, enter to confirm, <right> to select all, <left> to deselect all, or type to filter):",
	FailedToSelectPartitions: "Failed to select partitions: %v",
	FailedToListPartitions:   "Failed to list partitions: %v",
	NoPartitionsFound:        "No partitions found",
	SelectionCancelled:       "Selection cancelled: %v",

	// Total progress messages
	TotalProgress: "Total Progress",

	// Verification results
	VerificationAllOK:  "All verified",
	VerificationFailed: "Failed files:",
}

// Chinese extract messages
var ChineseExtractMessages = ExtractMessages{
	Use:   "extract [payload 文件链接/路径]",
	Short: "从 payload 文件中提取分区",
	Long:  `从 Android OTA payload 文件中提取全部或指定的分区。`,

	FlagPartitions:             "要提取的分区列表，用逗号分隔",
	FlagWorkers:                "每个分区的工作线程数（用于处理单个分区内的操作）",
	FlagPartitionWorkers:       "同时处理的分区数量",
	FlagMultithreadThresholdMB: "启用多线程的最小分区大小（MB）（设为0则始终使用多线程）",
	FlagHTTPWorkers:            "最大并发 HTTP Range 请求数 (0 = 不限制)",
	FlagHTTPCacheSize:          "HTTP 读取缓存大小，支持后缀 K/M/G，例如 4M；0 = 默认 1 MiB",
	ErrorInvalidHTTPCacheSize:  "无效的 http-cache-size: %v",
	FlagPprofAddr:              "启动 pprof HTTP 服务的地址（例如 localhost:6060）",
	FlagHeapProfile:            "在提取完成后将 heap profile 写入到文件",
	FlagMaxBufferMB:            "每个 worker 可使用的最大缓冲区大小（MB）（默认 64）",
	FlagVerify:                 "提取后验证分区的 SHA256 校验和",

	ErrorFailedToExtract: "无法提取分区: %v",
	ExtractionCompleted:  "提取完成！",

	// Progress related messages
	ProcessingPartition: "正在处理分区: %s (大小: %s, %d 个操作)",
	CompletedPartition:  "完成分区: %s",
	OperationProgress:   "正在处理操作...",

	// Partition selection messages
	SelectPartitions:         "请选择要提取的分区（输入用逗号分隔的数字，输入 'a' 选择全部，或直接按回车选择全部）：",
	AvailablePartitions:      "可用分区：",
	SelectAll:                "选择全部",
	ConfirmSelection:         "已选择分区：%s。是否继续？(y/N)：",
	InvalidSelection:         "无效选择：%s",
	NoPartitionsSelected:     "未选择任何分区进行提取。",
	InteractiveSelection:     "请选择要提取的分区 (空格键选择/取消选择，回车确认，<右键>全选，<左键>全不选，输入文本筛选):",
	FailedToSelectPartitions: "无法选择分区: %v",
	FailedToListPartitions:   "无法列出分区: %v",
	NoPartitionsFound:        "未找到分区",
	SelectionCancelled:       "选择已取消: %v",

	// Total progress messages
	TotalProgress: "总进度",

	// Verification results
	VerificationAllOK:  "全部验证通过",
	VerificationFailed: "验证失败的文件：",
}
