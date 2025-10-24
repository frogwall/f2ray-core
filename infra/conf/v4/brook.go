package v4

import (
	"github.com/golang/protobuf/proto"

	"github.com/frogwall/v2ray-core/v5/common/protocol"
	"github.com/frogwall/v2ray-core/v5/common/serial"
	"github.com/frogwall/v2ray-core/v5/infra/conf/cfgcommon"
	"github.com/frogwall/v2ray-core/v5/proxy/brook"
)

// BrookServerTarget is configuration of a single brook server
type BrookServerTarget struct {
	Address  *cfgcommon.Address `json:"address"`
	Port     uint16             `json:"port"`
	Email    string             `json:"email"`
	Level    byte               `json:"level"`
	Password string             `json:"password"`
	Method   string             `json:"method"` // tcp, ws, wss, quic
}

// BrookClientConfig is configuration of brook servers
type BrookClientConfig struct {
	Servers        []*BrookServerTarget `json:"servers"`
	Password       string               `json:"password"`
	WithoutBrook   bool                 `json:"withoutBrook"`   // for compatibility
	Path           string               `json:"path"`           // for websocket
	TLSFingerprint string               `json:"tlsFingerprint"` // for websocket
}

// Build implements Buildable
func (c *BrookClientConfig) Build() (proto.Message, error) {
	config := new(brook.ClientConfig)

	if len(c.Servers) == 0 {
		return nil, newError("0 Brook server configured.")
	}

	serverSpecs := make([]*protocol.ServerEndpoint, len(c.Servers))
	for idx, rec := range c.Servers {
		if rec.Address == nil {
			return nil, newError("Brook server address is not set.")
		}
		if rec.Port == 0 {
			return nil, newError("Invalid Brook port.")
		}
		account := &brook.Account{
			Password: rec.Password,
		}
		method := rec.Method
		if method == "" {
			method = "tcp" // default method
		}
		brookServer := &protocol.ServerEndpoint{
			Address: rec.Address.Build(),
			Port:    uint32(rec.Port),
			Method:  method,
			User: []*protocol.User{
				{
					Level:   uint32(rec.Level),
					Email:   rec.Email,
					Account: serial.ToTypedMessage(account),
				},
			},
		}

		serverSpecs[idx] = brookServer
	}

	config.Server = serverSpecs

	// Set global password if provided
	if c.Password != "" {
		config.Password = c.Password
	}

	// Set other options
	config.WithoutBrook = c.WithoutBrook
	config.Path = c.Path
	config.TlsFingerprint = c.TLSFingerprint

	return config, nil
}

// BrookServerConfig is Inbound configuration
type BrookServerConfig struct {
	Password string `json:"password"`
}

// Build implements Buildable
func (c *BrookServerConfig) Build() (proto.Message, error) {
	config := new(brook.ServerConfig)
	config.Password = c.Password
	return config, nil
}
