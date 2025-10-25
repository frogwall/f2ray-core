package juicity

//go:generate go run github.com/frogwall/f2ray-core/v5/common/errors/errorgen

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	gonet "net"
	"time"

	"github.com/daeuniverse/outbound/netproxy"
	outboundprotocol "github.com/daeuniverse/outbound/protocol"
	"github.com/daeuniverse/outbound/protocol/direct"
	"github.com/daeuniverse/outbound/protocol/juicity"

	"github.com/frogwall/f2ray-core/v5/common"
	"github.com/frogwall/f2ray-core/v5/common/buf"
	"github.com/frogwall/f2ray-core/v5/common/protocol"
	"github.com/frogwall/f2ray-core/v5/common/session"
	"github.com/frogwall/f2ray-core/v5/common/signal"
	"github.com/frogwall/f2ray-core/v5/common/task"
	"github.com/frogwall/f2ray-core/v5/features/policy"
	"github.com/frogwall/f2ray-core/v5/transport"
	"github.com/frogwall/f2ray-core/v5/transport/internet"
	v2tls "github.com/frogwall/f2ray-core/v5/transport/internet/tls"
	core "github.com/frogwall/f2ray-core/v5"
)

// Client is a Juicity outbound handler
type Client struct {
	config         *ClientConfig
	dialer         *juicity.Dialer
	policyManager  policy.Manager
}

// NewClient creates a new Juicity client
func New(ctx context.Context, config *ClientConfig) (*Client, error) {
	if len(config.Server) == 0 {
		return nil, newError("no server configured")
	}

	v := core.MustFromContext(ctx)
	client := &Client{
		config:        config,
		policyManager: v.GetFeature(policy.ManagerType()).(policy.Manager),
	}

	return client, nil
}

// Process implements proxy.Outbound
func (c *Client) Process(ctx context.Context, link *transport.Link, dialer internet.Dialer) error {
	outbound := session.OutboundFromContext(ctx)
	if outbound == nil || !outbound.Target.IsValid() {
		return newError("target not specified")
	}
	destination := outbound.Target
	network := destination.Network

	newError("opening connection to ", destination).WriteToLog(session.ExportIDToError(ctx))

	// Get server config
	server := c.config.Server[0]
	dest := server.Address.AsAddress()
	
	// Initialize dialer if not already done
	if c.dialer == nil {
		if err := c.initDialer(ctx, server, dialer); err != nil {
			return newError("failed to initialize dialer").Base(err)
		}
	}

	// Dial to destination
	conn, err := c.dialer.DialContext(ctx, network.SystemString(), destination.NetAddr())
	if err != nil {
		return newError("failed to dial to ", destination).Base(err)
	}
	defer conn.Close()

	newError("tunneling request to ", destination, " via ", dest).WriteToLog(session.ExportIDToError(ctx))

	// Get session policy
	sessionPolicy := c.policyManager.ForLevel(0)
	ctx, cancel := context.WithCancel(ctx)
	timer := signal.CancelAfterInactivity(ctx, cancel, sessionPolicy.Timeouts.ConnectionIdle)

	requestDone := func() error {
		defer timer.SetTimeout(sessionPolicy.Timeouts.DownlinkOnly)
		return buf.Copy(link.Reader, buf.NewWriter(conn), buf.UpdateActivity(timer))
	}

	responseDone := func() error {
		defer timer.SetTimeout(sessionPolicy.Timeouts.UplinkOnly)
		return buf.Copy(buf.NewReader(conn), link.Writer, buf.UpdateActivity(timer))
	}

	requestDonePost := task.OnSuccess(requestDone, task.Close(link.Writer))
	if err := task.Run(ctx, requestDonePost, responseDone); err != nil {
		return newError("connection ends").Base(err)
	}

	return nil
}

// initDialer initializes the Juicity dialer
func (c *Client) initDialer(ctx context.Context, server *protocol.ServerEndpoint, dialer internet.Dialer) error {
	dest := server.Address.AsAddress()
	port := int(server.Port)
	serverAddr := gonet.JoinHostPort(dest.String(), fmt.Sprint(port))

	// Extract UUID and password from server.User
	if len(server.User) == 0 {
		return newError("no user configured in server endpoint")
	}
	user := server.User[0]
	uuid := user.Email  // username is stored in Email field
	
	// Password is stored directly in the user
	// In JSON config: {"username": "uuid", "password": "pass"}
	// username maps to Email, password is handled by the protocol layer
	password := ""
	if user.Account != nil {
		// Password should be in the raw account data
		// For now, we'll use a simple approach
		password = string(user.Account.Value)
	}
	
	if uuid == "" {
		return newError("UUID (username) not configured")
	}
	if password == "" {
		return newError("password not configured")
	}

    // Build TLS config from streamSettings if available
    var tlsConfig *tls.Config
    // Try to get stream settings from the provided dialer (app/proxyman/outbound.Handler)
    type streamSettingsGetter interface{ StreamSettings() *internet.MemoryStreamConfig }
    if ssg, ok := dialer.(streamSettingsGetter); ok {
        if mss := ssg.StreamSettings(); mss != nil {
            if cfg := v2tls.ConfigFromStreamSettings(mss); cfg != nil {
                // Build crypto/tls.Config honoring allowInsecure, serverName, ALPN, etc.
                tlsConfig = cfg.GetTLSConfig(v2tls.WithNextProto("h3"))
            }
        }
    }
    // Fallback if no TLS settings provided
    if tlsConfig == nil {
        tlsConfig = &tls.Config{
            NextProtos: []string{"h3"},
            MinVersion: tls.VersionTLS13,
            ServerName: dest.String(),
        }
    } else {
        // Ensure HTTP/3 ALPN is present
        hasH3 := false
        for _, np := range tlsConfig.NextProtos {
            if np == "h3" { hasH3 = true; break }
        }
        if !hasH3 { tlsConfig.NextProtos = append(tlsConfig.NextProtos, "h3") }
        if tlsConfig.MinVersion == 0 { tlsConfig.MinVersion = tls.VersionTLS13 }
        if tlsConfig.ServerName == "" { tlsConfig.ServerName = dest.String() }
    }

	// Handle pinned certificate
    if c.config.PinnedCertchainSha256 != "" {
        pinnedHash, err := base64.URLEncoding.DecodeString(c.config.PinnedCertchainSha256)
        if err != nil {
            pinnedHash, err = base64.StdEncoding.DecodeString(c.config.PinnedCertchainSha256)
            if err != nil {
                pinnedHash, err = hex.DecodeString(c.config.PinnedCertchainSha256)
                if err != nil {
                    return newError("failed to decode pinned_certchain_sha256")
                }
            }
        }
        // When doing pinning, skip default verification and use our own
        tlsConfig.InsecureSkipVerify = true
        tlsConfig.VerifyPeerCertificate = func(rawCerts [][]byte, verifiedChains [][]*x509.Certificate) error {
            if !bytes.Equal(generateCertChainHash(rawCerts), pinnedHash) {
                return newError("pinned hash of cert chain does not match")
            }
            return nil
        }
    }

	// Create juicity dialer
	// Use direct.SymmetricDirect as the underlying dialer (supports UDP)
	dialerInterface, err := juicity.NewDialer(direct.SymmetricDirect, outboundprotocol.Header{
		ProxyAddress: serverAddr,
		Feature1:     c.config.CongestionControl,
		TlsConfig:    tlsConfig,
		User:         uuid,
		Password:     password,
		IsClient:     true,
		Flags:        0,
	})
	if err != nil {
		return newError("failed to create juicity dialer").Base(err)
	}

	// Type assert to juicity.Dialer
	juicityDialer, ok := dialerInterface.(*juicity.Dialer)
	if !ok {
		return newError("failed to assert dialer type")
	}

	c.dialer = juicityDialer
	return nil
}

// simpleDialer implements netproxy.Dialer for direct connections
type simpleDialer struct{}

func (d *simpleDialer) DialContext(ctx context.Context, network, addr string) (netproxy.Conn, error) {
	// QUIC uses UDP, so we need to support UDP connections
	// Debug: log the network type
	newError("DialContext called with network=", network, " addr=", addr).AtDebug().WriteToLog()
	
	switch network {
	case "udp", "udp4", "udp6":
		// For UDP, we need to use DialUDP
		udpAddr, err := gonet.ResolveUDPAddr(network, addr)
		if err != nil {
			return nil, newError("failed to resolve UDP address").Base(err)
		}
		conn, err := gonet.DialUDP(network, nil, udpAddr)
		if err != nil {
			return nil, newError("failed to dial UDP").Base(err)
		}
		return &simpleUDPConn{UDPConn: conn}, nil
	default:
		// For TCP and other protocols
		conn, err := gonet.Dial(network, addr)
		if err != nil {
			return nil, newError("failed to dial ", network).Base(err)
		}
		return &simpleConn{Conn: conn}, nil
	}
}

// simpleConn wraps gonet.Conn to implement netproxy.Conn
type simpleConn struct {
	gonet.Conn
}

func (c *simpleConn) Read(b []byte) (n int, err error) {
	return c.Conn.Read(b)
}

func (c *simpleConn) Write(b []byte) (n int, err error) {
	return c.Conn.Write(b)
}

func (c *simpleConn) Close() error {
	return c.Conn.Close()
}

func (c *simpleConn) LocalAddr() gonet.Addr {
	return c.Conn.LocalAddr()
}

func (c *simpleConn) RemoteAddr() gonet.Addr {
	return c.Conn.RemoteAddr()
}

func (c *simpleConn) SetDeadline(t time.Time) error {
	return c.Conn.SetDeadline(t)
}

func (c *simpleConn) SetReadDeadline(t time.Time) error {
	return c.Conn.SetReadDeadline(t)
}

func (c *simpleConn) SetWriteDeadline(t time.Time) error {
	return c.Conn.SetWriteDeadline(t)
}

func (c *simpleConn) ReadFrom(r io.Reader) (n int64, err error) {
	if rf, ok := c.Conn.(io.ReaderFrom); ok {
		return rf.ReadFrom(r)
	}
	return io.Copy(c.Conn, r)
}

func (c *simpleConn) WriteTo(w io.Writer) (n int64, err error) {
	if wt, ok := c.Conn.(io.WriterTo); ok {
		return wt.WriteTo(w)
	}
	return io.Copy(w, c.Conn)
}

// simpleUDPConn wraps gonet.UDPConn to implement netproxy.Conn
type simpleUDPConn struct {
	*gonet.UDPConn
}

func (c *simpleUDPConn) Read(b []byte) (n int, err error) {
	return c.UDPConn.Read(b)
}

func (c *simpleUDPConn) Write(b []byte) (n int, err error) {
	return c.UDPConn.Write(b)
}

func (c *simpleUDPConn) Close() error {
	return c.UDPConn.Close()
}

func (c *simpleUDPConn) LocalAddr() gonet.Addr {
	return c.UDPConn.LocalAddr()
}

func (c *simpleUDPConn) RemoteAddr() gonet.Addr {
	return c.UDPConn.RemoteAddr()
}

func (c *simpleUDPConn) SetDeadline(t time.Time) error {
	return c.UDPConn.SetDeadline(t)
}

func (c *simpleUDPConn) SetReadDeadline(t time.Time) error {
	return c.UDPConn.SetReadDeadline(t)
}

func (c *simpleUDPConn) SetWriteDeadline(t time.Time) error {
	return c.UDPConn.SetWriteDeadline(t)
}

func (c *simpleUDPConn) ReadFrom(r io.Reader) (n int64, err error) {
	return io.Copy(c.UDPConn, r)
}

func (c *simpleUDPConn) WriteTo(w io.Writer) (n int64, err error) {
	return io.Copy(w, c.UDPConn)
}

// generateCertChainHash generates SHA256 hash of certificate chain
func generateCertChainHash(rawCerts [][]byte) []byte {
	h := sha256.New()
	for _, cert := range rawCerts {
		h.Write(cert)
	}
	return h.Sum(nil)
}

func init() {
	common.Must(common.RegisterConfig((*ClientConfig)(nil), func(ctx context.Context, config interface{}) (interface{}, error) {
		return New(ctx, config.(*ClientConfig))
	}))
}
