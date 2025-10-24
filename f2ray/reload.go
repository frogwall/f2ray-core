package main

import (
	"fmt"
	"os"
	"strconv"
	"syscall"

	"github.com/v2fly/v2ray-core/v5/main/commands/base"
)

var f2CmdReload = &base.Command{
	UsageLine: "{{.Exec}} reload",
	Short:     "soft-reload f2ray by reloading config",
	Long:      "Send SIGHUP to running f2ray (reads pid from /tmp/f2ray.pid) to reload configuration.",
	Run: func(cmd *base.Command, args []string) {
		data, err := os.ReadFile("/tmp/f2ray.pid")
		if err != nil {
			base.Fatalf("cannot read /tmp/f2ray.pid: %v", err)
		}
		pid, err := strconv.Atoi(string(data))
		if err != nil {
			base.Fatalf("invalid pid in /tmp/f2ray.pid: %v", err)
		}
		if err := syscall.Kill(pid, syscall.SIGHUP); err != nil {
			base.Fatalf("failed to signal process %d: %v", pid, err)
		}
		fmt.Printf("reload signal sent to %d\n", pid)
	},
}
