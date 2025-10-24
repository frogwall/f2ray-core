# 编译 iOS 可用的链接库

本文档说明如何为 iOS 平台编译 v2ray-core 的动态链接库（Framework）。

## 📋 前提条件

### 1. 安装 Go 环境
```bash
# 确保 Go 版本 >= 1.20
go version
```

### 2. 安装 gomobile
```bash
# 安装 gomobile 工具
go install golang.org/x/mobile/cmd/gomobile@latest
go install golang.org/x/mobile/cmd/gobind@latest

# 初始化 gomobile
gomobile init
```

### 3. 安装 Xcode
- 需要安装完整的 Xcode（不是 Command Line Tools）
- 需要安装 iOS SDK 和 iOS Simulator Runtime

## 🔧 方法一：使用 gomobile（推荐）

### 特点
- ✅ 支持 iOS、iOS Simulator、macOS、Mac Catalyst
- ✅ 不会与其他框架冲突
- ✅ 适合大多数场景
- ⚠️ 需要 iOS Simulator Runtime
- ⚠️ 无法设置最低 macOS 版本（可能有编译警告）
- ⚠️ 不支持 tvOS

### 步骤

#### 1. 创建 mobile 包装层

创建文件 `mobile/mobile.go`：

```go
package mobile

import (
	"context"
	"fmt"
	"io"

	core "github.com/v2fly/v2ray-core/v5"
	"github.com/v2fly/v2ray-core/v5/common/serial"
	"github.com/v2fly/v2ray-core/v5/infra/conf"
	_ "github.com/v2fly/v2ray-core/v5/main/distro/all"
)

// V2RayInstance 表示一个 V2Ray 实例
type V2RayInstance struct {
	instance *core.Instance
}

// StartV2Ray 使用 JSON 配置启动 V2Ray
func StartV2Ray(configJSON string) (*V2RayInstance, error) {
	// 解析 JSON 配置
	config, err := serial.LoadJSONConfig([]byte(configJSON))
	if err != nil {
		return nil, fmt.Errorf("failed to parse config: %v", err)
	}

	// 创建 V2Ray 实例
	instance, err := core.New(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create instance: %v", err)
	}

	// 启动实例
	if err := instance.Start(); err != nil {
		return nil, fmt.Errorf("failed to start instance: %v", err)
	}

	return &V2RayInstance{instance: instance}, nil
}

// Stop 停止 V2Ray 实例
func (v *V2RayInstance) Stop() error {
	if v.instance != nil {
		return v.instance.Close()
	}
	return nil
}

// GetVersion 获取 V2Ray 版本
func GetVersion() string {
	return core.Version()
}

// TestConfig 测试配置是否有效
func TestConfig(configJSON string) error {
	_, err := serial.LoadJSONConfig([]byte(configJSON))
	return err
}
```

#### 2. 编译 iOS Framework

```bash
# 进入项目目录
cd /home/xxxx/Work/v2ray-core

# 编译 iOS Framework（支持 iOS 和 iOS Simulator）
gomobile bind -v \
  -target=ios \
  -o V2Ray.xcframework \
  -ldflags="-s -w" \
  ./mobile

# 如果需要支持 macOS 和 Mac Catalyst
gomobile bind -v \
  -target=ios,iossimulator,macos,maccatalyst \
  -o V2Ray.xcframework \
  -ldflags="-s -w" \
  ./mobile
```

#### 3. 编译参数说明

- `-target`: 目标平台
  - `ios`: iOS 设备（arm64）
  - `iossimulator`: iOS 模拟器（x86_64, arm64）
  - `macos`: macOS（amd64, arm64）
  - `maccatalyst`: Mac Catalyst
- `-o`: 输出文件名
- `-ldflags="-s -w"`: 减小二进制大小
  - `-s`: 去除符号表
  - `-w`: 去除 DWARF 调试信息
- `-v`: 显示详细编译信息

## 🔧 方法二：使用 CGO（高级）

### 特点
- ✅ 支持更多编译选项
- ✅ 可以输出 C 头文件
- ✅ 支持 iOS、iOS Simulator、macOS、tvOS
- ✅ 适合 FFI 集成（Swift、Kotlin、Dart）
- ⚠️ 需要 iOS Simulator Runtime 和 tvOS Simulator Runtime
- ⚠️ 生成的 xcframework 不包含 module.modulemap
- ⚠️ Swift 使用时需要创建桥接文件

### 步骤

#### 1. 创建 C 导出接口

创建文件 `mobile/export.go`：

```go
package mobile

/*
#include <stdlib.h>
*/
import "C"
import (
	"unsafe"
)

//export V2RayStart
func V2RayStart(configJSON *C.char) *C.char {
	config := C.GoString(configJSON)
	instance, err := StartV2Ray(config)
	if err != nil {
		return C.CString(err.Error())
	}
	// 保存实例引用
	return nil
}

//export V2RayStop
func V2RayStop() {
	// 停止实例
}

//export V2RayVersion
func V2RayVersion() *C.char {
	return C.CString(GetVersion())
}

//export V2RayFreeString
func V2RayFreeString(str *C.char) {
	C.free(unsafe.Pointer(str))
}
```

#### 2. 编译脚本

创建 `build_ios_cgo.sh`：

```bash
#!/bin/bash

set -e

PROJECT_DIR=$(pwd)
OUTPUT_DIR="$PROJECT_DIR/build/ios"
FRAMEWORK_NAME="V2Ray"

# 清理旧文件
rm -rf "$OUTPUT_DIR"
mkdir -p "$OUTPUT_DIR"

# iOS 设备 (arm64)
CGO_ENABLED=1 \
GOOS=ios \
GOARCH=arm64 \
SDK=iphoneos \
CC=$(xcrun --sdk iphoneos --find clang) \
CGO_CFLAGS="-arch arm64 -mios-version-min=12.0 -isysroot $(xcrun --sdk iphoneos --show-sdk-path)" \
CGO_LDFLAGS="-arch arm64 -mios-version-min=12.0 -isysroot $(xcrun --sdk iphoneos --show-sdk-path)" \
go build -buildmode=c-archive \
  -ldflags="-s -w" \
  -o "$OUTPUT_DIR/ios-arm64.a" \
  ./mobile

# iOS 模拟器 (arm64)
CGO_ENABLED=1 \
GOOS=ios \
GOARCH=arm64 \
SDK=iphonesimulator \
CC=$(xcrun --sdk iphonesimulator --find clang) \
CGO_CFLAGS="-arch arm64 -mios-simulator-version-min=12.0 -isysroot $(xcrun --sdk iphonesimulator --show-sdk-path)" \
CGO_LDFLAGS="-arch arm64 -mios-simulator-version-min=12.0 -isysroot $(xcrun --sdk iphonesimulator --show-sdk-path)" \
go build -buildmode=c-archive \
  -ldflags="-s -w" \
  -o "$OUTPUT_DIR/iossimulator-arm64.a" \
  ./mobile

# iOS 模拟器 (x86_64)
CGO_ENABLED=1 \
GOOS=ios \
GOARCH=amd64 \
SDK=iphonesimulator \
CC=$(xcrun --sdk iphonesimulator --find clang) \
CGO_CFLAGS="-arch x86_64 -mios-simulator-version-min=12.0 -isysroot $(xcrun --sdk iphonesimulator --show-sdk-path)" \
CGO_LDFLAGS="-arch x86_64 -mios-simulator-version-min=12.0 -isysroot $(xcrun --sdk iphonesimulator --show-sdk-path)" \
go build -buildmode=c-archive \
  -ldflags="-s -w" \
  -o "$OUTPUT_DIR/iossimulator-x86_64.a" \
  ./mobile

# 合并模拟器架构
lipo -create \
  "$OUTPUT_DIR/iossimulator-arm64.a" \
  "$OUTPUT_DIR/iossimulator-x86_64.a" \
  -output "$OUTPUT_DIR/iossimulator.a"

# 创建 xcframework
xcodebuild -create-xcframework \
  -library "$OUTPUT_DIR/ios-arm64.a" \
  -headers "$OUTPUT_DIR" \
  -library "$OUTPUT_DIR/iossimulator.a" \
  -headers "$OUTPUT_DIR" \
  -output "$OUTPUT_DIR/$FRAMEWORK_NAME.xcframework"

echo "✅ Framework 编译完成: $OUTPUT_DIR/$FRAMEWORK_NAME.xcframework"
```

运行编译：
```bash
chmod +x build_ios_cgo.sh
./build_ios_cgo.sh
```

## 📦 使用编译好的 Framework

### 在 Xcode 项目中使用

1. 将 `V2Ray.xcframework` 拖入 Xcode 项目
2. 在 **General** → **Frameworks, Libraries, and Embedded Content** 中添加
3. 设置为 **Embed & Sign**

### Swift 代码示例

```swift
import V2Ray

class V2RayManager {
    private var instance: V2RayInstance?
    
    func start(config: String) throws {
        instance = try MobileStartV2Ray(config)
    }
    
    func stop() {
        try? instance?.stop()
        instance = nil
    }
    
    func getVersion() -> String {
        return MobileGetVersion()
    }
}
```

## 🎯 针对本项目的特殊说明

### 增强协议支持

本 fork 包含以下增强协议，编译时会自动包含：

- **Naive Protocol** - HTTP/2 CONNECT with uTLS
- **Hysteria2 Protocol** - QUIC-based proxy
- **Mieru Protocol** - XChaCha20-Poly1305 encrypted
- **Brook Protocol** - Multi-transport proxy

### 依赖处理

确保所有依赖都已正确安装：

```bash
# 更新依赖
go mod tidy
go mod download

# 验证依赖
go mod verify
```

### 减小二进制大小

```bash
# 使用更激进的优化
gomobile bind -v \
  -target=ios \
  -o V2Ray.xcframework \
  -ldflags="-s -w -X github.com/v2fly/v2ray-core/v5.build=release" \
  -trimpath \
  ./mobile
```

## 🐛 常见问题

### 1. gomobile 找不到

```bash
# 确保 GOPATH/bin 在 PATH 中
export PATH=$PATH:$(go env GOPATH)/bin
```

### 2. iOS SDK 找不到

```bash
# 检查 Xcode 路径
xcode-select -p

# 如果不正确，重新设置
sudo xcode-select --switch /Applications/Xcode.app/Contents/Developer
```

### 3. 编译失败：cgo 错误

```bash
# 确保安装了 Command Line Tools
xcode-select --install
```

### 4. Framework 太大

```bash
# 使用 strip 减小大小
strip -x V2Ray.xcframework/ios-arm64/V2Ray.framework/V2Ray
```

### 5. 模拟器无法运行

确保编译时包含了模拟器架构：
```bash
gomobile bind -target=ios,iossimulator ...
```

## 📚 参考资源

- [gomobile 官方文档](https://pkg.go.dev/golang.org/x/mobile/cmd/gomobile)
- [XTLS/libXray](https://github.com/XTLS/libXray) - Xray-core 的移动端封装
- [v2ray-core discussions](https://github.com/v2fly/v2ray-core/discussions/2882)

## 🔄 自动化构建

可以创建 GitHub Actions 工作流自动构建：

```yaml
name: Build iOS Framework

on:
  push:
    tags:
      - 'v*'

jobs:
  build:
    runs-on: macos-latest
    steps:
      - uses: actions/checkout@v3
      
      - name: Setup Go
        uses: actions/setup-go@v4
        with:
          go-version: '1.21'
      
      - name: Install gomobile
        run: |
          go install golang.org/x/mobile/cmd/gomobile@latest
          go install golang.org/x/mobile/cmd/gobind@latest
          gomobile init
      
      - name: Build Framework
        run: |
          gomobile bind -v \
            -target=ios,iossimulator \
            -o V2Ray.xcframework \
            -ldflags="-s -w" \
            ./mobile
      
      - name: Upload Framework
        uses: actions/upload-artifact@v3
        with:
          name: V2Ray.xcframework
          path: V2Ray.xcframework
```

---

**最后更新**: 2025-10-18
**适用版本**: v2ray-core v5.x (F2Ray enhanced edition)
