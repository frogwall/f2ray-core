# Brook 协议实现总结

## 🎯 项目概述

Brook 是一个跨平台的代理软件，支持多种传输方式 (TCP、WebSocket、QUIC)。本项目实现了 Brook 协议作为 v2ray-core 的出站代理，提供了完整的协议实现和多传输方式支持。

### 核心特性
1. **AES-GCM 加密**：使用 AES-256-GCM 认证加密算法
2. **HKDF 密钥派生**：基于 HKDF 的安全密钥派生机制
3. **多传输支持**：支持 TCP、WebSocket、QUIC 传输方式
4. **时间戳验证**：内置时间戳验证防止重放攻击
5. **分片传输**：支持大数据包的分片传输

## ✅ 实现状态

### 🚀 核心实现 (部分完成)
- **主要文件**: 
  - `proxy/brook/client.go` (客户端实现)
  - `proxy/brook/encryptor.go` (加密实现)
  - `proxy/brook/config.proto` (配置定义)
- **状态**: ⚠️ **部分实现，需要完善**
- **核心特性**:
  - ✅ **AES-GCM 加密**：完整的加密/解密实现
  - ✅ **HKDF 密钥派生**：安全的密钥生成机制
  - ✅ **TCP 传输**：基本的 TCP 协议实现
  - ⚠️ **WebSocket 传输**：框架实现，需要完善
  - ⚠️ **QUIC 传输**：框架实现，需要完善
  - ✅ **协议握手**：完整的握手流程
  - ⚠️ **错误处理**：基础错误处理机制

### 📋 配置系统 (完整)
- **协议注册**: 已注册到 v2ray-core 配置系统
- **配置格式**: 标准 JSON 配置
- **参数支持**: 服务器地址、端口、密码、传输方式

### 🔧 实现架构
- **多传输抽象**: 统一的 StreamClient 接口
- **加密分离**: 独立的加密器和解密器
- **协议兼容**: 与原版 Brook 协议兼容

## 🔧 技术实现细节

### 1. 加密系统
```go
// HKDF 密钥派生
func NewBrookEncryptor(password string) (*BrookEncryptor, error) {
    encryptor := &BrookEncryptor{
        password: []byte(password),
        nonce:    make([]byte, NonceSize),
    }
    
    // 生成随机 nonce
    if _, err := io.ReadFull(rand.Reader, encryptor.nonce); err != nil {
        return nil, err
    }
    
    // 使用 HKDF 派生密钥
    key := make([]byte, KeySize)
    _, err := hkdf.New(sha256.New, encryptor.password, encryptor.nonce, []byte(ClientHKDFInfo)).Read(key)
    if err != nil {
        return nil, err
    }
    
    // 创建 AES-GCM 密码器
    block, err := aes.NewCipher(key)
    if err != nil {
        return nil, err
    }
    
    encryptor.aead, err = cipher.NewGCM(block)
    return encryptor, err
}
```

### 2. TCP 协议实现
```go
// TCP 流客户端
func NewTCPStreamClient(conn internet.Connection, password string, request *protocol.RequestHeader) (*TCPStreamClient, error) {
    client := &TCPStreamClient{
        conn:     conn,
        password: []byte(password),
        request:  request,
    }
    
    // 初始化加密器
    encryptor, err := NewBrookEncryptor(password)
    if err != nil {
        return nil, err
    }
    client.encryptor = encryptor
    
    // 初始化解密器
    decryptor, err := NewBrookDecryptor(password)
    if err != nil {
        return nil, err
    }
    client.decryptor = decryptor
    
    // 发送初始请求
    if err := client.sendRequest(); err != nil {
        return nil, err
    }
    
    // 等待服务器 nonce 响应
    return client, client.waitForServerNonce()
}
```

### 3. 协议握手流程
```go
// 发送请求
func (c *TCPStreamClient) sendRequest() error {
    // 1. 发送客户端 nonce
    _, err := c.conn.Write(c.encryptor.nonce)
    if err != nil {
        return err
    }
    
    // 2. 构建目标地址
    var dst []byte
    if c.request.Address.Family().IsIP() {
        ip := c.request.Address.IP()
        dst = make([]byte, 0, 1+len(ip)+2)
        if len(ip) == 4 {
            dst = append(dst, byte(0x01)) // IPv4
        } else {
            dst = append(dst, byte(0x04)) // IPv6
        }
        dst = append(dst, ip...)
    } else {
        // 域名处理
        domain := c.request.Address.Domain()
        dst = make([]byte, 0, 1+1+len(domain)+2)
        dst = append(dst, byte(0x03))        // 域名类型
        dst = append(dst, byte(len(domain))) // 域名长度
        dst = append(dst, []byte(domain)...)
    }
    dst = append(dst, byte(c.request.Port>>8), byte(c.request.Port))
    
    // 3. 创建时间戳 (TCP 必须为偶数)
    timestamp := uint32(time.Now().Unix())
    if timestamp%2 != 0 {
        timestamp += 1
    }
    
    // 4. 构建请求数据: 时间戳 + 目标地址
    requestData := make([]byte, 4+len(dst))
    binary.BigEndian.PutUint32(requestData[:4], timestamp)
    copy(requestData[4:], dst)
    
    // 5. 发送加密的请求数据
    return c.sendBrookData(requestData)
}
```

### 4. 数据传输格式
```go
// Brook 协议数据格式
func (c *TCPStreamClient) sendBrookDataSingle(data []byte) error {
    // 创建 2048 字节缓冲区 (与 Brook 原版一致)
    buffer := make([]byte, 2048)
    
    // 1. 长度前缀 (2 字节)
    binary.BigEndian.PutUint16(buffer[:2], uint16(len(data)))
    
    // 2. 加密长度前缀
    c.encryptor.aead.Seal(buffer[:0], c.encryptor.nonce, buffer[:2], nil)
    c.encryptor.incrementNonce()
    
    // 3. 复制数据到缓冲区
    copy(buffer[2+16:2+16+len(data)], data)
    
    // 4. 加密数据
    c.encryptor.aead.Seal(buffer[2+16:2+16], c.encryptor.nonce, buffer[2+16:2+16+len(data)], nil)
    c.encryptor.incrementNonce()
    
    // 5. 发送: 加密长度 + 加密数据
    totalLength := 2 + 16 + len(data) + 16
    _, err := c.conn.Write(buffer[:totalLength])
    return err
}
```

### 5. 多传输方式支持
```go
// StreamClient 接口
type StreamClient interface {
    io.ReadWriteCloser
    buf.Reader
    buf.Writer
}

// 根据传输方式创建客户端
func (c *Client) handleConnection(ctx context.Context, conn internet.Connection, link *transport.Link, request *protocol.RequestHeader, account *Account, method string) error {
    var streamClient StreamClient
    var err error
    
    switch method {
    case "tcp":
        streamClient, err = NewTCPStreamClient(conn, account.Password, request)
    case "ws", "wss":
        streamClient, err = NewWSStreamClient(conn, account.Password, request, c.config)
    case "quic":
        streamClient, err = NewQUICStreamClient(conn, account.Password, request, c.config)
    default:
        return newError("unsupported method: " + method)
    }
    
    if err != nil {
        return newError("failed to create stream client").Base(err)
    }
    
    return c.exchangeData(ctx, link, streamClient)
}
```

## 📊 实现特点分析

### 🔍 协议特征

| 特征维度 | Brook 实现 | 检测难度 | 说明 |
|----------|------------|----------|------|
| **加密算法** | AES-256-GCM | ⭐⭐⭐⭐⭐ | 标准认证加密，安全可靠 |
| **密钥管理** | HKDF 派生 | ⭐⭐⭐⭐ | 基于密码的安全密钥派生 |
| **协议识别** | 自定义二进制协议 | ⭐⭐⭐⭐ | 无明显协议特征 |
| **握手过程** | Nonce 交换 + 时间戳 | ⭐⭐⭐ | 相对简单的握手流程 |
| **传输方式** | TCP/WebSocket/QUIC | ⭐⭐⭐⭐ | 多种传输方式支持 |

### 🛡️ 安全性评估

**优势**:
- ✅ **认证加密**: AES-GCM 提供机密性和完整性保护
- ✅ **安全密钥派生**: HKDF 确保密钥安全性
- ✅ **重放保护**: 时间戳机制防止重放攻击
- ✅ **多传输支持**: 灵活的传输方式选择
- ✅ **协议混淆**: 自定义协议格式难以识别

**注意事项**:
- ⚠️ **密码强度**: 安全性依赖于密码强度
- ⚠️ **时间同步**: 时间戳验证需要时间同步
- ⚠️ **实现完整性**: WebSocket 和 QUIC 实现需要完善
- ⚠️ **错误处理**: 需要更完善的错误处理机制

### 🎯 适用场景

**推荐使用**:
- 需要多传输方式支持的环境
- 对协议简洁性有要求的场景
- 有现有 Brook 服务端的环境
- 需要 WebSocket/QUIC 传输的场景

**谨慎使用**:
- 对安全性有极高要求的环境
- 网络环境不稳定的场景
- 需要复杂功能的应用

## 🚀 使用指南

### 📦 编译构建
```bash
# 编译 (包含 brook 协议)
go build -o v2ray ./main

# 验证编译
./v2ray version
```

### ⚙️ 配置示例

**TCP 传输配置**:
```json
{
  "outbounds": [
    {
      "protocol": "brook",
      "settings": {
        "servers": [
          {
            "address": "server.example.com",
            "port": 9999,
            "password": "your_password",
            "method": "tcp"
          }
        ]
      },
      "streamSettings": {
        "network": "tcp"
      }
    }
  ]
}
```

**WebSocket 传输配置**:
```json
{
  "outbounds": [
    {
      "protocol": "brook",
      "settings": {
        "servers": [
          {
            "address": "server.example.com",
            "port": 443,
            "password": "your_password",
            "method": "wss"
          }
        ],
        "path": "/brook",
        "tlsFingerprint": "chrome"
      },
      "streamSettings": {
        "network": "ws",
        "security": "tls"
      }
    }
  ]
}
```

**完整配置示例**:
```json
{
  "log": {
    "loglevel": "info"
  },
  "inbounds": [
    {
      "port": 1080,
      "protocol": "socks",
      "settings": {
        "auth": "noauth",
        "udp": true
      }
    },
    {
      "port": 1081,
      "protocol": "http"
    }
  ],
  "outbounds": [
    {
      "protocol": "brook",
      "settings": {
        "servers": [
          {
            "address": "your-brook-server.com",
            "port": 9999,
            "password": "your_strong_password",
            "method": "tcp"
          }
        ]
      },
      "streamSettings": {
        "network": "tcp"
      }
    }
  ]
}
```

### 🚀 启动运行
```bash
# 使用配置文件启动
./v2ray run -c config.json

# 后台运行
nohup ./v2ray run -c config.json > v2ray.log 2>&1 &
```

## ⚠️ 当前限制和已知问题

### 🚧 实现限制
1. **WebSocket 实现不完整**: WebSocket 握手和帧处理需要完善
2. **QUIC 实现不完整**: QUIC 流处理需要完善
3. **单服务器支持**: 当前只使用第一个配置的服务器
4. **基础错误处理**: 错误处理机制需要进一步完善

### 🐛 已知问题
1. **WebSocket 握手**: WebSocket 握手逻辑标记为 TODO
2. **QUIC 流处理**: QUIC 数据读写逻辑标记为 TODO
3. **分片重组**: 大数据包分片后的重组逻辑需要验证
4. **连接管理**: 连接池和重用机制缺失

### 📋 测试状态
- ✅ **TCP 传输**: 基本功能可用
- ⚠️ **WebSocket 传输**: 框架实现，需要完善
- ⚠️ **QUIC 传输**: 框架实现，需要完善
- ⚠️ **加密系统**: 基本可用，需要更多测试
- ⚠️ **协议握手**: 基本可用，边界情况待测试

## 🔮 发展规划

### 🚀 短期目标 (v1.1)
1. **完善 WebSocket 实现**:
   - 实现完整的 WebSocket 握手
   - 添加 WebSocket 帧处理
   - 支持 WSS (WebSocket over TLS)

2. **完善 QUIC 实现**:
   - 实现 QUIC 流管理
   - 添加 QUIC 数据包处理
   - 支持 QUIC 连接复用

3. **错误处理改进**:
   - 完善错误恢复机制
   - 添加连接重试逻辑
   - 改进日志和调试信息

### 🎯 中期目标 (v1.5)
1. **功能增强**:
   - 多服务器负载均衡
   - 连接池管理
   - 健康检查机制

2. **性能优化**:
   - 减少内存分配
   - 优化加密性能
   - 改进并发处理

3. **兼容性提升**:
   - 与原版 Brook 完全兼容
   - 支持更多 Brook 特性
   - 添加向后兼容性

### 🌟 长期愿景 (v2.0)
1. **生态完善**:
   - 官方 Brook 服务端集成
   - 图形化配置工具
   - 详细部署文档

2. **高级特性**:
   - 端口跳跃支持
   - 流量混淆增强
   - 智能路由选择

## 🎉 项目总结

### ✅ 核心成就

我们成功实现了**Brook 协议的基础框架**，具备以下特点：

1. **🔒 安全加密系统**
   - AES-256-GCM 认证加密
   - HKDF 安全密钥派生
   - 时间戳重放保护

2. **🔧 多传输架构**
   - 统一的 StreamClient 接口
   - TCP 传输完整实现
   - WebSocket/QUIC 传输框架

3. **⚙️ 协议兼容性**
   - 与原版 Brook 协议兼容
   - 完整的握手流程
   - 标准的数据格式

4. **🛠️ 系统集成**
   - 完整的 v2ray-core 集成
   - 标准配置系统支持
   - 灵活的传输方式选择

### 🏆 技术优势

- **简洁性**: 协议设计简洁，易于理解和维护
- **灵活性**: 支持多种传输方式，适应不同网络环境
- **安全性**: 使用标准加密算法，安全可靠
- **兼容性**: 与原版 Brook 协议兼容

### 💡 使用建议

**✅ 推荐场景**:
- 需要简洁协议的环境
- 有现有 Brook 服务端支持
- 需要多传输方式的场景
- 对协议兼容性有要求

**⚠️ 注意事项**:
- TCP 传输相对稳定，WebSocket/QUIC 需要进一步完善
- 需要与 Brook 服务端配合使用
- 建议在测试环境充分验证后再用于生产
- 密码强度直接影响安全性

---

**项目状态**: 🟡 **部分完成** | **维护状态**: 🟢 **积极开发** | **推荐等级**: ⭐⭐⭐

> 🎯 **结论**: Brook 协议实现已具备基础功能框架，TCP 传输基本可用，但 WebSocket 和 QUIC 传输需要进一步完善。适合对协议简洁性有要求且有 Brook 服务端支持的场景使用。

---

## 📚 附录

### 🔗 相关资源
- [Brook 项目](https://github.com/txthinking/brook)
- [AES-GCM 规范](https://tools.ietf.org/html/rfc5116)
- [HKDF 规范](https://tools.ietf.org/html/rfc5869)
- [v2ray-core 文档](https://github.com/v2fly/v2ray-core)

### 🐛 问题反馈
如遇到问题，请提供以下信息：
1. 完整的配置文件
2. 错误日志输出
3. 使用的传输方式
4. v2ray 版本信息
5. Brook 服务端版本

### 📄 更新日志
- **v0.1.0**: 初始 Brook 协议框架实现
- **v0.1.1**: 完善 TCP 传输实现
- **v0.1.2**: 添加 WebSocket/QUIC 传输框架
- **v0.1.3**: 改进加密系统和错误处理
- **v0.1.4**: 完善文档和使用指南
