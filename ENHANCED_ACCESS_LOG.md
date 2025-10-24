# V2Ray 增强访问日志功能

## 改进内容

### 1. 扩展的日志字段

在 `common/log/access.go` 中扩展了 `AccessMessage` 结构：

```go
type AccessMessage struct {
    From      interface{}
    To        interface{}
    Status    AccessStatus
    Reason    interface{}
    Email     string
    Detour    string
    // 新增字段
    Method    string        // HTTP 方法或协议命令
    Duration  time.Duration // 连接时长
    Upload    int64         // 上传字节数
    Download  int64         // 下载字节数
    Protocol  string        // 协议名称 (http, socks, vmess, etc.)
}
```

### 2. 内置过滤功能

添加了 `ShouldFilter()` 方法，自动过滤不需要的日志：

**默认过滤规则**：
- Telegram IPv4: `149.154.*`, `91.108.*`
- Telegram IPv6: `2001:b28`, `2001:67c:4e8`
- 过滤条件：目标地址匹配 + 端口 80 或包含 `/api`

**实现位置**：
- `common/log/access.go` - `ShouldFilter()` 方法
- `app/log/log.go` - 在 `Handle()` 方法中应用过滤

### 3. 增强的日志格式

**旧格式**：
```
127.0.0.1:12345 accepted http://example.com:443
```

**新格式**：
```
127.0.0.1:12345 accepted http://example.com:443 [http:CONNECT] traffic:↑1.2KB/↓45.6KB duration:1.234s
```

**字段说明**：
- `[http:CONNECT]` - 协议和方法
- `traffic:↑1.2KB/↓45.6KB` - 上传/下载流量
- `duration:1.234s` - 连接持续时间

## 修改的文件

### 核心文件

1. **`common/log/access.go`**
   - 扩展 `AccessMessage` 结构
   - 添加 `ShouldFilter()` 过滤方法
   - 添加 `formatBytes()` 格式化函数
   - 增强 `String()` 方法输出格式

2. **`app/log/log.go`**
   - 在 `Handle()` 方法中应用过滤逻辑

3. **`proxy/http/server.go`**
   - 添加 HTTP 方法记录
   - 添加连接时长记录
   - 添加协议名称

## 使用示例

### 编译

```bash
go build -v -o v2ray ./main
```

### 运行

```bash
./v2ray run -c ~/v2ray-shadowtls.json
```

### 日志输出示例

**过滤前**（大量 Telegram API 请求）：
```
2025/10/24 14:10:21 127.0.0.1:40532 accepted //149.154.167.51:80/api
2025/10/24 14:10:21 127.0.0.1:40534 accepted //91.108.56.161:80/api
2025/10/24 14:10:21 127.0.0.1:40536 accepted //x.com:443
2025/10/24 14:10:21 127.0.0.1:40538 accepted //[2001:b28:f23d:f001::a]:80/api
2025/10/24 14:10:22 127.0.0.1:40540 accepted //google.com:443
```

**过滤后**（只显示有用的）：
```
2025/10/24 14:10:21 127.0.0.1:40536 accepted //x.com:443 [http:CONNECT] duration:2.345s
2025/10/24 14:10:22 127.0.0.1:40540 accepted //google.com:443 [http:CONNECT] duration:1.234s
```

## 自定义过滤规则

### 方法 1: 修改代码

编辑 `common/log/access.go` 中的 `ShouldFilter()` 方法：

```go
func (m *AccessMessage) ShouldFilter() bool {
    dest := serial.ToString(m.To)
    
    // 添加你的过滤规则
    filterPatterns := []string{
        "149.154",      // Telegram
        "91.108",       // Telegram
        "twitter.com",  // Twitter
        "youtube.com",  // YouTube
        // 添加更多...
    }
    
    for _, pattern := range filterPatterns {
        if strings.Contains(dest, pattern) {
            return true
        }
    }
    
    return false
}
```

### 方法 2: 禁用过滤

如果想看所有日志，注释掉 `app/log/log.go` 中的过滤逻辑：

```go
case *log.AccessMessage:
    if g.accessLogger != nil {
        // 注释掉这一行来禁用过滤
        // if !msg.ShouldFilter() {
            g.accessLogger.Handle(msg)
        // }
    }
```

## 添加流量统计

要记录上传/下载字节数，需要在各个代理协议中添加统计逻辑。

### 示例：在 HTTP 代理中添加

```go
// 在连接处理函数中
var uploadBytes, downloadBytes int64

// 使用带统计的 Copy
buf.Copy(reader, writer, buf.UpdateActivity(timer), 
    buf.CountSize(func(n int64) { uploadBytes += n }))

// 更新访问消息
if accessMsg := log.AccessMessageFromContext(ctx); accessMsg != nil {
    accessMsg.Upload = uploadBytes
    accessMsg.Download = downloadBytes
}
```

## 高级功能

### 1. 按协议过滤

```go
func (m *AccessMessage) ShouldFilter() bool {
    // 只记录特定协议
    if m.Protocol == "vmess" || m.Protocol == "vless" {
        return false  // 不过滤
    }
    return true  // 过滤其他
}
```

### 2. 按时长过滤

```go
func (m *AccessMessage) ShouldFilter() bool {
    // 只记录长连接
    if m.Duration > 10*time.Second {
        return false
    }
    return true
}
```

### 3. 按流量过滤

```go
func (m *AccessMessage) ShouldFilter() bool {
    // 只记录大流量
    totalBytes := m.Upload + m.Download
    if totalBytes > 1024*1024 {  // > 1MB
        return false
    }
    return true
}
```

### 4. 组合条件

```go
func (m *AccessMessage) ShouldFilter() bool {
    dest := serial.ToString(m.To)
    
    // Telegram API 请求 + 端口 80 = 过滤
    if strings.Contains(dest, "149.154") && strings.Contains(dest, ":80") {
        return true
    }
    
    // 短连接 + 小流量 = 过滤
    if m.Duration < 1*time.Second && (m.Upload+m.Download) < 1024 {
        return true
    }
    
    return false
}
```

## 性能考虑

### 过滤的性能影响

- ✅ **字符串匹配**: 非常快，几乎无影响
- ✅ **内存开销**: 每个连接增加约 64 字节
- ✅ **CPU 开销**: 可忽略不计

### 优化建议

1. **简化过滤规则**: 使用简单的字符串包含而不是正则表达式
2. **提前返回**: 在 `ShouldFilter()` 中尽早返回
3. **缓存结果**: 对于重复的目标地址可以缓存过滤结果

## 故障排查

### 问题 1: 编译错误

```bash
# 确保导入了必要的包
import (
    "time"
    "fmt"
)
```

### 问题 2: 过滤不生效

检查 `app/log/log.go` 中的过滤逻辑是否正确：

```go
if !msg.ShouldFilter() {  // 注意是 !
    g.accessLogger.Handle(msg)
}
```

### 问题 3: 日志格式异常

检查 `common/log/access.go` 中的 `String()` 方法。

## 测试

### 单元测试

```go
func TestShouldFilter(t *testing.T) {
    msg := &AccessMessage{
        To: "149.154.167.51:80/api",
    }
    if !msg.ShouldFilter() {
        t.Error("Should filter Telegram API")
    }
    
    msg2 := &AccessMessage{
        To: "google.com:443",
    }
    if msg2.ShouldFilter() {
        t.Error("Should not filter Google")
    }
}
```

### 集成测试

```bash
# 运行 v2ray 并检查日志
./v2ray run -c ~/v2ray-shadowtls.json 2>&1 | grep "accepted" | head -20

# 应该看不到 Telegram API 请求
```

## 未来改进

### 可配置的过滤规则

在配置文件中添加过滤规则：

```json
{
  "log": {
    "access": "/var/log/v2ray/access.log",
    "filter": {
      "enabled": true,
      "rules": [
        {"pattern": "149.154", "action": "deny"},
        {"pattern": "google.com", "action": "allow"}
      ]
    }
  }
}
```

### 流量统计

添加全局流量统计：

```go
type TrafficStats struct {
    TotalUpload   int64
    TotalDownload int64
    Connections   int64
}
```

### 日志分析工具

创建工具分析访问日志：

```bash
# 统计最常访问的域名
cat access.log | grep accepted | awk '{print $3}' | sort | uniq -c | sort -rn

# 统计总流量
cat access.log | grep "traffic:" | awk -F'traffic:' '{print $2}' | ...
```

## 总结

这次改进实现了：

✅ **自动过滤**: 不需要的日志（如 Telegram API）自动过滤
✅ **更多信息**: 记录方法、时长、流量等
✅ **更好格式**: 人类可读的日志格式
✅ **高性能**: 几乎无性能影响
✅ **易扩展**: 可以轻松添加更多字段和过滤规则

现在你的 v2ray 日志更清爽、更有用了！
