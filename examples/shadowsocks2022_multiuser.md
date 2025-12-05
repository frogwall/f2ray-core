# Shadowsocks-2022 多用户配置示例

## 配置说明

Shadowsocks-2022 支持两种模式：
1. **单用户模式**：向后兼容，使用服务器 PSK
2. **多用户模式**：每个用户有独立的 PSK，通过 EIH 识别

## 多用户配置

### 服务器端配置

```json
{
  "inbounds": [{
    "protocol": "shadowsocks",
    "port": 8388,
    "settings": {
      "method": "2022-blake3-aes-128-gcm",
      "password": "YWJjZGVmZ2hpamtsbW5vcA==",
      "users": [
        {
          "password": "dXNlcjFwc2sxMjM0NTY3OA==",
          "email": "user1@example.com",
          "level": 0
        },
        {
          "password": "dXNlcjJwc2sxMjM0NTY3OA==",
          "email": "user2@example.com",
          "level": 1
        },
        {
          "password": "dXNlcjNwc2sxMjM0NTY3OA==",
          "email": "user3@example.com",
          "level": 0
        }
      ]
    }
  }]
}
```

### 客户端配置

每个用户需要配置服务器 PSK 和自己的用户 PSK：

```json
{
  "outbounds": [{
    "protocol": "shadowsocks",
    "settings": {
      "servers": [{
        "address": "server.example.com",
        "port": 8388,
        "method": "2022-blake3-aes-128-gcm",
        "password": "YWJjZGVmZ2hpamtsbW5vcA==:dXNlcjFwc2sxMjM0NTY3OA=="
      }]
    }
  }]
}
```

**注意**：客户端密码格式为 `服务器PSK:用户PSK`

## 单用户配置（向后兼容）

### 服务器端

```json
{
  "inbounds": [{
    "protocol": "shadowsocks",
    "port": 8388,
    "settings": {
      "method": "2022-blake3-aes-128-gcm",
      "password": "YWJjZGVmZ2hpamtsbW5vcA==",
      "email": "user@example.com",
      "level": 0
    }
  }]
}
```

### 客户端

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

## 密钥生成

### AES-128-GCM (16字节)

```bash
# 生成服务器 PSK
openssl rand -base64 16
# 输出示例: YWJjZGVmZ2hpamtsbW5vcA==

# 生成用户 PSK
openssl rand -base64 16
# 输出示例: dXNlcjFwc2sxMjM0NTY3OA==
```

### AES-256-GCM (32字节)

```bash
# 生成服务器 PSK
openssl rand -base64 32
# 输出示例: YWJjZGVmZ2hpamtsbW5vcHFyc3R1dnd4eXoxMjM0NTY3OA==

# 生成用户 PSK
openssl rand -base64 32
# 输出示例: dXNlcjFwc2sxMjM0NTY3ODkwYWJjZGVmZ2hpamtsbW5vcA==
```

## 配置字段说明

### 服务器配置

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `method` | string | 是 | 加密方法 |
| `password` | string | 是 | 服务器 PSK (base64) |
| `users` | array | 否 | 用户列表（多用户模式） |
| `email` | string | 否 | 用户标识（单用户模式） |
| `level` | number | 否 | 用户等级（单用户模式） |

### 用户配置

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `password` | string | 是 | 用户 PSK (base64) |
| `email` | string | 是 | 用户标识，用于日志和统计 |
| `level` | number | 否 | 用户等级，影响策略，默认 0 |

## 多用户优势

1. **用户隔离**：每个用户独立的 PSK
2. **流量统计**：按用户统计流量
3. **用户管理**：可以单独禁用某个用户
4. **安全性**：用户 PSK 泄露不影响其他用户

## 注意事项

1. **密钥长度**：
   - AES-128-GCM: 16 字节
   - AES-256-GCM: 32 字节

2. **Base64 编码**：所有 PSK 必须 base64 编码

3. **客户端兼容性**：需要支持 Shadowsocks-2022 EIH 的客户端
   - shadowsocks-rust ✅
   - sing-box ✅
   - 传统 Shadowsocks 客户端 ❌

4. **时间同步**：服务器和客户端时间误差不超过 30 秒

## 故障排查

### 用户认证失败

```
Error: user lookup failed
```

**原因**：用户 PSK 不在服务器配置中

**解决**：检查用户 PSK 是否正确配置在服务器的 `users` 列表中

### 密钥长度错误

```
Error: invalid user PSK length for user1@example.com: expected 16 bytes, got 24
```

**原因**：PSK 长度不匹配加密方法

**解决**：
- AES-128-GCM 使用 16 字节密钥
- AES-256-GCM 使用 32 字节密钥

### EIH 解码失败

```
Error: failed to decode EIH
```

**原因**：客户端 PSK 配置错误或不支持 EIH

**解决**：
1. 确认客户端支持 Shadowsocks-2022 EIH
2. 检查客户端密码格式：`服务器PSK:用户PSK`
3. 确认两个 PSK 都正确
