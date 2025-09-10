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
	Use:   "list [payload_file]",
	Short: "List partitions in payload file",
	Long:  `List all partitions available in the Android OTA payload file.`,

	ErrorFailedToList:  "Failed to list partitions: %v",
	TotalPartitions:    "Total %d partitions",
	PartitionInfoSaved: "\nPartition information saved to %s",

	FlagSavePartitions: "save partition info to file",
}

// Chinese list messages
var ChineseListMessages = ListMessages{
	Use:   "list [payload文件]",
	Short: "列出payload文件中的分区",
	Long:  `列出Android OTA payload文件中的所有可用分区。`,

	ErrorFailedToList:  "无法列出分区: %v",
	TotalPartitions:    "共 %d 个分区",
	PartitionInfoSaved: "\n分区信息已保存到 %s",

	FlagSavePartitions: "将分区信息保存到文件",
}
