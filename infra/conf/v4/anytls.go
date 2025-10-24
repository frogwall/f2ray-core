package v4

import (
	"github.com/golang/protobuf/proto"

	anytls "github.com/v2fly/v2ray-core/v5/proxy/anytls"
)

// AnyTLSServerConfig represents the JSON configuration for an AnyTLS server
type AnyTLSServerConfig struct {
	Address  string `json:"address"`
	Port     uint16 `json:"port"`
	Password string `json:"password"`
}

// AnyTLSClientConfig represents the JSON configuration for AnyTLS outbound
type AnyTLSClientConfig struct {
	Servers                  []*AnyTLSServerConfig `json:"servers"`
	IdleSessionCheckInterval uint32                `json:"idle_session_check_interval,omitempty"`
	IdleSessionTimeout       uint32                `json:"idle_session_timeout,omitempty"`
	MinIdleSession           uint32                `json:"min_idle_session,omitempty"`
}

// Build implements Buildable interface
func (v *AnyTLSClientConfig) Build() (proto.Message, error) {
	if len(v.Servers) == 0 {
		return nil, newError("no anytls servers configured")
	}

	config := new(anytls.ClientConfig)
	config.Servers = make([]*anytls.ServerEndpoint, len(v.Servers))

	for idx, serverConfig := range v.Servers {
		if serverConfig.Address == "" {
			return nil, newError("anytls server address is required")
		}
		if serverConfig.Password == "" {
			return nil, newError("anytls server password is required")
		}

		anytlsServer := &anytls.ServerEndpoint{
			Address:  serverConfig.Address,
			Port:     uint32(serverConfig.Port),
			Password: serverConfig.Password,
		}

		if anytlsServer.Port == 0 {
			anytlsServer.Port = 443 // Default HTTPS port
		}

		config.Servers[idx] = anytlsServer
	}

	// Set session management parameters with defaults
	if v.IdleSessionCheckInterval > 0 {
		config.IdleSessionCheckInterval = v.IdleSessionCheckInterval
	} else {
		config.IdleSessionCheckInterval = 30 // Default 30 seconds
	}

	if v.IdleSessionTimeout > 0 {
		config.IdleSessionTimeout = v.IdleSessionTimeout
	} else {
		config.IdleSessionTimeout = 30 // Default 30 seconds
	}

	config.MinIdleSession = v.MinIdleSession // Default 0

	return config, nil
}
