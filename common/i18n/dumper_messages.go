package i18n

// DumperMessages holds dumper package translatable strings
type DumperMessages struct {
	// Error messages
	ErrorInvalidMagic              string
	ErrorUnsupportedFileFormat     string
	ErrorInsufficientDataForHeader string
	ErrorFailedToParseManifest     string
	ErrorFailedToCreateOutputFile  string
	ErrorFailedToOpenOldFile       string
	ErrorFailedToReadOperationData string
	ErrorFailedToDecompressBzip2   string
	ErrorFailedToCreateXZReader    string
	ErrorFailedToDecompressXZ      string
	ErrorSourceCopyOnlyForDiff     string
	ErrorUnsupportedOperationType  string
	ErrorFailedToWriteToFile       string
	ErrorFailedToReadFromOldFile   string
	ErrorFailedToWriteZeros        string
	ErrorFailedToExtractMetadata   string
	ErrorFailedToReadMetadata      string

	// Info messages
	PartitionNotFound        string
	NotOperatingOnPartitions string
	CompletedPartitions      string
	ProcessingPartition      string

	// Progress related
	Operations string
	OpsSuffix  string
}

// English dumper messages
var EnglishDumperMessages = DumperMessages{
	// Error messages
	ErrorInvalidMagic:              "invalid magic: %s",
	ErrorUnsupportedFileFormat:     "unsupported file format version: %d",
	ErrorInsufficientDataForHeader: "insufficient data for header",
	ErrorFailedToParseManifest:     "failed to parse manifest: %v",
	ErrorFailedToCreateOutputFile:  "failed to create output file: %v",
	ErrorFailedToOpenOldFile:       "failed to open old file: %v",
	ErrorFailedToReadOperationData: "failed to read operation data: %v",
	ErrorFailedToDecompressBzip2:   "failed to decompress bzip2 data: %v",
	ErrorFailedToCreateXZReader:    "failed to create xz reader: %v",
	ErrorFailedToDecompressXZ:      "failed to decompress xz data: %v",
	ErrorSourceCopyOnlyForDiff:     "SOURCE_COPY supported only for differential OTA",
	ErrorUnsupportedOperationType:  "unsupported operation type: %v",
	ErrorFailedToWriteToFile:       "failed to write to file: %v",
	ErrorFailedToReadFromOldFile:   "failed to read from old file: %v",
	ErrorFailedToWriteZeros:        "failed to write zeros: %v",
	ErrorFailedToExtractMetadata:   "failed to extract %s: %v",
	ErrorFailedToReadMetadata:      "failed to read metadata: %v",

	// Info messages
	PartitionNotFound:        "Partition %s not found in image",
	NotOperatingOnPartitions: "Not operating on any partitions",
	CompletedPartitions:      "Completed partitions: ",
	ProcessingPartition:      "Processing partition %s: %v",

	// Progress related
	Operations: "ops",
	OpsSuffix:  "ops/s",
}

// Chinese dumper messages
var ChineseDumperMessages = DumperMessages{
	// Error messages
	ErrorInvalidMagic:              "无效的文件标识: %s",
	ErrorUnsupportedFileFormat:     "不支持的文件格式版本: %d",
	ErrorInsufficientDataForHeader: "文件头数据不足",
	ErrorFailedToParseManifest:     "解析清单文件失败: %v",
	ErrorFailedToCreateOutputFile:  "创建输出文件失败: %v",
	ErrorFailedToOpenOldFile:       "打开旧文件失败: %v",
	ErrorFailedToReadOperationData: "读取操作数据失败: %v",
	ErrorFailedToDecompressBzip2:   "解压bzip2数据失败: %v",
	ErrorFailedToCreateXZReader:    "创建xz读取器失败: %v",
	ErrorFailedToDecompressXZ:      "解压xz数据失败: %v",
	ErrorSourceCopyOnlyForDiff:     "SOURCE_COPY操作仅支持差分OTA",
	ErrorUnsupportedOperationType:  "不支持的操作类型: %v",
	ErrorFailedToWriteToFile:       "写入文件失败: %v",
	ErrorFailedToReadFromOldFile:   "从旧文件读取失败: %v",
	ErrorFailedToWriteZeros:        "写入零值失败: %v",
	ErrorFailedToExtractMetadata:   "提取%s失败: %v",
	ErrorFailedToReadMetadata:      "读取元数据失败: %v",

	// Info messages
	PartitionNotFound:        "镜像中未找到分区 %s",
	NotOperatingOnPartitions: "没有分区需要处理",
	CompletedPartitions:      "已完成的分区: ",
	ProcessingPartition:      "正在处理分区 %s: %v",

	// Progress related
	Operations: "操作",
	OpsSuffix:  "操作/秒",
}
