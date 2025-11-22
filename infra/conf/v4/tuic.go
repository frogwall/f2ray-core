package v4

import (
	"github.com/golang/protobuf/proto"

	"github.com/frogwall/f2ray-core/v5/common/protocol"
	"github.com/frogwall/f2ray-core/v5/common/serial"
	"github.com/frogwall/f2ray-core/v5/infra/conf/cfgcommon"
	"github.com/frogwall/f2ray-core/v5/proxy/tuic"
)

type TUICServerTarget struct {
	Address  *cfgcommon.Address `json:"address"`
	Port     uint16             `json:"port"`
	UUID     string             `json:"uuid"`
	Password string             `json:"password"`
}

type TUICClientConfig struct {
	Servers               []*TUICServerTarget `json:"servers"`
	UDPRelayMode          string              `json:"udpRelayMode"`
	CongestionControl     string              `json:"congestionControl"`
	ReduceRTT             bool                `json:"reduceRtt"`
	MaxUDPRelayPacketSize int32               `json:"maxUdpRelayPacketSize"`
	QUIC                  *TUICQUICConfig     `json:"quic"`
}

type TUICQUICConfig struct {
	InitialStreamReceiveWindow     uint64 `json:"initialStreamReceiveWindow"`
	MaxStreamReceiveWindow         uint64 `json:"maxStreamReceiveWindow"`
	InitialConnectionReceiveWindow uint64 `json:"initialConnectionReceiveWindow"`
	MaxConnectionReceiveWindow     uint64 `json:"maxConnectionReceiveWindow"`
	MaxIdleTimeout                 int64  `json:"maxIdleTimeout"`
	KeepAlivePeriod                int64  `json:"keepAlivePeriod"`
	DisablePathMTUDiscovery        bool   `json:"disablePathMtuDiscovery"`
}

func (c *TUICClientConfig) Build() (proto.Message, error) {
	config := new(tuic.ClientConfig)

	if len(c.Servers) == 0 {
		return nil, newError("0 TUIC server configured.")
	}

	serverSpecs := make([]*protocol.ServerEndpoint, len(c.Servers))
	for idx, server := range c.Servers {
		if server.Address == nil {
			return nil, newError("TUIC server address is not set.")
		}
		if server.Port == 0 {
			return nil, newError("Invalid TUIC port.")
		}
		if server.UUID == "" {
			return nil, newError("TUIC UUID is not specified.")
		}
		if server.Password == "" {
			return nil, newError("TUIC password is not specified.")
		}

		account := &tuic.Account{
			Uuid:     server.UUID,
			Password: server.Password,
		}

		ss := &protocol.ServerEndpoint{
			Address: server.Address.Build(),
			Port:    uint32(server.Port),
			User: []*protocol.User{
				{
					Account: serial.ToTypedMessage(account),
				},
			},
		}

		serverSpecs[idx] = ss
	}

	config.Server = serverSpecs

	// Set UDP relay mode
	if c.UDPRelayMode != "" {
		config.UdpRelayMode = c.UDPRelayMode
	} else {
		config.UdpRelayMode = "native" // default
	}

	// Set congestion control
	if c.CongestionControl != "" {
		config.CongestionControl = c.CongestionControl
	} else {
		config.CongestionControl = "bbr" // default
	}

	// Set reduce RTT
	config.ReduceRtt = c.ReduceRTT

	// Set max UDP relay packet size
	if c.MaxUDPRelayPacketSize > 0 {
		config.MaxUdpRelayPacketSize = c.MaxUDPRelayPacketSize
	} else {
		config.MaxUdpRelayPacketSize = 1400 // default
	}

	// Set QUIC config if provided
	if c.QUIC != nil {
		config.Quic = &tuic.QUICConfig{
			InitialStreamReceiveWindow:     c.QUIC.InitialStreamReceiveWindow,
			MaxStreamReceiveWindow:         c.QUIC.MaxStreamReceiveWindow,
			InitialConnectionReceiveWindow: c.QUIC.InitialConnectionReceiveWindow,
			MaxConnectionReceiveWindow:     c.QUIC.MaxConnectionReceiveWindow,
			MaxIdleTimeout:                 c.QUIC.MaxIdleTimeout,
			KeepAlivePeriod:                c.QUIC.KeepAlivePeriod,
			DisablePathMtuDiscovery:        c.QUIC.DisablePathMTUDiscovery,
		}
	}

	return config, nil
}
