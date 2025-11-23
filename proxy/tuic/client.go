package tuic

import (
	"context"
	"crypto/tls"
	"net"
	"net/netip"
	"time"

	core "github.com/frogwall/f2ray-core/v5"
	"github.com/frogwall/f2ray-core/v5/common/buf"
	v2net "github.com/frogwall/f2ray-core/v5/common/net"
	"github.com/frogwall/f2ray-core/v5/common/protocol"
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

	newError("TUIC: Processing connection to ", destination, " via ", network.String()).AtInfo().WriteToLog(session.ExportIDToError(ctx))

	// Pick server
	server := c.serverPicker.PickServer()
	user := server.PickUser()
	account, ok := user.Account.(*MemoryAccount)
	if !ok {
		return newError("user account is not TUIC account")
	}

	sessionPolicy := c.policyManager.ForLevel(user.Level)
	ctx, cancel := context.WithCancel(ctx)
	timer := signal.CancelAfterInactivity(ctx, cancel, sessionPolicy.Timeouts.ConnectionIdle)

	newError("TUIC: Connection idle timeout: ", sessionPolicy.Timeouts.ConnectionIdle).AtDebug().WriteToLog(session.ExportIDToError(ctx))

	// Create TUIC dialer - TUIC uses UDP for QUIC, so we don't need to establish a connection first
	// The outbound library will handle the UDP connection internally
	tuicDialer, err := c.createTUICDialer(ctx, server, account)
	if err != nil {
		return newError("failed to create TUIC dialer").Base(err)
	}

	if network == v2net.Network_TCP {
		return c.handleTCP(ctx, link, tuicDialer, destination, timer)
	} else {
		return c.handleUDP(ctx, link, tuicDialer, destination, timer)
	}
}

func (c *Client) createTUICDialer(ctx context.Context, server *protocol.ServerSpec, account *MemoryAccount) (netproxy.Dialer, error) {
	// Create TLS config for QUIC connection
	// TUIC uses QUIC which requires TLS, but we don't have a pre-established connection
	// The outbound library will establish the QUIC connection using UDP
	tlsConfig := &tls.Config{
		ServerName:         server.Destination().Address.String(),
		InsecureSkipVerify: false,
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

	// Create base dialer - TUIC needs UDP, so we create a simple UDP dialer
	// The outbound library will use this to create UDP connections for QUIC
	baseDialer := &udpDialer{}

	// Create TUIC dialer using the outbound library
	newError("TUIC: Creating TUIC protocol dialer for server ", server.Destination().NetAddr()).AtDebug().WriteToLog(session.ExportIDToError(ctx))
	tuicDialer, err := outboundProtocol.NewDialer("tuic", baseDialer, header)
	if err != nil {
		newError("TUIC: Failed to create TUIC protocol dialer, error: ", err).AtError().WriteToLog(session.ExportIDToError(ctx))
		return nil, newError("failed to create TUIC protocol dialer").Base(err)
	}
	newError("TUIC: Successfully created TUIC protocol dialer").AtDebug().WriteToLog(session.ExportIDToError(ctx))

	return tuicDialer, nil
}

func (c *Client) handleTCP(ctx context.Context, link *transport.Link, dialer netproxy.Dialer, destination v2net.Destination, timer *signal.ActivityTimer) error {
	// Dial TCP connection through TUIC
	newError("TUIC: Dialing TCP connection to ", destination.NetAddr()).AtInfo().WriteToLog(session.ExportIDToError(ctx))
	tuicConn, err := dialer.DialContext(ctx, "tcp", destination.NetAddr())
	if err != nil {
		newError("TUIC: Failed to dial TCP through TUIC to ", destination.NetAddr(), ", error: ", err).AtError().WriteToLog(session.ExportIDToError(ctx))
		return newError("failed to dial TCP through TUIC").Base(err)
	}
	defer tuicConn.Close()
	newError("TUIC: Successfully dialed TCP connection to ", destination.NetAddr()).AtInfo().WriteToLog(session.ExportIDToError(ctx))

	// Wrap connection for buf.Reader/Writer
	reader := buf.NewReader(tuicConn)
	writer := buf.NewWriter(tuicConn)

	requestDone := func() error {
		newError("TUIC: Starting to copy request data to ", destination).AtDebug().WriteToLog(session.ExportIDToError(ctx))
		err := buf.Copy(link.Reader, writer, buf.UpdateActivity(timer))
		if err != nil {
			newError("TUIC: Error copying request data to ", destination, ", error: ", err).AtError().WriteToLog(session.ExportIDToError(ctx))
		} else {
			newError("TUIC: Finished copying request data to ", destination).AtDebug().WriteToLog(session.ExportIDToError(ctx))
		}
		return err
	}

	responseDone := func() error {
		newError("TUIC: Starting to copy response data from ", destination).AtDebug().WriteToLog(session.ExportIDToError(ctx))
		err := buf.Copy(reader, link.Writer, buf.UpdateActivity(timer))
		if err != nil {
			newError("TUIC: Error copying response data from ", destination, ", error: ", err).AtError().WriteToLog(session.ExportIDToError(ctx))
		} else {
			newError("TUIC: Finished copying response data from ", destination).AtDebug().WriteToLog(session.ExportIDToError(ctx))
		}
		return err
	}

	if err := task.Run(ctx, requestDone, responseDone); err != nil {
		newError("TUIC: Connection ends for ", destination, ", error: ", err).AtInfo().WriteToLog(session.ExportIDToError(ctx))
		return newError("connection ends").Base(err)
	}
	newError("TUIC: Connection completed successfully for ", destination).AtInfo().WriteToLog(session.ExportIDToError(ctx))
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
                    newError("TUIC UDP write size=", len(payload), " dest=", destination).AtDebug().WriteToLog(session.ExportIDToError(ctx))
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
                n, addr, err := packetConn.ReadFrom(readBuf)
                if err != nil {
                    return err
                }
                newError("TUIC UDP read size=", n, " src=", addr).AtDebug().WriteToLog(session.ExportIDToError(ctx))
                timer.Update()

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

// udpDialer implements netproxy.Dialer for UDP connections
// TUIC uses QUIC which requires UDP, not TCP
type udpDialer struct{}

func (d *udpDialer) DialContext(ctx context.Context, network, addr string) (netproxy.Conn, error) {
	// TUIC outbound library expects UDP PacketConn
	// Parse the network to ensure it's UDP
	magicNetwork, err := netproxy.ParseMagicNetwork(network)
	if err != nil {
		return nil, newError("failed to parse network").Base(err)
	}

	// Ensure we're using UDP
	if magicNetwork.Network != "udp" {
		return nil, newError("TUIC requires UDP network, got: ", magicNetwork.Network)
	}

	// For TUIC, prefer a non-connected UDP socket, similar to hysteria2 transport
	// This allows the protocol layer to use WriteTo/ReadFrom freely
	conn, err := net.ListenUDP("udp", nil)
	if err != nil {
		return nil, newError("failed to listen UDP").Base(err)
	}

	// Wrap as netproxy.PacketConn
	// net.UDPConn implements both net.Conn and net.PacketConn
	// We need to implement netproxy.PacketConn interface
	return &udpPacketConnWrapper{UDPConn: conn}, nil
}

// udpPacketConnWrapper wraps net.UDPConn to implement netproxy.PacketConn
type udpPacketConnWrapper struct {
	*net.UDPConn
}

func (w *udpPacketConnWrapper) ReadFrom(p []byte) (n int, addr netip.AddrPort, err error) {
	n, addrNet, err := w.UDPConn.ReadFromUDP(p)
	if err != nil {
		return 0, netip.AddrPort{}, err
	}
	if addrNet != nil {
		ip, _ := netip.ParseAddr(addrNet.IP.String())
		addr = netip.AddrPortFrom(ip, uint16(addrNet.Port))
	}
	return n, addr, nil
}

func (w *udpPacketConnWrapper) WriteTo(p []byte, addr string) (n int, err error) {
	if w.UDPConn.RemoteAddr() != nil {
		return w.UDPConn.Write(p)
	}
	udpAddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		return 0, err
	}
	return w.UDPConn.WriteTo(p, udpAddr)
}
