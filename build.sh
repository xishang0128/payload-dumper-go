#!/bin/bash

# Build script for payload-dumper-go with automatic XZ implementation selection
# Usage: ./build.sh [fast|pure|both|auto|static]

set -e

BUILD_TYPE="${1:-auto}"
OUTPUT_DIR="./bin"

# Embed version and build time into binaries. VERSION defaults to 'dev' when not provided.
VERSION="${VERSION:-dev}"
# Use UTC build time if not provided
BUILDTIME="${BUILDTIME:-$(date -u +%Y-%m-%dT%H:%M:%SZ)}"

# Common ldflags to embed version and build time into the binary
LDFLAGS="-s -w -X 'github.com/xishang0128/payload-dumper-go/constant.Version=${VERSION}' -X 'github.com/xishang0128/payload-dumper-go/constant.BuildTime=${BUILDTIME}'"

mkdir -p "$OUTPUT_DIR"

echo "🔨 Building payload-dumper-go..."

case "$BUILD_TYPE" in
    "fast"|"cgo")
        echo "📦 Building with CGO (fast XZ decompression)..."
        if [[ "$OSTYPE" != "darwin"* ]] && command -v pkg-config >/dev/null 2>&1 && pkg-config --exists liblzma; then
            # Use pkg-config if available
            CGO_ENABLED=1 \
            CGO_CFLAGS="$(pkg-config --cflags liblzma)" \
            CGO_LDFLAGS="$(pkg-config --libs liblzma)" \
            go build -ldflags="$LDFLAGS" -o "$OUTPUT_DIR/payload-dumper-cgo-gcc" cmd/payload-dumper/*.go
            echo "✅ CGO version built successfully: $OUTPUT_DIR/payload-dumper-cgo-gcc"
        elif [[ "$OSTYPE" == "darwin"* ]] && command -v brew >/dev/null 2>&1; then
            # macOS with Homebrew
            XZ_PREFIX=$(brew --prefix xz 2>/dev/null || echo "/opt/homebrew/opt/xz")
            if [[ -f "$XZ_PREFIX/include/lzma.h" ]]; then
                CGO_ENABLED=1 \
                CGO_CFLAGS="-I$XZ_PREFIX/include" \
                CGO_LDFLAGS="-L$XZ_PREFIX/lib" \
                go build -ldflags="$LDFLAGS" -o "$OUTPUT_DIR/payload-dumper-cgo-brew" cmd/payload-dumper/*.go
                echo "✅ CGO version built successfully: $OUTPUT_DIR/payload-dumper-cgo-brew"
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
    "static")
        echo "📦 Building static binary with CGO (fast XZ decompression)..."
        if [[ "$OSTYPE" == "darwin"* ]] && command -v brew >/dev/null 2>&1; then
            # macOS with Homebrew - use static library
            XZ_PREFIX=$(brew --prefix xz 2>/dev/null || echo "/opt/homebrew/opt/xz")
            if [[ -f "$XZ_PREFIX/include/lzma.h" && -f "$XZ_PREFIX/lib/liblzma.a" ]]; then
                echo "🔗 Building with static liblzma for macOS..."
                echo "   Note: Attempting to force static linking of liblzma"
                
                # Force static linking and disable dynamic liblzma
                CGO_ENABLED=1 \
                CGO_CFLAGS="-I$XZ_PREFIX/include" \
                CGO_LDFLAGS="$XZ_PREFIX/lib/liblzma.a -static-libgcc" \
                go build -a -ldflags="${LDFLAGS} -linkmode external -extldflags '-static'" -o "$OUTPUT_DIR/payload-dumper-cgo-static" cmd/payload-dumper/*.go 2>/dev/null || \
                {
                    echo "⚠️  Full static linking failed on macOS (expected), trying partial static..."
                    # Fallback: Just embed the static library without forcing full static
                    CGO_ENABLED=1 \
                    CGO_CFLAGS="-I$XZ_PREFIX/include" \
                    CGO_LDFLAGS="$XZ_PREFIX/lib/liblzma.a" \
                    go build -a -ldflags="${LDFLAGS}" -o "$OUTPUT_DIR/payload-dumper-cgo-static" cmd/payload-dumper/*.go
                }
                echo "✅ Static liblzma version built successfully: $OUTPUT_DIR/payload-dumper-cgo-static"

                echo ""
                echo "🔍 Checking binary info:"
                if command -v otool >/dev/null 2>&1; then
                    echo "Dynamic libraries (system libraries are expected on macOS):"
                    otool -L "$OUTPUT_DIR/payload-dumper-cgo-static" 2>/dev/null || true
                    echo ""
                    echo "Size comparison:"
                    if [[ -f "$OUTPUT_DIR/payload-dumper-cgo-static" ]]; then
                        echo "  CGO version:    $(du -h "$OUTPUT_DIR/payload-dumper-cgo-static" | cut -f1)"
                    fi
                    echo "  Static version: $(du -h "$OUTPUT_DIR/payload-dumper-cgo-static" | cut -f1)"
                fi
                if command -v file >/dev/null 2>&1; then
                    echo ""
                    echo "Binary info:"
                    file "$OUTPUT_DIR/payload-dumper-cgo-static"
                fi
                
                # Test if liblzma is statically linked by checking for lzma symbols
                echo ""
                echo "🔍 Verifying liblzma static linking..."
                if nm "$OUTPUT_DIR/payload-dumper-cgo-static" 2>/dev/null | grep -q "lzma_"; then
                    echo "✅ liblzma symbols found in binary (statically linked)"
                else
                    echo "⚠️  liblzma symbols not found or stripped"
                fi
            else
                echo "❌ liblzma static library not found at $XZ_PREFIX/lib/liblzma.a"
                echo "Please ensure xz is properly installed with: brew install xz"
                exit 1
            fi
        elif command -v pkg-config >/dev/null 2>&1 && pkg-config --exists liblzma; then
            # Linux/other systems - try full static linking
            echo "🔗 Building fully statically linked binary..."
            CGO_ENABLED=1 \
            CGO_CFLAGS="$(pkg-config --cflags liblzma)" \
            CGO_LDFLAGS="$(pkg-config --libs --static liblzma)" \
            go build -a -ldflags="${LDFLAGS} -extldflags \"-static\"" -o "$OUTPUT_DIR/payload-dumper-cgo-static" cmd/payload-dumper/*.go
            echo "✅ Static CGO version built successfully: $OUTPUT_DIR/payload-dumper-cgo-static"

            # Check if it's truly static
            echo ""
            echo "🔍 Checking if binary is statically linked:"
            if command -v ldd >/dev/null 2>&1; then
                if ldd "$OUTPUT_DIR/payload-dumper-cgo-static" 2>&1 | grep -q "not a dynamic executable"; then
                    echo "✅ Binary is statically linked!"
                else
                    echo "⚠️  Binary has dynamic dependencies:"
                    ldd "$OUTPUT_DIR/payload-dumper-cgo-static" 2>/dev/null || true
                fi
            elif command -v file >/dev/null 2>&1; then
                echo "Binary info:"
                file "$OUTPUT_DIR/payload-dumper-cgo-static"
            fi
        else
            echo "❌ liblzma development files not found."
            echo "Please install:"
            echo "  macOS: brew install xz"
            echo "  Ubuntu/Debian: apt-get install liblzma-dev"
            echo "  CentOS/RHEL: yum install xz-devel"
            exit 1
        fi
        ;;
    "pure"|"go")
        echo "📦 Building with Pure Go (slower but no dependencies)..."
    CGO_ENABLED=0 go build -ldflags="$LDFLAGS" -o "$OUTPUT_DIR/payload-dumper" cmd/payload-dumper/*.go
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
            go build -ldflags="$LDFLAGS" -o "$OUTPUT_DIR/payload-dumper-cgo-gcc" cmd/payload-dumper/*.go
            echo "🚀 CGO version built with optimal performance"
        elif [[ "$OSTYPE" == "darwin"* ]] && command -v brew >/dev/null 2>&1; then
            XZ_PREFIX=$(brew --prefix xz 2>/dev/null || echo "/opt/homebrew/opt/xz")
            if [[ -f "$XZ_PREFIX/include/lzma.h" ]]; then
                echo "✅ Found liblzma via Homebrew, building CGO version..."
                CGO_ENABLED=1 \
                CGO_CFLAGS="-I$XZ_PREFIX/include" \
                CGO_LDFLAGS="-L$XZ_PREFIX/lib" \
                go build -ldflags="$LDFLAGS" -o "$OUTPUT_DIR/payload-dumper-cgo-brew" cmd/payload-dumper/*.go
                echo "🚀 CGO version built with optimal performance"
            else
                echo "⚠️  liblzma not found, falling back to Pure Go version..."
            CGO_ENABLED=0 go build -ldflags="$LDFLAGS" -o "$OUTPUT_DIR/payload-dumper" cmd/payload-dumper/*.go
                echo "✅ Pure Go version built (install 'brew install xz' for better performance)"
            fi
        else
            echo "⚠️  liblzma not detected, building Pure Go version..."
            CGO_ENABLED=0 go build -o "$OUTPUT_DIR/payload-dumper" cmd/payload-dumper/*.go
            echo "✅ Pure Go version built (install liblzma-dev for better performance)"
        fi
        ;;
    "both")
        echo "📦 Building both CGO versions (dynamic and static)..."
        
        # Build pure Go version first (always works)
        echo "Building Pure Go version..."
    CGO_ENABLED=0 go build -ldflags="$LDFLAGS" -o "$OUTPUT_DIR/payload-dumper-pure" cmd/payload-dumper/*.go
        echo "✅ Pure Go version built: $OUTPUT_DIR/payload-dumper-pure"
        
        # Try to build dynamic CGO version
        echo ""
        echo "Building Dynamic CGO version..."
        if command -v pkg-config >/dev/null 2>&1 && pkg-config --exists liblzma; then
            CGO_ENABLED=1 \
            CGO_CFLAGS="$(pkg-config --cflags liblzma)" \
            CGO_LDFLAGS="$(pkg-config --libs liblzma)" \
            go build -ldflags="$LDFLAGS" -o "$OUTPUT_DIR/payload-dumper-cgo-brew" cmd/payload-dumper/*.go
            echo "✅ Dynamic CGO version built: $OUTPUT_DIR/payload-dumper-cgo-brew"
        elif [[ "$OSTYPE" == "darwin"* ]] && command -v brew >/dev/null 2>&1; then
            XZ_PREFIX=$(brew --prefix xz 2>/dev/null || echo "/opt/homebrew/opt/xz")
            if [[ -f "$XZ_PREFIX/include/lzma.h" ]]; then
                CGO_ENABLED=1 \
                CGO_CFLAGS="-I$XZ_PREFIX/include" \
                CGO_LDFLAGS="-L$XZ_PREFIX/lib" \
                go build -ldflags="$LDFLAGS" -o "$OUTPUT_DIR/payload-dumper-cgo-brew" cmd/payload-dumper/*.go
                echo "✅ Dynamic CGO version built: $OUTPUT_DIR/payload-dumper-cgo-brew"
            else
                echo "⚠️  Dynamic CGO version not built: liblzma not found"
                echo "   Install with: brew install xz"
            fi
        else
            echo "⚠️  Dynamic CGO version not built: liblzma development files not found"
        fi
        
        # Try to build static CGO version
        echo ""
        echo "Building Static CGO version..."
        if [[ "$OSTYPE" == "darwin"* ]] && command -v brew >/dev/null 2>&1; then
            XZ_PREFIX=$(brew --prefix xz 2>/dev/null || echo "/opt/homebrew/opt/xz")
            if [[ -f "$XZ_PREFIX/include/lzma.h" && -f "$XZ_PREFIX/lib/liblzma.a" ]]; then
                echo "🔗 Building with static liblzma for macOS..."
                CGO_ENABLED=1 \
                CGO_CFLAGS="-I$XZ_PREFIX/include" \
                CGO_LDFLAGS="$XZ_PREFIX/lib/liblzma.a" \
                go build -a -ldflags="${LDFLAGS}" -o "$OUTPUT_DIR/payload-dumper-cgo-static" cmd/payload-dumper/*.go
                echo "✅ Static CGO version built: $OUTPUT_DIR/payload-dumper-cgo-static"
            else
                echo "⚠️  Static CGO version not built: liblzma.a not found"
            fi
        elif command -v pkg-config >/dev/null 2>&1 && pkg-config --exists liblzma; then
            # Linux static build
            echo "🔗 Building fully statically linked binary..."
            CGO_ENABLED=1 \
            CGO_CFLAGS="$(pkg-config --cflags liblzma)" \
            CGO_LDFLAGS="$(pkg-config --libs --static liblzma)" \
            go build -a -ldflags="${LDFLAGS} -linkmode external -extldflags \"-static\"" -o "$OUTPUT_DIR/payload-dumper-cgo-static" cmd/payload-dumper/*.go 2>/dev/null && \
            echo "✅ Static CGO version built: $OUTPUT_DIR/payload-dumper-cgo-static" || \
            echo "⚠️  Full static linking failed (normal on some systems)"
        else
            echo "⚠️  Static CGO version not built: liblzma development files not found"
        fi
        
        # Create default symlink to the best available version
        echo ""
        echo "🔗 Setting up default binary..."
        if [[ -f "$OUTPUT_DIR/payload-dumper-cgo-static" ]]; then
            ln -sf payload-dumper-cgo-static "$OUTPUT_DIR/payload-dumper"
            echo "🔗 Default binary linked to static CGO version"
        elif [[ -f "$OUTPUT_DIR/payload-dumper-cgo-brew" ]]; then
            ln -sf payload-dumper-cgo-brew "$OUTPUT_DIR/payload-dumper"
            echo "🔗 Default binary linked to dynamic CGO version"
        else
            ln -sf payload-dumper-pure "$OUTPUT_DIR/payload-dumper"
            echo "🔗 Default binary linked to pure Go version"
        fi
        ;;
    *)
        echo "❌ Invalid build type: $BUILD_TYPE"
        echo "Usage: $0 [fast|pure|both|auto|static]"
        echo "  fast   - Build with CGO (requires liblzma) - dynamic linking"
        echo "  pure   - Build with Pure Go (no dependencies)"
        echo "  both   - Build both dynamic and static CGO versions + pure Go"
        echo "  auto   - Auto-detect and build best version (default)"
        echo "  static - Build statically linked binary with CGO"
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
