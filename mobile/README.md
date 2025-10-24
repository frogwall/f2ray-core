# V2Ray Mobile Library

v2ray-core 的移动端封装，支持 iOS 和 Android 平台。

## 📦 目录结构

```
mobile/
├── README.md              # 本文档
├── mobile.go              # Go 移动端接口
├── example_swift.md       # Swift 使用示例
└── example_kotlin.md      # Kotlin 使用示例（待添加）
```

## 🚀 快速开始

### iOS

#### 1. 编译 Framework

```bash
# 使用提供的脚本
./scripts/build_ios_framework.sh

# 或手动编译
gomobile bind -v \
  -target=ios,iossimulator \
  -o V2Ray.xcframework \
  -ldflags="-s -w" \
  ./mobile
```

#### 2. 集成到 Xcode 项目

1. 将 `V2Ray.xcframework` 拖入项目
2. 在 **General** → **Frameworks, Libraries, and Embedded Content** 中设置为 **Embed & Sign**
3. 在代码中导入：`import V2Ray`

#### 3. 使用示例

```swift
import V2Ray

// 启动 V2Ray
let config = """
{
  "inbounds": [...],
  "outbounds": [...]
}
"""

do {
    let instance = try MobileStartV2Ray(config)
    print("V2Ray 启动成功")
    
    // 停止
    try instance.stop()
} catch {
    print("错误: \(error)")
}
```

详细示例请查看 [example_swift.md](example_swift.md)

### Android

#### 1. 编译 AAR

```bash
# 使用 gomobile
gomobile bind -v \
  -target=android \
  -o v2ray.aar \
  -ldflags="-s -w" \
  ./mobile
```

#### 2. 集成到 Android 项目

1. 将 `v2ray.aar` 复制到 `app/libs/`
2. 在 `build.gradle` 中添加依赖：
```gradle
dependencies {
    implementation files('libs/v2ray.aar')
}
```

3. 在代码中使用：
```kotlin
import mobile.Mobile

val config = """
{
  "inbounds": [...],
  "outbounds": [...]
}
"""

try {
    val instance = Mobile.startV2Ray(config)
    println("V2Ray 启动成功")
    
    // 停止
    instance.stop()
} catch (e: Exception) {
    println("错误: ${e.message}")
}
```

## 📚 API 文档

### StartV2Ray

启动 V2Ray 实例

```go
func StartV2Ray(configJSON string) (*V2RayInstance, error)
```

**参数:**
- `configJSON`: JSON 格式的配置字符串

**返回:**
- `*V2RayInstance`: V2Ray 实例
- `error`: 错误信息

**示例:**
```swift
let instance = try MobileStartV2Ray(configJSON)
```

### V2RayInstance.Stop

停止 V2Ray 实例

```go
func (v *V2RayInstance) Stop() error
```

**返回:**
- `error`: 错误信息

**示例:**
```swift
try instance.stop()
```

### GetVersion

获取 V2Ray 版本

```go
func GetVersion() string
```

**返回:**
- `string`: 版本字符串

**示例:**
```swift
let version = MobileGetVersion()
print("V2Ray \(version)")
```

### TestConfig

测试配置是否有效

```go
func TestConfig(configJSON string) string
```

**参数:**
- `configJSON`: JSON 格式的配置字符串

**返回:**
- `string`: 空字符串表示成功，否则返回错误信息

**示例:**
```swift
let error = MobileTestConfig(configJSON)
if error.isEmpty {
    print("配置有效")
} else {
    print("配置错误: \(error)")
}
```

### V2RayInstance.QueryStats

查询统计信息

```go
func (v *V2RayInstance) QueryStats(pattern string) (string, error)
```

**参数:**
- `pattern`: 查询模式

**返回:**
- `string`: JSON 格式的统计信息
- `error`: 错误信息

**示例:**
```swift
let stats = try instance.queryStats("")
print("统计: \(stats)")
```

## 🎯 支持的协议

本 fork 包含以下增强协议：

### 1. Naive Protocol
HTTP/2 CONNECT tunnel with uTLS fingerprinting

```json
{
  "protocol": "naive",
  "settings": {
    "address": "server.example.com",
    "port": 443,
    "username": "user",
    "password": "pass"
  }
}
```

### 2. Hysteria2 Protocol
High-performance QUIC-based proxy

```json
{
  "protocol": "hysteria2",
  "settings": {
    "servers": [{
      "address": "server.example.com",
      "port": 443,
      "password": "your_password"
    }]
  }
}
```

### 3. Mieru Protocol
XChaCha20-Poly1305 encrypted proxy

```json
{
  "protocol": "mieru",
  "settings": {
    "servers": [{
      "address": "server.example.com",
      "port": 8964,
      "password": "your_password"
    }]
  }
}
```

### 4. Brook Protocol
Multi-transport proxy (TCP/WebSocket/QUIC)

```json
{
  "protocol": "brook",
  "settings": {
    "servers": [{
      "address": "server.example.com",
      "port": 9999,
      "password": "your_password",
      "method": "tcp"
    }]
  }
}
```

## 🔧 编译选项

### 减小二进制大小

```bash
gomobile bind -v \
  -target=ios \
  -o V2Ray.xcframework \
  -ldflags="-s -w -X github.com/v2fly/v2ray-core/v5.build=release" \
  -trimpath \
  ./mobile
```

参数说明：
- `-s`: 去除符号表
- `-w`: 去除 DWARF 调试信息
- `-trimpath`: 去除文件路径信息
- `-X`: 设置编译时变量

### 支持多平台

```bash
# iOS + iOS Simulator
gomobile bind -target=ios,iossimulator ...

# iOS + iOS Simulator + macOS
gomobile bind -target=ios,iossimulator,macos ...

# iOS + iOS Simulator + macOS + Mac Catalyst
gomobile bind -target=ios,iossimulator,macos,maccatalyst ...
```

## 🐛 常见问题

### 1. gomobile 找不到

```bash
export PATH=$PATH:$(go env GOPATH)/bin
```

### 2. iOS SDK 找不到

```bash
xcode-select -p
sudo xcode-select --switch /Applications/Xcode.app/Contents/Developer
```

### 3. 编译失败：cgo 错误

```bash
xcode-select --install
```

### 4. Framework 太大

使用 strip 减小大小：
```bash
strip -x V2Ray.xcframework/ios-arm64/V2Ray.framework/V2Ray
```

### 5. 内存占用过高

- 优化配置，减少并发连接数
- 使用更轻量的协议
- 定期重启实例

## 📖 更多资源

- [完整编译指南](../BUILD_IOS_FRAMEWORK.md)
- [Swift 使用示例](example_swift.md)
- [v2ray-core 文档](https://www.v2fly.org)
- [gomobile 文档](https://pkg.go.dev/golang.org/x/mobile/cmd/gomobile)

## 🤝 贡献

欢迎提交 Issue 和 Pull Request！

## 📄 许可证

MIT License - 详见 [LICENSE](../LICENSE)

---

**最后更新**: 2025-10-18  
**适用版本**: v2ray-core v5.x (F2Ray enhanced edition)
