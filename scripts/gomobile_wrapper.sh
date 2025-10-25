#!/bin/bash

# gomobile 包装器脚本 - 自动修复临时 go.mod 中的 genproto 冲突

# 保存原始 gomobile 路径
REAL_GOMOBILE=$(which gomobile)

# 启动后台进程监控并修复临时 go.mod
(
    while true; do
        # 查找 gomobile 创建的临时目录
        for tmpdir in /var/folders/*/T/gomobile-work-*/*/src-*/; do
            if [ -f "$tmpdir/go.mod" ]; then
                # 检查是否需要添加 exclude
                if ! grep -q "exclude google.golang.org/genproto v0.0.0-20230110181048-76db0878b65f" "$tmpdir/go.mod" 2>/dev/null; then
                    echo "" >> "$tmpdir/go.mod"
                    echo "exclude google.golang.org/genproto v0.0.0-20230110181048-76db0878b65f" >> "$tmpdir/go.mod"
                fi
            fi
        done
        sleep 0.5
    done
) &
MONITOR_PID=$!

# 运行真正的 gomobile
$REAL_GOMOBILE "$@"
EXIT_CODE=$?

# 停止监控进程
kill $MONITOR_PID 2>/dev/null || true

exit $EXIT_CODE
