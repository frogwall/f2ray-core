package commands

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"
	_ "unsafe"

	"github.com/frogwall/f2ray-core/v5/common/cmdarg"
	"github.com/frogwall/f2ray-core/v5/common/net"
	"github.com/frogwall/f2ray-core/v5/common/platform"
	core "github.com/frogwall/f2ray-core/v5"
	"github.com/frogwall/f2ray-core/v5/main/commands/base"
	"github.com/frogwall/f2ray-core/v5/transport/internet/tagged"
)

//go:linkname toContext github.com/frogwall/f2ray-core/v5.toContext
func toContext(ctx context.Context, v *core.Instance) context.Context

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
		Test URL. (default "https://connectivitycheck.gstatic.com/generate_204")

	-v, -verbose
		Show verbose output including debug information.

	-w, -warmup
		Warmup connections before testing (recommended for accurate results).

Examples:

	{{.Exec}} {{.LongName}} -c config.json
	{{.Exec}} {{.LongName}} -d /etc/f2ray -t 5
	{{.Exec}} {{.LongName}} -u https://www.cloudflare.com/cdn-cgi/trace
	{{.Exec}} {{.LongName}} -c config.json -v
	{{.Exec}} {{.LongName}} -c config.json -w
	`,
	Run: executePing,
}

var (
	pingConfigFiles cmdarg.Arg
	pingConfigDirs  cmdarg.Arg
	pingTimeout     *int
	pingURL         *string
	pingFormat      *string
	pingVerbose     *bool
	pingWarmup      *bool
)

func setPingFlags(cmd *base.Command) {
	pingFormat = cmd.Flag.String("format", core.FormatAuto, "")
	pingTimeout = cmd.Flag.Int("t", 5, "")
	pingTimeout = cmd.Flag.Int("timeout", 5, "")
	pingURL = cmd.Flag.String("u", "https://connectivitycheck.gstatic.com/generate_204", "")
	pingURL = cmd.Flag.String("url", "https://connectivitycheck.gstatic.com/generate_204", "")
	pingVerbose = cmd.Flag.Bool("v", false, "")
	pingVerbose = cmd.Flag.Bool("verbose", false, "")
	pingWarmup = cmd.Flag.Bool("w", false, "")
	pingWarmup = cmd.Flag.Bool("warmup", false, "")

	cmd.Flag.Var(&pingConfigFiles, "config", "")
	cmd.Flag.Var(&pingConfigFiles, "c", "")
	cmd.Flag.Var(&pingConfigDirs, "confdir", "")
	cmd.Flag.Var(&pingConfigDirs, "d", "")
}

func executePing(cmd *base.Command, args []string) {
	setPingFlags(cmd)
	cmd.Flag.Parse(args)

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

	// Extract outbound tags
	outbounds := extractOutbounds(config)
	if len(outbounds) == 0 {
		fmt.Println("No outbound nodes found in config")
		return
	}

	if *pingVerbose {
		fmt.Printf("Testing %d outbound node(s)...\n\n", len(outbounds))
	}

	// Test all outbounds in parallel
	testAllOutbounds(config, outbounds, *pingURL, *pingTimeout, *pingVerbose, *pingWarmup)
}

// pingClient is adapted from observatory/burst/ping.go
type pingClient struct {
	destination string
	httpClient  *http.Client
}

// newPingClient creates a ping client for a specific outbound handler
func newPingClient(ctx context.Context, destination string, timeout time.Duration, handler string) *pingClient {
	return &pingClient{
		destination: destination,
		httpClient:  newHTTPClient(ctx, handler, timeout),
	}
}

// newHTTPClient creates an HTTP client that routes through a specific outbound handler
// This is adapted from observatory/burst/ping.go
func newHTTPClient(ctxv context.Context, handler string, timeout time.Duration) *http.Client {
	tr := &http.Transport{
		DisableKeepAlives: true,
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			dest, err := net.ParseDestination(network + ":" + addr)
			if err != nil {
				return nil, err
			}
			// Use tagged dialer to route through specific outbound handler
			return tagged.Dialer(ctxv, dest, handler)
		},
	}
	return &http.Client{
		Transport: tr,
		Timeout:   timeout,
		// Don't follow redirect
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}

// MeasureDelay measures the delay time of the request to destination
// This is adapted from observatory/burst/ping.go
func (s *pingClient) MeasureDelay() (time.Duration, error) {
	if s.httpClient == nil {
		return 0, fmt.Errorf("pingClient not initialized")
	}
	// Use HEAD method to avoid downloading response body
	req, err := http.NewRequest(http.MethodHead, s.destination, nil)
	if err != nil {
		return 0, err
	}
	// Don't set User-Agent to match Observatory behavior
	
	start := time.Now()
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return 0, err
	}
	// Don't wait for body
	resp.Body.Close()
	return time.Since(start), nil
}

// OutboundInfo stores outbound node information
type OutboundInfo struct {
	Tag      string
	Protocol string
}

// PingResult stores ping test result
type PingResult struct {
	Tag     string
	Latency int64
	Error   error
}

// extractOutbounds extracts all outbound configurations from config
func extractOutbounds(config *core.Config) []OutboundInfo {
	var outbounds []OutboundInfo
	
	// Tags to skip (non-proxy outbounds)
	skipTags := map[string]bool{
		"block":     true,
		"blocked":   true,
		"reject":    true,
		"blackhole": true,
		"direct":    true,
		"dns":       true,
	}
	
	for _, outbound := range config.Outbound {
		if outbound.Tag == "" {
			continue
		}
		
		// Skip special outbounds
		tagLower := strings.ToLower(outbound.Tag)
		if skipTags[tagLower] {
			continue
		}
		
		// Skip tags containing "block" or "reject"
		if strings.Contains(tagLower, "block") || 
		   strings.Contains(tagLower, "reject") ||
		   strings.Contains(tagLower, "direct") {
			continue
		}
		
		outbounds = append(outbounds, OutboundInfo{
			Tag: outbound.Tag,
		})
	}
	return outbounds
}

// warmupOutbounds performs warmup requests to establish connections
func warmupOutbounds(ctx context.Context, outbounds []OutboundInfo, testURL string, timeout int, verbose bool) {
	var wg sync.WaitGroup
	for i, outbound := range outbounds {
		wg.Add(1)
		// Spread warmup requests
		delay := time.Duration(i*100) * time.Millisecond
		go func(ob OutboundInfo, d time.Duration) {
			defer wg.Done()
			if d > 0 {
				time.Sleep(d)
			}
			
			// Create ping client
			client := newPingClient(ctx, testURL, time.Duration(timeout)*time.Second, ob.Tag)
			
			// Perform warmup request (ignore result)
			_, err := client.MeasureDelay()
			if verbose && err != nil {
				fmt.Printf("  [%s] warmup failed: %v\n", ob.Tag, err)
			} else if verbose {
				fmt.Printf("  [%s] warmed up\n", ob.Tag)
			}
		}(outbound, delay)
	}
	wg.Wait()
}

// testAllOutbounds tests all outbound nodes in parallel
func testAllOutbounds(config *core.Config, outbounds []OutboundInfo, testURL string, timeout int, verbose bool, warmup bool) {
	// Create f2ray instance
	server, err := core.New(config)
	if err != nil {
		base.Fatalf("Failed to create instance: %s", err)
	}

	// Start instance
	if err := server.Start(); err != nil {
		base.Fatalf("Failed to start instance: %s", err)
	}
	defer server.Close()

	// Create context with server instance
	ctx := toContext(context.Background(), server)

	// Warmup phase: establish connections first
	if warmup {
		if verbose {
			fmt.Println("Warming up connections...")
		}
		warmupOutbounds(ctx, outbounds, testURL, timeout, verbose)
		if verbose {
			fmt.Println("Warmup complete. Starting actual tests...\n")
		}
	}

	results := make(chan PingResult, len(outbounds))
	var wg sync.WaitGroup

	// Add small delays to avoid all requests starting at the same time
	// This matches Observatory's behavior using time.AfterFunc
	for i, outbound := range outbounds {
		wg.Add(1)
		// Spread requests over 500ms to avoid connection congestion
		delay := time.Duration(i*100) * time.Millisecond
		go func(ob OutboundInfo, d time.Duration) {
			defer wg.Done()
			if d > 0 {
				time.Sleep(d)
			}
			latency, err := testSingleOutbound(ctx, ob.Tag, testURL, timeout)
			results <- PingResult{
				Tag:     ob.Tag,
				Latency: latency,
				Error:   err,
			}
		}(outbound, delay)
	}

	// Wait for all tests to complete
	go func() {
		wg.Wait()
		close(results)
	}()

	// Collect and display results
	var allResults []PingResult
	for result := range results {
		allResults = append(allResults, result)
	}

	// Sort by latency (successful ones first, then by latency)
	sort.Slice(allResults, func(i, j int) bool {
		if allResults[i].Error != nil && allResults[j].Error == nil {
			return false
		}
		if allResults[i].Error == nil && allResults[j].Error != nil {
			return true
		}
		if allResults[i].Error != nil && allResults[j].Error != nil {
			return allResults[i].Tag < allResults[j].Tag
		}
		return allResults[i].Latency < allResults[j].Latency
	})

	// Display results
	if verbose {
		fmt.Println("Results:")
		fmt.Println(strings.Repeat("-", 50))
	}
	for _, result := range allResults {
		if result.Error != nil {
			if verbose {
				fmt.Printf("[%-20s] ❌ Failed: %v\n", result.Tag, result.Error)
			} else {
				fmt.Printf("[%-20s] Failed\n", result.Tag)
			}
		} else {
			fmt.Printf("[%-20s] %d ms\n", result.Tag, result.Latency)
		}
	}
}

// testSingleOutbound tests a single outbound node using tagged dialer
func testSingleOutbound(ctx context.Context, outboundTag, testURL string, timeout int) (int64, error) {
	// Create ping client with tagged dialer
	client := newPingClient(ctx, testURL, time.Duration(timeout)*time.Second, outboundTag)
	
	// Measure delay
	delay, err := client.MeasureDelay()
	if err != nil {
		return 0, err
	}

	return delay.Milliseconds(), nil
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
