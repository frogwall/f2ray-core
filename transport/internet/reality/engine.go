package reality

import (
	"context"

	"github.com/frogwall/f2ray-core/v5/common/net"
	"github.com/frogwall/f2ray-core/v5/transport/internet/security"
)

type Engine struct {
	config *Config
}

func (e *Engine) Client(conn net.Conn, opts ...security.Option) (security.Conn, error) {
	// Client-only REALITY: perform outbound handshake and return wrapped conn
	c, err := UClient(context.Background(), conn, e.config)
	if err != nil {
		return nil, err
	}
	return c, nil
}

func NewRealitySecurityEngineFromConfig(config *Config) (security.Engine, error) {
	return &Engine{config: config}, nil
}
