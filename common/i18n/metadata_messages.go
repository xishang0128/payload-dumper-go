package i18n

// MetadataMessages holds metadata command translatable strings
type MetadataMessages struct {
	Use   string
	Short string
	Long  string

	FlagRaw string

	ErrorFailedToGetMetadata string
	MetadataSaved            string
}

// English metadata messages
var EnglishMetadataMessages = MetadataMessages{
	Use:   "metadata [payload_file URL/path]",
	Short: "Extract metadata from payload file",
	Long:  `Extract and display metadata information from Android OTA payload file.`,

	FlagRaw: "include raw content in JSON output",

	ErrorFailedToGetMetadata: "Failed to extract metadata: %v",
	MetadataSaved:            "\nMetadata saved to %s",
}

// Chinese metadata messages
var ChineseMetadataMessages = MetadataMessages{
	Use:   "metadata [payload 文件链接/路径]",
	Short: "从 payload 文件中提取元数据",
	Long:  `从 Android OTA payload 文件中提取并显示元数据信息。`,

	FlagRaw: "在 JSON 输出中包含原始内容",

	ErrorFailedToGetMetadata: "无法提取元数据: %v",
	MetadataSaved:            "\n元数据已保存到 %s",
}
