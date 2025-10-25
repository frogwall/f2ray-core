#!/bin/bash

# iOS Framework 编译脚本
# 使用 gomobile 编译 f2ray-core 为 iOS Framework

set -e

# 颜色输出
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

echo_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

echo_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# 检查环境
check_environment() {
    echo_info "检查编译环境..."
    
    # 检查 Go
    if ! command -v go &> /dev/null; then
        echo_error "Go 未安装，请先安装 Go"
        exit 1
    fi
    echo_info "Go 版本: $(go version)"
    
    # 检查 gomobile
    if ! command -v gomobile &> /dev/null; then
        echo_warn "gomobile 未安装，正在安装..."
        go install golang.org/x/mobile/cmd/gomobile@latest
        go install golang.org/x/mobile/cmd/gobind@latest
        gomobile init
    fi
    echo_info "gomobile 已安装"
    
    # 检查 Xcode
    if ! command -v xcodebuild &> /dev/null; then
        echo_error "Xcode 未安装，请先安装 Xcode"
        exit 1
    fi
    echo_info "Xcode 版本: $(xcodebuild -version | head -n 1)"
}

# 清理旧文件
clean_old_files() {
    echo_info "清理旧文件..."
    rm -rf F2Ray.xcframework
    rm -rf V2Ray.xcframework
    rm -rf build/ios
}

# 编译 Framework
build_framework() {
    local TARGET=$1
    local OUTPUT=$2
    
    echo_info "开始编译 iOS Framework..."
    echo_info "目标平台: $TARGET"
    echo_info "输出文件: $OUTPUT"
    
    # 确保 mobile 目录存在
    if [ ! -d "mobile" ]; then
        echo_error "mobile 目录不存在，请先创建 mobile/mobile.go"
        exit 1
    fi
    
    # 编译
    gomobile bind -v \
        -target="$TARGET" \
        -o "$OUTPUT" \
        -ldflags="-s -w -X github.com/frogwall/f2ray-core/v5.build=release" \
        -trimpath \
        ./mobile
    
    if [ $? -eq 0 ]; then
        echo_info "✅ 编译成功!"
    else
        echo_error "❌ 编译失败"
        exit 1
    fi
}

# 显示文件信息
show_info() {
    local OUTPUT=$1
    
    if [ -d "$OUTPUT" ]; then
        echo_info "Framework 信息:"
        echo "  路径: $(pwd)/$OUTPUT"
        echo "  大小: $(du -sh "$OUTPUT" | cut -f1)"
        
        # 显示架构信息
        if [ -f "$OUTPUT/ios-arm64/F2Ray.framework/F2Ray" ]; then
            echo "  iOS 架构: $(lipo -info "$OUTPUT/ios-arm64/F2Ray.framework/F2Ray" 2>/dev/null || echo "N/A")"
        fi
        
        if [ -f "$OUTPUT/ios-arm64_x86_64-simulator/F2Ray.framework/F2Ray" ]; then
            echo "  模拟器架构: $(lipo -info "$OUTPUT/ios-arm64_x86_64-simulator/F2Ray.framework/F2Ray" 2>/dev/null || echo "N/A")"
        fi
    fi
}

# 主函数
main() {
    echo_info "=========================================="
    echo_info "  F2Ray iOS Framework 编译脚本"
    echo_info "=========================================="
    echo ""
    
    # 检查环境
    check_environment
    echo ""
    
    # 清理
    clean_old_files
    echo ""
    
    # 解析参数
    TARGET="ios,iossimulator"
    OUTPUT="F2Ray.xcframework"
    
    while [[ $# -gt 0 ]]; do
        case $1 in
            --target)
                TARGET="$2"
                shift 2
                ;;
            --output)
                OUTPUT="$2"
                shift 2
                ;;
            --with-macos)
                TARGET="ios,iossimulator,macos"
                shift
                ;;
            --with-catalyst)
                TARGET="ios,iossimulator,macos,maccatalyst"
                shift
                ;;
            --help)
                echo "用法: $0 [选项]"
                echo ""
                echo "选项:"
                echo "  --target <platforms>    指定目标平台 (默认: ios,iossimulator)"
                echo "  --output <file>         指定输出文件 (默认: F2Ray.xcframework)"
                echo "  --with-macos            包含 macOS 支持"
                echo "  --with-catalyst         包含 Mac Catalyst 支持"
                echo "  --help                  显示此帮助信息"
                echo ""
                echo "示例:"
                echo "  $0                                    # 编译 iOS 和模拟器"
                echo "  $0 --with-macos                       # 包含 macOS"
                echo "  $0 --target ios,iossimulator          # 指定平台"
                exit 0
                ;;
            *)
                echo_error "未知选项: $1"
                echo "使用 --help 查看帮助"
                exit 1
                ;;
        esac
    done
    
    # 编译
    build_framework "$TARGET" "$OUTPUT"
    echo ""
    
    # 显示信息
    show_info "$OUTPUT"
    echo ""
    
    echo_info "=========================================="
    echo_info "  编译完成!"
    echo_info "=========================================="
    echo ""
    echo_info "下一步:"
    echo "  1. 将 $OUTPUT 拖入 Xcode 项目"
    echo "  2. 在 General → Frameworks 中设置为 Embed & Sign"
    echo "  3. 在代码中 import F2Ray 使用"
}

# 运行主函数
main "$@"
