# ShadowTLS 快速开始指南

## 快速配置

### 最简配置（客户端）

```json
{
  "outbounds": [{
    "protocol": "vmess",
    "settings": {
      "vnext": [{
        "address": "your-server.com",
        "port": 443,
        "users": [{"id": "your-uuid"}]
      }]
    },
    "streamSettings": {
      "network": "shadowtls",
      "shadowtlsSettings": {
        "version": 3,
        "password": "your-password",
        "handshakeServer": "bing.com"
      }
    }
  }]
}
```

### 最简配置（服务端）

```json
{
  "inbounds": [{
    "port": 443,
    "protocol": "vmess",
    "settings": {
      "clients": [{"id": "your-uuid"}]
    },
    "streamSettings": {
      "network": "shadowtls",
      "shadowtlsSettings": {
        "version": 3,
        "password": "your-password",
        "handshakeServer": "bing.com"
      }
    }
  }]
}
```

## 配置步骤

### 1. 选择要模仿的服务器

选择一个稳定、可访问的知名网站作为 `handshakeServer`：
- ✅ 推荐：`bing.com`, `google.com`, `cloudflare.com`, `microsoft.com`
- ❌ 避免：被墙的网站、不稳定的网站

### 2. 设置密码

使用强密码作为 ShadowTLS 的认证密码：
```bash
# 生成随机密码
openssl rand -base64 32
```

### 3. 选择协议版本

- **v1**：最基础，无密码认证（不推荐）
- **v2**：添加密码认证
- **v3**：支持多用户，最安全（推荐）

### 4. 配置应用层协议

ShadowTLS 可以与任何协议结合：

#### VMess
```json
"protocol": "vmess"
```

#### VLESS
```json
"protocol": "vless"
```

#### Trojan
```json
"protocol": "trojan"
```

#### Shadowsocks
```json
"protocol": "shadowsocks"
```

## 常见问题

### Q: 为什么连接失败？
A: 检查以下几点：
1. `handshakeServer` 是否可访问
2. 密码是否匹配
3. 端口是否正确
4. 防火墙是否开放

### Q: 为什么速度慢？
A: ShadowTLS 需要额外的 TLS 握手，首次连接会较慢。后续连接会更快。

### Q: 可以不设置 handshakePort 吗？
A: 可以，默认使用 443 端口。

### Q: strictMode 是什么？
A: 严格模式会进行更严格的验证，提高安全性但可能影响兼容性。

## 完整示例

查看 `/examples` 目录下的完整配置示例：
- `shadowtls-vmess-client.json` - VMess 客户端配置
- `shadowtls-vmess-server.json` - VMess 服务端配置

## 测试连接

### 1. 启动服务端
```bash
./v2ray run -c examples/shadowtls-vmess-server.json
```

### 2. 启动客户端
```bash
./v2ray run -c examples/shadowtls-vmess-client.json
```

### 3. 测试连接
```bash
curl -x socks5://127.0.0.1:1080 https://www.google.com
```

## 进阶配置

### 多用户配置（服务端）

```json
{
  "streamSettings": {
    "network": "shadowtls",
    "shadowtlsSettings": {
      "version": 3,
      "users": [
        {"name": "alice", "password": "alice-pass"},
        {"name": "bob", "password": "bob-pass"}
      ],
      "handshakeServer": "bing.com",
      "strictMode": true
    }
  }
}
```

### 与其他传输层对比

| 特性 | ShadowTLS | WebSocket | gRPC |
|------|-----------|-----------|------|
| 伪装性 | ⭐⭐⭐⭐⭐ | ⭐⭐⭐ | ⭐⭐⭐⭐ |
| 性能 | ⭐⭐⭐ | ⭐⭐⭐⭐ | ⭐⭐⭐ |
| 配置复杂度 | ⭐⭐ | ⭐ | ⭐⭐ |
| 抗检测能力 | ⭐⭐⭐⭐⭐ | ⭐⭐⭐ | ⭐⭐⭐⭐ |

## 故障排除

### 日志级别
设置详细日志以便调试：
```json
{
  "log": {
    "loglevel": "debug"
  }
}
```

### 常见错误

#### "failed to dial TCP connection"
- 检查服务器地址和端口
- 检查网络连接

#### "failed to create ShadowTLS client"
- 检查配置格式
- 检查 handshakeServer 是否有效

#### "failed to establish ShadowTLS connection"
- 检查密码是否匹配
- 检查协议版本是否一致
- 检查 handshakeServer 是否可访问

## 性能优化建议

1. **使用 CDN 友好的 handshakeServer**
   - 选择有 CDN 的网站
   - 确保低延迟

2. **启用连接复用**
   - 在应用层协议配置中启用

3. **调整超时设置**
   - 根据网络情况调整

## 安全建议

1. ✅ 使用 v3 版本
2. ✅ 使用强密码（至少 32 字符）
3. ✅ 定期更换密码
4. ✅ 启用 strictMode（生产环境）
5. ✅ 配合其他安全措施使用

## 更多信息

- 完整文档：`README.md`
- 协议规范：[ShadowTLS GitHub](https://github.com/ihciah/shadow-tls)
- 实现总结：`/SHADOWTLS_IMPLEMENTATION_SUMMARY.md`
