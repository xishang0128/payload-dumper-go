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
}
