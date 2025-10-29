<div>
  <img width="190" height="210" align="left" src="https://raw.githubusercontent.com/v2fly/v2fly-github-io/master/docs/.vuepress/public/readme-logo.png" alt="V2Ray"/>
  <br>
  <h1>Project F</h1>
  <p>Project F 从 Project V 源码 fork而来，主要目标是方便地集成多个协议，以 outbounds 出站实现为主，不保证入站功能的完整性。代码从 v2ray-core v5.40.0 开始修改，大多数代码由AI编写，因此不好保证代码质量，紧供测试使用。</p>
</div>

原版 V2Ray 的文档请访问 [V2Ray 官网](https://www.v2fly.org)。

## 新添加的协议

此分支包含了扩展 V2Ray 功能的额外出站协议实现：

- **[Naive 协议](NAIVE_ENHANCEMENT_SUMMARY.md)** - 带 uTLS 指纹伪装的 HTTP/2 CONNECT 隧道
- **[Hysteria2 协议](HYSTERIA2_IMPLEMENTATION_SUMMARY.md)** - 基于 QUIC 的高性能代理，支持拥塞控制
- **[Mieru 协议](MIERU_IMPLEMENTATION_SUMMARY.md)** - 使用 XChaCha20-Poly1305 加密的代理，支持基于时间的密钥轮换
- **[Brook 协议](BROOK_IMPLEMENTATION_SUMMARY.md)** - 支持 TCP/WebSocket/QUIC 多传输方式的代理
- **[ShadowTls 协议](SHADOWTLS_IMPLEMENTATION_SUMMARY.md)** - 将shadowTls作为传输层，实现了shadowsocks+shadowTls的出站功能
- **[Juicity 协议](JUICITY_IMPLEMENTATION_SUMMARY.md)** - 基于 QUIC 和 HTTP/3 的代理协议，支持 TLS 1.3 加密和拥塞控制
- **[Shadowsocks-2022 协议]** - 在传统 Shadowsocks 协议中集成了 Shadowsocks2022 算法支持，包括 `2022-blake3-aes-128-gcm` 和 `2022-blake3-aes-256-gcm` 加密方法

> ⚠️ **注意**：这些协议实现：
> - 为 **AI 生成的代码**，目前处于**测试阶段**
> - 主要是**仅出站**的实现
> - 在生产环境使用前可能需要额外的测试和验证
> - 不属于官方 V2Ray 项目的一部分

