# Payload Dumper Go

A Go library and command-line tool for extracting Android OTA payload files.

[中文](README.zh-CN.md)

## Features

- Multi-threaded high-performance extraction
- Support for local files, HTTP URLs, and ZIP archives
- Differential OTA support

## Installation

### Command-line tool
```bash
go install github.com/xishang/payload-dumper-go/cmd/payload-dumper@latest
```

### Go library
```bash
go get github.com/xishang/payload-dumper-go
```

## Usage

### Basic usage
```bash
# Extract all partitions
payload-dumper extract payload.bin

# Extract specific partitions
payload-dumper extract payload.bin -p system,boot,vendor

# Extract from URL
payload-dumper extract https://example.com/ota-update.zip -p boot

# Differential OTA extraction
payload-dumper extract-diff incremental_payload.bin --old /path/to/base/images
```

### Other commands
```bash
# List partitions
payload-dumper list payload.bin

# Extract metadata
payload-dumper metadata payload.bin

# Get help
payload-dumper --help
```

## Library Usage

```go
package main

import (
    "github.com/xishang/payload-dumper-go/dumper"
    "github.com/xishang/payload-dumper-go/common/file"
)

func main() {
    // Open file
    reader, err := file.NewLocalFile("payload.bin")
    if err != nil {
        panic(err)
    }
    defer reader.Close()

    // Create dumper and extract
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
