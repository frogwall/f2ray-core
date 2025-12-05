package v4

import (
	"github.com/frogwall/f2ray-core/v5/transport/internet/shadowtls"
	"github.com/golang/protobuf/proto"
)

type ShadowTLSConfig struct {
	Version         uint32                `json:"version"`
	Password        string                `json:"password"`
	HandshakeServer string                `json:"handshakeServer"`
	HandshakePort   uint32                `json:"handshakePort"`
	StrictMode      bool                  `json:"strictMode"`
	Users           []ShadowTLSUserConfig `json:"users"`
}

type ShadowTLSUserConfig struct {
	Name     string `json:"name"`
	Password string `json:"password"`
}

func (c *ShadowTLSConfig) Build() (proto.Message, error) {
	config := &shadowtls.Config{
		Version:         c.Version,
		Password:        c.Password,
		HandshakeServer: c.HandshakeServer,
		HandshakePort:   c.HandshakePort,
		StrictMode:      c.StrictMode,
	}

	if len(c.Users) > 0 {
		config.Users = make([]*shadowtls.User, len(c.Users))
		for i, u := range c.Users {
			config.Users[i] = &shadowtls.User{
				Name:     u.Name,
				Password: u.Password,
			}
		}
	}

	return config, nil
}
