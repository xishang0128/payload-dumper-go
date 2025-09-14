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
go install github.com/xishang0128/payload-dumper-go/cmd/payload-dumper@latest
```

Or download prebuilt binaries from the Releases page: https://github.com/xishang0128/payload-dumper-go/releases

#### About release names

- `payload-dumper-<os>-<arch>-<tag>`
- `<os>`: operating system, e.g. `linux`, `darwin`, `windows`
- `<arch>`: architecture, e.g. `amd64`, `arm64`
- `<tag>`: tag/variant

##### Tag meanings

- no tag: Go static build (default). Works on most distributions but extraction may be slower.
- `cgo`: CGO enabled, faster extraction.
- `static`: statically linked build, suitable for most distributions.
- `mingw`: MinGW build for Windows.
- `termux`: usable only in Termux environment.
- `gcc`: built using the system's default gcc.
- `brew`: for macOS Homebrew environment; requires `brew install xz`.

### Go library
```bash
go get github.com/xishang0128/payload-dumper-go
```

## Usage

### Basic usage
```bash
# Use selector to extract partitions
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
    "github.com/xishang0128/payload-dumper-go/dumper"
    "github.com/xishang0128/payload-dumper-go/common/file"
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
