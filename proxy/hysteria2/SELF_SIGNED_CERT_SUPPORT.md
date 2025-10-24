# Hysteria2 自签名证书支持分析

## 概述

Hysteria2 协议基于 QUIC 和 HTTP/3，因此需要 TLS 证书来建立安全连接。本文档分析了 v2ray-core 中 hysteria2 实现对自签名证书的支持情况。

## 自签名证书支持情况

### ✅ 完全支持自签名证书

v2ray-core 的 hysteria2 实现完全支持自签名证书，通过以下机制实现：

### 1. TLS 配置继承

**文件**: `transport/internet/hysteria2/dialer.go`

```go
func GetClientTLSConfig(dest net.Destination, streamSettings *internet.MemoryStreamConfig) (*hyClient.TLSConfig, error) {
    config := tls.ConfigFromStreamSettings(streamSettings)
    if config == nil {
        return nil, newError(Hy2MustNeedTLS)
    }
    tlsConfig := config.GetTLSConfig(tls.WithDestination(dest))

    return &hyClient.TLSConfig{
        RootCAs:               tlsConfig.RootCAs,
        ServerName:            tlsConfig.ServerName,
        InsecureSkipVerify:    tlsConfig.InsecureSkipVerify,  // 关键：支持跳过证书验证
        VerifyPeerCertificate: tlsConfig.VerifyPeerCertificate,
    }, nil
}
```

### 2. 自签名证书配置选项

**文件**: `transport/internet/tls/config.proto`

```protobuf
message Config {
  // 是否允许不安全的证书（自签名证书）
  bool allow_insecure = 1;
  
  // 证书固定哈希（替代 allow_insecure）
  repeated bytes pinned_peer_certificate_chain_sha256 = 7;
  
  // 当使用证书固定时是否允许不安全证书
  bool allow_insecure_if_pinned_peer_certificate = 11;
}
```

### 3. 证书验证逻辑

**文件**: `transport/internet/tls/config.go`

```go
func (c *Config) GetTLSConfig(opts ...Option) *tls.Config {
    config := &tls.Config{
        RootCAs:                root,
        InsecureSkipVerify:     c.AllowInsecure,  // 自签名证书支持
        VerifyPeerCertificate:  c.verifyPeerCert,
    }
    
    // 证书固定支持
    if c.AllowInsecureIfPinnedPeerCertificate && c.PinnedPeerCertificateChainSha256 != nil {
        config.InsecureSkipVerify = true
    }
}
```

## 配置示例

### 1. 基础自签名证书配置

#### 客户端配置
```json
{
  "outbounds": [
    {
      "protocol": "hysteria2",
      "settings": {
        "servers": [
          {
            "address": "192.168.1.100",
            "port": 443,
            "users": [{"password": "your-password"}]
          }
        ]
      },
      "streamSettings": {
        "security": "tls",
        "tlsSettings": {
          "allowInsecure": true,
          "serverName": "localhost"
        }
      }
    }
  ]
}
```

#### 服务器配置
```json
{
  "inbounds": [
    {
      "protocol": "hysteria2",
      "settings": {
        "password": "your-password"
      },
      "streamSettings": {
        "security": "tls",
        "tlsSettings": {
          "certificates": [
            {
              "certificate": ["-----BEGIN CERTIFICATE-----\n...\n-----END CERTIFICATE-----"],
              "key": ["-----BEGIN PRIVATE KEY-----\n...\n-----END PRIVATE KEY-----"]
            }
          ]
        }
      }
    }
  ]
}
```

### 2. 证书固定配置（推荐）

#### 客户端配置
```json
{
  "outbounds": [
    {
      "protocol": "hysteria2",
      "settings": {
        "servers": [
          {
            "address": "192.168.1.100",
            "port": 443,
            "users": [{"password": "your-password"}]
          }
        ]
      },
      "streamSettings": {
        "security": "tls",
        "tlsSettings": {
          "allowInsecure": false,
          "pinnedPeerCertificateChainSha256": [
          "sha256_hash_of_your_certificate"
          ],
          "allowInsecureIfPinnedPeerCertificate": true,
          "serverName": "localhost"
        }
      }
    }
  ]
}
```

### 3. 自定义 CA 证书配置

#### 客户端配置
```json
{
  "outbounds": [
    {
      "protocol": "hysteria2",
      "settings": {
        "servers": [
          {
            "address": "192.168.1.100",
            "port": 443,
            "users": [{"password": "your-password"}]
          }
        ]
      },
      "streamSettings": {
        "security": "tls",
        "tlsSettings": {
          "allowInsecure": false,
          "certificates": [
            {
              "certificate": ["-----BEGIN CERTIFICATE-----\n...\n-----END CERTIFICATE-----"],
              "key": ["-----BEGIN PRIVATE KEY-----\n...\n-----END PRIVATE KEY-----"],
              "usage": "AUTHORITY_VERIFY"
            }
          ],
          "serverName": "localhost"
        }
      }
    }
  ]
}
```

## 自签名证书生成

### 1. 使用 OpenSSL 生成自签名证书

```bash
# 生成私钥
openssl genrsa -out server.key 2048

# 生成自签名证书
openssl req -new -x509 -key server.key -out server.crt -days 365 -subj "/C=CN/ST=State/L=City/O=Organization/CN=localhost"

# 转换为 PEM 格式
openssl x509 -in server.crt -out server.pem -outform PEM
```

### 2. 使用 mkcert 生成本地信任证书

```bash
# 安装 mkcert
curl -JLO "https://dl.filippo.io/mkcert/latest?for=linux/amd64"
chmod +x mkcert-v*-linux-amd64
sudo mv mkcert-v*-linux-amd64 /usr/local/bin/mkcert

# 安装本地 CA
mkcert -install

# 生成证书
mkcert localhost 192.168.1.100 ::1
```

## 安全考虑

### 1. 生产环境建议

- **避免使用 `allowInsecure: true`**
- **使用证书固定** (`pinnedPeerCertificateChainSha256`)
- **使用自定义 CA 证书**
- **定期轮换证书**

### 2. 开发/测试环境

- **可以使用 `allowInsecure: true`**
- **确保网络环境安全**
- **仅用于内网测试**

### 3. 证书管理最佳实践

```json
{
  "tlsSettings": {
    "allowInsecure": false,
    "pinnedPeerCertificateChainSha256": [
      "sha256_hash_of_trusted_certificate"
    ],
    "allowInsecureIfPinnedPeerCertificate": true,
    "serverName": "your-server-domain"
  }
}
```

## 故障排除

### 1. 常见错误

#### 证书验证失败
```
Error: x509: certificate signed by unknown authority
```
**解决方案**: 设置 `allowInsecure: true` 或添加正确的 CA 证书

#### 主机名不匹配
```
Error: x509: cannot validate certificate for 192.168.1.100 because it doesn't contain any IP SANs
```
**解决方案**: 设置正确的 `serverName` 或生成包含 IP 地址的证书

### 2. 调试方法

#### 启用详细日志
```json
{
  "log": {
    "loglevel": "debug"
  }
}
```

#### 验证证书
```bash
# 检查证书信息
openssl x509 -in server.crt -text -noout

# 验证证书链
openssl verify -CAfile ca.crt server.crt
```

## 性能影响

### 1. 自签名证书 vs CA 签名证书

| 特性 | 自签名证书 | CA 签名证书 |
|------|------------|-------------|
| 连接建立时间 | 相同 | 相同 |
| 加密强度 | 相同 | 相同 |
| 验证开销 | 相同 | 相同 |
| 信任建立 | 手动配置 | 自动信任 |

### 2. 证书固定性能

- **首次连接**: 需要计算证书哈希
- **后续连接**: 哈希比较，开销极小
- **内存使用**: 增加证书存储

## 总结

v2ray-core 的 hysteria2 实现对自签名证书提供了**完整支持**：

### ✅ 支持的功能
- 跳过证书验证 (`allowInsecure`)
- 证书固定验证 (`pinnedPeerCertificateChainSha256`)
- 自定义 CA 证书
- 灵活的证书配置

### 🔧 配置灵活性
- 多种验证方式
- 细粒度控制
- 向后兼容

### 🛡️ 安全特性
- 证书固定防止中间人攻击
- 自定义 CA 支持
- 灵活的信任模型

这使得 hysteria2 协议在开发、测试和生产环境中都能灵活使用自签名证书，同时保持适当的安全级别。
