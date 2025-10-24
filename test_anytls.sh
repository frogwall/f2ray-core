#!/bin/bash

echo "测试 AnyTLS 协议连接..."
echo ""

# 测试 HTTP 代理
echo "1. 测试 HTTP 代理 (端口 1081):"
curl -x http://127.0.0.1:1081 -m 10 "http://httpbin.org/ip" 2>&1
echo ""

# 测试 SOCKS5 代理
echo "2. 测试 SOCKS5 代理 (端口 1080):"
curl -x socks5://127.0.0.1:1080 -m 10 "http://httpbin.org/ip" 2>&1
echo ""

echo "测试完成"
