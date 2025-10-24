package shadowtls

import (
	"context"
	"crypto/tls"

	"github.com/sagernet/sing-shadowtls"
	M "github.com/sagernet/sing/common/metadata"
	"github.com/frogwall/v2ray-core/v5/common"
	"github.com/frogwall/v2ray-core/v5/common/net"
	"github.com/frogwall/v2ray-core/v5/common/session"
	"github.com/frogwall/v2ray-core/v5/transport/internet"
)

// Dial dials a ShadowTLS connection to the given destination.
func Dial(ctx context.Context, dest net.Destination, streamSettings *internet.MemoryStreamConfig) (internet.Connection, error) {
	newError("creating ShadowTLS connection to ", dest).WriteToLog(session.ExportIDToError(ctx))

	stSettings := streamSettings.ProtocolSettings.(*Config)
	
	// Validate settings
	if stSettings.Version == 0 {
		stSettings.Version = 3 // Default to v3
	}
	if stSettings.Version < 1 || stSettings.Version > 3 {
		return nil, newError("invalid ShadowTLS version: ", stSettings.Version)
	}
	if (stSettings.Version == 2 || stSettings.Version == 3) && stSettings.Password == "" {
		return nil, newError("password is required for ShadowTLS v", stSettings.Version)
	}
	if stSettings.HandshakeServer == "" {
		return nil, newError("handshakeServer is required for ShadowTLS")
	}

	// Prepare handshake server address
	handshakeServer := stSettings.HandshakeServer
	handshakePort := stSettings.HandshakePort
	if handshakePort == 0 {
		handshakePort = 443
	}

	// Create TLS handshake function
	newError("ShadowTLS config: version=", stSettings.Version, " handshakeServer=", handshakeServer, ":", handshakePort).AtInfo().WriteToLog()
	tlsHandshakeFunc := createTLSHandshakeFunc(stSettings, handshakeServer)

	// Create v2ray dialer wrapper
	v2rayDialer := &v2rayDialerWrapper{
		ctx:            ctx,
		streamSettings: streamSettings,
	}

	// Create ShadowTLS client
	// Server is the ShadowTLS server address 
	// The ShadowTLS server will forward TLS handshake to the handshake server (bing.com:443)
	client, err := shadowtls.NewClient(shadowtls.ClientConfig{
		Version:      int(stSettings.Version),
		Password:     stSettings.Password,
		Server:       M.ParseSocksaddr(dest.NetAddr()),  // ShadowTLS server address
		Dialer:       v2rayDialer,
		TLSHandshake: tlsHandshakeFunc,
		Logger:       &stLogger{},
	})
	if err != nil {
		return nil, newError("failed to create ShadowTLS client").Base(err)
	}

	// Dial through ShadowTLS
	stConn, err := client.DialContext(ctx)
	if err != nil {
		return nil, newError("failed to establish ShadowTLS connection").Base(err)
	}

	return internet.Connection(stConn), nil
}

// createTLSHandshakeFunc creates the TLS handshake function for ShadowTLS
func createTLSHandshakeFunc(config *Config, serverName string) shadowtls.TLSHandshakeFunc {
	switch config.Version {
	case 1, 2:
		// For v1 and v2, use standard TLS handshake
		return func(ctx context.Context, conn net.Conn, _ shadowtls.TLSSessionIDGeneratorFunc) error {
			tlsConfig := &tls.Config{
				ServerName:         serverName,
				InsecureSkipVerify: false,
				MinVersion:         tls.VersionTLS12,
				MaxVersion:         tls.VersionTLS12,
			}
			
			if config.Version == 2 {
				// v2 can use TLS 1.3
				tlsConfig.MinVersion = tls.VersionTLS12
				tlsConfig.MaxVersion = tls.VersionTLS13
			}

			tlsConn := tls.Client(conn, tlsConfig)
			err := tlsConn.HandshakeContext(ctx)
			if err != nil {
				return err
			}
			// Don't close the TLS connection, just complete the handshake
			return nil
		}
	case 3:
		// For v3, use the default handshake function with session ID generator
		tlsConfig := &tls.Config{
			ServerName:         serverName,
			InsecureSkipVerify: false,
			MinVersion:         tls.VersionTLS12,
			MaxVersion:         tls.VersionTLS13,
		}
		return shadowtls.DefaultTLSHandshakeFunc(config.Password, tlsConfig)
	default:
		return nil
	}
}

// v2rayDialerWrapper wraps v2ray dialer to implement sing's N.Dialer interface
type v2rayDialerWrapper struct {
	ctx            context.Context
	streamSettings *internet.MemoryStreamConfig
}

func (d *v2rayDialerWrapper) DialContext(ctx context.Context, network string, destination M.Socksaddr) (net.Conn, error) {
	var netNetwork net.Network
	switch network {
	case "tcp", "tcp4", "tcp6":
		netNetwork = net.Network_TCP
	case "udp", "udp4", "udp6":
		netNetwork = net.Network_UDP
	default:
		return nil, newError("unsupported network: ", network)
	}
	
	dest := net.Destination{
		Network: netNetwork,
		Address: net.ParseAddress(destination.AddrString()),
		Port:    net.Port(destination.Port),
	}
	return internet.DialSystem(ctx, dest, d.streamSettings.SocketSettings)
}

func (d *v2rayDialerWrapper) ListenPacket(ctx context.Context, destination M.Socksaddr) (net.PacketConn, error) {
	return nil, newError("ListenPacket not supported in ShadowTLS")
}

func init() {
	common.Must(internet.RegisterTransportDialer(protocolName, Dial))
}
