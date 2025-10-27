# Xray-core 项目中 VLESS + REALITY 实现方式总结

## 项目概述

本项目实现了 VLESS 协议与 REALITY 传输层的完整集成，提供了一个高性能、安全的代理解决方案。

## 核心架构

### 1. VLESS 协议层 (`proxy/vless/`)

#### 协议定义 (`vless.go`)
```go
const (
    None = "none"
    XRV  = "xtls-rprx-vision"
)
```
支持两种 Flow 模式：
- `none`: 基础模式
- `xtls-rprx-vision`: Vision 优化模式，可进行零拷贝加速

#### 账户管理 (`account.go`)
```go
type MemoryAccount struct {
    ID         *protocol.ID    // 用户ID
    Flow       string          // 流量控制Flow
    Encryption string          // 加密配置
    XorMode    uint32          // XOR模式
    Seconds    uint32          // 时间配置
    Padding    string          // 填充配置
    Reverse    *Reverse        // 反向代理配置
}
```

#### 数据编码 (`encoding/encoding.go`)
关键代码：
```go
// 请求头编码
func EncodeRequestHeader(writer io.Writer, request *protocol.RequestHeader, requestAddons *Addons) error {
    buffer := buf.StackNew()
    defer buffer.Release()
    
    // 写入版本号
    buffer.WriteByte(request.Version)
    // 写入用户ID（16字节）
    buffer.Write(request.User.Account.(*vless.MemoryAccount).ID.Bytes())
    // 写入附加信息
    EncodeHeaderAddons(&buffer, requestAddons)
    // 写入命令
    buffer.WriteByte(byte(request.Command))
    
    // 写入目标地址和端口
    if request.Command != protocol.RequestCommandMux && 
       request.Command != protocol.RequestCommandRvs {
        addrParser.WriteAddressPort(&buffer, request.Address, request.Port)
    }
    
    writer.Write(buffer.Bytes())
    return nil
}
```

### 2. ML-KEM 加密层 (`proxy/vless/encryption/`)

#### 客户端握手 (`encryption/client.go`)
核心流程：

**步骤1：生成初始化向量和转发密钥**
```go
func (i *ClientInstance) Handshake(conn net.Conn) (*CommonConn, error) {
    c := NewCommonConn(conn, protocol.HasAESGCMHardwareSupport)
    
    // 准备ClientHello: IV(16字节) + 转发密钥 + 密钥交换 + 填充
    ivAndRealysLength := 16 + i.RelaysLength
    pfsKeyExchangeLength := 18 + 1184 + 32 + 16  // ML-KEM-768 + X25519
    clientHello := make([]byte, ivAndRealysLength+pfsKeyExchangeLength+paddingLength)
    
    // 生成随机IV
    iv := clientHello[:16]
    rand.Read(iv)
    
    // 生成转发密钥链
    relays := clientHello[16:ivAndRealysLength]
    var nfsKey []byte
    var lastCTR cipher.Stream
    
    for j, k := range i.NfsPKeys {
        if k, ok := k.(*mlkem.EncapsulationKey768); ok {
            // ML-KEM-768 封装
            nfsKey, ciphertext = k.Encapsulate()
            copy(relays, ciphertext)
        } else if k, ok := k.(*ecdh.PublicKey); ok {
            // X25519 密钥交换
            privateKey, _ := ecdh.X25519().GenerateKey(rand.Reader)
            copy(relays, privateKey.PublicKey().Bytes())
            nfsKey, _ = privateKey.ECDH(k)
        }
        
        // 如果启用XOR模式，对密钥进行混淆
        if i.XorMode > 0 {
            NewCTR(i.NfsPKeysBytes[j], iv).XORKeyStream(relays, relays[:index])
        }
    }
}
```

**步骤2：0-RTT 快速握手**
```go
// 0-RTT支持快速重连
if i.Seconds > 0 {
    i.RWLock.RLock()
    if time.Now().Before(i.Expire) {
        // 使用缓存的密钥快速建立连接
        c.Client = i
        c.UnitedKey = append(i.PfsKey, nfsKey...)
        // 加密ticket
        nfsAEAD.Seal(clientHello[:ivAndRealysLength], nil, EncodeLength(32), nil)
        nfsAEAD.Seal(clientHello[:ivAndRealysLength+18], nil, i.Ticket, nil)
        return c, nil
    }
    i.RWLock.RUnlock()
}
```

**步骤3：完整握手（1-RTT）**
```go
// 发送密钥交换和填充
pfsKeyExchange := clientHello[ivAndRealysLength : ivAndRealysLength+pfsKeyExchangeLength]
nfsAEAD.Seal(pfsKeyExchange[:0], nil, EncodeLength(pfsKeyExchangeLength-18), nil)

// 生成ML-KEM-768和X25519密钥对
mlkem768DKey, _ := mlkem.GenerateKey768()
x25519SKey, _ := ecdh.X25519().GenerateKey(rand.Reader)
pfsPublicKey := append(mlkem768DKey.EncapsulationKey().Bytes(), 
                       x25519SKey.PublicKey().Bytes()...)
nfsAEAD.Seal(pfsKeyExchange[:18], nil, pfsPublicKey, nil)

// 接收服务器响应
encryptedPfsPublicKey := make([]byte, 1088+32+16)
io.ReadFull(conn, encryptedPfsPublicKey)
nfsAEAD.Open(encryptedPfsPublicKey[:0], MaxNonce, encryptedPfsPublicKey, nil)

// 解封装ML-KEM-768并完成X25519交换
mlkem768Key, _ := mlkem768DKey.Decapsulate(encryptedPfsPublicKey[:1088])
peerX25519PKey, _ := ecdh.X25519().NewPublicKey(encryptedPfsPublicKey[1088:1088+32])
x25519Key, _ := x25519SKey.ECDH(peerX25519PKey)

// 组合最终密钥
pfsKey := make([]byte, 32+32)
copy(pfsKey, mlkem768Key)
copy(pfsKey[32:], x25519Key)
c.UnitedKey = append(pfsKey, nfsKey...)
```

#### 服务器握手 (`encryption/server.go`)
关键代码：

**解析转发密钥链**
```go
func (i *ServerInstance) Handshake(conn net.Conn, fallback *[]byte) (*CommonConn, error) {
    // 读取IV和转发密钥
    ivAndRelays := make([]byte, 16+i.RelaysLength)
    io.ReadFull(conn, ivAndRelays)
    
    iv := ivAndRelays[:16]
    relays := ivAndRelays[16:]
    
    var nfsKey []byte
    var lastCTR cipher.Stream
    
    for j, k := range i.NfsSKeys {
        if lastCTR != nil {
            // 使用前一个密钥恢复当前转发的混淆
            lastCTR.XORKeyStream(relays, relays[:32])
        }
        
        var index = 32
        if _, ok := k.(*mlkem.DecapsulationKey768); ok {
            index = 1088
        }
        
        if i.XorMode > 0 {
            // 解密XOR混淆
            NewCTR(i.NfsPKeysBytes[j], iv).XORKeyStream(relays, relays[:index])
        }
        
        if k, ok := k.(*ecdh.PrivateKey); ok {
            // X25519解包
            publicKey, _ := ecdh.X25519().NewPublicKey(relays[:index])
            if publicKey.Bytes()[31] > 127 {
                return nil, errors.New("invalid public key")
            }
            nfsKey, _ = k.ECDH(publicKey)
        } else if k, ok := k.(*mlkem.DecapsulationKey768); ok {
            // ML-KEM-768解包
            nfsKey, _ = k.Decapsulate(relays[:index])
        }
        
        // 验证转发密钥的正确性
        if j < len(i.NfsSKeys)-1 {
            lastCTR = NewCTR(nfsKey, iv)
            lastCTR.XORKeyStream(relays, relays[:32])
            if !bytes.Equal(relays[:32], i.Hash32s[j+1][:]) {
                return nil, errors.New("unexpected hash32")
            }
        }
    }
    
    nfsAEAD := NewAEAD(iv, nfsKey, c.UseAES)
}
```

**0-RTT和1-RTT处理**
```go
// 检查是否是0-RTT重连
if length == 32 {
    encryptedTicket := make([]byte, 32)
    io.ReadFull(conn, encryptedTicket)
    ticket, _ := nfsAEAD.Open(nil, nil, encryptedTicket, nil)
    
    // 查找会话
    s := i.Sessions[[16]byte(ticket)]
    if s == nil {
        // 会话已过期，发送噪声让客户端重新握手
        noises := make([]byte, crypto.RandBetween(1279, 2279))
        rand.Read(noises)
        conn.Write(noises)
        return nil, errors.New("expired ticket")
    }
    
    // 防止重放攻击
    if _, loaded := s.NfsKeys.LoadOrStore([32]byte(nfsKey), true); loaded {
        return nil, errors.New("replay detected")
    }
    
    c.UnitedKey = append(s.PfsKey, nfsKey...)
    return c, nil
}

// 1-RTT完整握手
encryptedPfsPublicKey := make([]byte, length)
io.ReadFull(conn, encryptedPfsPublicKey)
nfsAEAD.Open(encryptedPfsPublicKey[:0], nil, encryptedPfsPublicKey, nil)

// 生成服务器ML-KEM-768密钥
mlkem768EKey, _ := mlkem.NewEncapsulationKey768(encryptedPfsPublicKey[:1184])
mlkem768Key, encapsulatedPfsKey := mlkem768EKey.Encapsulate()

// 生成服务器X25519密钥
x25519SKey, _ := ecdh.X25519().GenerateKey(rand.Reader)
peerX25519PKey, _ := ecdh.X25519().NewPublicKey(encryptedPfsPublicKey[1184:1184+32])
x25519Key, _ := x25519SKey.ECDH(peerX25519PKey)

// 组合最终密钥
pfsKey := make([]byte, 32+32)
copy(pfsKey, mlkem768Key)
copy(pfsKey[32:], x25519Key)
c.UnitedKey = append(pfsKey, nfsKey...)
```

### 3. REALITY 传输层 (`transport/internet/reality/`)

#### 核心实现 (`reality.go`)

**服务器实现**
```go
// REALITY服务器连接
type Conn struct {
    *reality.Conn
}

func Server(c net.Conn, config *reality.Config) (net.Conn, error) {
    realityConn, err := reality.Server(context.Background(), c, config)
    return &Conn{Conn: realityConn}, err
}
```

**客户端实现**
```go
type UConn struct {
    *utls.UConn
    Config     *Config
    ServerName string
    AuthKey    []byte
    Verified   bool
}

func UClient(c net.Conn, config *Config, ctx context.Context, dest net.Destination) (net.Conn, error) {
    uConn := &UConn{Config: config}
    
    utlsConfig := &utls.Config{
        VerifyPeerCertificate: uConn.VerifyPeerCertificate,
        ServerName:            config.ServerName,
        InsecureSkipVerify:    true,
        SessionTicketsDisabled: true,
        KeyLogWriter:          KeyLogWriterFromConfig(config),
    }
    
    // 使用指定指纹
    fingerprint := tls.GetFingerprint(config.Fingerprint)
    uConn.UConn = utls.UClient(c, utlsConfig, *fingerprint)
    
    // 构建握手状态
    uConn.BuildHandshakeState()
    hello := uConn.HandshakeState.Hello
    
    // 自定义Session ID（包含版本和ShortId）
    hello.SessionId = make([]byte, 32)
    hello.SessionId[0] = core.Version_x
    hello.SessionId[1] = core.Version_y
    hello.SessionId[2] = core.Version_z
    hello.SessionId[3] = 0  // reserved
    binary.BigEndian.PutUint32(hello.SessionId[4:], uint32(time.Now().Unix()))
    copy(hello.SessionId[8:], config.ShortId)
    
    // 生成AuthKey用于证书验证
    publicKey, _ := ecdh.X25519().NewPublicKey(config.PublicKey)
    ecdhe := uConn.HandshakeState.State13.KeyShareKeys.Ecdhe
    uConn.AuthKey, _ = ecdhe.ECDH(publicKey)
    
    // 使用HKDF派生密钥
    hkdf.New(sha256.New, uConn.AuthKey, hello.Random[:20], []byte("REALITY")).
        Read(uConn.AuthKey)
    
    // 加密SessionId
    aead := crypto.NewAesGcm(uConn.AuthKey)
    aead.Seal(hello.SessionId[:0], hello.Random[20:], hello.SessionId[:16], hello.Raw)
    copy(hello.Raw[39:], hello.SessionId)
    
    // 执行握手
    uConn.HandshakeContext(ctx)
    
    // 证书验证
    if !uConn.Verified {
        // 触发爬虫行为以伪装流量
        go func() {
            client := &http.Client{
                Transport: &http2.Transport{DialTLSContext: ...},
            }
            // 爬取网站链接以伪装成正常浏览
            get(true)
            concurrency := int(crypto.RandBetween(config.SpiderY[2], config.SpiderY[3]))
            for i := 0; i < concurrency; i++ {
                go get(false)
            }
        }()
        time.Sleep(time.Duration(crypto.RandBetween(config.SpiderY[8], config.SpiderY[9])) * time.Millisecond)
        return nil, errors.New("REALITY: processed invalid connection")
    }
    
    return uConn, nil
}
```

**证书验证**
```go
func (c *UConn) VerifyPeerCertificate(rawCerts [][]byte, verifiedChains [][]*x509.Certificate) error {
    // 检查是否使用ML-KEM-768和ML-DSA-65
    if c.Config.Show {
        fmt.Printf("is using X25519MLKEM768 for TLS: %v\n", 
                   c.HandshakeState.ServerHello.ServerShare.Group == utls.X25519MLKEM768)
        fmt.Printf("is using ML-DSA-65 for cert: %v\n", 
                   len(c.Config.Mldsa65Verify) > 0)
    }
    
    // 获取证书
    certs := ... // 从连接中提取证书
    
    // 验证Ed25519证书签名
    if pub, ok := certs[0].PublicKey.(ed25519.PublicKey); ok {
        h := hmac.New(sha512.New, c.AuthKey)
        h.Write(pub)
        
        // 检查签名匹配
        if bytes.Equal(h.Sum(nil), certs[0].Signature) {
            // 可选：ML-DSA-65验证
            if len(c.Config.Mldsa65Verify) > 0 {
                h.Write(c.HandshakeState.Hello.Raw)
                h.Write(c.HandshakeState.ServerHello.Raw)
                verify, _ := mldsa65.Scheme().UnmarshalBinaryPublicKey(c.Config.Mldsa65Verify)
                if mldsa65.Verify(verify.(*mldsa65.PublicKey), h.Sum(nil), nil, 
                                   certs[0].Extensions[0].Value) {
                    c.Verified = true
                    return nil
                }
            } else {
                c.Verified = true
                return nil
            }
        }
    }
    
    // 标准TLS证书验证
    opts := x509.VerifyOptions{
        DNSName:       c.ServerName,
        Intermediates: x509.NewCertPool(),
    }
    for _, cert := range certs[1:] {
        opts.Intermediates.AddCert(cert)
    }
    _, err := certs[0].Verify(opts)
    return err
}
```

### 4. 入站处理 (`proxy/vless/inbound/`)

**关键代码：**
```go
func (h *Handler) Process(ctx context.Context, network net.Network, 
                          connection stat.Connection, dispatcher routing.Dispatcher) error {
    // 1. 应用ML-KEM加密握手
    if h.decryption != nil {
        connection, err = h.decryption.Handshake(connection, nil)
        if err != nil {
            return errors.New("ML-KEM-768 handshake failed").Base(err)
        }
    }
    
    // 2. 读取并解码VLESS请求头
    first := buf.FromBytes(make([]byte, buf.Size))
    firstLen, errR := first.ReadFrom(connection)
    
    reader := &buf.BufferedReader{
        Reader: buf.NewReader(connection),
        Buffer: buf.MultiBuffer{first},
    }
    
    // 3. 解码请求头
    userSentID, request, requestAddons, isfb, err := 
        encoding.DecodeRequestHeader(isfb, first, reader, h.validator)
    
    // 4. 处理Fallback（如果请求无效）
    if err != nil && isfb {
        // 检查TLS或REALITY连接
        name := ""
        alpn := ""
        if realityConn, ok := iConn.(*reality.Conn); ok {
            cs := realityConn.ConnectionState()
            name = cs.ServerName
            alpn = cs.NegotiatedProtocol
        }
        
        // 路由到fallback目标
        // ...
    }
    
    // 5. 应用Vision流量优化
    switch requestAddons.Flow {
    case vless.XRV:
        inbound.CanSpliceCopy = 2
        // 提取原始连接用于零拷贝
        if realityConn, ok := iConn.(*reality.Conn); ok {
            t = reflect.TypeOf(realityConn.Conn).Elem()
            p = uintptr(unsafe.Pointer(realityConn.Conn))
        }
        input = (*bytes.Reader)(unsafe.Pointer(p + i.Offset))
        rawInput = (*bytes.Buffer)(unsafe.Pointer(p + r.Offset))
    }
    
    // 6. 编码响应头
    bufferWriter := buf.NewBufferedWriter(buf.NewWriter(connection))
    encoding.EncodeResponseHeader(bufferWriter, request, responseAddons)
    
    // 7. 开始数据传输
    clientReader := encoding.DecodeBodyAddons(reader, request, requestAddons)
    clientWriter := encoding.EncodeBodyAddons(bufferWriter, request, requestAddons, 
                                             trafficState, false, ctx, connection, nil)
    
    // 8. 转发到Dispatcher
    dispatcher.DispatchLink(ctx, request.Destination(), &transport.Link{
        Reader: clientReader,
        Writer: clientWriter,
    })
    
    return nil
}
```

### 5. 出站处理 (`proxy/vless/outbound/`)

**关键代码：**
```go
func (h *Handler) Process(ctx context.Context, link *transport.Link, dialer internet.Dialer) error {
    // 1. 建立TCP连接
    conn, err := dialer.Dial(ctx, rec.Destination)
    
    // 2. 应用ML-KEM客户端加密
    if h.encryption != nil {
        conn, err = h.encryption.Handshake(conn)
    }
    
    // 3. 构建请求头
    request := &protocol.RequestHeader{
        Version: encoding.Version,
        User:    rec.User,
        Command: command,
        Address: target.Address,
        Port:    target.Port,
    }
    
    account := request.User.Account.(*vless.MemoryAccount)
    requestAddons := &encoding.Addons{Flow: account.Flow}
    
    // 4. 应用Vision优化
    switch requestAddons.Flow {
    case vless.XRV:
        ob.CanSpliceCopy = 2
        // 提取原始连接指针
        if realityConn, ok := iConn.(*reality.UConn); ok {
            t = reflect.TypeOf(realityConn.Conn).Elem()
            p = uintptr(unsafe.Pointer(realityConn.Conn))
        }
        input = (*bytes.Reader)(unsafe.Pointer(p + i.Offset))
        rawInput = (*bytes.Buffer)(unsafe.Pointer(p + r.Offset))
    }
    
    // 5. 编码并发送请求
    postRequest := func() error {
        bufferWriter := buf.NewBufferedWriter(buf.NewWriter(conn))
        encoding.EncodeRequestHeader(bufferWriter, request, requestAddons)
        serverWriter := encoding.EncodeBodyAddons(bufferWriter, request, requestAddons, 
                                                 trafficState, true, ctx, conn, ob)
        
        // 如果启用了Vision，使用零拷贝
        if requestAddons.Flow == vless.XRV {
            if tlsConn, ok := iConn.(*tls.Conn); ok {
                if tlsConn.ConnectionState().Version != gotls.VersionTLS13 {
                    return errors.New("requires TLS 1.3")
                }
            }
        }
        
        // 传输数据
        buf.Copy(clientReader, serverWriter, buf.UpdateActivity(timer))
        return nil
    }
    
    // 6. 接收响应
    getResponse := func() error {
        responseAddons, _ := encoding.DecodeResponseHeader(conn, request)
        serverReader := encoding.DecodeBodyAddons(conn, request, responseAddons)
        
        if requestAddons.Flow == vless.XRV {
            serverReader = proxy.NewVisionReader(serverReader, trafficState, false, 
                                                ctx, conn, input, rawInput, ob)
        }
        
        // 使用XTLS零拷贝读取
        if requestAddons.Flow == vless.XRV {
            err = encoding.XtlsRead(serverReader, clientWriter, timer, conn, 
                                   trafficState, false, ctx)
        } else {
            err = buf.Copy(serverReader, clientWriter, buf.UpdateActivity(timer))
        }
        return err
    }
    
    task.Run(ctx, postRequest, task.OnSuccess(getResponse, task.Close(clientWriter)))
    return nil
}
```

## 核心技术特点

### 1. 多层加密
- **ML-KEM-768**: 后量子密码学密钥封装
- **X25519**: 经典椭圆曲线密钥交换
- **混合模式**: NFS（Next Forward Security）+ PFS（Perfect Forward Security）

### 2. 0-RTT支持
- 使用Ticket进行快速重连
- 会话缓存减少握手开销
- 重放攻击防护

### 3. REALITY伪装
- 使用uTLS模拟真实浏览器指纹
- 自定义Session ID传递配置信息
- 自动爬虫行为掩盖流量模式
- 支持ML-DSA-65签名验证

### 4. Vision零拷贝
- 直接操作底层连接缓冲区
- 绕过用户态拷贝提升性能
- 支持RAW传输优化

### 5. XOR模式
- 可选的对明文进行XOR混淆
- 支持native、xorpub、random三种模式
- 增强流量特征分析抵御

## 安全特性

1. **前向安全性**: 密钥交换采用ML-KEM-768 + X25519双层保护
2. **重放保护**: 服务端使用NfsKeys防止重放攻击
3. **会话验证**: REALITY使用HMAC验证服务器身份
4. **证书绑定**: 支持Ed25519 + ML-DSA-65双重签名验证
5. **流量伪装**: 自动模拟真实HTTPS浏览行为

## 性能优化

1. **0-RTT快速重连**: Ticket机制减少握手时间
2. **Vision零拷贝**: 绕过内核态-用户态拷贝
3. **连接复用**: 会话缓存机制
4. **硬件加速**: 自动检测并使用AES-GCM硬件加速

## 配置示例

### 客户端配置
```json
{
  "outbounds": [{
    "protocol": "vless",
    "settings": {
      "vnext": [{
        "address": "example.com",
        "port": 443,
        "users": [{
          "id": "uuid-here",
          "flow": "xtls-rprx-vision",
          "encryption": "mlkem768x25519plus.native.1rtt.<padding>"
        }]
      }]
    },
    "streamSettings": {
      "network": "tcp",
      "security": "reality",
      "realitySettings": {
        "show": false,
        "fingerprint": "chrome",
        "serverName": "example.com",
        "publicKey": "base64-pubkey",
        "shortId": "01234567",
        "spiderX": "/",
        "spiderY": [0, 0, 0, 0, 0, 0, 0, 0, 0, 0]
      }
    }
  }]
}
```

### 服务端配置
```json
{
  "inbounds": [{
    "protocol": "vless",
    "settings": {
      "clients": [{
        "id": "uuid-here",
        "flow": "xtls-rprx-vision"
      }],
      "decryption": "mlkem768x25519plus.xorpub.1-3600.<padding>"
    },
    "streamSettings": {
      "network": "tcp",
      "security": "reality",
      "realitySettings": {
        "dest": "www.microsoft.com:443",
        "serverNames": ["www.microsoft.com"],
        "privateKey": "base64-privatekey",
        "shortIds": ["01234567"]
      }
    }
  }]
}
```

## 总结

本项目实现了完整的VLESS协议 + REALITY传输层，具有以下优势：

1. **安全性**: 采用ML-KEM-768后量子密码学和双层密钥交换
2. **性能**: 支持0-RTT和Vision零拷贝加速
3. **伪装**: REALITY完全模拟真实HTTPS流量
4. **灵活**: 支持多种加密模式和流量控制
5. **前沿**: 使用最新的后量子密码学算法

该实现展现了现代代理协议的先进技术，结合了密码学、网络编程和系统优化的最佳实践。

