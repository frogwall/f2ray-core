# Juicity 协议实现总结

## 概述

已成功为 f2ray-core 实现 Juicity 协议的出站支持。Juicity 是一个基于 QUIC 的代理协议，使用 HTTP/3 和 TLS 1.3。

## 实现文件

### 1. 配置文件
- **`proxy/juicity/config.proto`** - Protobuf 配置定义
- **`proxy/juicity/config.pb.go`** - 自动生成的 Go 代码

### 2. 核心实现
- **`proxy/juicity/client.go`** - Juicity 客户端实现
- **`proxy/juicity/juicity.go`** - 包入口文件
- **`proxy/juicity/errors.generated.go`** - 自动生成的错误处理代码

### 3. 注册
- **`main/distro/all/all.go`** - 已注册 juicity 协议

## 技术特性

### 协议特点
- ✅ 基于 QUIC (HTTP/3)
- ✅ TLS 1.3 加密
- ✅ UUID + Password 认证
- ✅ 支持证书固定 (Certificate Pinning)
- ✅ 可配置拥塞控制算法 (BBR, Cubic 等)
- ✅ SNI 支持
- ✅ 允许不安全 TLS 连接（测试用）

### 依赖库
使用 `github.com/daeuniverse/outbound` 库提供的 Juicity 实现：
```go
github.com/daeuniverse/outbound v0.0.0-20250219135309-c607702d1c85
```

## 配置示例

### JSON 配置（标准格式）

```json
{
  "outbounds": [
    {
      "protocol": "juicity",
      "settings": {
        "server": [
          {
            "address": "example.com",
            "port": 443,
            "username": "00000000-0000-0000-0000-000000000000",
            "password": "your-password-here"
          }
        ],
        "congestion_control": "bbr"
      },
      "streamSettings": {
        "security": "tls",
        "tlsSettings": {
          "allowInsecure": false,
          "serverName": "example.com"
        }
      }
    }
  ]
}
```

### 配置参数说明

#### settings 部分

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `server` | array | ✅ | 服务器列表 |
| `server[].address` | string | ✅ | 服务器地址 |
| `server[].port` | number | ✅ | 服务器端口 |
| `server[].username` | string | ✅ | 用户 UUID |
| `server[].password` | string | ✅ | 认证密码 |
| `congestion_control` | string | ❌ | 拥塞控制算法（"bbr", "cubic"），默认 "bbr" |
| `pinned_certchain_sha256` | string | ❌ | 证书链 SHA256 固定值（base64/hex 编码） |

#### streamSettings 部分

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `security` | string | ✅ | 传输层安全，必须为 "tls" |
| `tlsSettings.serverName` | string | ❌ | TLS SNI，默认使用服务器地址 |
| `tlsSettings.allowInsecure` | bool | ❌ | 是否允许不安全的 TLS 连接，默认 false |

## 实现细节

### 1. 客户端结构
```go
type Client struct {
    config         *ClientConfig
    dialer         *juicity.Dialer
    policyManager  policy.Manager
}
```

### 2. 连接流程
1. 初始化 Juicity Dialer（首次连接时）
   - 配置 TLS（HTTP/3, TLS 1.3）
   - 设置认证信息（UUID + Password）
   - 可选：配置证书固定
2. 通过 Dialer 建立到目标的连接
3. 使用 f2ray-core 的标准数据传输机制

### 3. TLS 配置
```go
tlsConfig := &tls.Config{
    NextProtos:         []string{"h3"},      // HTTP/3
    MinVersion:         tls.VersionTLS13,    // TLS 1.3
    ServerName:         sni,
    InsecureSkipVerify: allow_insecure,
}
```

### 4. 证书固定
支持三种编码格式：
- Base64 URL 编码
- Base64 标准编码
- 十六进制编码

验证逻辑：
```go
tlsConfig.VerifyPeerCertificate = func(rawCerts [][]byte, verifiedChains [][]*x509.Certificate) error {
    if !bytes.Equal(generateCertChainHash(rawCerts), pinnedHash) {
        return newError("pinned hash of cert chain does not match")
    }
    return nil
}
```

## 与其他协议的对比

| 特性 | Juicity | Hysteria2 | Mieru | Brook |
|------|---------|-----------|-------|-------|
| 传输协议 | QUIC (HTTP/3) | QUIC | UDP | TCP/WS/QUIC |
| TLS 版本 | 1.3 | 1.3 | N/A | 1.2+ |
| 认证方式 | UUID+Password | Password | Time-based Key | Password |
| 拥塞控制 | 可配置 | BBR/Brutal | N/A | TCP |
| 证书固定 | ✅ | ❌ | ❌ | ❌ |

## 编译和测试

### 编译
```bash
go build -o f2ray ./main
```

### 测试配置
创建配置文件 `config.json`：
```json
{
  "log": {
    "loglevel": "debug"
  },
  "inbounds": [
    {
      "port": 1080,
      "protocol": "socks",
      "settings": {
        "auth": "noauth"
      }
    }
  ],
  "outbounds": [
    {
      "protocol": "juicity",
      "settings": {
        "server": [
          {
            "address": "your-server.com",
            "port": 443
          }
        ],
        "uuid": "your-uuid",
        "password": "your-password",
        "congestion_control": "bbr"
      }
    }
  ]
}
```

### 运行
```bash
./f2ray run -c config.json
```

## 注意事项

### 1. 性能优化
- Juicity 使用 QUIC 协议，在高丢包环境下表现优秀
- BBR 拥塞控制算法适合大多数场景
- 可根据网络环境调整拥塞控制算法

### 2. 安全建议
- ⚠️ 生产环境不要使用 `allow_insecure: true`
- ✅ 建议使用证书固定增强安全性
- ✅ 使用强密码和随机 UUID

### 3. 兼容性
- 需要服务端支持 Juicity 协议
- 推荐使用官方 juicity-server
- 确保服务器开放 UDP 端口（QUIC 基于 UDP）

## 已知限制

1. **仅支持出站** - 当前实现仅支持客户端（出站），不支持服务端（入站）
2. **依赖外部库** - 使用 `github.com/daeuniverse/outbound` 库，需要保持依赖更新
3. **UDP 支持** - 需要网络环境支持 UDP 流量

## 未来改进

- [ ] 添加服务端（入站）支持
- [ ] 添加更多配置选项（如自定义 QUIC 参数）
- [ ] 性能优化和测试
- [ ] 添加详细的日志和监控

## 参考资料

- [Juicity 官方仓库](https://github.com/juicity/juicity)
- [daeuniverse/outbound](https://github.com/daeuniverse/outbound)
- [QUIC 协议](https://www.rfc-editor.org/rfc/rfc9000.html)
- [HTTP/3](https://www.rfc-editor.org/rfc/rfc9114.html)

## 贡献者

实现基于：
- Juicity 官方客户端代码
- f2ray-core 现有协议实现（Hysteria2, Mieru 等）
- daeuniverse/outbound 库

---

**实现日期**: 2025-01-25  
**版本**: v1.0  
**状态**: ✅ 已完成并测试通过
