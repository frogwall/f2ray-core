package v4

import (
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"strings"

	"github.com/golang/protobuf/proto"

	"github.com/frogwall/f2ray-core/v5/common/protocol"
	"github.com/frogwall/f2ray-core/v5/common/serial"
	"github.com/frogwall/f2ray-core/v5/infra/conf/cfgcommon"
	"github.com/frogwall/f2ray-core/v5/infra/conf/cfgcommon/loader"
	"github.com/frogwall/f2ray-core/v5/infra/conf/cfgcommon/socketcfg"
	"github.com/frogwall/f2ray-core/v5/infra/conf/cfgcommon/tlscfg"
	"github.com/frogwall/f2ray-core/v5/transport/internet"
	"github.com/frogwall/f2ray-core/v5/transport/internet/domainsocket"
	httpheader "github.com/frogwall/f2ray-core/v5/transport/internet/headers/http"
	"github.com/frogwall/f2ray-core/v5/transport/internet/http"
	"github.com/frogwall/f2ray-core/v5/transport/internet/hysteria2"
	"github.com/frogwall/f2ray-core/v5/transport/internet/kcp"
	"github.com/frogwall/f2ray-core/v5/transport/internet/quic"
	"github.com/frogwall/f2ray-core/v5/transport/internet/tcp"
	"github.com/frogwall/f2ray-core/v5/transport/internet/websocket"
	reality "github.com/frogwall/f2ray-core/v5/transport/internet/reality"
)

var (
	kcpHeaderLoader = loader.NewJSONConfigLoader(loader.ConfigCreatorCache{
		"none":         func() interface{} { return new(NoOpAuthenticator) },
		"srtp":         func() interface{} { return new(SRTPAuthenticator) },
		"utp":          func() interface{} { return new(UTPAuthenticator) },
		"wechat-video": func() interface{} { return new(WechatVideoAuthenticator) },
		"dtls":         func() interface{} { return new(DTLSAuthenticator) },
		"wireguard":    func() interface{} { return new(WireguardAuthenticator) },
	}, "type", "")

	tcpHeaderLoader = loader.NewJSONConfigLoader(loader.ConfigCreatorCache{
		"none": func() interface{} { return new(NoOpConnectionAuthenticator) },
		"http": func() interface{} { return new(Authenticator) },
	}, "type", "")
)

type KCPConfig struct {
	Mtu             *uint32         `json:"mtu"`
	Tti             *uint32         `json:"tti"`
	UpCap           *uint32         `json:"uplinkCapacity"`
	DownCap         *uint32         `json:"downlinkCapacity"`
	Congestion      *bool           `json:"congestion"`
	ReadBufferSize  *uint32         `json:"readBufferSize"`
	WriteBufferSize *uint32         `json:"writeBufferSize"`
	HeaderConfig    json.RawMessage `json:"header"`
	Seed            *string         `json:"seed"`
}

// Build implements Buildable.
func (c *KCPConfig) Build() (proto.Message, error) {
	config := new(kcp.Config)

	if c.Mtu != nil {
		mtu := *c.Mtu
		if mtu < 576 || mtu > 1460 {
			return nil, newError("invalid mKCP MTU size: ", mtu).AtError()
		}
		config.Mtu = &kcp.MTU{Value: mtu}
	}
	if c.Tti != nil {
		tti := *c.Tti
		if tti < 10 || tti > 100 {
			return nil, newError("invalid mKCP TTI: ", tti).AtError()
		}
		config.Tti = &kcp.TTI{Value: tti}
	}
	if c.UpCap != nil {
		config.UplinkCapacity = &kcp.UplinkCapacity{Value: *c.UpCap}
	}
	if c.DownCap != nil {
		config.DownlinkCapacity = &kcp.DownlinkCapacity{Value: *c.DownCap}
	}
	if c.Congestion != nil {
		config.Congestion = *c.Congestion
	}
	if c.ReadBufferSize != nil {
		size := *c.ReadBufferSize
		if size > 0 {
			config.ReadBuffer = &kcp.ReadBuffer{Size: size * 1024 * 1024}
		} else {
			config.ReadBuffer = &kcp.ReadBuffer{Size: 512 * 1024}
		}
	}
	if c.WriteBufferSize != nil {
		size := *c.WriteBufferSize
		if size > 0 {
			config.WriteBuffer = &kcp.WriteBuffer{Size: size * 1024 * 1024}
		} else {
			config.WriteBuffer = &kcp.WriteBuffer{Size: 512 * 1024}
		}
	}
	if len(c.HeaderConfig) > 0 {
		headerConfig, _, err := kcpHeaderLoader.Load(c.HeaderConfig)
		if err != nil {
			return nil, newError("invalid mKCP header config.").Base(err).AtError()
		}
		ts, err := headerConfig.(cfgcommon.Buildable).Build()
		if err != nil {
			return nil, newError("invalid mKCP header config").Base(err).AtError()
		}
		config.HeaderConfig = serial.ToTypedMessage(ts)
	}

	if c.Seed != nil {
		config.Seed = &kcp.EncryptionSeed{Seed: *c.Seed}
	}

	return config, nil
}

type TCPConfig struct {
	HeaderConfig        json.RawMessage `json:"header"`
	AcceptProxyProtocol bool            `json:"acceptProxyProtocol"`
}

// Build implements Buildable.
func (c *TCPConfig) Build() (proto.Message, error) {
	config := new(tcp.Config)
	if len(c.HeaderConfig) > 0 {
		headerConfig, _, err := tcpHeaderLoader.Load(c.HeaderConfig)
		if err != nil {
			return nil, newError("invalid TCP header config").Base(err).AtError()
		}
		ts, err := headerConfig.(cfgcommon.Buildable).Build()
		if err != nil {
			return nil, newError("invalid TCP header config").Base(err).AtError()
		}
		config.HeaderSettings = serial.ToTypedMessage(ts)
	}
	if c.AcceptProxyProtocol {
		config.AcceptProxyProtocol = c.AcceptProxyProtocol
	}
	return config, nil
}

type Hy2ConfigCongestion struct {
	Type     string `json:"type"`
	UpMbps   uint64 `json:"up_mbps"`
	DownMbps uint64 `json:"down_mbps"`
}

type Hy2Config struct {
	Congestion            Hy2ConfigCongestion   `json:"congestion"`
	UseUDPExtension       bool                  `json:"use_udp_extension"`
	IgnoreClientBandwidth bool                  `json:"ignore_client_bandwidth"`
	FastOpen              bool                  `json:"fast_open"`
	Obfs                  *Hy2ObfuscationConfig `json:"obfs"`
}

type Hy2ObfuscationConfig struct {
	Type     string `json:"type"`
	Password string `json:"password"`
}

// Build implements Buildable.
func (c *Hy2Config) Build() (proto.Message, error) {
	config := &hysteria2.Config{
		Congestion: &hysteria2.Congestion{
			Type:     c.Congestion.Type,
			DownMbps: c.Congestion.DownMbps,
			UpMbps:   c.Congestion.UpMbps,
		},
		UseUdpExtension:       c.UseUDPExtension,
		IgnoreClientBandwidth: c.IgnoreClientBandwidth,
		FastOpen:              c.FastOpen,
	}

	// Set obfuscation config if provided
	if c.Obfs != nil {
		config.Obfs = &hysteria2.ObfuscationConfig{
			Type:     c.Obfs.Type,
			Password: c.Obfs.Password,
		}
	}

	return config, nil
}

type WebSocketConfig struct {
	Path                 string            `json:"path"`
	Headers              map[string]string `json:"headers"`
	AcceptProxyProtocol  bool              `json:"acceptProxyProtocol"`
	MaxEarlyData         int32             `json:"maxEarlyData"`
	UseBrowserForwarding bool              `json:"useBrowserForwarding"`
	EarlyDataHeaderName  string            `json:"earlyDataHeaderName"`
}

// Build implements Buildable.
func (c *WebSocketConfig) Build() (proto.Message, error) {
	path := c.Path
	header := make([]*websocket.Header, 0, 32)
	for key, value := range c.Headers {
		header = append(header, &websocket.Header{
			Key:   key,
			Value: value,
		})
	}
	config := &websocket.Config{
		Path:                 path,
		Header:               header,
		MaxEarlyData:         c.MaxEarlyData,
		UseBrowserForwarding: c.UseBrowserForwarding,
		EarlyDataHeaderName:  c.EarlyDataHeaderName,
	}
	if c.AcceptProxyProtocol {
		config.AcceptProxyProtocol = c.AcceptProxyProtocol
	}
	return config, nil
}

type HTTPConfig struct {
	Host    *cfgcommon.StringList            `json:"host"`
	Path    string                           `json:"path"`
	Method  string                           `json:"method"`
	Headers map[string]*cfgcommon.StringList `json:"headers"`
}

// Build implements Buildable.
func (c *HTTPConfig) Build() (proto.Message, error) {
	config := &http.Config{
		Path: c.Path,
	}
	if c.Host != nil {
		config.Host = []string(*c.Host)
	}
	if c.Method != "" {
		config.Method = c.Method
	}
	if len(c.Headers) > 0 {
		config.Header = make([]*httpheader.Header, 0, len(c.Headers))
		headerNames := sortMapKeys(c.Headers)
		for _, key := range headerNames {
			value := c.Headers[key]
			if value == nil {
				return nil, newError("empty HTTP header value: " + key).AtError()
			}
			config.Header = append(config.Header, &httpheader.Header{
				Name:  key,
				Value: append([]string(nil), (*value)...),
			})
		}
	}
	return config, nil
}

type QUICConfig struct {
	Header   json.RawMessage `json:"header"`
	Security string          `json:"security"`
	Key      string          `json:"key"`
}

// Build implements Buildable.
func (c *QUICConfig) Build() (proto.Message, error) {
	config := &quic.Config{
		Key: c.Key,
	}

	if len(c.Header) > 0 {
		headerConfig, _, err := kcpHeaderLoader.Load(c.Header)
		if err != nil {
			return nil, newError("invalid QUIC header config.").Base(err).AtError()
		}
		ts, err := headerConfig.(cfgcommon.Buildable).Build()
		if err != nil {
			return nil, newError("invalid QUIC header config").Base(err).AtError()
		}
		config.Header = serial.ToTypedMessage(ts)
	}

	var st protocol.SecurityType
	switch strings.ToLower(c.Security) {
	case "aes-128-gcm":
		st = protocol.SecurityType_AES128_GCM
	case "chacha20-poly1305":
		st = protocol.SecurityType_CHACHA20_POLY1305
	default:
		st = protocol.SecurityType_NONE
	}

	config.Security = &protocol.SecurityConfig{
		Type: st,
	}

	return config, nil
}

type DomainSocketConfig struct {
	Path     string `json:"path"`
	Abstract bool   `json:"abstract"`
	Padding  bool   `json:"padding"`
}

// Build implements Buildable.
func (c *DomainSocketConfig) Build() (proto.Message, error) {
	return &domainsocket.Config{
		Path:     c.Path,
		Abstract: c.Abstract,
		Padding:  c.Padding,
	}, nil
}

// RealitySecurityConfig is the v4 JSON for REALITY security (client side only for now).
// Note: actual proto.Config will be built after protobuf generation is added.
type RealitySecurityConfig struct {
	ServerName  string `json:"serverName"`
	PublicKey   string `json:"publicKey"`   // hex
	ShortId     string `json:"shortId"`     // hex (8~16 bytes)
	Fingerprint string `json:"fingerprint"` // utls fingerprint name
	Show        bool   `json:"show"`
	SpiderX     string `json:"spiderX"`
}

// Build implements Buildable.
func (c *RealitySecurityConfig) Build() (proto.Message, error) {
	var pub []byte
	var sid []byte
	var err error

	// helper: try decode with hex and multiple base64 variants
	decodeFlexible := func(s string) ([]byte, error) {
		if s == "" {
			return nil, nil
		}
		if b, e := hex.DecodeString(s); e == nil {
			return b, nil
		}
		if b, e := base64.StdEncoding.DecodeString(s); e == nil {
			return b, nil
		}
		if b, e := base64.RawStdEncoding.DecodeString(s); e == nil {
			return b, nil
		}
		if b, e := base64.URLEncoding.DecodeString(s); e == nil {
			return b, nil
		}
		if b, e := base64.RawURLEncoding.DecodeString(s); e == nil {
			return b, nil
		}
		return nil, newError("Failed to decode (expect hex/base64)")
	}

	if c.PublicKey != "" {
		pub, err = decodeFlexible(c.PublicKey)
		if err != nil {
			return nil, newError("Failed to decode REALITY publicKey").Base(err)
		}
		if len(pub) != 32 {
			return nil, newError("REALITY publicKey must be 32 bytes (X25519)")
		}
	}
	if c.ShortId != "" {
		sid, err = decodeFlexible(c.ShortId)
		if err != nil {
			return nil, newError("Failed to decode REALITY shortId").Base(err)
		}
		if len(sid) > 16 {
			return nil, newError("REALITY shortId must be <= 16 bytes")
		}
	}
	return &reality.Config{
		ServerName:  c.ServerName,
		PublicKey:   pub,
		ShortId:     sid,
		Fingerprint: c.Fingerprint,
		Show:        c.Show,
		SpiderX:     c.SpiderX,
	}, nil
}

type TransportProtocol string

// Build implements Buildable.
func (p TransportProtocol) Build() (string, error) {
	switch strings.ToLower(string(p)) {
	case "tcp":
		return "tcp", nil
	case "kcp", "mkcp":
		return "mkcp", nil
	case "ws", "websocket":
		return "websocket", nil
	case "h2", "http":
		return "http", nil
	case "ds", "domainsocket":
		return "domainsocket", nil
	case "quic":
		return "quic", nil
	case "gun", "grpc":
		return "gun", nil
	case "hy2", "hysteria2":
		return "hysteria2", nil
	case "shadowtls":
		return "shadowtls", nil
	default:
		return "", newError("Config: unknown transport protocol: ", p)
	}
}

type StreamConfig struct {
	Network           *TransportProtocol      `json:"network"`
	Security          string                  `json:"security"`
	TLSSettings       *tlscfg.TLSConfig       `json:"tlsSettings"`
	RealitySettings   *RealitySecurityConfig  `json:"realitySettings"`
	TCPSettings       *TCPConfig              `json:"tcpSettings"`
	KCPSettings       *KCPConfig              `json:"kcpSettings"`
	WSSettings        *WebSocketConfig        `json:"wsSettings"`
	HTTPSettings      *HTTPConfig             `json:"httpSettings"`
	DSSettings        *DomainSocketConfig     `json:"dsSettings"`
	QUICSettings      *QUICConfig             `json:"quicSettings"`
	GunSettings       *GunConfig              `json:"gunSettings"`
	GRPCSettings      *GunConfig              `json:"grpcSettings"`
	Hy2Settings       *Hy2Config              `json:"hy2Settings"`
	ShadowTLSSettings *ShadowTLSConfig        `json:"shadowtlsSettings"`
	SocketSettings    *socketcfg.SocketConfig `json:"sockopt"`
}

// Build implements Buildable.
func (c *StreamConfig) Build() (*internet.StreamConfig, error) {
	config := &internet.StreamConfig{
		ProtocolName: "tcp",
	}
	if c.Network != nil {
		protocol, err := c.Network.Build()
		if err != nil {
			return nil, err
		}
		config.ProtocolName = protocol
	}
	if strings.EqualFold(c.Security, "tls") {
		tlsSettings := c.TLSSettings
		if tlsSettings == nil {
			tlsSettings = &tlscfg.TLSConfig{}
		}
		ts, err := tlsSettings.Build()
		if err != nil {
			return nil, newError("Failed to build TLS config.").Base(err)
		}
		tm := serial.ToTypedMessage(ts)
		config.SecuritySettings = append(config.SecuritySettings, tm)
		config.SecurityType = serial.V2Type(tm)
	} else if strings.EqualFold(c.Security, "reality") {
		// Build REALITY security settings
		rs := c.RealitySettings
		if rs == nil {
			rs = &RealitySecurityConfig{}
		}
		ts, err := rs.Build()
		if err != nil {
			return nil, newError("Failed to build REALITY config.").Base(err)
		}
		tm := serial.ToTypedMessage(ts)
		config.SecuritySettings = append(config.SecuritySettings, tm)
		config.SecurityType = serial.V2Type(tm)
	}
	if c.TCPSettings != nil {
		ts, err := c.TCPSettings.Build()
		if err != nil {
			return nil, newError("Failed to build TCP config.").Base(err)
		}
		config.TransportSettings = append(config.TransportSettings, &internet.TransportConfig{
			ProtocolName: "tcp",
			Settings:     serial.ToTypedMessage(ts),
		})
	}
	if c.KCPSettings != nil {
		ts, err := c.KCPSettings.Build()
		if err != nil {
			return nil, newError("Failed to build mKCP config.").Base(err)
		}
		config.TransportSettings = append(config.TransportSettings, &internet.TransportConfig{
			ProtocolName: "mkcp",
			Settings:     serial.ToTypedMessage(ts),
		})
	}
	if c.WSSettings != nil {
		ts, err := c.WSSettings.Build()
		if err != nil {
			return nil, newError("Failed to build WebSocket config.").Base(err)
		}
		config.TransportSettings = append(config.TransportSettings, &internet.TransportConfig{
			ProtocolName: "websocket",
			Settings:     serial.ToTypedMessage(ts),
		})
	}
	if c.HTTPSettings != nil {
		ts, err := c.HTTPSettings.Build()
		if err != nil {
			return nil, newError("Failed to build HTTP config.").Base(err)
		}
		config.TransportSettings = append(config.TransportSettings, &internet.TransportConfig{
			ProtocolName: "http",
			Settings:     serial.ToTypedMessage(ts),
		})
	}
	if c.DSSettings != nil {
		ds, err := c.DSSettings.Build()
		if err != nil {
			return nil, newError("Failed to build DomainSocket config.").Base(err)
		}
		config.TransportSettings = append(config.TransportSettings, &internet.TransportConfig{
			ProtocolName: "domainsocket",
			Settings:     serial.ToTypedMessage(ds),
		})
	}
	if c.QUICSettings != nil {
		qs, err := c.QUICSettings.Build()
		if err != nil {
			return nil, newError("Failed to build QUIC config.").Base(err)
		}
		config.TransportSettings = append(config.TransportSettings, &internet.TransportConfig{
			ProtocolName: "quic",
			Settings:     serial.ToTypedMessage(qs),
		})
	}
	if c.GunSettings == nil {
		c.GunSettings = c.GRPCSettings
	}
	if c.GunSettings != nil {
		gs, err := c.GunSettings.Build()
		if err != nil {
			return nil, newError("Failed to build Gun config.").Base(err)
		}
		config.TransportSettings = append(config.TransportSettings, &internet.TransportConfig{
			ProtocolName: "gun",
			Settings:     serial.ToTypedMessage(gs),
		})
	}
	if c.Hy2Settings != nil {
		hy2, err := c.Hy2Settings.Build()
		if err != nil {
			return nil, newError("Failed to build hy2 config.").Base(err)
		}
		config.TransportSettings = append(config.TransportSettings, &internet.TransportConfig{
			ProtocolName: "hysteria2",
			Settings:     serial.ToTypedMessage(hy2),
		})
	}
	if c.ShadowTLSSettings != nil {
		st, err := c.ShadowTLSSettings.Build()
		if err != nil {
			return nil, newError("Failed to build ShadowTLS config.").Base(err)
		}
		config.TransportSettings = append(config.TransportSettings, &internet.TransportConfig{
			ProtocolName: "shadowtls",
			Settings:     serial.ToTypedMessage(st),
		})
	}
	if c.SocketSettings != nil {
		ss, err := c.SocketSettings.Build()
		if err != nil {
			return nil, newError("Failed to build sockopt.").Base(err)
		}
		config.SocketSettings = ss
	}
	return config, nil
}
