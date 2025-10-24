package main

import (
	"github.com/frogwall/f2ray-core/v5/main/commands"
	"github.com/frogwall/f2ray-core/v5/main/commands/base"
	_ "github.com/frogwall/f2ray-core/v5/main/distro/all"
)

func main() {
	base.RootCommand.Long = "A unified platform for anti-censorship. (f2ray)"
	// Keep upstream commands for compatibility (except CmdRun, we use our own)
	base.RegisterCommand(commands.CmdVersion)
	base.RegisterCommand(commands.CmdTest)
	// f2ray-specific commands
	base.RegisterCommand(f2CmdRun)
	base.RegisterCommand(f2CmdReload)
	base.SortLessFunc = runIsTheFirst
	base.SortCommands()
	base.Execute()
}

func runIsTheFirst(i, j *base.Command) bool {
	left := i.Name()
	right := j.Name()
	if left == "run" {
		return true
	}
	if right == "run" {
		return false
	}
	return left < right
}
