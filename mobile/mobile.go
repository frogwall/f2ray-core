package mobile

import (
	"context"
	"fmt"
	"strings"

	core "github.com/v2fly/v2ray-core/v5"
	"github.com/v2fly/v2ray-core/v5/common/serial"
	_ "github.com/v2fly/v2ray-core/v5/main/distro/all"
)

// V2RayInstance represents a V2Ray instance
type V2RayInstance struct {
	instance *core.Instance
	ctx      context.Context
	cancel   context.CancelFunc
}

// StartV2Ray starts V2Ray with JSON configuration
// Returns nil on success, error message on failure
func StartV2Ray(configJSON string) (*V2RayInstance, error) {
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

	// Create V2Ray instance
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

	return &V2RayInstance{
		instance: instance,
		ctx:      ctx,
		cancel:   cancel,
	}, nil
}

// Stop stops the V2Ray instance
func (v *V2RayInstance) Stop() error {
	if v.instance != nil {
		err := v.instance.Close()
		if v.cancel != nil {
			v.cancel()
		}
		return err
	}
	return nil
}

// GetVersion returns V2Ray version
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
func (v *V2RayInstance) QueryStats(pattern string) (string, error) {
	if v.instance == nil {
		return "", fmt.Errorf("instance is not running")
	}
	// TODO: Implement stats query
	return "{}", nil
}
