package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"

	core "github.com/frogwall/v2ray-core/v5"
	"github.com/frogwall/v2ray-core/v5/common/cmdarg"
	verrors "github.com/frogwall/v2ray-core/v5/common/errors"
	"github.com/frogwall/v2ray-core/v5/common/platform"
	cmdmain "github.com/frogwall/v2ray-core/v5/main/commands"
	"github.com/frogwall/v2ray-core/v5/main/commands/base"
)

var f2CmdRun = &base.Command{
	CustomFlags: true,
	UsageLine:   "{{.Exec}} run [-c config.json] [-d dir]",
	Short:       "run f2ray with config (supports SIGHUP reload)",
	Long:        "Run f2ray with config. Send SIGHUP to reload config.",
	Run:         f2ExecuteRun,
}

var (
	f2ConfigFiles          cmdarg.Arg
	f2ConfigDirs           cmdarg.Arg
	f2ConfigFormat         *string
	f2ConfigDirRecursively *bool
)

func f2SetConfigFlags(cmd *base.Command) {
	f2ConfigFormat = cmd.Flag.String("format", core.FormatAuto, "")
	f2ConfigDirRecursively = cmd.Flag.Bool("r", false, "")

	cmd.Flag.Var(&f2ConfigFiles, "config", "")
	cmd.Flag.Var(&f2ConfigFiles, "c", "")
	cmd.Flag.Var(&f2ConfigDirs, "confdir", "")
	cmd.Flag.Var(&f2ConfigDirs, "d", "")
}

func f2ExecuteRun(cmd *base.Command, args []string) {
	f2SetConfigFlags(cmd)
	cmd.Flag.Parse(args)
	cmdmain.CmdVersion.Run(nil, nil)
	f2ConfigFiles = f2GetConfigFilePath()

	server, err := f2StartServer()
	if err != nil {
		base.Fatalf("Failed to start: %s", err)
	}

	// write pid file before starting server
	pidFile := "/tmp/f2ray.pid"
	log.Printf("Writing PID file: %s (PID: %d)", pidFile, os.Getpid())
	if err := os.WriteFile(pidFile, []byte(fmt.Sprintf("%d", os.Getpid())), 0644); err != nil {
		base.Fatalf("Failed to write pid file: %s", err)
	}
	log.Printf("PID file written successfully")
	defer func() {
		log.Printf("Removing PID file: %s", pidFile)
		os.Remove(pidFile)
	}()

	if err := server.Start(); err != nil {
		base.Fatalf("Failed to start: %s", err)
	}
	defer server.Close()

	runtime.GC()

	osSignals := make(chan os.Signal, 1)
	signal.Notify(osSignals, os.Interrupt, syscall.SIGTERM, syscall.SIGHUP)
	for {
		sig := <-osSignals
		switch sig {
		case syscall.SIGHUP:
			// reload
			server.Close()
			s, err := f2StartServer()
			if err != nil {
				log.Printf("reload failed to create server: %v", err)
				continue
			}
			if err := s.Start(); err != nil {
				log.Printf("reload failed to start: %v", err)
				continue
			}
			server = s
			runtime.GC()
			log.Printf("reloaded configuration")
		default:
			return
		}
	}
}

func f2FileExists(file string) bool {
	info, err := os.Stat(file)
	return err == nil && !info.IsDir()
}

func f2DirExists(file string) bool {
	if file == "" {
		return false
	}
	info, err := os.Stat(file)
	return err == nil && info.IsDir()
}

func f2ReadConfDir(dirPath string, extension []string) cmdarg.Arg {
	confs, err := os.ReadDir(dirPath)
	if err != nil {
		base.Fatalf("failed to read dir %s: %s", dirPath, err)
	}
	files := make(cmdarg.Arg, 0)
	for _, f := range confs {
		ext := filepath.Ext(f.Name())
		for _, e := range extension {
			if strings.EqualFold(e, ext) {
				files.Set(filepath.Join(dirPath, f.Name()))
				break
			}
		}
	}
	return files
}

func f2ReadConfDirRecursively(dirPath string, extension []string) cmdarg.Arg {
	files := make(cmdarg.Arg, 0)
	_ = filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
		ext := filepath.Ext(path)
		for _, e := range extension {
			if strings.EqualFold(e, ext) {
				files.Set(path)
				break
			}
		}
		return nil
	})
	return files
}

func f2GetConfigFilePath() cmdarg.Arg {
	extension, err := core.GetLoaderExtensions(*f2ConfigFormat)
	if err != nil {
		base.Fatalf("%v", err.Error())
	}
	dirReader := f2ReadConfDir
	if *f2ConfigDirRecursively {
		dirReader = f2ReadConfDirRecursively
	}
	if len(f2ConfigDirs) > 0 {
		for _, d := range f2ConfigDirs {
			log.Println("Using confdir from arg:", d)
			f2ConfigFiles = append(f2ConfigFiles, dirReader(d, extension)...)
		}
	} else if envConfDir := platform.GetConfDirPath(); f2DirExists(envConfDir) {
		log.Println("Using confdir from env:", envConfDir)
		f2ConfigFiles = append(f2ConfigFiles, dirReader(envConfDir, extension)...)
	}
	if len(f2ConfigFiles) > 0 {
		return f2ConfigFiles
	}

	if len(f2ConfigFiles) == 0 && len(f2ConfigDirs) > 0 {
		base.Fatalf("no config file found with extension: %s", extension)
	}

	if workingDir, err := os.Getwd(); err == nil {
		configFile := filepath.Join(workingDir, "config.json")
		if f2FileExists(configFile) {
			log.Println("Using default config: ", configFile)
			return cmdarg.Arg{configFile}
		}
	}

	if configFile := platform.GetConfigurationPath(); f2FileExists(configFile) {
		log.Println("Using config from env: ", configFile)
		return cmdarg.Arg{configFile}
	}

	return nil
}

func f2StartServer() (core.Server, error) {
	config, err := core.LoadConfig(*f2ConfigFormat, f2ConfigFiles)
	if err != nil {
		if len(f2ConfigFiles) == 0 {
			err = verrors.New("failed to load config").Base(err)
		} else {
			err = verrors.New(fmt.Sprintf("failed to load config: %s", f2ConfigFiles)).Base(err)
		}
		return nil, err
	}
	server, err := core.New(config)
	if err != nil {
		return nil, verrors.New("failed to create server").Base(err)
	}
	return server, nil
}
