package hysteria2

import (
	"context"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"sync"

	"github.com/apernet/quic-go/quicvarint"
	// TODO: Replace with sing-quic hysteria2 when available
	hyClient "github.com/v2fly/hysteria/core/v2/client"
	hyProtocol "github.com/v2fly/hysteria/core/v2/international/protocol"

	"github.com/frogwall/f2ray-core/v5/common"
	"github.com/frogwall/f2ray-core/v5/common/net"
	"github.com/frogwall/f2ray-core/v5/common/session"
	"github.com/frogwall/f2ray-core/v5/transport/internet"
	"github.com/frogwall/f2ray-core/v5/transport/internet/tls"
)

type dialerConf struct {
	net.Destination
	*internet.MemoryStreamConfig
}

var (
	RunningClient map[dialerConf](hyClient.Client)
	ClientMutex   sync.Mutex
	MBps          uint64 = 1000000 / 8 // MByte
)

// ConfigFromStreamSettings gets hysteria2 config from stream settings
func ConfigFromStreamSettings(streamSettings *internet.MemoryStreamConfig) *Config {
	if streamSettings == nil {
		return nil
	}
	config, ok := streamSettings.ProtocolSettings.(*Config)
	if !ok {
		return nil
	}
	return config
}

func GetClientTLSConfig(dest net.Destination, streamSettings *internet.MemoryStreamConfig) (*hyClient.TLSConfig, error) {
	config := tls.ConfigFromStreamSettings(streamSettings)
	if config == nil {
		return nil, newError(Hy2MustNeedTLS)
	}
	tlsConfig := config.GetTLSConfig(tls.WithDestination(dest))

	hyTLSConfig := &hyClient.TLSConfig{
		RootCAs:            tlsConfig.RootCAs,
		ServerName:         tlsConfig.ServerName,
		InsecureSkipVerify: tlsConfig.InsecureSkipVerify,
	}

	// Handle certificate pinning if configured
	if len(config.PinnedPeerCertificateChainSha256) > 0 {
		pinnedHashes := make([][]byte, len(config.PinnedPeerCertificateChainSha256))
		copy(pinnedHashes, config.PinnedPeerCertificateChainSha256)

		// When using certificate pinning, we need to skip standard verification
		// and rely on our custom verification function
		hyTLSConfig.InsecureSkipVerify = true

		hyTLSConfig.VerifyPeerCertificate = func(rawCerts [][]byte, verifiedChains [][]*x509.Certificate) error {
			newError("certificate pinning verification started").WriteToLog(session.ExportIDToError(context.Background()))

			// If we have pinned certificates, verify against them
			for _, rawCert := range rawCerts {
				cert, err := x509.ParseCertificate(rawCert)
				if err != nil {
					newError("failed to parse certificate: ", err.Error()).WriteToLog(session.ExportIDToError(context.Background()))
					continue
				}

				// Calculate SHA256 hash of the certificate
				certHash := sha256.Sum256(cert.Raw)
				certHashB64 := base64.StdEncoding.EncodeToString(certHash[:])

				newError("received certificate hash: ", certHashB64).WriteToLog(session.ExportIDToError(context.Background()))

				// Check if this certificate matches any of our pinned hashes
				for i, pinnedHash := range pinnedHashes {
					pinnedHashB64 := base64.StdEncoding.EncodeToString(pinnedHash)
					newError("comparing with pinned hash ", i, ": ", pinnedHashB64).WriteToLog(session.ExportIDToError(context.Background()))

					if len(pinnedHash) == len(certHash) {
						match := true
						for j := 0; j < len(certHash); j++ {
							if certHash[j] != pinnedHash[j] {
								match = false
								break
							}
						}
						if match {
							newError("certificate pinning verification successful for hash ", i).WriteToLog(session.ExportIDToError(context.Background()))
							return nil // Certificate matches, verification successful
						}
					}
				}
			}

			// If we reach here, no certificate matched our pinned hashes
			return newError("certificate does not match any pinned certificate")
		}
	} else {
		// Use the original VerifyPeerCertificate if no pinning is configured
		hyTLSConfig.VerifyPeerCertificate = tlsConfig.VerifyPeerCertificate
	}

	return hyTLSConfig, nil
}

func ResolveAddress(dest net.Destination) (net.Addr, error) {
	var destAddr *net.UDPAddr
	if dest.Address.Family().IsIP() {
		destAddr = &net.UDPAddr{
			IP:   dest.Address.IP(),
			Port: int(dest.Port),
		}
	} else {
		addr, err := net.ResolveUDPAddr("udp", dest.NetAddr())
		if err != nil {
			return nil, err
		}
		destAddr = addr
	}
	return destAddr, nil
}

type connFactory struct {
	hyClient.ConnFactory

	NewFunc func(addr net.Addr) (net.PacketConn, error)
}

func (f *connFactory) New(addr net.Addr) (net.PacketConn, error) {
	return f.NewFunc(addr)
}

func NewHyClient(dest net.Destination, streamSettings *internet.MemoryStreamConfig, password string) (hyClient.Client, error) {
	tlsConfig, err := GetClientTLSConfig(dest, streamSettings)
	if err != nil {
		return nil, err
	}

	serverAddr, err := ResolveAddress(dest)
	if err != nil {
		return nil, err
	}

	config := streamSettings.ProtocolSettings.(*Config)

	// Use password passed from protocol layer
	if len(password) > 0 {
		newError("transport layer using password: ", password[:min(8, len(password))]+"...").WriteToLog(session.ExportIDToError(context.Background()))
	} else {
		newError("transport layer using empty password").WriteToLog(session.ExportIDToError(context.Background()))
	}

	// Create obfuscator if configured
	var obfsObfuscator *SalamanderObfuscator
	if config.Obfs != nil && config.Obfs.Type == "salamander" {
		obfsObfuscator, err = NewSalamanderObfuscator(config.Obfs.Password)
		if err != nil {
			return nil, newError("failed to create obfuscator").Base(err)
		}
		newError("created salamander obfuscator for transport layer").WriteToLog(session.ExportIDToError(context.Background()))
	}

	client, _, err := hyClient.NewClient(&hyClient.Config{
		Auth:       password,
		TLSConfig:  *tlsConfig,
		ServerAddr: serverAddr,
		ConnFactory: &connFactory{
			NewFunc: func(addr net.Addr) (net.PacketConn, error) {
				rawConn, err := internet.ListenSystemPacket(context.Background(), &net.UDPAddr{
					IP:   []byte{0, 0, 0, 0},
					Port: 0,
				}, streamSettings.SocketSettings)
				if err != nil {
					return nil, err
				}

				// Apply obfuscation if configured
				if obfsObfuscator != nil {
					obfsConn := WrapPacketConn(rawConn.(*net.UDPConn), obfsObfuscator)
					newError("applied salamander obfuscation to UDP connection").WriteToLog(session.ExportIDToError(context.Background()))
					return obfsConn, nil
				}

				return rawConn.(*net.UDPConn), nil
			},
		},
		BandwidthConfig: hyClient.BandwidthConfig{MaxTx: config.Congestion.GetUpMbps() * MBps, MaxRx: config.GetCongestion().GetDownMbps() * MBps},
	})
	if err != nil {
		return nil, err
	}

	return client, nil
}

func CloseHyClient(dest net.Destination, streamSettings *internet.MemoryStreamConfig) error {
	ClientMutex.Lock()
	defer ClientMutex.Unlock()

	client, found := RunningClient[dialerConf{dest, streamSettings}]
	if found {
		delete(RunningClient, dialerConf{dest, streamSettings})
		return client.Close()
	}
	return nil
}

func GetHyClient(dest net.Destination, streamSettings *internet.MemoryStreamConfig) (hyClient.Client, error) {
	return GetHyClientWithPassword(dest, streamSettings, "")
}

func GetHyClientWithPassword(dest net.Destination, streamSettings *internet.MemoryStreamConfig, password string) (hyClient.Client, error) {
	var err error
	var client hyClient.Client

	ClientMutex.Lock()
	client, found := RunningClient[dialerConf{dest, streamSettings}]
	ClientMutex.Unlock()
	if !found || !CheckHyClientHealthy(client) {
		if found {
			// retry
			CloseHyClient(dest, streamSettings)
		}
		client, err = NewHyClient(dest, streamSettings, password)
		if err != nil {
			return nil, err
		}
		ClientMutex.Lock()
		RunningClient[dialerConf{dest, streamSettings}] = client
		ClientMutex.Unlock()
	}
	return client, nil
}

func CheckHyClientHealthy(client hyClient.Client) bool {
	quicConn := client.GetQuicConn()
	if quicConn == nil {
		return false
	}
	select {
	case <-quicConn.Context().Done():
		return false
	default:
	}
	return true
}

func Dial(ctx context.Context, dest net.Destination, streamSettings *internet.MemoryStreamConfig) (internet.Connection, error) {
	config := streamSettings.ProtocolSettings.(*Config)

	// Get password from context (passed from protocol layer)
	password := ""
	if ctxPassword := ctx.Value("hysteria2_password"); ctxPassword != nil {
		if pwd, ok := ctxPassword.(string); ok {
			password = pwd
		}
	}

	client, err := GetHyClientWithPassword(dest, streamSettings, password)
	if err != nil {
		CloseHyClient(dest, streamSettings)
		return nil, err
	}

	quicConn := client.GetQuicConn()
	conn := &HyConn{
		local:  quicConn.LocalAddr(),
		remote: quicConn.RemoteAddr(),
	}

	outbound := session.OutboundFromContext(ctx)
	network := net.Network_TCP
	if outbound != nil {
		network = outbound.Target.Network
	}

	if network == net.Network_UDP && config.GetUseUdpExtension() { // only hysteria2 can use udpExtension
		conn.IsUDPExtension = true
		conn.IsServer = false
		conn.ClientUDPSession, err = client.UDP()
		if err != nil {
			CloseHyClient(dest, streamSettings)
			return nil, err
		}
		return conn, nil
	}

	conn.stream, err = client.OpenStream()
	if err != nil {
		CloseHyClient(dest, streamSettings)
		return nil, err
	}

	// write TCP frame type
	frameSize := quicvarint.Len(hyProtocol.FrameTypeTCPRequest)
	buf := make([]byte, frameSize)
	hyProtocol.VarintPut(buf, hyProtocol.FrameTypeTCPRequest)
	_, err = conn.stream.Write(buf)
	if err != nil {
		CloseHyClient(dest, streamSettings)
		return nil, err
	}
	return conn, nil
}

func init() {
	RunningClient = make(map[dialerConf]hyClient.Client)
	common.Must(internet.RegisterTransportDialer(protocolName, Dial))
}
