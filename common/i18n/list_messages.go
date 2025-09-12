package i18n

// ListMessages holds list command translatable strings
type ListMessages struct {
	Use   string
	Short string
	Long  string

	ErrorFailedToList  string
	TotalPartitions    string
	PartitionInfoSaved string

	FlagSavePartitions string
}

// English list messages
var EnglishListMessages = ListMessages{
	Use:   "list [payload_file URL/path]",
	Short: "List partitions in payload file",
	Long:  `List all partitions available in the Android OTA payload file.`,

	ErrorFailedToList:  "Failed to list partitions: %v",
	TotalPartitions:    "Total %d partitions",
	PartitionInfoSaved: "\nPartition information saved to %s",

	FlagSavePartitions: "save partition info to file",
}

// Chinese list messages
var ChineseListMessages = ListMessages{
	Use:   "list [payload 文件链接/路径]",
	Short: "列出 payload 文件中的分区",
	Long:  `列出 Android OTA payload 文件中的所有可用分区。`,

	ErrorFailedToList:  "无法列出分区: %v",
	TotalPartitions:    "共 %d 个分区",
	PartitionInfoSaved: "\n分区信息已保存到 %s",

	FlagSavePartitions: "将分区信息保存到文件",
}
