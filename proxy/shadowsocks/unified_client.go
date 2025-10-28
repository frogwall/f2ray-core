package shadowsocks

import (
	"context"
	"encoding/base64"

	core "github.com/frogwall/f2ray-core/v5"
	"github.com/frogwall/f2ray-core/v5/common"
	"github.com/frogwall/f2ray-core/v5/common/protocol"
	"github.com/frogwall/f2ray-core/v5/common/serial"
	"github.com/frogwall/f2ray-core/v5/features/policy"
	"github.com/frogwall/f2ray-core/v5/proxy/shadowsocks2022"
	"github.com/frogwall/f2ray-core/v5/transport"
	"github.com/frogwall/f2ray-core/v5/transport/internet"
)

// UnifiedClient is a unified client that supports both legacy Shadowsocks and Shadowsocks2022
type UnifiedClient struct {
	legacyClient  *Client
	ss2022Client  *shadowsocks2022.Client
	serverPicker  protocol.ServerPicker
	policyManager policy.Manager
	useSS2022     bool
}

// NewUnifiedClient creates a new unified Shadowsocks client
func NewUnifiedClient(ctx context.Context, config *ClientConfig) (*UnifiedClient, error) {
	serverList := protocol.NewServerList()
	for _, rec := range config.Server {
		s, err := protocol.NewServerSpecFromPB(rec)
		if err != nil {
			return nil, newError("failed to parse server spec").Base(err)
		}
		serverList.AddServer(s)
	}
	if serverList.Size() == 0 {
		return nil, newError("0 server")
	}

	v := core.MustFromContext(ctx)
	policyManager := v.GetFeature(policy.ManagerType()).(policy.Manager)

	// Check if any server uses Shadowsocks2022
	useSS2022 := false
	for _, server := range config.Server {
		user := server.User[0]
		accountMsg, err := serial.GetInstanceOf(user.Account)
		if err != nil {
			continue
		}
		account := accountMsg.(*Account)
		if isSS2022Cipher(account.CipherType) {
			useSS2022 = true
			break
		}
	}

	if useSS2022 {
		// Create Shadowsocks2022 client
		ss2022Config := convertToSS2022Config(config)
		ss2022Client, err := shadowsocks2022.NewClient(ctx, ss2022Config)
		if err != nil {
			return nil, newError("failed to create Shadowsocks2022 client").Base(err)
		}
		return &UnifiedClient{
			ss2022Client:  ss2022Client,
			serverPicker:  protocol.NewRoundRobinServerPicker(serverList),
			policyManager: policyManager,
			useSS2022:     true,
		}, nil
	} else {
		// Create legacy Shadowsocks client
		legacyClient, err := NewClient(ctx, config)
		if err != nil {
			return nil, newError("failed to create legacy Shadowsocks client").Base(err)
		}
		return &UnifiedClient{
			legacyClient:  legacyClient,
			serverPicker:  protocol.NewRoundRobinServerPicker(serverList),
			policyManager: policyManager,
			useSS2022:     false,
		}, nil
	}
}

// Process implements OutboundHandler.Process()
func (c *UnifiedClient) Process(ctx context.Context, link *transport.Link, dialer internet.Dialer) error {
	if c.useSS2022 {
		return c.ss2022Client.Process(ctx, link, dialer)
	} else {
		return c.legacyClient.Process(ctx, link, dialer)
	}
}

// isSS2022Cipher checks if the cipher type is a Shadowsocks2022 method
func isSS2022Cipher(cipher CipherType) bool {
	return cipher == CipherType_SS2022_BLAKE3_AES_128_GCM ||
		cipher == CipherType_SS2022_BLAKE3_AES_256_GCM
}

// convertToSS2022Config converts legacy Shadowsocks config to Shadowsocks2022 config
func convertToSS2022Config(config *ClientConfig) *shadowsocks2022.ClientConfig {
	if len(config.Server) == 0 {
		return nil
	}

	// Use the first server for now (can be extended to support multiple servers)
	server := config.Server[0]
	user := server.User[0]
	accountMsg, err := serial.GetInstanceOf(user.Account)
	if err != nil {
		return nil
	}
	account := accountMsg.(*Account)

	// Try to decode as base64 first, fallback to raw password
	var psk []byte
	if decoded, err := base64.StdEncoding.DecodeString(account.Password); err == nil {
		psk = decoded
	} else {
		// If base64 decoding fails, use password as raw bytes
		psk = []byte(account.Password)
	}

	// Determine method based on cipher type
	var method string
	switch account.CipherType {
	case CipherType_SS2022_BLAKE3_AES_128_GCM:
		method = "2022-blake3-aes-128-gcm"
	case CipherType_SS2022_BLAKE3_AES_256_GCM:
		method = "2022-blake3-aes-256-gcm"
	default:
		method = "2022-blake3-aes-128-gcm" // fallback
	}

	return &shadowsocks2022.ClientConfig{
		Method:  method,
		Psk:     psk,
		Address: server.Address,
		Port:    server.Port,
	}
}

func init() {
	common.Must(common.RegisterConfig((*ClientConfig)(nil), func(ctx context.Context, config interface{}) (interface{}, error) {
		return NewUnifiedClient(ctx, config.(*ClientConfig))
	}))
}
