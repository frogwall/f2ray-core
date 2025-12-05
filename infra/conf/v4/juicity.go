package v4

import (
	"github.com/golang/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"

	"github.com/frogwall/f2ray-core/v5/common/protocol"
	"github.com/frogwall/f2ray-core/v5/infra/conf/cfgcommon"
	"github.com/frogwall/f2ray-core/v5/proxy/juicity"
)

// JuicityServerTarget is configuration of a single juicity server
type JuicityServerTarget struct {
	Address  *cfgcommon.Address `json:"address"`
	Port     uint16             `json:"port"`
	Email    string             `json:"email"`
	Level    byte               `json:"level"`
	Username string             `json:"username"` // UUID
	Password string             `json:"password"`
}

// JuicityClientConfig is configuration of juicity client
type JuicityClientConfig struct {
	Servers               []*JuicityServerTarget `json:"servers"`
	CongestionControl     string                 `json:"congestion_control"`
	PinnedCertchainSha256 string                 `json:"pinned_certchain_sha256"`
}

// Build implements Buildable
func (c *JuicityClientConfig) Build() (proto.Message, error) {
	config := new(juicity.ClientConfig)

	if len(c.Servers) == 0 {
		return nil, newError("0 Juicity server configured.")
	}

	serverSpecs := make([]*protocol.ServerEndpoint, len(c.Servers))
	for idx, rec := range c.Servers {
		if rec.Address == nil {
			return nil, newError("Juicity server address is not set.")
		}
		if rec.Port == 0 {
			return nil, newError("Invalid Juicity port.")
		}
		if rec.Username == "" {
			return nil, newError("Juicity username (UUID) is not set.")
		}
		if rec.Password == "" {
			return nil, newError("Juicity password is not set.")
		}

		// Create server endpoint
		// Password is stored as raw bytes in Account.Value
		juicityServer := &protocol.ServerEndpoint{
			Address: rec.Address.Build(),
			Port:    uint32(rec.Port),
			User: []*protocol.User{
				{
					Level: uint32(rec.Level),
					Email: rec.Username, // Store UUID in Email field
					Account: &anypb.Any{
						Value: []byte(rec.Password),
					},
				},
			},
		}

		serverSpecs[idx] = juicityServer
	}

	config.Server = serverSpecs

	// Set congestion control
	if c.CongestionControl != "" {
		config.CongestionControl = c.CongestionControl
	}

	// Set pinned certificate chain hash
	if c.PinnedCertchainSha256 != "" {
		config.PinnedCertchainSha256 = c.PinnedCertchainSha256
	}

	return config, nil
}
