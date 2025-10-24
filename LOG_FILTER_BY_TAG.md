# 按标签过滤访问日志

## 功能说明

现在可以按照出站/入站的标签（tag）过滤访问日志，避免记录本地服务的连接。

## 使用场景

### 问题
在 v2ray 配置中，通常会有一些标签为 `api` 的出站，用于本地服务通信：

```json
{
  "outbounds": [
    {
      "tag": "proxy",
      "protocol": "shadowsocks",
      // ...
    },
    {
      "tag": "direct",
      "protocol": "freedom"
    },
    {
      "tag": "api",
      "protocol": "freedom",
      "settings": {}
    }
  ]
}
```

这些本地服务的连接会产生大量无用的访问日志：

```
127.0.0.1:12345 accepted //127.0.0.1:10085 via:api
127.0.0.1:12346 accepted //127.0.0.1:10085 via:api
127.0.0.1:12347 accepted //127.0.0.1:10085 via:api
```

### 解决方案

现在这些标签为 `api` 的连接会被自动过滤掉。

## 实现原理

### 代码位置

`common/log/access.go` 中的 `ShouldFilter()` 方法：

```go
func (m *AccessMessage) ShouldFilter() bool {
    // Filter by tag - skip "api" tag (used for local services)
    if m.Detour == "api" {
        return true
    }
    
    // ... 其他过滤规则
}
```

### 工作流程

1. 连接建立时，dispatcher 会设置 `accessMessage.Detour = tag`
2. 记录日志前，调用 `ShouldFilter()` 检查
3. 如果 tag 为 `api`，返回 `true`（过滤）
4. 日志处理器跳过这条记录

## 配置示例

### 典型的 v2ray 配置

```json
{
  "inbounds": [
    {
      "tag": "http-in",
      "port": 1081,
      "protocol": "http"
    },
    {
      "tag": "socks-in",
      "port": 1080,
      "protocol": "socks"
    },
    {
      "tag": "api-in",
      "port": 10085,
      "protocol": "dokodemo-door",
      "settings": {
        "address": "127.0.0.1"
      }
    }
  ],
  "outbounds": [
    {
      "tag": "proxy",
      "protocol": "shadowsocks",
      "settings": {
        "servers": [...]
      }
    },
    {
      "tag": "direct",
      "protocol": "freedom"
    },
    {
      "tag": "api",
      "protocol": "freedom",
      "settings": {}
    }
  ],
  "routing": {
    "rules": [
      {
        "type": "field",
        "inboundTag": ["api-in"],
        "outboundTag": "api"
      },
      {
        "type": "field",
        "domain": ["geosite:cn"],
        "outboundTag": "direct"
      }
    ]
  }
}
```

### 日志效果

**过滤前**：
```
127.0.0.1:40532 accepted //x.com:443 via:proxy
127.0.0.1:40534 accepted //127.0.0.1:10085 via:api
127.0.0.1:40536 accepted //127.0.0.1:10085 via:api
127.0.0.1:40538 accepted //google.com:443 via:proxy
127.0.0.1:40540 accepted //127.0.0.1:10085 via:api
127.0.0.1:40542 accepted //baidu.com:443 via:direct
```

**过滤后**：
```
127.0.0.1:40532 accepted //x.com:443 via:proxy
127.0.0.1:40538 accepted //google.com:443 via:proxy
127.0.0.1:40542 accepted //baidu.com:443 via:direct
```

## 自定义过滤标签

### 过滤多个标签

如果你想过滤更多标签，修改 `common/log/access.go`：

```go
func (m *AccessMessage) ShouldFilter() bool {
    // 过滤多个标签
    filterTags := []string{"api", "direct", "block"}
    for _, tag := range filterTags {
        if m.Detour == tag {
            return true
        }
    }
    
    // ... 其他过滤规则
}
```

### 只记录特定标签

反向逻辑，只记录你关心的标签：

```go
func (m *AccessMessage) ShouldFilter() bool {
    // 只记录这些标签
    allowTags := []string{"proxy", "vmess", "trojan"}
    
    if m.Detour != "" {
        for _, tag := range allowTags {
            if m.Detour == tag {
                return false  // 不过滤
            }
        }
        return true  // 过滤其他所有标签
    }
    
    // ... 其他过滤规则
}
```

### 按标签前缀过滤

过滤所有以 `local-` 开头的标签：

```go
func (m *AccessMessage) ShouldFilter() bool {
    // 过滤特定前缀
    if strings.HasPrefix(m.Detour, "local-") {
        return true
    }
    
    if strings.HasPrefix(m.Detour, "test-") {
        return true
    }
    
    // ... 其他过滤规则
}
```

## 常见使用场景

### 场景 1: 只看代理流量

```go
func (m *AccessMessage) ShouldFilter() bool {
    // 过滤直连和 API
    if m.Detour == "direct" || m.Detour == "api" {
        return true
    }
    return false
}
```

**效果**：只显示通过代理的连接

### 场景 2: 只看国外流量

```go
func (m *AccessMessage) ShouldFilter() bool {
    // 过滤国内直连
    if m.Detour == "direct" || m.Detour == "api" {
        return true
    }
    
    // 过滤国内域名
    dest := serial.ToString(m.To)
    if strings.Contains(dest, ".cn") || 
       strings.Contains(dest, "baidu.com") ||
       strings.Contains(dest, "qq.com") {
        return true
    }
    
    return false
}
```

**效果**：只显示访问国外网站的记录

### 场景 3: 监控特定标签

```go
func (m *AccessMessage) ShouldFilter() bool {
    // 只监控 "important" 标签
    if m.Detour != "important" {
        return true
    }
    return false
}
```

**效果**：只显示通过 "important" 出站的连接

## 调试技巧

### 查看所有标签

临时禁用过滤，查看所有连接使用的标签：

```bash
# 修改代码，注释掉标签过滤
# if m.Detour == "api" {
#     return true
# }

# 重新编译运行
go build -v -o v2ray ./main
./v2ray run -c ~/v2ray-shadowtls.json 2>&1 | grep "via:"
```

### 统计标签使用

```bash
# 统计各个标签的使用次数
./v2ray run -c ~/v2ray-shadowtls.json 2>&1 | \
    grep "via:" | \
    grep -oP 'via:\K\w+' | \
    sort | uniq -c | sort -rn
```

输出示例：
```
    150 api
     45 proxy
     30 direct
      5 block
```

### 测试过滤效果

```bash
# 运行前统计总日志数
./v2ray run -c ~/v2ray-shadowtls.json 2>&1 | grep "accepted" | wc -l

# 运行后统计（应该减少了）
./v2ray run -c ~/v2ray-shadowtls.json 2>&1 | grep "accepted" | wc -l
```

## 与其他过滤规则的关系

### 过滤优先级

```go
func (m *AccessMessage) ShouldFilter() bool {
    // 1. 首先检查标签（最快）
    if m.Detour == "api" {
        return true
    }
    
    // 2. 然后检查目标地址
    dest := serial.ToString(m.To)
    
    // 3. 最后检查其他条件
    // ...
}
```

### 组合过滤

```go
func (m *AccessMessage) ShouldFilter() bool {
    // 标签过滤
    if m.Detour == "api" {
        return true
    }
    
    // Telegram 过滤
    dest := serial.ToString(m.To)
    if strings.Contains(dest, "149.154") {
        return true
    }
    
    // 短连接过滤
    if m.Duration > 0 && m.Duration < 1*time.Second {
        return true
    }
    
    return false
}
```

## 性能影响

### 标签过滤的性能

- **字符串比较**: O(1) 时间复杂度
- **内存开销**: 0（标签已经存在）
- **CPU 开销**: < 0.1μs

### 结论

✅ 标签过滤是最快的过滤方式，推荐优先使用

## 总结

### 优点

- ✅ **简单高效**: 直接比较标签字符串
- ✅ **灵活配置**: 可以过滤任意标签
- ✅ **无性能影响**: 几乎零开销
- ✅ **易于维护**: 标签在配置文件中定义

### 使用建议

1. **本地服务**: 使用 `api` 标签并过滤
2. **直连流量**: 根据需要决定是否过滤 `direct`
3. **测试环境**: 使用 `test-` 前缀并过滤
4. **重要流量**: 使用特定标签并只记录这些

### 快速开始

1. 确保你的配置中有标签定义
2. 编译新版本：`go build -v -o v2ray ./main`
3. 运行：`./v2ray run -c config.json`
4. 观察日志：标签为 `api` 的连接不再显示

现在你的日志更清爽了！🎉
