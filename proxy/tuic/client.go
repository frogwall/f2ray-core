package tuic

import (
	"context"
	"crypto/tls"
	"net"
	"time"

	core "github.com/frogwall/f2ray-core/v5"
	"github.com/frogwall/f2ray-core/v5/common/buf"
	v2net "github.com/frogwall/f2ray-core/v5/common/net"
	"github.com/frogwall/f2ray-core/v5/common/protocol"
	"github.com/frogwall/f2ray-core/v5/common/retry"
	"github.com/frogwall/f2ray-core/v5/common/session"
	"github.com/frogwall/f2ray-core/v5/common/signal"
	"github.com/frogwall/f2ray-core/v5/common/task"
	"github.com/frogwall/f2ray-core/v5/features/policy"
	"github.com/frogwall/f2ray-core/v5/transport"
	"github.com/frogwall/f2ray-core/v5/transport/internet"

	"github.com/daeuniverse/outbound/netproxy"
	outboundProtocol "github.com/daeuniverse/outbound/protocol"
	tuicCommon "github.com/daeuniverse/outbound/protocol/tuic/common"
	quic "github.com/daeuniverse/quic-go"
)

type Client struct {
	serverPicker  protocol.ServerPicker
	policyManager policy.Manager
	config        *ClientConfig
}

func NewClient(ctx context.Context, config *ClientConfig) (*Client, error) {
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
	return &Client{
		serverPicker:  protocol.NewRoundRobinServerPicker(serverList),
		policyManager: v.GetFeature(policy.ManagerType()).(policy.Manager),
		config:        config,
	}, nil
}

func (c *Client) Process(ctx context.Context, link *transport.Link, dialer internet.Dialer) error {
	outbound := session.OutboundFromContext(ctx)
	if outbound == nil || !outbound.Target.IsValid() {
		return newError("target not specified")
	}
	destination := outbound.Target
	network := destination.Network

	var server *protocol.ServerSpec
	var conn internet.Connection

	err := retry.ExponentialBackoff(5, 100).On(func() error {
		server = c.serverPicker.PickServer()
		dest := server.Destination()
		dest.Network = v2net.Network_TCP

		var err error
		conn, err = dialer.Dial(ctx, dest)
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return newError("failed to find an available destination").Base(err)
	}
	defer conn.Close()

	user := server.PickUser()
	account, ok := user.Account.(*MemoryAccount)
	if !ok {
		return newError("user account is not TUIC account")
	}

	sessionPolicy := c.policyManager.ForLevel(user.Level)
	ctx, cancel := context.WithCancel(ctx)
	timer := signal.CancelAfterInactivity(ctx, cancel, sessionPolicy.Timeouts.ConnectionIdle)

	// Create TUIC dialer
	tuicDialer, err := c.createTUICDialer(ctx, conn, server, account)
	if err != nil {
		return newError("failed to create TUIC dialer").Base(err)
	}

	if network == v2net.Network_TCP {
		return c.handleTCP(ctx, link, tuicDialer, destination, timer)
	} else {
		return c.handleUDP(ctx, link, tuicDialer, destination, timer)
	}
}

func (c *Client) createTUICDialer(ctx context.Context, conn internet.Connection, server *protocol.ServerSpec, account *MemoryAccount) (netproxy.Dialer, error) {
	// Get TLS config from connection if available
	tlsConn, ok := conn.(*tls.Conn)
	var tlsConfig *tls.Config
	if ok {
		connState := tlsConn.ConnectionState()
		tlsConfig = &tls.Config{
			ServerName:         connState.ServerName,
			InsecureSkipVerify: false,
		}
	} else {
		// Create basic TLS config
		tlsConfig = &tls.Config{
			ServerName:         server.Destination().Address.String(),
			InsecureSkipVerify: false,
		}
	}

	// Apply TLS config from settings
	if c.config.Tls != nil {
		if c.config.Tls.ServerName != "" {
			tlsConfig.ServerName = c.config.Tls.ServerName
		}
		if len(c.config.Tls.Alpn) > 0 {
			tlsConfig.NextProtos = c.config.Tls.Alpn
		}
		tlsConfig.InsecureSkipVerify = c.config.Tls.AllowInsecure
	}

	// Determine UDP relay mode
	udpRelayMode := tuicCommon.NATIVE
	if c.config.UdpRelayMode == "quic" {
		udpRelayMode = tuicCommon.QUIC
	}

	// Create QUIC config
	quicConfig := &quic.Config{
		InitialStreamReceiveWindow:     tuicCommon.InitialStreamReceiveWindow,
		MaxStreamReceiveWindow:         tuicCommon.MaxStreamReceiveWindow,
		InitialConnectionReceiveWindow: tuicCommon.InitialConnectionReceiveWindow,
		MaxConnectionReceiveWindow:     tuicCommon.MaxConnectionReceiveWindow,
		KeepAlivePeriod:                3 * time.Second,
		DisablePathMTUDiscovery:        false,
		EnableDatagrams:                true,
		HandshakeIdleTimeout:           8 * time.Second,
	}

	// Override with custom QUIC config if provided
	if c.config.Quic != nil {
		if c.config.Quic.InitialStreamReceiveWindow > 0 {
			quicConfig.InitialStreamReceiveWindow = c.config.Quic.InitialStreamReceiveWindow
		}
		if c.config.Quic.MaxStreamReceiveWindow > 0 {
			quicConfig.MaxStreamReceiveWindow = c.config.Quic.MaxStreamReceiveWindow
		}
		if c.config.Quic.InitialConnectionReceiveWindow > 0 {
			quicConfig.InitialConnectionReceiveWindow = c.config.Quic.InitialConnectionReceiveWindow
		}
		if c.config.Quic.MaxConnectionReceiveWindow > 0 {
			quicConfig.MaxConnectionReceiveWindow = c.config.Quic.MaxConnectionReceiveWindow
		}
		if c.config.Quic.KeepAlivePeriod > 0 {
			quicConfig.KeepAlivePeriod = time.Duration(c.config.Quic.KeepAlivePeriod) * time.Second
		}
		quicConfig.DisablePathMTUDiscovery = c.config.Quic.DisablePathMtuDiscovery
	}

	// Determine congestion control
	congestionControl := "bbr"
	if c.config.CongestionControl != "" {
		congestionControl = c.config.CongestionControl
	}

	// Max UDP relay packet size is configured in the header but not used directly here
	// The TUIC library handles packet sizing internally

	// Create protocol header for TUIC dialer
	header := outboundProtocol.Header{
		ProxyAddress: server.Destination().NetAddr(),
		TlsConfig:    tlsConfig,
		User:         account.UUID.String(),
		Password:     account.Password,
		Feature1:     congestionControl,
		IsClient:     true,
	}

	// Set UDP relay mode flag
	if udpRelayMode == tuicCommon.QUIC {
		header.Flags = outboundProtocol.Flags_Tuic_UdpRelayModeQuic
	}

	// Create base dialer (wraps the connection)
	baseDialer := &connectionDialer{conn: conn}

	// Create TUIC dialer using the outbound library
	tuicDialer, err := outboundProtocol.NewDialer("tuic", baseDialer, header)
	if err != nil {
		return nil, newError("failed to create TUIC protocol dialer").Base(err)
	}

	return tuicDialer, nil
}

func (c *Client) handleTCP(ctx context.Context, link *transport.Link, dialer netproxy.Dialer, destination v2net.Destination, timer *signal.ActivityTimer) error {
	// Dial TCP connection through TUIC
	tuicConn, err := dialer.DialContext(ctx, "tcp", destination.NetAddr())
	if err != nil {
		return newError("failed to dial TCP through TUIC").Base(err)
	}
	defer tuicConn.Close()

	// Wrap connection for buf.Reader/Writer
	reader := buf.NewReader(tuicConn)
	writer := buf.NewWriter(tuicConn)

	requestDone := func() error {
		return buf.Copy(link.Reader, writer, buf.UpdateActivity(timer))
	}

	responseDone := func() error {
		return buf.Copy(reader, link.Writer, buf.UpdateActivity(timer))
	}

	if err := task.Run(ctx, requestDone, responseDone); err != nil {
		return newError("connection ends").Base(err)
	}
	return nil
}

func (c *Client) handleUDP(ctx context.Context, link *transport.Link, dialer netproxy.Dialer, destination v2net.Destination, timer *signal.ActivityTimer) error {
	// Dial UDP connection through TUIC
	tuicConn, err := dialer.DialContext(ctx, "udp", destination.NetAddr())
	if err != nil {
		return newError("failed to dial UDP through TUIC").Base(err)
	}
	defer tuicConn.Close()

	// TUIC returns a PacketConn for UDP
	packetConn, ok := tuicConn.(netproxy.PacketConn)
	if !ok {
		return newError("TUIC dialer did not return PacketConn for UDP")
	}

	requestDone := func() error {
		for {
			mb, err := link.Reader.ReadMultiBuffer()
			if err != nil {
				return err
			}
			timer.Update()

			for _, buffer := range mb {
				payload := buffer.Bytes()
				// Write UDP packet (destination is already set in TUIC connection)
				_, err := packetConn.Write(payload)
				buffer.Release()
				if err != nil {
					return err
				}
			}
		}
	}

	responseDone := func() error {
		readBuf := make([]byte, buf.Size)
		for {
			n, _, err := packetConn.ReadFrom(readBuf)
			if err != nil {
				return err
			}
			timer.Update()

			// Create buffer and write to link
			buffer := buf.New()
			buffer.Write(readBuf[:n])
			if err := link.Writer.WriteMultiBuffer(buf.MultiBuffer{buffer}); err != nil {
				buffer.Release()
				return err
			}
		}
	}

	if err := task.Run(ctx, requestDone, responseDone); err != nil {
		return newError("connection ends").Base(err)
	}
	return nil
}

// connectionDialer wraps an internet.Connection to implement netproxy.Dialer
type connectionDialer struct {
	conn internet.Connection
}

func (d *connectionDialer) DialContext(ctx context.Context, network, addr string) (netproxy.Conn, error) {
	// For TUIC, we return the underlying connection wrapped as netproxy.Conn
	// The TUIC library will handle the QUIC transport
	// We use a simple wrapper that implements netproxy.Conn interface
	return &netproxyConnWrapper{Conn: d.conn}, nil
}

// netproxyConnWrapper wraps internet.Connection to implement netproxy.Conn
type netproxyConnWrapper struct {
	net.Conn
}

func (w *netproxyConnWrapper) NeedAdditionalReadDeadline() bool {
	return false
}
