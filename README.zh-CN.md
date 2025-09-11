# Payload Dumper Go

一个用于提取 Android OTA payload 文件的 Go 语言工具。

## 特性

- 多线程高性能提取
- 支持本地文件、HTTP URL 和 ZIP 压缩包
- 支持增量 OTA 更新

## 安装

### 命令行工具
```bash
go install github.com/xishang0128/payload-dumper-go/cmd/payload-dumper@latest
```

或者下载预编译的二进制文件：[Releases](https://github.com/xishang0128/payload-dumper-go/releases)

### Go 库
```bash
go get github.com/xishang0128/payload-dumper-go
```

## 使用

### 基本用法
```bash
# 使用选择器提取分区
payload-dumper extract payload.bin

# 提取指定分区
payload-dumper extract payload.bin -p system,boot,vendor

# 从 URL 提取
payload-dumper extract https://example.com/ota-update.zip -p boot

# 增量 OTA 提取
payload-dumper extract-diff incremental_payload.bin --old /path/to/base/images
```

### 其他命令
```bash
# 列出分区
payload-dumper list payload.bin

# 提取元数据
payload-dumper metadata payload.bin

# 获取帮助
payload-dumper --help
```

## 库使用

```go
package main

import (
    "github.com/xishang0128/payload-dumper-go/dumper"
    "github.com/xishang0128/payload-dumper-go/common/file"
)

func main() {
    // 打开文件
    reader, err := file.NewLocalFile("payload.bin")
    if err != nil {
        panic(err)
    }
    defer reader.Close()

    // 创建 dumper 并提取
    d, err := dumper.New(reader)
    if err != nil {
        panic(err)
    }

    err = d.ExtractPartitions("output", []string{"system", "boot"}, 4)
    if err != nil {
        panic(err)
    }
}
```
