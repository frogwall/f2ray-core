# AnyTLS Protocol Support

AnyTLS is a protocol that provides TLS-based proxy functionality with session management and padding support.

## Features

- **TLS-based tunneling**: All traffic is encrypted using TLS
- **Session management**: Automatic idle session cleanup and connection pooling
- **Padding support**: Built-in traffic obfuscation
- **TCP and UDP support**: Full support for both protocols via UoT (UDP over TCP)

## Configuration

### Outbound Configuration

```json
{
  "protocol": "anytls",
  "settings": {
    "servers": [
      {
        "address": "example.com",
        "port": 443,
        "password": "your-password-here"
      }
    ],
    "idle_session_check_interval": 30,
    "idle_session_timeout": 30,
    "min_idle_session": 5
  },
  "streamSettings": {
    "network": "tcp",
    "security": "tls",
    "tlsSettings": {
      "serverName": "example.com",
      "allowInsecure": false
    }
  }
}
```

### Configuration Fields

#### servers (required)

Array of AnyTLS server endpoints. Each server object contains:

- **address** (string, required): Server hostname or IP address
- **port** (number, required): Server port (typically 443 for HTTPS)
- **password** (string, required): Authentication password

#### idle_session_check_interval (optional)

Interval in seconds for checking idle sessions. Default: 30 seconds.

#### idle_session_timeout (optional)

Timeout in seconds for closing idle sessions. Default: 30 seconds.

#### min_idle_session (optional)

Minimum number of idle sessions to keep open. Default: 0.

### Stream Settings

AnyTLS requires TLS to be configured in `streamSettings`:

```json
{
  "streamSettings": {
    "network": "tcp",
    "security": "tls",
    "tlsSettings": {
      "serverName": "example.com",
      "allowInsecure": false,
      "alpn": ["h2", "http/1.1"]
    }
  }
}
```

## Complete Example

```json
{
  "outbounds": [
    {
      "tag": "anytls-out",
      "protocol": "anytls",
      "settings": {
        "servers": [
          {
            "address": "proxy.example.com",
            "port": 443,
            "password": "8JCsPssfgS8tiRwiMlhARg=="
          }
        ],
        "idle_session_check_interval": 30,
        "idle_session_timeout": 30,
        "min_idle_session": 5
      },
      "streamSettings": {
        "network": "tcp",
        "security": "tls",
        "tlsSettings": {
          "serverName": "proxy.example.com",
          "allowInsecure": false,
          "alpn": ["h2", "http/1.1"],
          "fingerprint": "chrome"
        }
      }
    }
  ]
}
```

## Implementation Details

This implementation is based on the [sing-anytls](https://github.com/anytls/sing-anytls) library and provides:

1. **Automatic session management**: The client maintains a pool of TLS connections to the server
2. **Connection multiplexing**: Multiple proxy connections can share the same TLS session
3. **Padding**: Built-in traffic padding to resist traffic analysis
4. **UDP support**: UDP traffic is tunneled over TCP using UoT protocol

## Compatibility

- Compatible with AnyTLS servers implementing the same protocol
- Requires TLS 1.2 or higher
- Supports both IPv4 and IPv6
- Works with domain names and IP addresses

## Security Considerations

1. Always use strong passwords for authentication
2. Enable TLS certificate verification in production (`allowInsecure: false`)
3. Use appropriate TLS fingerprints to match your use case
4. Consider using ALPN to negotiate HTTP/2 for better performance

## Performance Tuning

- **idle_session_check_interval**: Lower values provide faster cleanup but more overhead
- **idle_session_timeout**: Balance between connection reuse and resource usage
- **min_idle_session**: Keep some connections warm for faster initial requests

## Troubleshooting

### Connection Fails

- Verify server address and port are correct
- Check that TLS settings match server requirements
- Ensure password is correct
- Verify firewall allows outbound connections on the specified port

### Performance Issues

- Increase `min_idle_session` to keep more connections ready
- Adjust timeout values based on your network conditions
- Consider using HTTP/2 ALPN for better multiplexing

## Related Protocols

- **Naive**: Similar TLS-based protocol with HTTP/2 CONNECT
- **Trojan**: Another TLS-based protocol with different characteristics
- **VLESS**: Lightweight protocol with optional TLS
