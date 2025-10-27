package reality

import (
	"context"

	"github.com/frogwall/f2ray-core/v5/common"
)

func init() {
	// Register so security.CreateSecurityEngineFromSettings can instantiate engine from generated *Config
	common.Must(common.RegisterConfig((*Config)(nil), func(ctx context.Context, config interface{}) (interface{}, error) {
		return NewRealitySecurityEngineFromConfig(config.(*Config))
	}))
}
