package main

import (
	"github.com/frogwall/f2ray-core/v5/main/commands"
	"github.com/frogwall/f2ray-core/v5/main/commands/base"
	_ "github.com/frogwall/f2ray-core/v5/main/distro/all"
)

func main() {
	base.RootCommand.Long = "A unified platform for anti-censorship."
	base.RegisterCommand(commands.CmdRun)
	base.RegisterCommand(commands.CmdVersion)
	base.RegisterCommand(commands.CmdTest)
	base.RegisterCommand(commands.CmdReload)
	base.RegisterCommand(commands.CmdPing)
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
