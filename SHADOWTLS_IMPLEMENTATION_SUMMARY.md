# ShadowTLS 传输层协议实现总结

## 概述

成功将 ShadowTLS 作为通用传输层协议实现到 v2ray-core 中，可与任何应用层协议（VMess、VLESS、Trojan、Shadowsocks 等）结合使用。

## 实现状态

✅ **完全实现并编译通过**

## 创建的文件

### 核心实现
1. **`/transport/internet/shadowtls/config.proto`**
   - Protobuf 配置定义
   - 支持 v1/v2/v3 三个版本
   - 多用户配置支持

2. **`/transport/internet/shadowtls/config.pb.go`**
   - 自动生成的 Protobuf Go 代码

3. **`/transport/internet/shadowtls/shadowtls.go`**
   - 包初始化和常量定义
   - Logger 适配器实现（实现 sing 的 logger.ContextLogger 接口）

4. **`/transport/internet/shadowtls/errors.generated.go`**
   - 自动生成的错误处理代码

5. **`/transport/internet/shadowtls/dialer.go`**
   - 客户端拨号器实现
   - TLS 握手函数创建
   - v2ray dialer 包装器（实现 sing 的 N.Dialer 接口）
   - 支持 v1/v2/v3 协议版本

6. **`/transport/internet/shadowtls/hub.go`**
   - 服务端监听器实现
   - 连接处理逻辑
   - Handler 适配器（实现 sing 的 N.TCPConnectionHandlerEx 接口）

7. **`/transport/internet/shadowtls/README.md`**
   - 完整的使用文档
   - 配置示例
   - 参数说明

### 配置解析
8. **`/infra/conf/v4/shadowtls.go`**
   - JSON 配置结构定义
   - Build() 方法实现

### 注册和集成
9. **修改 `/infra/conf/v4/transport_internet.go`**
   - 添加 `ShadowTLSSettings` 字段到 `StreamConfig`
   - 添加 `shadowtls` 到 `TransportProtocol.Build()`
   - 添加 ShadowTLS 配置构建逻辑

10. **修改 `/main/distro/all/all.go`**
    - 注册 ShadowTLS 传输协议导入

## 依赖

添加到 `go.mod`：
```
github.com/sagernet/sing-shadowtls v0.2.0
```

## 配置格式

### 简化配置
```json
{
  "streamSettings": {
    "network": "shadowtls",
    "shadowtlsSettings": {
      "version": 3,
      "password": "xxx",
      "handshakeServer": "bing.com"
    }
  }
}
```

### 完整配置
```json
{
  "streamSettings": {
    "network": "shadowtls",
    "security": "none",
    "shadowtlsSettings": {
      "version": 3,
      "password": "your-password",
      "handshakeServer": "bing.com",
      "handshakePort": 443,
      "strictMode": true,
      "users": [
        {"name": "user1", "password": "pass1"},
        {"name": "user2", "password": "pass2"}
      ]
    }
  }
}
```

## 技术实现细节

### 1. 架构设计
- **传输层实现**：作为 `transport/internet` 的一部分
- **协议无关**：可与任何应用层协议结合
- **标准接口**：实现 v2ray 的 `internet.Connection` 接口

### 2. 关键组件

#### Dialer（客户端）
- 实现 `Dial(ctx, dest, streamSettings)` 函数
- 创建 v2ray dialer wrapper 适配 sing 的 N.Dialer 接口
- 根据版本创建不同的 TLS 握手函数
- 使用 `shadowtls.NewClient()` 创建客户端
- 调用 `client.DialContext()` 建立连接

#### Listener（服务端）
- 实现 `Listen(ctx, address, port, streamSettings, handler)` 函数
- 创建 `shadowtls.NewService()` 服务
- 实现 `shadowtlsHandler` 适配 v2ray 的连接处理
- 监听 TCP 端口并处理传入连接

#### Logger 适配器
- 实现 `logger.ContextLogger` 接口
- 包含所有必需方法：
  - `Trace/Debug/Info/Warn/Error/Fatal/Panic`
  - `TraceContext/DebugContext/InfoContext/WarnContext/ErrorContext/FatalContext/PanicContext`
- 将日志转发到 v2ray 的错误系统

#### Dialer Wrapper
- 实现 `N.Dialer` 接口
- `DialContext()`: 转换地址格式并调用 v2ray 的 `internet.DialSystem()`
- `ListenPacket()`: 返回不支持错误（ShadowTLS 仅支持 TCP）

### 3. 协议版本支持

#### v1
- TLS 1.2 only
- 无密码认证
- 基础握手逻辑

#### v2
- TLS 1.2/1.3
- 密码认证
- 改进的安全性

#### v3（推荐）
- TLS 1.2/1.3
- 多用户支持
- 严格模式
- 会话 ID 生成器
- 使用 `shadowtls.DefaultTLSHandshakeFunc()`

### 4. 与 sing-shadowtls 的集成

使用的 API：
- `shadowtls.NewClient(ClientConfig)` - 创建客户端
- `shadowtls.NewService(ServiceConfig)` - 创建服务端
- `shadowtls.DefaultTLSHandshakeFunc()` - v3 握手函数
- `shadowtls.TLSHandshakeFunc` - 握手函数类型
- `shadowtls.User` - 用户结构
- `shadowtls.HandshakeConfig` - 握手配置

## 使用场景

### 1. 与 VMess 结合
```json
{
  "protocol": "vmess",
  "streamSettings": {
    "network": "shadowtls",
    "shadowtlsSettings": {
      "version": 3,
      "password": "xxx",
      "handshakeServer": "google.com"
    }
  }
}
```

### 2. 与 VLESS 结合
```json
{
  "protocol": "vless",
  "streamSettings": {
    "network": "shadowtls",
    "shadowtlsSettings": {
      "version": 3,
      "password": "xxx",
      "handshakeServer": "bing.com"
    }
  }
}
```

### 3. 与 Trojan 结合
```json
{
  "protocol": "trojan",
  "streamSettings": {
    "network": "shadowtls",
    "shadowtlsSettings": {
      "version": 3,
      "password": "shadowtls-pass",
      "handshakeServer": "cloudflare.com"
    }
  }
}
```

### 4. 与 Shadowsocks 结合
```json
{
  "protocol": "shadowsocks",
  "streamSettings": {
    "network": "shadowtls",
    "shadowtlsSettings": {
      "version": 3,
      "password": "xxx",
      "handshakeServer": "microsoft.com"
    }
  }
}
```

## 优势

1. **通用性**：可与任何应用层协议结合
2. **伪装性**：通过真实 TLS 握手提供强大的流量伪装
3. **灵活性**：支持多版本、多用户
4. **标准化**：遵循 v2ray 传输层标准接口
5. **易用性**：配置简单直观

## 与其他实现的对比

### vs sing-box
- **相同点**：
  - 使用相同的 `sing-shadowtls` 库
  - 支持相同的协议版本
  - 配置参数类似

- **不同点**：
  - v2ray-core: 作为传输层实现，可与任何协议结合
  - sing-box: 使用 detour 机制，配置相对复杂

### vs 原始 ShadowTLS
- **优势**：
  - 集成到 v2ray 生态系统
  - 可与现有协议无缝结合
  - 统一的配置格式
  - 更好的可维护性

## 编译和测试

### 编译
```bash
go build -v -o v2ray ./main
```

### 测试配置
创建测试配置文件并运行：
```bash
./v2ray run -c config.json
```

## 注意事项

1. **handshakeServer 选择**：
   - 选择稳定、可访问的知名网站
   - 确保支持 TLS 1.2/1.3
   - 避免使用被墙的网站

2. **性能考虑**：
   - 首次连接需要额外的 TLS 握手
   - 建议配合连接复用使用

3. **安全建议**：
   - 推荐使用 v3 版本
   - 使用强密码
   - 定期更换密码

## 未来改进

1. **性能优化**：
   - 连接池管理
   - 握手缓存

2. **功能增强**：
   - 支持 uTLS 指纹伪装
   - 自定义握手服务器列表
   - 动态服务器选择

3. **监控和诊断**：
   - 连接统计
   - 握手成功率监控
   - 详细的调试日志

## 参考资料

- [ShadowTLS 协议规范](https://github.com/ihciah/shadow-tls)
- [sing-shadowtls 实现](https://github.com/sagernet/sing-shadowtls)
- [sing-box ShadowTLS 实现](https://github.com/SagerNet/sing-box/tree/main/protocol/shadowtls)

## 总结

ShadowTLS 传输层协议已成功实现并集成到 v2ray-core 中。作为一个通用的传输层协议，它可以与任何应用层协议结合使用，提供强大的流量伪装能力。实现遵循 v2ray 的标准架构，配置简单直观，易于使用和维护。

**实现状态：✅ 完成并通过编译**
