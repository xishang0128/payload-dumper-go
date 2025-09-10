package i18n

// ExtractMessages holds extract command translatable strings
type ExtractMessages struct {
	Use   string
	Short string
	Long  string

	FlagPartitions string
	FlagWorkers    string

	ErrorFailedToExtract string
	ExtractionCompleted  string

	DefaultOld string

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
}

// English extract messages
var EnglishExtractMessages = ExtractMessages{
	Use:   "extract [payload_file URL/path]",
	Short: "Extract partitions from payload file",
	Long:  `Extract all or specified partitions from Android OTA payload file.`,

	FlagPartitions: "comma separated list of partitions to extract",
	FlagWorkers:    "number of workers",

	ErrorFailedToExtract: "Failed to extract partitions: %v",
	ExtractionCompleted:  "Extraction completed successfully!",

	DefaultOld: "old",

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
	InteractiveSelection:     "Please select partitions to extract (space to select/deselect, enter to confirm, <right> to select all, <left> to deselect all):",
	FailedToSelectPartitions: "Failed to select partitions: %v",
	FailedToListPartitions:   "Failed to list partitions: %v",
	NoPartitionsFound:        "No partitions found",
	SelectionCancelled:       "Selection cancelled: %v",
}

// Chinese extract messages
var ChineseExtractMessages = ExtractMessages{
	Use:   "extract [payload 文件链接/路径]",
	Short: "从payload文件中提取分区",
	Long:  `从Android OTA payload文件中提取全部或指定的分区。`,

	FlagPartitions: "要提取的分区列表，用逗号分隔",
	FlagWorkers:    "工作线程数",

	ErrorFailedToExtract: "无法提取分区: %v",
	ExtractionCompleted:  "提取完成！",

	DefaultOld: "旧版本",

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
	InteractiveSelection:     "请选择要提取的分区 (空格键选择/取消选择，回车确认，<右键>全选，<左键>全不选):",
	FailedToSelectPartitions: "无法选择分区: %v",
	FailedToListPartitions:   "无法列出分区: %v",
	NoPartitionsFound:        "未找到分区",
	SelectionCancelled:       "选择已取消: %v",
}
