#!/bin/bash

set -e

BUILD_TYPE="${1:-auto}"
OUTPUT_DIR="./bin"
VERSION="${VERSION:-dev}"
BUILDTIME="${BUILDTIME:-$(date -u +%Y-%m-%dT%H:%M:%SZ)}"
LDFLAGS="-s -w -X 'github.com/xishang0128/payload-dumper-go/constant.Version=${VERSION}' -X 'github.com/xishang0128/payload-dumper-go/constant.BuildTime=${BUILDTIME}'"

mkdir -p "$OUTPUT_DIR"

echo "🔨 Building payload-dumper-go..."

case "$BUILD_TYPE" in
    "fast"|"cgo")
        echo "📦 Building with CGO..."
        if [[ "$OSTYPE" == "darwin"* ]] && command -v brew >/dev/null 2>&1; then
            XZ_PREFIX=$(brew --prefix xz 2>/dev/null || echo "/opt/homebrew/opt/xz")
            BROTLI_PREFIX=$(brew --prefix brotli 2>/dev/null || echo "/opt/homebrew/opt/brotli")
            
            if [[ -f "$XZ_PREFIX/include/lzma.h" && -f "$BROTLI_PREFIX/include/brotli/decode.h" ]]; then
                CGO_ENABLED=1 \
                CGO_CFLAGS="-I$XZ_PREFIX/include -I$BROTLI_PREFIX/include" \
                CGO_LDFLAGS="-L$XZ_PREFIX/lib -L$BROTLI_PREFIX/lib" \
                go build -ldflags="$LDFLAGS" -o "$OUTPUT_DIR/payload-dumper-cgo-brew" cmd/payload-dumper/*.go
                echo "✅ CGO version built: $OUTPUT_DIR/payload-dumper-cgo-brew"
            else
                echo "❌ Missing libraries. Install: brew install xz brotli"
                exit 1
            fi
        elif command -v pkg-config >/dev/null 2>&1 && pkg-config --exists liblzma libbrotlidec; then
            CGO_ENABLED=1 \
            CGO_CFLAGS="$(pkg-config --cflags liblzma libbrotlidec)" \
            CGO_LDFLAGS="$(pkg-config --libs liblzma libbrotlidec)" \
            go build -ldflags="$LDFLAGS" -o "$OUTPUT_DIR/payload-dumper-cgo" cmd/payload-dumper/*.go
            echo "✅ CGO version built: $OUTPUT_DIR/payload-dumper-cgo"
        else
            echo "❌ CGO libraries not found"
            exit 1
        fi
        ;;
    "static")
        echo "📦 Building optimized static binary..."
        if [[ "$OSTYPE" == "darwin"* ]] && command -v brew >/dev/null 2>&1; then
            XZ_PREFIX=$(brew --prefix xz 2>/dev/null || echo "/opt/homebrew/opt/xz")
            
            if [[ -f "$XZ_PREFIX/include/lzma.h" && -f "$XZ_PREFIX/lib/liblzma.a" ]]; then
                echo "🏷️  Using build tags: cgo_xz cgo_zstd pure_brotli"
                CGO_ENABLED=1 \
                CGO_CFLAGS="-I$XZ_PREFIX/include" \
                CGO_LDFLAGS="$XZ_PREFIX/lib/liblzma.a -Wl,-search_paths_first" \
                go build -tags="cgo_xz cgo_zstd pure_brotli" -a -ldflags="${LDFLAGS}" -o "$OUTPUT_DIR/payload-dumper-cgo-static" cmd/payload-dumper/*.go
                echo "✅ Static version built: $OUTPUT_DIR/payload-dumper-cgo-static"
            else
                echo "❌ Missing XZ library. Install: brew install xz"
                exit 1
            fi
        elif command -v pkg-config >/dev/null 2>&1 && pkg-config --exists liblzma; then
            CGO_ENABLED=1 \
            CGO_CFLAGS="$(pkg-config --cflags liblzma)" \
            CGO_LDFLAGS="$(pkg-config --libs --static liblzma) -Wl,--allow-multiple-definition" \
            go build -tags="cgo_xz cgo_zstd pure_brotli" -a -ldflags="${LDFLAGS} -extldflags \"-static\"" -o "$OUTPUT_DIR/payload-dumper-cgo-static" cmd/payload-dumper/*.go
            echo "✅ Static version built: $OUTPUT_DIR/payload-dumper-cgo-static"
        else
            echo "❌ Missing liblzma library"
            exit 1
        fi
        ;;
    "pure"|"go")
        echo "📦 Building with Pure Go..."
        CGO_ENABLED=0 go build -ldflags="$LDFLAGS" -o "$OUTPUT_DIR/payload-dumper" cmd/payload-dumper/*.go
        echo "✅ Pure Go version built: $OUTPUT_DIR/payload-dumper"
        ;;
    "auto")
        echo "📦 Auto-detecting best XZ implementation..."
        
        # Try CGO first if compression libraries are available
        if command -v pkg-config >/dev/null 2>&1; then
            MISSING_LIBS=()
            CGO_CFLAGS_PARTS=()
            CGO_LDFLAGS_PARTS=()
            
            # Check for liblzma (XZ)
            if pkg-config --exists liblzma; then
                CGO_CFLAGS_PARTS+=("$(pkg-config --cflags liblzma)")
                CGO_LDFLAGS_PARTS+=("$(pkg-config --libs liblzma)")
            else
                MISSING_LIBS+=("liblzma-dev")
            fi
            
            # Check for libbrotli
            if pkg-config --exists libbrotlidec; then
                CGO_CFLAGS_PARTS+=("$(pkg-config --cflags libbrotlidec)")
                CGO_LDFLAGS_PARTS+=("$(pkg-config --libs libbrotlidec)")
            else
                MISSING_LIBS+=("libbrotli-dev")
            fi
            
            if [[ ${#MISSING_LIBS[@]} -eq 0 ]]; then
                echo "✅ Found compression libraries via pkg-config, building CGO version..."
                CGO_ENABLED=1 \
                CGO_CFLAGS="${CGO_CFLAGS_PARTS[*]}" \
                CGO_LDFLAGS="${CGO_LDFLAGS_PARTS[*]}" \
                go build -ldflags="$LDFLAGS" -o "$OUTPUT_DIR/payload-dumper-cgo-gcc" cmd/payload-dumper/*.go
                echo "🚀 CGO version built with optimal performance"
            else
                echo "⚠️  Missing some libraries (${MISSING_LIBS[*]}), falling back to Pure Go version..."
                CGO_ENABLED=0 go build -ldflags="$LDFLAGS" -o "$OUTPUT_DIR/payload-dumper" cmd/payload-dumper/*.go
                echo "✅ Pure Go version built (install ${MISSING_LIBS[*]} for better performance)"
            fi
        elif [[ "$OSTYPE" == "darwin"* ]] && command -v brew >/dev/null 2>&1; then
            XZ_PREFIX=$(brew --prefix xz 2>/dev/null || echo "/opt/homebrew/opt/xz")
            BROTLI_PREFIX=$(brew --prefix brotli 2>/dev/null || echo "/opt/homebrew/opt/brotli")
            
            MISSING_LIBS=()
            CGO_CFLAGS_PARTS=()
            CGO_LDFLAGS_PARTS=()
            
            # Check XZ
            if [[ -f "$XZ_PREFIX/include/lzma.h" ]]; then
                CGO_CFLAGS_PARTS+=("-I$XZ_PREFIX/include")
                CGO_LDFLAGS_PARTS+=("-L$XZ_PREFIX/lib")
            else
                MISSING_LIBS+=("xz")
            fi
            
            # Check Brotli
            if [[ -f "$BROTLI_PREFIX/include/brotli/decode.h" ]]; then
                CGO_CFLAGS_PARTS+=("-I$BROTLI_PREFIX/include")
                CGO_LDFLAGS_PARTS+=("-L$BROTLI_PREFIX/lib")
            else
                MISSING_LIBS+=("brotli")
            fi
            
            if [[ ${#MISSING_LIBS[@]} -eq 0 ]]; then
                echo "✅ Found compression libraries via Homebrew, building CGO version..."
                CGO_ENABLED=1 \
                CGO_CFLAGS="${CGO_CFLAGS_PARTS[*]}" \
                CGO_LDFLAGS="${CGO_LDFLAGS_PARTS[*]}" \
                go build -ldflags="$LDFLAGS" -o "$OUTPUT_DIR/payload-dumper-cgo-brew" cmd/payload-dumper/*.go
                echo "🚀 CGO version built with optimal performance"
            else
                echo "⚠️  Missing libraries (${MISSING_LIBS[*]}), falling back to Pure Go version..."
                CGO_ENABLED=0 go build -ldflags="$LDFLAGS" -o "$OUTPUT_DIR/payload-dumper" cmd/payload-dumper/*.go
                echo "✅ Pure Go version built (install 'brew install ${MISSING_LIBS[*]}' for better performance)"
            fi
        else
            echo "⚠️  Compression libraries not detected, building Pure Go version..."
            CGO_ENABLED=0 go build -o "$OUTPUT_DIR/payload-dumper" cmd/payload-dumper/*.go
            echo "✅ Pure Go version built (install compression libraries for better performance)"
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
        if command -v pkg-config >/dev/null 2>&1; then
            MISSING_LIBS=()
            CGO_CFLAGS_PARTS=()
            CGO_LDFLAGS_PARTS=()
            
            # Check for liblzma (XZ)
            if pkg-config --exists liblzma; then
                CGO_CFLAGS_PARTS+=("$(pkg-config --cflags liblzma)")
                CGO_LDFLAGS_PARTS+=("$(pkg-config --libs liblzma)")
            else
                MISSING_LIBS+=("liblzma-dev")
            fi
            
            # Check for libbrotli
            if pkg-config --exists libbrotlidec; then
                CGO_CFLAGS_PARTS+=("$(pkg-config --cflags libbrotlidec)")
                CGO_LDFLAGS_PARTS+=("$(pkg-config --libs libbrotlidec)")
            else
                MISSING_LIBS+=("libbrotli-dev")
            fi
            
            if [[ ${#MISSING_LIBS[@]} -eq 0 ]]; then
                CGO_ENABLED=1 \
                CGO_CFLAGS="${CGO_CFLAGS_PARTS[*]}" \
                CGO_LDFLAGS="${CGO_LDFLAGS_PARTS[*]}" \
                go build -ldflags="$LDFLAGS" -o "$OUTPUT_DIR/payload-dumper-cgo-gcc" cmd/payload-dumper/*.go
                echo "✅ Dynamic CGO version built: $OUTPUT_DIR/payload-dumper-cgo-gcc"
            else
                echo "⚠️  Dynamic CGO version not built: missing ${MISSING_LIBS[*]}"
            fi
        elif [[ "$OSTYPE" == "darwin"* ]] && command -v brew >/dev/null 2>&1; then
            XZ_PREFIX=$(brew --prefix xz 2>/dev/null || echo "/opt/homebrew/opt/xz")
            BROTLI_PREFIX=$(brew --prefix brotli 2>/dev/null || echo "/opt/homebrew/opt/brotli")
            
            MISSING_LIBS=()
            CGO_CFLAGS_PARTS=()
            CGO_LDFLAGS_PARTS=()
            
            # Check XZ
            if [[ -f "$XZ_PREFIX/include/lzma.h" ]]; then
                CGO_CFLAGS_PARTS+=("-I$XZ_PREFIX/include")
                CGO_LDFLAGS_PARTS+=("-L$XZ_PREFIX/lib")
            else
                MISSING_LIBS+=("xz")
            fi
            
            # Check Brotli
            if [[ -f "$BROTLI_PREFIX/include/brotli/decode.h" ]]; then
                CGO_CFLAGS_PARTS+=("-I$BROTLI_PREFIX/include")
                CGO_LDFLAGS_PARTS+=("-L$BROTLI_PREFIX/lib")
            else
                MISSING_LIBS+=("brotli")
            fi
            
            if [[ ${#MISSING_LIBS[@]} -eq 0 ]]; then
                CGO_ENABLED=1 \
                CGO_CFLAGS="${CGO_CFLAGS_PARTS[*]}" \
                CGO_LDFLAGS="${CGO_LDFLAGS_PARTS[*]}" \
                go build -ldflags="$LDFLAGS" -o "$OUTPUT_DIR/payload-dumper-cgo-brew" cmd/payload-dumper/*.go
                echo "✅ Dynamic CGO version built: $OUTPUT_DIR/payload-dumper-cgo-brew"
            else
                echo "⚠️  Dynamic CGO version not built: missing ${MISSING_LIBS[*]}"
                echo "   Install with: brew install ${MISSING_LIBS[*]}"
            fi
        else
            echo "⚠️  Dynamic CGO version not built: compression libraries development files not found"
        fi
        
        # Try to build static CGO version
        echo ""
        echo "Building Static CGO version..."
        if [[ "$OSTYPE" == "darwin"* ]] && command -v brew >/dev/null 2>&1; then
            XZ_PREFIX=$(brew --prefix xz 2>/dev/null || echo "/opt/homebrew/opt/xz")
            BROTLI_PREFIX=$(brew --prefix brotli 2>/dev/null || echo "/opt/homebrew/opt/brotli")
            
            MISSING_LIBS=()
            CGO_CFLAGS_PARTS=()
            CGO_LDFLAGS_PARTS=()
            
            # Check XZ
            if [[ -f "$XZ_PREFIX/include/lzma.h" && -f "$XZ_PREFIX/lib/liblzma.a" ]]; then
                CGO_CFLAGS_PARTS+=("-I$XZ_PREFIX/include")
                CGO_LDFLAGS_PARTS+=("$XZ_PREFIX/lib/liblzma.a")
            else
                MISSING_LIBS+=("xz")
            fi
            
            # Check Brotli
            if [[ -f "$BROTLI_PREFIX/include/brotli/encode.h" && -f "$BROTLI_PREFIX/lib/libbrotlidec.a" ]]; then
                CGO_CFLAGS_PARTS+=("-I$BROTLI_PREFIX/include")
                CGO_LDFLAGS_PARTS+=("$BROTLI_PREFIX/lib/libbrotlidec.a" "$BROTLI_PREFIX/lib/libbrotlicommon.a")
            else
                MISSING_LIBS+=("brotli")
            fi
            
            if [[ ${#MISSING_LIBS[@]} -eq 0 ]]; then
                echo "🔗 Building with static liblzma and brotli for macOS..."
                STATIC_LDFLAGS="${CGO_LDFLAGS_PARTS[*]} -Wl,-search_paths_first"
                CGO_ENABLED=1 \
                CGO_CFLAGS="${CGO_CFLAGS_PARTS[*]}" \
                CGO_LDFLAGS="$STATIC_LDFLAGS" \
                go build -a -ldflags="${LDFLAGS}" -o "$OUTPUT_DIR/payload-dumper-cgo-static" cmd/payload-dumper/*.go
                echo "✅ Static CGO version built: $OUTPUT_DIR/payload-dumper-cgo-static"
            else
                echo "⚠️  Static CGO version not built: missing ${MISSING_LIBS[*]}"
            fi
        elif command -v pkg-config >/dev/null 2>&1; then
            # Linux static build
            MISSING_LIBS=()
            CGO_CFLAGS_PARTS=()
            CGO_LDFLAGS_PARTS=()
            
            # Check for liblzma (XZ)
            if pkg-config --exists liblzma; then
                CGO_CFLAGS_PARTS+=("$(pkg-config --cflags liblzma)")
                CGO_LDFLAGS_PARTS+=("$(pkg-config --libs --static liblzma)")
            else
                MISSING_LIBS+=("liblzma-dev")
            fi
            
            # Check for libbrotli
            if pkg-config --exists libbrotlidec; then
                CGO_CFLAGS_PARTS+=("$(pkg-config --cflags libbrotlidec)")
                CGO_LDFLAGS_PARTS+=("$(pkg-config --libs --static libbrotlidec)")
            else
                MISSING_LIBS+=("libbrotli-dev")
            fi
            
            if [[ ${#MISSING_LIBS[@]} -eq 0 ]]; then
                echo "🔗 Building fully statically linked binary..."
                STATIC_LDFLAGS="${CGO_LDFLAGS_PARTS[*]} -Wl,--allow-multiple-definition"
                CGO_ENABLED=1 \
                CGO_CFLAGS="${CGO_CFLAGS_PARTS[*]}" \
                CGO_LDFLAGS="$STATIC_LDFLAGS" \
                go build -a -ldflags="${LDFLAGS} -linkmode external -extldflags \"-static\"" -o "$OUTPUT_DIR/payload-dumper-cgo-static" cmd/payload-dumper/*.go 2>/dev/null && \
                echo "✅ Static CGO version built: $OUTPUT_DIR/payload-dumper-cgo-static" || \
                echo "⚠️  Full static linking failed (normal on some systems)"
            else
                echo "⚠️  Static CGO version not built: missing ${MISSING_LIBS[*]}"
            fi
        else
            echo "⚠️  Static CGO version not built: liblzma and/or brotli development files not found"
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
