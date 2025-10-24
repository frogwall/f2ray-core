package v4

import (
	"github.com/golang/protobuf/proto"
	"github.com/frogwall/f2ray-core/v5/proxy/mieru"
)

type MieruServer struct {
	Address  string `json:"address"`
	Port     int    `json:"port"`
	Username string `json:"username"`
	Password string `json:"password"`
}

type MieruClientConfig struct {
	Servers []*MieruServer `json:"servers"`
	MTU     int            `json:"mtu"`
}

func (v *MieruClientConfig) Build() (proto.Message, error) {
	config := &mieru.ClientConfig{
		Mtu: int32(v.MTU),
	}

	for _, server := range v.Servers {
		serverConfig := &mieru.Server{
			Address:  server.Address,
			Port:     int32(server.Port),
			Username: server.Username,
			Password: server.Password,
		}
		config.Servers = append(config.Servers, serverConfig)
	}

	return config, nil
}
