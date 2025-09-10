package i18n

// CommonMessages holds common translatable strings
type CommonMessages struct {
	// Error messages
	ErrorFailedToOpen         string
	ErrorFailedToCreateDumper string
	ErrorFailedToCreateDir    string
	ErrorFailedToWriteFile    string
	ErrorFailedToMarshalJSON  string

	// Common flag descriptions
	FlagOut  string
	FlagJSON string
	FlagSave string
}

// English common messages
var EnglishCommonMessages = CommonMessages{
	ErrorFailedToOpen:         "Failed to open payload file: %v",
	ErrorFailedToCreateDumper: "Failed to create dumper: %v",
	ErrorFailedToCreateDir:    "Failed to create output directory: %v",
	ErrorFailedToWriteFile:    "Failed to write file: %v",
	ErrorFailedToMarshalJSON:  "Failed to marshal JSON: %v",

	FlagOut:  "output directory",
	FlagJSON: "output as JSON",
	FlagSave: "save to file",
}

// Chinese common messages
var ChineseCommonMessages = CommonMessages{
	ErrorFailedToOpen:         "无法打开payload文件: %v",
	ErrorFailedToCreateDumper: "无法创建提取器: %v",
	ErrorFailedToCreateDir:    "无法创建输出目录: %v",
	ErrorFailedToWriteFile:    "无法写入文件: %v",
	ErrorFailedToMarshalJSON:  "无法序列化JSON: %v",

	FlagOut:  "输出目录",
	FlagJSON: "以JSON格式输出",
	FlagSave: "保存到文件",
}
