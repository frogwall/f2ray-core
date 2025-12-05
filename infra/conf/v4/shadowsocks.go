package v4

import (
	"encoding/base64"
	"strings"

	"github.com/golang/protobuf/proto"

	"github.com/frogwall/f2ray-core/v5/common/net"
	"github.com/frogwall/f2ray-core/v5/common/net/packetaddr"
	"github.com/frogwall/f2ray-core/v5/common/protocol"
	"github.com/frogwall/f2ray-core/v5/common/serial"
	"github.com/frogwall/f2ray-core/v5/infra/conf/cfgcommon"
	"github.com/frogwall/f2ray-core/v5/proxy/shadowsocks"
	"github.com/frogwall/f2ray-core/v5/proxy/shadowsocks2022"
)

type Shadowsocks2022User struct {
	Password string `json:"password"` // Base64 encoded user PSK
	Email    string `json:"email"`
	Level    byte   `json:"level"`
}

type ShadowsocksServerConfig struct {
	Cipher         string                 `json:"method"`
	Password       string                 `json:"password"`
	UDP            bool                   `json:"udp"`
	Level          byte                   `json:"level"`
	Email          string                 `json:"email"`
	NetworkList    *cfgcommon.NetworkList `json:"network"`
	IVCheck        bool                   `json:"ivCheck"`
	PacketEncoding string                 `json:"packetEncoding"`
	// For Shadowsocks-2022 multi-user
	Users []Shadowsocks2022User `json:"users"`
}

func (v *ShadowsocksServerConfig) Build() (proto.Message, error) {
	if v.Password == "" {
		return nil, newError("Shadowsocks password is not specified.")
	}

	// Check if this is a Shadowsocks-2022 method
	if isShadowsocks2022Method(v.Cipher) {
		return v.buildShadowsocks2022Config()
	}

	// Legacy Shadowsocks configuration
	config := new(shadowsocks.ServerConfig)
	config.UdpEnabled = v.UDP
	config.Network = v.NetworkList.Build()

	account := &shadowsocks.Account{
		Password: v.Password,
		IvCheck:  v.IVCheck,
	}
	account.CipherType = shadowsocks.CipherFromString(v.Cipher)
	if account.CipherType == shadowsocks.CipherType_UNKNOWN {
		return nil, newError("unknown cipher method: ", v.Cipher)
	}

	config.User = &protocol.User{
		Email:   v.Email,
		Level:   uint32(v.Level),
		Account: serial.ToTypedMessage(account),
	}

	switch v.PacketEncoding {
	case "Packet":
		config.PacketEncoding = packetaddr.PacketAddrType_Packet
	case "", "None":
		config.PacketEncoding = packetaddr.PacketAddrType_None
	}

	return config, nil
}

// isShadowsocks2022Method checks if the cipher method is a Shadowsocks-2022 method
func isShadowsocks2022Method(method string) bool {
	return strings.HasPrefix(method, "2022-blake3-")
}

// buildShadowsocks2022Config builds a Shadowsocks-2022 server configuration
func (v *ShadowsocksServerConfig) buildShadowsocks2022Config() (proto.Message, error) {
	config := new(shadowsocks2022.ServerConfig)
	config.Method = v.Cipher

	// Parse server PSK from password (base64 encoded or raw bytes)
	var psk []byte
	var err error

	// Try to decode as base64 first
	psk, err = base64.StdEncoding.DecodeString(v.Password)
	if err != nil {
		// If not base64, use raw password bytes
		psk = []byte(v.Password)
	}

	// Validate PSK length based on method
	expectedLen := 0
	switch v.Cipher {
	case "2022-blake3-aes-128-gcm":
		expectedLen = 16
	case "2022-blake3-aes-256-gcm":
		expectedLen = 32
	default:
		return nil, newError("unsupported Shadowsocks-2022 method: ", v.Cipher)
	}

	if len(psk) != expectedLen {
		return nil, newError("invalid server PSK length for ", v.Cipher, ": expected ", expectedLen, " bytes, got ", len(psk))
	}

	config.Psk = psk

	// Set network
	if v.NetworkList != nil {
		config.Network = v.NetworkList.Build()
	}
	if len(config.Network) == 0 {
		config.Network = []net.Network{net.Network_TCP}
	}
	if v.UDP {
		// Ensure UDP is in the network list
		hasUDP := false
		for _, n := range config.Network {
			if n == net.Network_UDP {
				hasUDP = true
				break
			}
		}
		if !hasUDP {
			config.Network = append(config.Network, net.Network_UDP)
		}
	}

	// Set packet encoding
	switch v.PacketEncoding {
	case "Packet":
		config.PacketEncoding = packetaddr.PacketAddrType_Packet
	case "", "None":
		config.PacketEncoding = packetaddr.PacketAddrType_None
	}

	// Parse users (multi-user mode)
	if len(v.Users) > 0 {
		// Multi-user mode
		for _, user := range v.Users {
			// Parse user PSK
			var userPsk []byte
			userPsk, err = base64.StdEncoding.DecodeString(user.Password)
			if err != nil {
				// If not base64, use raw password bytes
				userPsk = []byte(user.Password)
			}

			if len(userPsk) != expectedLen {
				return nil, newError("invalid user PSK length for ", user.Email, ": expected ", expectedLen, " bytes, got ", len(userPsk))
			}

			// Create account with user PSK
			account := &shadowsocks2022.Account{
				UserPsk: userPsk,
			}

			config.Users = append(config.Users, &protocol.User{
				Email:   user.Email,
				Level:   uint32(user.Level),
				Account: serial.ToTypedMessage(account),
			})
		}
	} else {
		// Single-user mode (backward compatibility)
		if v.Email != "" {
			// Create account with server PSK as user PSK
			account := &shadowsocks2022.Account{
				UserPsk: psk,
			}

			config.Users = append(config.Users, &protocol.User{
				Email:   v.Email,
				Level:   uint32(v.Level),
				Account: serial.ToTypedMessage(account),
			})
		}
	}

	return config, nil
}

type ShadowsocksServerTarget struct {
	Address  *cfgcommon.Address `json:"address"`
	Port     uint16             `json:"port"`
	Cipher   string             `json:"method"`
	Password string             `json:"password"`
	Email    string             `json:"email"`
	Ota      bool               `json:"ota"`
	Level    byte               `json:"level"`
	IVCheck  bool               `json:"ivCheck"`
}

type ShadowsocksClientConfig struct {
	Servers []*ShadowsocksServerTarget `json:"servers"`
}

func (v *ShadowsocksClientConfig) Build() (proto.Message, error) {
	config := new(shadowsocks.ClientConfig)

	if len(v.Servers) == 0 {
		return nil, newError("0 Shadowsocks server configured.")
	}

	serverSpecs := make([]*protocol.ServerEndpoint, len(v.Servers))
	for idx, server := range v.Servers {
		if server.Address == nil {
			return nil, newError("Shadowsocks server address is not set.")
		}
		if server.Port == 0 {
			return nil, newError("Invalid Shadowsocks port.")
		}
		if server.Password == "" {
			return nil, newError("Shadowsocks password is not specified.")
		}
		account := &shadowsocks.Account{
			Password: server.Password,
		}
		account.CipherType = shadowsocks.CipherFromString(server.Cipher)
		if account.CipherType == shadowsocks.CipherType_UNKNOWN {
			return nil, newError("unknown cipher method: ", server.Cipher)
		}

		account.IvCheck = server.IVCheck

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
