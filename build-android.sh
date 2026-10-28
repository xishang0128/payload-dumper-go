#!/bin/bash

# Android ARM64 cross-compilation script for payload-dumper-go
# Usage: ./build-android.sh [clean]

set -e

BUILD_TYPE="${1:-build}"
OUTPUT_DIR="./bin"
ANDROID_LIBS_DIR="${ANDROID_LIBS_DIR:-/Users/atri/git/other/android-arm64}"

# Embed version and build time into binaries
VERSION="${VERSION:-dev}"
BUILDTIME="${BUILDTIME:-$(date -u +%Y-%m-%dT%H:%M:%SZ)}"

# Android NDK configuration
export ANDROID_NDK_ROOT="/Users/atri/Library/Android/sdk/ndk/29.0.14033849"
export TOOLCHAIN="$ANDROID_NDK_ROOT/toolchains/llvm/prebuilt/darwin-x86_64"
export TARGET="aarch64-linux-android35"

# Cross-compilation environment
export CGO_ENABLED=1
export GOOS=android
export GOARCH=arm64
export CC="$TOOLCHAIN/bin/$TARGET-clang"
export CXX="$TOOLCHAIN/bin/$TARGET-clang++"
export AR="$TOOLCHAIN/bin/llvm-ar"
export RANLIB="$TOOLCHAIN/bin/llvm-ranlib"
export STRIP="$TOOLCHAIN/bin/llvm-strip"

# CGO flags for Android ARM64 libraries
export CGO_CFLAGS="-I$ANDROID_LIBS_DIR/include"
export CGO_LDFLAGS="-L$ANDROID_LIBS_DIR/lib -llzma -lbrotlidec -lbrotlienc -lbrotlicommon -lbz2 -lzstd -Wl,-rpath,/data/data/com.termux/files/usr/lib"

# Common ldflags to embed version and build time into the binary
LDFLAGS="-s -w -linkmode external -extldflags '-Wl,-rpath,/data/data/com.termux/files/usr/lib' -X 'github.com/xishang0128/payload-dumper-go/constant.Version=${VERSION}' -X 'github.com/xishang0128/payload-dumper-go/constant.BuildTime=${BUILDTIME}'"

mkdir -p "$OUTPUT_DIR"

case "$BUILD_TYPE" in
    "clean")
        echo "🧹 Cleaning Android builds..."
        rm -f "$OUTPUT_DIR/payload-dumper-android-arm64"*
        echo "✅ Cleanup completed!"
        ;;
    "build"|*)
        echo "🔨 Building payload-dumper-go for Android ARM64..."
        echo ""
        echo "Environment:"
        echo "  TARGET: $TARGET"
        echo "  CC: $CC"
        echo "  ANDROID_LIBS: $ANDROID_LIBS_DIR"
        echo "  VERSION: $VERSION"
        echo "  BUILDTIME: $BUILDTIME"
        echo ""

        # Verify required libraries exist
        echo "🔍 Verifying required libraries..."
        MISSING_LIBS=()
        
        # Check header files
        if [[ ! -f "$ANDROID_LIBS_DIR/include/lzma.h" ]]; then
            MISSING_LIBS+=("lzma.h (XZ)")
        fi
        if [[ ! -f "$ANDROID_LIBS_DIR/include/brotli/decode.h" ]]; then
            MISSING_LIBS+=("brotli headers")
        fi
        if [[ ! -f "$ANDROID_LIBS_DIR/include/bzlib.h" ]]; then
            MISSING_LIBS+=("bzlib.h (bzip2)")
        fi
        if [[ ! -f "$ANDROID_LIBS_DIR/include/zstd.h" ]]; then
            MISSING_LIBS+=("zstd.h")
        fi
        
        # Check library files
        if [[ ! -f "$ANDROID_LIBS_DIR/lib/liblzma.so" ]]; then
            MISSING_LIBS+=("liblzma.so")
        fi
        if [[ ! -f "$ANDROID_LIBS_DIR/lib/libbrotlidec.so" ]]; then
            MISSING_LIBS+=("libbrotlidec.so")
        fi
        if [[ ! -f "$ANDROID_LIBS_DIR/lib/libzstd.so" ]]; then
            MISSING_LIBS+=("libzstd.so")
        fi
        
        if [[ ${#MISSING_LIBS[@]} -gt 0 ]]; then
            echo "❌ Missing required libraries:"
            for lib in "${MISSING_LIBS[@]}"; do
                echo "   - $lib"
            done
            echo ""
            echo "Please ensure all Android ARM64 libraries are compiled and placed in:"
            echo "  Headers: $ANDROID_LIBS_DIR/include/"
            echo "  Libraries: $ANDROID_LIBS_DIR/lib/"
            exit 1
        fi
        
        echo "✅ All required libraries found!"
        echo ""

        PKG_CONFIG_DIR="$ANDROID_LIBS_DIR/lib/pkgconfig"
        mkdir -p "$PKG_CONFIG_DIR"
        for library in brotlicommon brotlidec brotlienc; do
            cat > "$PKG_CONFIG_DIR/lib${library}.pc" <<EOF
prefix=$ANDROID_LIBS_DIR
exec_prefix=\${prefix}
libdir=\${prefix}/lib
includedir=\${prefix}/include

Name: lib${library}
Description: Android ARM64 Brotli ${library} library
Version: 1
Libs: -L\${libdir} -l${library}
Cflags: -I\${includedir}
EOF
        done
        export PKG_CONFIG_PATH="$PKG_CONFIG_DIR${PKG_CONFIG_PATH:+:$PKG_CONFIG_PATH}"
        
        # Build the Android ARM64 binary
        echo "📦 Building with CGO (Android ARM64 native libraries)..."
        go build -a -ldflags="$LDFLAGS" -o "$OUTPUT_DIR/payload-dumper-android-arm64" cmd/payload-dumper/*.go
        
        if [[ $? -eq 0 ]]; then
            echo "✅ Android ARM64 build successful!"
            echo ""
            
            # Show binary information
            echo "🔍 Binary Information:"
            ls -lh "$OUTPUT_DIR/payload-dumper-android-arm64"
            echo ""
            
            if command -v file >/dev/null 2>&1; then
                echo "File type:"
                file "$OUTPUT_DIR/payload-dumper-android-arm64"
                echo ""
            fi
            
            # Test version output
            echo "Version information:"
            if "$OUTPUT_DIR/payload-dumper-android-arm64" version 2>/dev/null; then
                echo "✅ Binary can execute and show version"
            else
                echo "⚠️  Version check failed (normal on non-Android host)"
            fi
            
            echo ""
            echo "🎉 Build completed successfully!"
            echo ""
            echo "Usage on Android device:"
            echo "  1. Copy the binary to your Android device"
            echo "  2. Make it executable: chmod +x payload-dumper-android-arm64"
            echo "  3. Run: ./payload-dumper-android-arm64 [options] <payload.bin>"
            echo ""
            echo "Note: This binary requires Android ARM64 and the libraries to be"
            echo "      available in the system or Termux environment."
            
        else
            echo "❌ Build failed!"
            exit 1
        fi
        ;;
esac

echo ""
echo "Available binaries in $OUTPUT_DIR:"
ls -la "$OUTPUT_DIR/"payload-dumper* 2>/dev/null || echo "No payload-dumper binaries found"
