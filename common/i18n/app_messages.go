package i18n

// AppMessages holds application-level translatable strings
type AppMessages struct {
	AppDescription     string
	AppLongDescription string

	// Version command messages
	VersionTitle           string
	VersionLabel           string
	GoVersionLabel         string
	PlatformLabel          string
	XZImplementationLabel  string
	PerformanceFastMessage string
	PerformanceSlowMessage string
	PerformanceSlowAdvice  string
	VersionCmdShort        string
	VersionCmdLong         string

	// Compression algorithm messages
	CompressionImplementationsTitle string
	PureGoImplementation            string
	CGOImplementation               string
	CGOPerformanceMessage           string
	PureGoPerformanceMessage        string
	PerformanceAdvice               string
	CompatibilityLabel              string
}

// English app messages
var EnglishAppMessages = AppMessages{
	AppDescription: "Android OTA payload dumper",
	AppLongDescription: `A tool for extracting Android OTA payload files.

This tool can extract partitions from payload.bin files,
list available partitions, and extract metadata.`,

	VersionTitle:           "payload-dumper-go",
	VersionLabel:           "Version",
	GoVersionLabel:         "Go Version",
	PlatformLabel:          "Platform",
	XZImplementationLabel:  "XZ Implementation",
	PerformanceFastMessage: "Using fast CGO-based XZ decompression (faster but requires CGO)",
	PerformanceSlowMessage: "Using pure Go XZ decompression (slower but portable)",
	PerformanceSlowAdvice:  "For better performance, rebuild with CGO enabled and liblzma installed",
	VersionCmdShort:        "Show version information",
	VersionCmdLong:         "Display version information including XZ implementation details",

	CompressionImplementationsTitle: "Compression Algorithm Implementations:",
	PureGoImplementation:            "Pure Go",
	CGOImplementation:               "CGO",
	CGOPerformanceMessage:           "✅ Some compression algorithms use high-performance CGO implementations",
	PureGoPerformanceMessage:        "⚠️  All compression algorithms use standard Pure Go implementations",
	PerformanceAdvice:               "💡 Install relevant C libraries and enable CGO for better performance",
	CompatibilityLabel:              "[Compatibility]",
}

// Chinese app messages
var ChineseAppMessages = AppMessages{
	AppDescription: "Android OTA payload 提取工具",
	AppLongDescription: `用于提取 Android OTA payload 文件的工具。

此工具可以从 payload.bin 文件中提取分区、
列出可用分区并提取元数据。`,

	VersionTitle:           "payload-dumper-go",
	VersionLabel:           "版本",
	GoVersionLabel:         "Go 版本",
	PlatformLabel:          "平台",
	XZImplementationLabel:  "XZ 实现",
	PerformanceFastMessage: "使用快速的 CGO-based XZ 解压缩 (更快但是依赖 CGO)",
	PerformanceSlowMessage: "使用纯 Go XZ 解压缩 (较慢但便携)",
	PerformanceSlowAdvice:  "如需更好性能，请启用 CGO 并安装 liblzma 重新构建",
	VersionCmdShort:        "显示版本信息",
	VersionCmdLong:         "显示版本信息，包括 XZ 实现详情",

	CompressionImplementationsTitle: "压缩算法实现:",
	PureGoImplementation:            "Pure Go",
	CGOImplementation:               "CGO",
	CGOPerformanceMessage:           "✅ 部分压缩算法使用高性能 CGO 实现",
	PureGoPerformanceMessage:        "⚠️  所有压缩算法使用标准 Pure Go 实现",
	PerformanceAdvice:               "💡 安装相关 C 库并启用 CGO 可获得更好的性能",
	CompatibilityLabel:              "[兼容性]",
}
