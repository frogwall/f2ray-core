package mobile

import (
	"context"
	"fmt"
	"strings"

	core "github.com/frogwall/f2ray-core/v5"
	"github.com/frogwall/f2ray-core/v5/infra/conf/serial"
	_ "github.com/frogwall/f2ray-core/v5/main/distro/all"
)

// F2RayInstance represents a F2Ray instance
type F2RayInstance struct {
	instance *core.Instance
	ctx      context.Context
	cancel   context.CancelFunc
}

// StartF2Ray starts F2Ray with JSON configuration
// Returns nil on success, error message on failure
func StartF2Ray(configJSON string) (*F2RayInstance, error) {
	if configJSON == "" {
		return nil, fmt.Errorf("config is empty")
	}

	// Parse JSON config
	config, err := serial.LoadJSONConfig(strings.NewReader(configJSON))
	if err != nil {
		return nil, fmt.Errorf("failed to parse config: %v", err)
	}

	// Create context
	ctx, cancel := context.WithCancel(context.Background())

	// Create F2Ray instance
	instance, err := core.NewWithContext(ctx, config)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to create instance: %v", err)
	}

	// Start instance
	if err := instance.Start(); err != nil {
		cancel()
		return nil, fmt.Errorf("failed to start instance: %v", err)
	}

	return &F2RayInstance{
		instance: instance,
		ctx:      ctx,
		cancel:   cancel,
	}, nil
}

// Stop stops the F2Ray instance
func (v *F2RayInstance) Stop() error {
	if v.instance != nil {
		err := v.instance.Close()
		if v.cancel != nil {
			v.cancel()
		}
		return err
	}
	return nil
}

// GetVersion returns F2Ray version
func GetVersion() string {
	return core.Version()
}

// TestConfig tests if the configuration is valid
// Returns empty string on success, error message on failure
func TestConfig(configJSON string) string {
	if configJSON == "" {
		return "config is empty"
	}

	_, err := serial.LoadJSONConfig(strings.NewReader(configJSON))
	if err != nil {
		return err.Error()
	}
	return ""
}

// QueryStats queries statistics by pattern
// Returns statistics in JSON format
func (v *F2RayInstance) QueryStats(pattern string) (string, error) {
	if v.instance == nil {
		return "", fmt.Errorf("instance is not running")
	}
	// TODO: Implement stats query
	return "{}", nil
}
