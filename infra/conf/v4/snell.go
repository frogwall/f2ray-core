package v4

import (
	"github.com/golang/protobuf/proto"

	"github.com/frogwall/f2ray-core/v5/common/protocol"
	"github.com/frogwall/f2ray-core/v5/common/serial"
	"github.com/frogwall/f2ray-core/v5/infra/conf/cfgcommon"
	"github.com/frogwall/f2ray-core/v5/proxy/snell"
)

type SnellServerConfig struct {
	Cipher   string `json:"method"`
	Password string `json:"password"`
	Level    byte   `json:"level"`
	Email    string `json:"email"`
}

func (v *SnellServerConfig) Build() (proto.Message, error) {
	config := new(snell.ServerConfig)

	if v.Password == "" {
		return nil, newError("Snell password is not specified.")
	}
	if v.Cipher == "" {
		return nil, newError("Snell cipher is not specified.")
	}

	account := &snell.Account{
		Password: v.Password,
		Cipher:   v.Cipher,
	}

	config.User = &protocol.User{
		Email:   v.Email,
		Level:   uint32(v.Level),
		Account: serial.ToTypedMessage(account),
	}

	return config, nil
}

type SnellServerTarget struct {
	Address  *cfgcommon.Address `json:"address"`
	Port     uint16             `json:"port"`
	Cipher   string             `json:"method"`
	Password string             `json:"password"`
	Email    string             `json:"email"`
	Level    byte               `json:"level"`
}

type SnellClientConfig struct {
	Servers []*SnellServerTarget `json:"servers"`
}

func (v *SnellClientConfig) Build() (proto.Message, error) {
	config := new(snell.ClientConfig)

	if len(v.Servers) == 0 {
		return nil, newError("0 Snell server configured.")
	}

	serverSpecs := make([]*protocol.ServerEndpoint, len(v.Servers))
	for idx, server := range v.Servers {
		if server.Address == nil {
			return nil, newError("Snell server address is not set.")
		}
		if server.Port == 0 {
			return nil, newError("Invalid Snell port.")
		}
		if server.Password == "" {
			return nil, newError("Snell password is not specified.")
		}
		if server.Cipher == "" {
			return nil, newError("Snell cipher is not specified.")
		}

		account := &snell.Account{
			Password: server.Password,
			Cipher:   server.Cipher,
		}

		ss := &protocol.ServerEndpoint{
			Address: server.Address.Build(),
			Port:    uint32(server.Port),
			User: []*protocol.User{
				{
					Level:   uint32(server.Level),
					Email:   server.Email,
					Account: serial.ToTypedMessage(account),
				},
			},
		}

		serverSpecs[idx] = ss
	}

	config.Server = serverSpecs

	return config, nil
}
