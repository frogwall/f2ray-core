#!/bin/bash

# 修复 genproto 冲突的辅助脚本
# 在 gomobile 编译过程中动态修复临时 go.mod

set -e

echo "[FIX] 开始修复 genproto 版本冲突..."

# 删除旧版本缓存
chmod -R +w ~/go/pkg/mod/google.golang.org/genproto@v0.0.0-20230110181048-76db0878b65f 2>/dev/null || true
rm -rf ~/go/pkg/mod/google.golang.org/genproto@v0.0.0-20230110181048-76db0878b65f 2>/dev/null || true

echo "[FIX] 已清理旧版本 genproto 缓存"
