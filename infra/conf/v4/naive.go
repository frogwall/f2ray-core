package v4

import (
	"net/url"
	"strconv"

	"github.com/golang/protobuf/proto"

	naive "github.com/frogwall/v2ray-core/v5/proxy/naive"
)

type NaiveRemoteConfig struct {
	// Address can be either a string URL (e.g., "https://user:pass@host:443")
	// or a structured address object compatible with cfgcommon.Address.
	Address  string `json:"address"`
	Port     uint16 `json:"port"`
	Username string `json:"username"`
	Password string `json:"password"`
}

type NaiveClientConfig struct {
	Servers []*NaiveRemoteConfig `json:"servers"`
}

func (v *NaiveClientConfig) Build() (proto.Message, error) {
	if len(v.Servers) == 0 {
		return nil, newError("no naive servers configured")
	}

	config := new(naive.ClientConfig)
	config.Servers = make([]*naive.NaiveServerEndpoint, len(v.Servers))

	for idx, serverConfig := range v.Servers {
		if serverConfig.Address == "" {
			return nil, newError("naive server address is required")
		}

		naiveServer := &naive.NaiveServerEndpoint{}

		// Check if address is a URL format
		if u, err := url.Parse(serverConfig.Address); err == nil && u.Scheme != "" && u.Host != "" {
			// URL format: https://user:pass@host:port
			host := u.Hostname()
			if host == "" {
				return nil, newError("invalid hostname in naive server url")
			}

			portStr := u.Port()
			port := 0
			if portStr == "" {
				// Default port based on scheme
				if u.Scheme == "https" || u.Scheme == "quic" {
					port = 443
				} else {
					port = 80
				}
			} else {
				p, err := strconv.Atoi(portStr)
				if err != nil {
					return nil, newError("invalid port in naive server url").Base(err).AtError()
				}
				if p <= 0 || p > 65535 {
					return nil, newError("port out of range: ", p)
				}
				port = p
			}
			naiveServer.Address = host
			naiveServer.Port = uint32(port)
			if u.User != nil {
				naiveServer.Username = u.User.Username()
				naiveServer.Password, _ = u.User.Password()
			}
		} else {
			// Separate fields format: address, port, username, password
			naiveServer.Address = serverConfig.Address
			if serverConfig.Port == 0 {
				naiveServer.Port = 443 // Default HTTPS port
			} else {
				naiveServer.Port = uint32(serverConfig.Port)
			}
			naiveServer.Username = serverConfig.Username
			naiveServer.Password = serverConfig.Password
		}

		config.Servers[idx] = naiveServer
	}
	return config, nil
}
