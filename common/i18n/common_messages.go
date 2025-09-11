package i18n

// CommonMessages holds common translatable strings
type CommonMessages struct {
	// Error messages
	ErrorFailedToOpen         string
	ErrorFailedToCreateDumper string
	ErrorFailedToCreateDir    string
	ErrorFailedToWriteFile    string
	ErrorFailedToMarshalJSON  string

	// DNS and network messages
	DNSResolvConfNotFound       string
	DNSUsingFallbackServers     string
	DNSFailedToConnectToServers string
	DNSNoIPAddressesFound       string
	DNSFailedToConnect          string

	// TLS and certificate messages
	TLSUsingFallbackCertificates  string
	TLSCertificateVerificationOff string
	TLSSystemCertPoolNotAvailable string
	TLSSystemCertPoolWithFallback string
	TLSBuiltInCertsLoadFailed     string
	TLSBuiltInCertsLoadSuccess    string

	// HTTP error messages
	HTTPRemoteDoesNotSupportRanges string
	HTTPRemoteHasNoLength          string
	HTTPInvalidContentLength       string
	HTTPRemoteDidNotReturnPartial  string

	// Common flag descriptions
	FlagOut     string
	FlagJSON    string
	FlagSave    string
	ElapsedTime string
}

// English common messages
var EnglishCommonMessages = CommonMessages{
	ErrorFailedToOpen:         "Failed to open payload file: %v",
	ErrorFailedToCreateDumper: "Failed to create dumper: %v",
	ErrorFailedToCreateDir:    "Failed to create output directory: %v",
	ErrorFailedToWriteFile:    "Failed to write file: %v",
	ErrorFailedToMarshalJSON:  "Failed to marshal JSON: %v",

	DNSResolvConfNotFound:       "System does not have /etc/resolv.conf",
	DNSUsingFallbackServers:     "Using fallback DNS servers: %s",
	DNSFailedToConnectToServers: "failed to connect to any DNS server: %v",
	DNSNoIPAddressesFound:       "no IP addresses found for host %s",
	DNSFailedToConnect:          "failed to connect to %s",

	TLSUsingFallbackCertificates:  "Using built-in fallback certificates for TLS verification",
	TLSCertificateVerificationOff: "TLS certificate verification disabled for compatibility",
	TLSSystemCertPoolNotAvailable: "System cert pool not available, using built-in certificates: %v",
	TLSSystemCertPoolWithFallback: "Using system cert pool with built-in certificates as fallback",
	TLSBuiltInCertsLoadFailed:     "Warning: Failed to append some built-in certificates",
	TLSBuiltInCertsLoadSuccess:    "Successfully loaded built-in CA certificates",

	HTTPRemoteDoesNotSupportRanges: "remote does not support ranges",
	HTTPRemoteHasNoLength:          "remote has no length",
	HTTPInvalidContentLength:       "invalid content length: %v",
	HTTPRemoteDidNotReturnPartial:  "remote did not return partial content: %d",

	FlagOut:     "output directory",
	FlagJSON:    "output as JSON",
	FlagSave:    "save to file",
	ElapsedTime: "Elapsed time: %s",
}

// Chinese common messages
var ChineseCommonMessages = CommonMessages{
	ErrorFailedToOpen:         "无法打开payload文件: %v",
	ErrorFailedToCreateDumper: "无法创建提取器: %v",
	ErrorFailedToCreateDir:    "无法创建输出目录: %v",
	ErrorFailedToWriteFile:    "无法写入文件: %v",
	ErrorFailedToMarshalJSON:  "无法序列化JSON: %v",

	DNSResolvConfNotFound:       "系统没有 /etc/resolv.conf 文件",
	DNSUsingFallbackServers:     "使用备用DNS服务器: %s",
	DNSFailedToConnectToServers: "无法连接到任何DNS服务器: %v",
	DNSNoIPAddressesFound:       "未找到主机 %s 的IP地址",
	DNSFailedToConnect:          "无法连接到 %s",

	TLSUsingFallbackCertificates:  "使用内置备用证书进行TLS验证",
	TLSCertificateVerificationOff: "为兼容性已禁用TLS证书验证",
	TLSSystemCertPoolNotAvailable: "系统证书池不可用，使用内置证书: %v",
	TLSSystemCertPoolWithFallback: "使用系统证书池，内置证书作为备用",
	TLSBuiltInCertsLoadFailed:     "警告: 部分内置证书加载失败",
	TLSBuiltInCertsLoadSuccess:    "成功加载内置CA证书",

	HTTPRemoteDoesNotSupportRanges: "远程服务器不支持范围请求",
	HTTPRemoteHasNoLength:          "远程服务器未提供内容长度",
	HTTPInvalidContentLength:       "无效的内容长度: %v",
	HTTPRemoteDidNotReturnPartial:  "远程服务器未返回部分内容: %d",

	FlagOut:     "输出目录",
	FlagJSON:    "以JSON格式输出",
	FlagSave:    "保存到文件",
	ElapsedTime: "耗时: %s",
}
