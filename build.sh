#!/bin/bash

# Build script for payload-dumper-go with automatic XZ implementation selection
# Usage: ./build.sh [fast|pure|both|auto]

set -e

BUILD_TYPE="${1:-auto}"
OUTPUT_DIR="./bin"

mkdir -p "$OUTPUT_DIR"

echo "🔨 Building payload-dumper-go..."

case "$BUILD_TYPE" in
    "fast"|"cgo")
        echo "📦 Building with CGO (fast XZ decompression)..."
        if command -v pkg-config >/dev/null 2>&1 && pkg-config --exists liblzma; then
            # Use pkg-config if available
            CGO_ENABLED=1 \
            CGO_CFLAGS="$(pkg-config --cflags liblzma)" \
            CGO_LDFLAGS="$(pkg-config --libs liblzma)" \
            go build -o "$OUTPUT_DIR/payload-dumper" cmd/payload-dumper/*.go
            echo "✅ CGO version built successfully: $OUTPUT_DIR/payload-dumper"
        elif [[ "$OSTYPE" == "darwin"* ]] && command -v brew >/dev/null 2>&1; then
            # macOS with Homebrew
            XZ_PREFIX=$(brew --prefix xz 2>/dev/null || echo "/opt/homebrew/opt/xz")
            if [[ -f "$XZ_PREFIX/include/lzma.h" ]]; then
                CGO_ENABLED=1 \
                CGO_CFLAGS="-I$XZ_PREFIX/include" \
                CGO_LDFLAGS="-L$XZ_PREFIX/lib" \
                go build -o "$OUTPUT_DIR/payload-dumper" cmd/payload-dumper/*.go
                echo "✅ CGO version built successfully: $OUTPUT_DIR/payload-dumper"
            else
                echo "❌ liblzma not found. Please install with: brew install xz"
                exit 1
            fi
        else
            echo "❌ liblzma development files not found."
            echo "Please install liblzma-dev (Ubuntu/Debian) or xz-devel (CentOS/RHEL)"
            exit 1
        fi
        ;;
    "pure"|"go")
        echo "📦 Building with Pure Go (slower but no dependencies)..."
        CGO_ENABLED=0 go build -o "$OUTPUT_DIR/payload-dumper" cmd/payload-dumper/*.go
        echo "✅ Pure Go version built successfully: $OUTPUT_DIR/payload-dumper"
        ;;
    "auto")
        echo "📦 Auto-detecting best XZ implementation..."
        
        # Try CGO first if liblzma is available
        if command -v pkg-config >/dev/null 2>&1 && pkg-config --exists liblzma; then
            echo "✅ Found liblzma via pkg-config, building CGO version..."
            CGO_ENABLED=1 \
            CGO_CFLAGS="$(pkg-config --cflags liblzma)" \
            CGO_LDFLAGS="$(pkg-config --libs liblzma)" \
            go build -o "$OUTPUT_DIR/payload-dumper" cmd/payload-dumper/*.go
            echo "🚀 CGO version built with optimal performance"
        elif [[ "$OSTYPE" == "darwin"* ]] && command -v brew >/dev/null 2>&1; then
            XZ_PREFIX=$(brew --prefix xz 2>/dev/null || echo "/opt/homebrew/opt/xz")
            if [[ -f "$XZ_PREFIX/include/lzma.h" ]]; then
                echo "✅ Found liblzma via Homebrew, building CGO version..."
                CGO_ENABLED=1 \
                CGO_CFLAGS="-I$XZ_PREFIX/include" \
                CGO_LDFLAGS="-L$XZ_PREFIX/lib" \
                go build -o "$OUTPUT_DIR/payload-dumper" cmd/payload-dumper/*.go
                echo "🚀 CGO version built with optimal performance"
            else
                echo "⚠️  liblzma not found, falling back to Pure Go version..."
                CGO_ENABLED=0 go build -o "$OUTPUT_DIR/payload-dumper" cmd/payload-dumper/*.go
                echo "✅ Pure Go version built (install 'brew install xz' for better performance)"
            fi
        else
            echo "⚠️  liblzma not detected, building Pure Go version..."
            CGO_ENABLED=0 go build -o "$OUTPUT_DIR/payload-dumper" cmd/payload-dumper/*.go
            echo "✅ Pure Go version built (install liblzma-dev for better performance)"
        fi
        ;;
    "both")
        echo "📦 Building both versions..."
        
        # Build pure Go version first (always works)
        echo "Building Pure Go version..."
        CGO_ENABLED=0 go build -o "$OUTPUT_DIR/payload-dumper-pure" cmd/payload-dumper/*.go
        echo "✅ Pure Go version built: $OUTPUT_DIR/payload-dumper-pure"
        
        # Try to build CGO version
        echo "Building CGO version..."
        if command -v pkg-config >/dev/null 2>&1 && pkg-config --exists liblzma; then
            CGO_ENABLED=1 \
            CGO_CFLAGS="$(pkg-config --cflags liblzma)" \
            CGO_LDFLAGS="$(pkg-config --libs liblzma)" \
            go build -o "$OUTPUT_DIR/payload-dumper-fast" cmd/payload-dumper/*.go
            echo "✅ CGO version built: $OUTPUT_DIR/payload-dumper-fast"
        elif [[ "$OSTYPE" == "darwin"* ]] && command -v brew >/dev/null 2>&1; then
            XZ_PREFIX=$(brew --prefix xz 2>/dev/null || echo "/opt/homebrew/opt/xz")
            if [[ -f "$XZ_PREFIX/include/lzma.h" ]]; then
                CGO_ENABLED=1 \
                CGO_CFLAGS="-I$XZ_PREFIX/include" \
                CGO_LDFLAGS="-L$XZ_PREFIX/lib" \
                go build -o "$OUTPUT_DIR/payload-dumper-fast" cmd/payload-dumper/*.go
                echo "✅ CGO version built: $OUTPUT_DIR/payload-dumper-fast"
            else
                echo "⚠️  CGO version not built: liblzma not found"
                echo "   Install with: brew install xz"
            fi
        else
            echo "⚠️  CGO version not built: liblzma development files not found"
        fi
        
        # Create default symlink to the fastest available version
        if [[ -f "$OUTPUT_DIR/payload-dumper-fast" ]]; then
            ln -sf payload-dumper-fast "$OUTPUT_DIR/payload-dumper"
            echo "🔗 Default binary linked to fast version"
        else
            ln -sf payload-dumper-pure "$OUTPUT_DIR/payload-dumper"
            echo "🔗 Default binary linked to pure version"
        fi
        ;;
    *)
        echo "❌ Invalid build type: $BUILD_TYPE"
        echo "Usage: $0 [fast|pure|both|auto]"
        echo "  fast - Build with CGO (requires liblzma)"
        echo "  pure - Build with Pure Go (no dependencies)"
        echo "  both - Build both versions"
        echo "  auto - Auto-detect and build best version (default)"
        exit 1
        ;;
esac

echo ""
echo "🎉 Build completed!"
echo ""
echo "Built binaries:"
ls -la "$OUTPUT_DIR/"
echo ""

# Show version info
if [[ -f "$OUTPUT_DIR/payload-dumper" ]]; then
    echo "Version information:"
    "$OUTPUT_DIR/payload-dumper" version
fi
