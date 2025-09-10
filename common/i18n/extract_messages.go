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
}
