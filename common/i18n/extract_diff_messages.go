package i18n

// ExtractDiffMessages holds extract-diff command translatable strings
type ExtractDiffMessages struct {
	Use   string
	Short string
	Long  string

	FlagOld string

	ErrorFailedToExtractDiff string
	DiffExtractionCompleted  string
}

// English extract-diff messages
var EnglishExtractDiffMessages = ExtractDiffMessages{
	Use:   "extract-diff [payload_file URL/path]",
	Short: "Extract partitions from differential OTA payload",
	Long:  `Extract partitions from differential OTA payload file using base images.`,

	FlagOld: "directory with original images for differential OTA",

	ErrorFailedToExtractDiff: "Failed to extract differential partitions: %v",
	DiffExtractionCompleted:  "Differential extraction completed successfully!",
}

// Chinese extract-diff messages
var ChineseExtractDiffMessages = ExtractDiffMessages{
	Use:   "extract-diff [payload 文件链接/路径]",
	Short: "从差分 OTA payload 中提取分区",
	Long:  `使用基础镜像从差分 OTA payload 文件中提取分区。`,

	FlagOld: "包含原始镜像的目录，用于差分 OTA",

	ErrorFailedToExtractDiff: "无法提取差分分区: %v",
	DiffExtractionCompleted:  "差分提取完成！",
}
