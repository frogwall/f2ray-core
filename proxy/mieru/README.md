# Mieru Protocol Integration for v2ray-core

This directory contains the implementation of the Mieru protocol as an outbound proxy for v2ray-core.

## Overview

Mieru is a secure, hard to classify, hard to probe, TCP or UDP protocol-based network proxy software. This integration allows v2ray-core to use Mieru as an outbound protocol.

## Features

- **Bypass v2ray transport layer**: Direct connection to Mieru servers
- **XChaCha20-Poly1305 encryption**: Strong encryption algorithm
- **Time-based key generation**: Keys generated based on username, password, and system time
- **Session management**: Full session lifecycle management
- **Error recovery**: Enhanced error handling and recovery mechanisms

## Configuration

### Basic Configuration

```json
{
  "outbounds": [
    {
      "protocol": "mieru",
      "settings": {
        "servers": [
          {
            "address": "example.com",
            "port": 443,
            "username": "user123",
            "password": "your-password"
          }
        ],
        "mtu": 1500
      },
      "streamSettings":{
        "network":"tcp"
      }
    }
  ]
}
```

### Configuration Parameters

- `servers`: Array of Mieru servers
  - `address`: Server address
  - `port`: Server port
  - `username`: Authentication username
  - `password`: Authentication password
- `mtu`: Maximum transmission unit (default: 1500)
- `streamSettings.network`: Transport protocol ("tcp" or "udp")

## Protocol Details

### Key Generation

The Mieru protocol uses a time-based key generation system:

1. **Hashed Password**: `SHA256(password + "\x00" + username)`
2. **Time Salt**: Current time rounded to nearest 2 minutes
3. **Key**: `PBKDF2(hashedPassword, timeSalt, 64, 32, SHA256)`

### Encryption

- **Algorithm**: XChaCha20-Poly1305
- **Key Size**: 32 bytes
- **Nonce Size**: 24 bytes
- **Overhead**: 16 bytes

### Session Management

- **Session States**: INIT, ATTACHED, ESTABLISHED, CLOSED
- **Session ID**: Unique identifier for each session
- **Session Timeout**: Configurable session timeout

## Implementation Notes

### Transport Layer Bypass

This implementation bypasses v2ray's transport layer to avoid conflicts:

- Direct system network connections
- No v2ray transport layer processing
- Direct Mieru protocol handling

### Error Handling

- **Connection Retry**: Automatic retry with exponential backoff
- **Key Tolerance**: Multiple key attempts for time synchronization
- **Session Recovery**: Automatic session recovery on errors

## Usage Example

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
        "auth": "noauth",
        "udp": true
      }
    }
  ],
  "outbounds": [
    {
      "protocol": "mieru",
      "settings": {
        "servers": [
          {
            "address": "server.example.com",
            "port": 443,
        
                "username": "user123",
                "password": "password123"
              
            ]
          }
        ],
        "mtu": 1500
      },
      "streamSettings": {
        "network": "tcp"
      }
    }
  ]
}
```

## Security Considerations

- **Time Synchronization**: Client and server time must be within 4 minutes
- **Key Rotation**: Keys are time-based and rotate every 2 minutes
- **Replay Protection**: Built-in replay attack detection
- **Padding**: Random padding to prevent traffic analysis

## Limitations

- **No Port Hopping**: Port hopping feature is not implemented in this version
- **Simple Implementation**: This is a simplified implementation focusing on basic functionality
- **No Advanced Features**: Advanced Mieru features like congestion control are not implemented

## Future Enhancements

- Port hopping support
- Advanced congestion control
- Multiple server support
- Load balancing
- Health checking

## Troubleshooting

### Common Issues

1. **Connection Failed**: Check server address and port
2. **Authentication Failed**: Verify username and password
3. **Time Sync Issues**: Ensure client and server time are synchronized
4. **Key Generation Failed**: Check system time and credentials

### Debug Mode

Enable debug logging to troubleshoot issues:

```json
{
  "log": {
    "loglevel": "debug"
  }
}
```

## Contributing

When contributing to this implementation:

1. Follow the existing code style
2. Add appropriate error handling
3. Include tests for new features
4. Update documentation

## License

This implementation is licensed under the GNU General Public License v3.0.
