package commands

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"syscall"
	"time"

	"golang.org/x/net/proxy"

	"github.com/frogwall/f2ray-core/v5/common/cmdarg"
	"github.com/frogwall/f2ray-core/v5/common/platform"
	core "github.com/frogwall/f2ray-core/v5"
	"github.com/frogwall/f2ray-core/v5/main/commands/base"
)

var CmdPing = &base.Command{
	CustomFlags: true,
	UsageLine:   "{{.Exec}} ping [-c config.json] [-d dir] [-t timeout]",
	Short:       "test proxy latency",
	Long: `
Test proxy node latency.

If f2ray is running in background, it will test the running instance.
Otherwise, it will start a temporary instance for testing.

Arguments:

	-c, -config <file>
		Config file for f2ray. Multiple assign is accepted.

	-d, -confdir <dir>
		A directory with config files. Multiple assign is accepted.

	-t, -timeout <seconds>
		Timeout for ping test in seconds. (default 10)

	-u, -url <url>
		Test URL. (default "https://www.gstatic.com/generate_204")

Examples:

	{{.Exec}} {{.LongName}} -c config.json
	{{.Exec}} {{.LongName}} -d /etc/f2ray -t 5
	{{.Exec}} {{.LongName}} -u https://www.cloudflare.com/cdn-cgi/trace
	`,
	Run: executePing,
}

var (
	pingConfigFiles cmdarg.Arg
	pingConfigDirs  cmdarg.Arg
	pingTimeout     *int
	pingURL         *string
	pingFormat      *string
)

func setPingFlags(cmd *base.Command) {
	pingFormat = cmd.Flag.String("format", core.FormatAuto, "")
	pingTimeout = cmd.Flag.Int("t", 10, "")
	pingTimeout = cmd.Flag.Int("timeout", 10, "")
	pingURL = cmd.Flag.String("u", "https://www.gstatic.com/generate_204", "")
	pingURL = cmd.Flag.String("url", "https://www.gstatic.com/generate_204", "")

	cmd.Flag.Var(&pingConfigFiles, "config", "")
	cmd.Flag.Var(&pingConfigFiles, "c", "")
	cmd.Flag.Var(&pingConfigDirs, "confdir", "")
	cmd.Flag.Var(&pingConfigDirs, "d", "")
}

func executePing(cmd *base.Command, args []string) {
	setPingFlags(cmd)
	cmd.Flag.Parse(args)

	// Check if f2ray is running
	isRunning, pid := checkF2RayRunning()
	
	if isRunning {
		fmt.Printf("F2Ray is running (PID: %d), testing proxy latency...\n", pid)
		testRunningProxy(*pingURL, *pingTimeout)
	} else {
		fmt.Println("F2Ray is not running, starting temporary instance for testing...")
		testWithTempInstance(*pingURL, *pingTimeout)
	}
}

// checkF2RayRunning checks if f2ray is running by reading PID file
func checkF2RayRunning() (bool, int) {
	pidFile := "/tmp/f2ray.pid"
	data, err := os.ReadFile(pidFile)
	if err != nil {
		return false, 0
	}

	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return false, 0
	}

	// Check if process exists
	process, err := os.FindProcess(pid)
	if err != nil {
		return false, 0
	}

	// Send signal 0 to check if process is alive
	err = process.Signal(syscall.Signal(0))
	if err != nil {
		return false, 0
	}

	return true, pid
}

// testRunningProxy tests the latency of a running proxy
func testRunningProxy(testURL string, timeout int) {
	// Assume SOCKS5 proxy is running on localhost:1080
	// You may need to parse config to get the actual inbound port
	proxyAddr := "127.0.0.1:1080"
	
	latency, err := measureLatency(proxyAddr, testURL, timeout)
	if err != nil {
		fmt.Printf("❌ Ping failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✅ Latency: %d ms\n", latency)
}

// testWithTempInstance starts a temporary instance and tests latency
func testWithTempInstance(testURL string, timeout int) {
	// Get config files
	pingConfigFiles = getPingConfigFilePath()
	if len(pingConfigFiles) == 0 {
		base.Fatalf("No config file found")
	}

	// Load config
	config, err := core.LoadConfig(*pingFormat, pingConfigFiles)
	if err != nil {
		base.Fatalf("Failed to load config: %s", err)
	}

	// Create instance
	server, err := core.New(config)
	if err != nil {
		base.Fatalf("Failed to create instance: %s", err)
	}

	// Start instance
	if err := server.Start(); err != nil {
		base.Fatalf("Failed to start instance: %s", err)
	}
	defer server.Close()

	// Wait a bit for server to be ready
	time.Sleep(500 * time.Millisecond)

	// Test latency
	proxyAddr := "127.0.0.1:1080"
	latency, err := measureLatency(proxyAddr, testURL, timeout)
	if err != nil {
		fmt.Printf("❌ Ping failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✅ Latency: %d ms\n", latency)
}

// measureLatency measures the latency through a SOCKS5 proxy
func measureLatency(proxyAddr, testURL string, timeout int) (int64, error) {
	// Create SOCKS5 dialer
	dialer, err := proxy.SOCKS5("tcp", proxyAddr, nil, proxy.Direct)
	if err != nil {
		return 0, fmt.Errorf("failed to create SOCKS5 dialer: %w", err)
	}

	// Create HTTP client with proxy
	client := &http.Client{
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				return dialer.Dial(network, addr)
			},
		},
		Timeout: time.Duration(timeout) * time.Second,
	}

	// Measure latency
	start := time.Now()
	
	req, err := http.NewRequest("GET", testURL, nil)
	if err != nil {
		return 0, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("User-Agent", "F2Ray-Ping/1.0")

	resp, err := client.Do(req)
	if err != nil {
		return 0, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	// Read response to ensure full connection
	io.Copy(io.Discard, resp.Body)

	latency := time.Since(start).Milliseconds()
	
	if resp.StatusCode >= 400 {
		return latency, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	return latency, nil
}

// getPingConfigFilePath gets config file path for ping command
func getPingConfigFilePath() cmdarg.Arg {
	extension, err := core.GetLoaderExtensions(*pingFormat)
	if err != nil {
		base.Fatalf("%v", err.Error())
	}

	dirReader := readConfDir
	if len(pingConfigDirs) > 0 {
		for _, d := range pingConfigDirs {
			pingConfigFiles = append(pingConfigFiles, dirReader(d, extension)...)
		}
	} else if envConfDir := platform.GetConfDirPath(); dirExists(envConfDir) {
		pingConfigFiles = append(pingConfigFiles, dirReader(envConfDir, extension)...)
	}

	if len(pingConfigFiles) > 0 {
		return pingConfigFiles
	}

	if configFile := platform.GetConfigurationPath(); fileExists(configFile) {
		return cmdarg.Arg{configFile}
	}

	return nil
}
