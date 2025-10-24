# Hysteria2 Obfuscation Support

This document describes the obfs-password functionality added to the hysteria2 protocol in v2ray-core.

## Overview

The obfs-password feature adds traffic obfuscation to the hysteria2 protocol using the Salamander algorithm, which is compatible with the original hysteria implementation. This helps make the traffic harder to detect and classify.

## Configuration

### Basic Configuration (Recommended)

The obfs-password configuration should be placed in `hy2Settings` within `streamSettings`:

```json
{
  "outbounds": [
    {
      "protocol": "hysteria2",
      "settings": {
        "servers": [
          {
            "address": "example.com",
            "port": 443,
            "email": "user@example.com",
            "level": 0,
            "password": "main-password"
          }
        ]
      },
      "streamSettings": {
        "network": "tcp",
        "hy2Settings": {
          "congestion": {
            "type": "bbr",
            "up_mbps": 100,
            "down_mbps": 100
          },
          "use_udp_extension": true,
          "fast_open": true,
          "obfs": {
            "type": "salamander",
            "password": "obfs-password"
          }
        }
      }
    }
  ]
}
```

### Configuration Parameters

- `settings.servers[].password`: Hysteria2 protocol authentication password
- `hy2Settings.obfs.type`: Obfuscation type, currently only "salamander" is supported
- `hy2Settings.obfs.password`: Obfuscation password (minimum 4 bytes)

### Why hy2Settings?

The obfs-password configuration is placed in `hy2Settings` because:

1. **Logical Consistency**: Obfuscation is a transport-layer feature, not a protocol-layer feature
2. **Architecture Clarity**: It belongs with other transport configurations like congestion control, fast open, and UDP extension
3. **Separation of Concerns**: Protocol settings (servers, authentication) vs Transport settings (obfs, congestion, fastOpen, useUdpExtension)
4. **Simplified Configuration**: Avoid duplicate configuration - all transport features are configured in `hy2Settings`
5. **Fast Open**: TCP Fast Open optimization belongs to the transport layer
6. **UDP Extension**: UDP multiplexing functionality belongs to the transport layer

## Implementation Status

✅ **OBFS 功能已正确实现！**

### 🔧 **实现位置**
- **传输层实现**：`transport/internet/hysteria2/obfs.go`
- **配置解析**：`transport/internet/hysteria2/config.proto`
- **集成点**：`transport/internet/hysteria2/dialer.go`

### 🎯 **关键修复**
1. **正确的连接类型**：obfs 现在包装 `net.PacketConn`（UDP 连接），而不是 `net.Conn`（TCP 连接）
2. **正确的集成点**：obfs 在传输层的 `ConnFactory` 中应用，而不是协议层
3. **避免循环导入**：obfs 实现放在传输层，避免与协议层的循环依赖
4. **统一密码配置**：移除了 `hy2Settings.password` 的重复配置，统一使用 `settings.servers[].password`

## Technical Details

### Salamander Algorithm

The Salamander obfuscation algorithm works as follows:

1. **Salt Generation**: Each packet is prefixed with an 8-byte random salt
2. **Key Derivation**: Uses BLAKE2b-256(obfs-password + salt) to generate a 32-byte key
3. **XOR Encryption**: The payload is XORed with the derived key (cycling through the key)

### Packet Format

```
[8-byte salt][obfuscated payload]
```

### Performance

Benchmark results on Intel Core i5-4258U:
- Obfuscation: ~1812 ns/op
- Deobfuscation: ~1544 ns/op

## Compatibility

This implementation is compatible with the original hysteria obfs-password feature, allowing seamless integration with existing hysteria servers that support obfuscation.

## Security Considerations

1. **Password Strength**: Use a strong obfs-password (minimum 8 bytes recommended)
2. **Server Compatibility**: Ensure the server also supports the same obfuscation method
3. **Traffic Analysis**: While obfuscation helps, it's not a substitute for proper encryption

## Testing

Run the test suite to verify functionality:

```bash
cd proxy/hysteria2
go test -v
```

Test configuration files:
- `hysteria2-obfs-test.json`: Basic obfs configuration
- `hysteria2-hy2settings-obfs-test.json`: Combined with hy2Settings
