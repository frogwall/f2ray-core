# Shadowsocks-2022 入站配置示例

## 基本配置

### 使用 AES-128-GCM (推荐用于性能)

```json
{
  "inbounds": [{
    "protocol": "shadowsocks",
    "port": 8388,
    "settings": {
      "method": "2022-blake3-aes-128-gcm",
      "password": "YWJjZGVmZ2hpamtsbW5vcA==",
      "network": "tcp,udp"
    }
  }]
}
```

**密钥生成:**
```bash
# 生成 16 字节随机密钥并 base64 编码
openssl rand -base64 16
# 输出示例: YWJjZGVmZ2hpamtsbW5vcA==
```

### 使用 AES-256-GCM (推荐用于安全性)

```json
{
  "inbounds": [{
    "protocol": "shadowsocks",
    "port": 8388,
    "settings": {
      "method": "2022-blake3-aes-256-gcm",
      "password": "YWJjZGVmZ2hpamtsbW5vcHFyc3R1dnd4eXoxMjM0NTY=",
      "network": "tcp,udp"
    }
  }]
}
```

**密钥生成:**
```bash
# 生成 32 字节随机密钥并 base64 编码
openssl rand -base64 32
# 输出示例: YWJjZGVmZ2hpamtsbW5vcHFyc3R1dnd4eXoxMjM0NTY=
```

## 高级配置

### 仅 TCP

```json
{
  "inbounds": [{
    "protocol": "shadowsocks",
    "port": 8388,
    "settings": {
      "method": "2022-blake3-aes-128-gcm",
      "password": "YWJjZGVmZ2hpamtsbW5vcA==",
      "network": "tcp"
    }
  }]
}
```

### 启用 UDP 包地址编码

```json
{
  "inbounds": [{
    "protocol": "shadowsocks",
    "port": 8388,
    "settings": {
      "method": "2022-blake3-aes-128-gcm",
      "password": "YWJjZGVmZ2hpamtsbW5vcA==",
      "network": "tcp,udp",
      "packetEncoding": "Packet"
    }
  }]
}
```

### 带用户标识

```json
{
  "inbounds": [{
    "protocol": "shadowsocks",
    "port": 8388,
    "settings": {
      "method": "2022-blake3-aes-128-gcm",
      "password": "YWJjZGVmZ2hpamtsbW5vcA==",
      "email": "user@example.com",
      "level": 1
    }
  }]
}
```

## 配置字段说明

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `method` | string | 是 | 加密方法，支持 `2022-blake3-aes-128-gcm` 或 `2022-blake3-aes-256-gcm` |
| `password` | string | 是 | Base64 编码的密钥，AES-128 需要 16 字节，AES-256 需要 32 字节 |
| `network` | string | 否 | 支持的网络类型，可选 `tcp`、`udp` 或 `tcp,udp`，默认为 `tcp` |
| `packetEncoding` | string | 否 | UDP 包编码类型，可选 `None` 或 `Packet`，默认为 `None` |
| `email` | string | 否 | 用户标识，用于日志和统计 |
| `level` | number | 否 | 用户等级，影响策略应用，默认为 0 |

## 密钥要求

### AES-128-GCM
- **密钥长度**: 16 字节 (128 位)
- **Base64 编码后**: 约 24 个字符
- **生成命令**: `openssl rand -base64 16`

### AES-256-GCM
- **密钥长度**: 32 字节 (256 位)
- **Base64 编码后**: 约 44 个字符
- **生成命令**: `openssl rand -base64 32`

## 与传统 Shadowsocks 的区别

### Shadowsocks-2022 优势
1. ✅ **强制重放保护**: 使用时间戳验证，防止重放攻击
2. ✅ **BLAKE3 密钥派生**: 更安全的密钥派生算法
3. ✅ **固定长度密钥**: 必须使用正确长度的密钥，不再使用密码派生
4. ✅ **更好的性能**: 优化的 UDP 会话管理

### 配置差异
- **传统 Shadowsocks**: 可以使用任意长度的密码
- **Shadowsocks-2022**: 必须使用固定长度的 Base64 编码密钥

## 客户端配置示例

对应的客户端配置（出站）：

```json
{
  "outbounds": [{
    "protocol": "shadowsocks",
    "settings": {
      "servers": [{
        "address": "server.example.com",
        "port": 8388,
        "method": "2022-blake3-aes-128-gcm",
        "password": "YWJjZGVmZ2hpamtsbW5vcA=="
      }]
    }
  }]
}
```

## 兼容性

- ✅ 兼容标准 Shadowsocks-2022 客户端（shadowsocks-rust, sing-box 等）
- ✅ 支持 TCP 和 UDP 协议
- ✅ 支持多种包编码模式
- ⚠️ 不兼容传统 Shadowsocks 客户端（需使用 2022 版本客户端）

## 故障排查

### 密钥长度错误
```
Error: invalid PSK length for 2022-blake3-aes-128-gcm: expected 16 bytes, got 24
```
**解决**: 确保使用正确长度的密钥并正确进行 Base64 编码

### 时间戳验证失败
```
Error: timestamp is too far away, timeDifference = 45
```
**解决**: 确保服务器和客户端时间同步（误差不超过 30 秒）

### 未知加密方法
```
Error: unknown cipher method: 2022-blake3-aes-128-gcm
```
**解决**: 确保使用最新版本的 f2ray-core，旧版本不支持 Shadowsocks-2022
