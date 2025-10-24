# ShadowTLS Transport Layer

ShadowTLS 是一个传输层协议，通过模仿真实的 TLS 握手来伪装代理流量，使其难以被检测和阻断。

## 特性

- **TLS 流量伪装**：与真实服务器进行 TLS 握手，混淆代理流量
- **多版本支持**：支持 ShadowTLS v1、v2、v3 协议
- **协议无关**：可与任何应用层协议结合（VMess、VLESS、Trojan、Shadowsocks 等）
- **密码认证**：v2/v3 支持密码认证
- **多用户支持**：v3 支持多用户配置

## 配置示例

### 客户端配置

#### 基础配置（与 VMess 结合）

```json
{
  "outbounds": [
    {
      "protocol": "vmess",
      "settings": {
        "vnext": [{
          "address": "example.com",
          "port": 443,
          "users": [{"id": "uuid-here"}]
        }]
      },
      "streamSettings": {
        "network": "shadowtls",
        "security": "none",
        "shadowtlsSettings": {
          "version": 3,
          "password": "your-password",
          "handshakeServer": "bing.com",
          "handshakePort": 443
        }
      }
    }
  ]
}
```

#### 与 VLESS 结合

```json
{
  "outbounds": [
    {
      "protocol": "vless",
      "settings": {
        "vnext": [{
          "address": "example.com",
          "port": 443,
          "users": [{"id": "uuid-here", "encryption": "none"}]
        }]
      },
      "streamSettings": {
        "network": "shadowtls",
        "shadowtlsSettings": {
          "version": 3,
          "password": "your-password",
          "handshakeServer": "google.com"
        }
      }
    }
  ]
}
```

#### 与 Shadowsocks 结合

```json
{
  "outbounds": [
    {
      "protocol": "shadowsocks",
      "settings": {
        "servers": [{
          "address": "example.com",
          "port": 443,
          "method": "aes-256-gcm",
          "password": "ss-password"
        }]
      },
      "streamSettings": {
        "network": "shadowtls",
        "shadowtlsSettings": {
          "version": 3,
          "password": "shadowtls-password",
          "handshakeServer": "cloudflare.com"
        }
      }
    }
  ]
}
```

### 服务端配置

#### 基础服务端（与 VMess 结合）

```json
{
  "inbounds": [
    {
      "port": 443,
      "protocol": "vmess",
      "settings": {
        "clients": [{"id": "uuid-here"}]
      },
      "streamSettings": {
        "network": "shadowtls",
        "shadowtlsSettings": {
          "version": 3,
          "password": "your-password",
          "handshakeServer": "bing.com",
          "handshakePort": 443
        }
      }
    }
  ]
}
```

#### 多用户配置（v3）

```json
{
  "inbounds": [
    {
      "port": 443,
      "protocol": "vmess",
      "settings": {
        "clients": [
          {"id": "uuid-1"},
          {"id": "uuid-2"}
        ]
      },
      "streamSettings": {
        "network": "shadowtls",
        "shadowtlsSettings": {
          "version": 3,
          "users": [
            {"name": "user1", "password": "password1"},
            {"name": "user2", "password": "password2"}
          ],
          "handshakeServer": "google.com",
          "strictMode": true
        }
      }
    }
  ]
}
```

## 配置参数

### shadowtlsSettings

| 参数 | 类型 | 必填 | 默认值 | 说明 |
|------|------|------|--------|------|
| `version` | number | 否 | 3 | ShadowTLS 协议版本（1、2 或 3） |
| `password` | string | v2/v3 必填 | - | 认证密码 |
| `handshakeServer` | string | 是 | - | 用于 TLS 握手的真实服务器域名（如 google.com、bing.com） |
| `handshakePort` | number | 否 | 443 | 握手服务器端口 |
| `strictMode` | boolean | 否 | false | 严格模式（仅 v3） |
| `users` | array | 否 | - | 多用户配置（仅 v3，服务端） |

### users（仅 v3 服务端）

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `name` | string | 否 | 用户名 |
| `password` | string | 是 | 用户密码 |

## 协议版本说明

### v1
- 最基础的版本
- 仅支持 TLS 1.2
- 无密码认证

### v2
- 添加密码认证
- 支持 TLS 1.2 和 1.3
- 更好的安全性

### v3（推荐）
- 支持多用户
- 支持严格模式
- 改进的会话 ID 生成
- 最佳的安全性和灵活性

## 工作原理

1. **客户端**：
   - 连接到服务端
   - 通过 ShadowTLS 协议进行认证
   - 与 `serverName` 指定的真实服务器进行 TLS 握手（用于伪装）
   - 建立加密隧道
   - 在隧道内传输应用层协议数据（VMess/VLESS/Trojan 等）

2. **服务端**：
   - 监听指定端口
   - 验证 ShadowTLS 认证
   - 解包 ShadowTLS 层
   - 将数据传递给应用层协议处理器

## 注意事项

1. **handshakeServer 选择**：
   - 选择稳定、可访问的知名网站（如 google.com、bing.com、cloudflare.com）
   - 确保该网站支持 TLS 1.2/1.3
   - 避免使用被墙的网站

2. **端口说明**：
   - outbound 的 `address` 和 `port` 是 ShadowTLS 服务器地址
   - `handshakeServer` 和 `handshakePort` 是用于 TLS 握手伪装的真实服务器

3. **安全性**：
   - 推荐使用 v3 版本
   - 使用强密码
   - 定期更换密码
   - 启用 strictMode 提高安全性

4. **性能**：
   - ShadowTLS 会增加一次额外的 TLS 握手开销
   - 首次连接可能较慢
   - 建议配合连接复用使用

4. **与其他传输层的区别**：
   - **vs WebSocket**：ShadowTLS 流量特征更接近真实 TLS，更难检测
   - **vs gRPC**：ShadowTLS 不依赖 HTTP/2，更通用
   - **vs 原始 TLS**：ShadowTLS 通过与真实服务器握手提供额外的伪装层

## 依赖

- `github.com/sagernet/sing-shadowtls` - ShadowTLS 核心实现

## 参考

- [ShadowTLS v1/v2 协议规范](https://github.com/ihciah/shadow-tls/blob/master/docs/protocol-en.md)
- [ShadowTLS v3 协议规范](https://github.com/ihciah/shadow-tls/blob/master/docs/protocol-v3-en.md)
- [sing-shadowtls 实现](https://github.com/sagernet/sing-shadowtls)
