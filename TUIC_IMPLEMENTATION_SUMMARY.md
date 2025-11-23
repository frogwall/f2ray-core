# TUIC 协议实现总结

## 项目概述

TUIC 是一种基于 QUIC 的高性能代理协议，强调低延迟、良好的弱网表现和灵活的 UDP 转发模式。本项目实现了完整的 TUIC 出站客户端集成，支持标准的认证流程、拥塞控制选择、QUIC 参数调优以及 TLS 配置。

## 核心特性
- QUIC 传输：依赖 `quic-go` 提供低延迟可靠传输
- TLS 配置：支持 SNI、ALPN、允许自签等常见选项
- 拥塞控制：支持 `bbr`、`cubic`、`new_reno`
- UDP 转发模式：`native` 与 `quic` 两种转发策略
- 0-RTT：可选降 RTT 配置以缩短握手耗时

## 实现状态
- 客户端：生产可用
- 服务端：不在本仓库实现范围内，按标准 TUIC 服务端兼容
- 配置系统：完整接入 v4 JSON 配置与 Protobuf

## 代码结构
- `proxy/tuic/client.go`：TUIC 出站客户端逻辑（TCP/UDP）
- `proxy/tuic/config.proto` / `config.pb.go`：协议配置定义与生成代码
- `infra/conf/v4/tuic.go`：v4 JSON 配置到 Protobuf 的映射
- `infra/conf/v4/v2ray.go`：`streamSettings` 到 TUIC 的特例映射（如 `network=quic` -> `udp_relay_mode=quic`）

## 关键实现说明

### 客户端创建与会话流程
- 通过 `protocol.ServerPicker` 选择出站服务器，提取 `Account(UUID, Password)`，位置 `proxy/tuic/client.go:33-71`
- 构造 TLS 与 QUIC 配置，应用来自 `ClientConfig` 的参数，位置 `proxy/tuic/client.go:92-148`
- 选择拥塞算法并传入 TUIC 协议拨号器头部，位置 `proxy/tuic/client.go:150-167`
- 通过外部 `outbound/protocol` 的 TUIC 拨号器创建协议级连接，位置 `proxy/tuic/client.go:169-187`

### TCP 与 UDP 处理
- TCP：使用 TUIC 拨号得到 `net.Conn`，双向拷贝数据，位置 `proxy/tuic/client.go:190-233`
- UDP：使用 TUIC 拨号得到 `netproxy.PacketConn`；未连接 UDP 套接字用于 QUIC 数据报；读写路径带有调试日志显示包长与来源/目的，位置 `proxy/tuic/client.go:235-292`

### UDP 套接字策略
- 使用未连接 UDP 套接字（`net.ListenUDP("udp", nil)`）以便协议层灵活使用 `WriteTo/ReadFrom`，位置 `proxy/tuic/client.go:298-322`
- 这会导致一般依赖 `RemoteAddr()` 的日志显示为 `127.0.0.1:0`；本实现改以会话目标与 `ReadFrom` 返回地址记录真实端点

## 配置与映射

### v4 JSON 到 Protobuf
- 入口：`infra/conf/v4/tuic.go:45-93`
- 字段：
  - 服务器：`servers[].address/port/uuid/password`
  - 拥塞算法：`congestionControl`（字符串）
  - UDP 转发模式：`udp_relay_mode`（由 `streamSettings.network` 推导或直接配置）
  - QUIC 参数：窗口、超时、KeepAlive、PMTU 设置
  - TLS：`serverName`、`alpn`、`allowInsecure`

### streamSettings 特例
- `infra/conf/v4/v2ray.go:275-317`：当 `protocol == "tuic"` 且 `streamSettings.network == "quic"` 时，自动设置 `udp_relay_mode = "quic"`；并将 TLS 设置（SNI、ALPN、allowInsecure）映射至 TUIC 配置

## 拥塞控制
- 可选值：`bbr`、`cubic`、`new_reno`
- 传递方式：在拨号头部 `Feature1` 中携带算法名，位置 `proxy/tuic/client.go:160-167`

## 示例配置
- 路径：`examples/tuic_outbound.json`
- 关键片段：
  - `settings.congestionControl`：选择拥塞算法
  - `settings.tls.serverName` 与 `streamSettings.security: tls`
  - `streamSettings.network: quic` 以启用 `udp_relay_mode: quic`

## 注意事项
- 未连接 UDP 套接字会在通用日志处显示占位远端地址；建议使用会话目标和 `ReadFrom` 地址进行观测
- 服务端需兼容 TUIC 协议并与所选拥塞算法与 ALPN/SNI 设置一致
- 如使用 `allowInsecure: true`，仅限测试环境

## 测试与构建
- 示例 JSON 文件通过 `jq` 校验
- 项目 `go build ./...` 构建通过

