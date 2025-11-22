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

type TUICTLSConfig struct {
	ServerName    string   `json:"serverName"`
	ALPN          []string `json:"alpn"`
	AllowInsecure bool     `json:"allowInsecure"`
}

type TUICClientConfig struct {
	Servers               []*TUICServerTarget `json:"servers"`
	UdpRelayMode          string              `json:"udpRelayMode"`
	CongestionControl     string              `json:"congestionControl"`
	ReduceRtt             bool                `json:"reduceRtt"`
	MaxUdpRelayPacketSize int32               `json:"maxUdpRelayPacketSize"`
	QUIC                  *TUICQUICConfig     `json:"quic"`
	TLS                   *TUICTLSConfig      `json:"tls"`
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
	config := &tuic.ClientConfig{
		UdpRelayMode:          c.UdpRelayMode,
		CongestionControl:     c.CongestionControl,
		ReduceRtt:             c.ReduceRtt,
		MaxUdpRelayPacketSize: c.MaxUdpRelayPacketSize,
	}

	if c.Servers != nil {
		for _, server := range c.Servers {
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
			config.Server = append(config.Server, ss)
		}
	}

	if c.QUIC != nil {
		config.Quic = &tuic.QUICConfig{
			InitialStreamReceiveWindow:     uint64(c.QUIC.InitialStreamReceiveWindow),
			MaxStreamReceiveWindow:         uint64(c.QUIC.MaxStreamReceiveWindow),
			InitialConnectionReceiveWindow: uint64(c.QUIC.InitialConnectionReceiveWindow),
			MaxConnectionReceiveWindow:     uint64(c.QUIC.MaxConnectionReceiveWindow),
			MaxIdleTimeout:                 int64(c.QUIC.MaxIdleTimeout),
			KeepAlivePeriod:                int64(c.QUIC.KeepAlivePeriod),
			DisablePathMtuDiscovery:        c.QUIC.DisablePathMTUDiscovery,
		}
	}

	if c.TLS != nil {
		config.Tls = &tuic.TLSConfig{
			ServerName:    c.TLS.ServerName,
			Alpn:          c.TLS.ALPN,
			AllowInsecure: c.TLS.AllowInsecure,
		}
	}

	return config, nil
}
