package shadowtls

import (
	"context"

	"github.com/sagernet/sing-shadowtls"
	M "github.com/sagernet/sing/common/metadata"
	"github.com/frogwall/f2ray-core/v5/common"
	"github.com/frogwall/f2ray-core/v5/common/net"
	"github.com/frogwall/f2ray-core/v5/common/session"
	"github.com/frogwall/f2ray-core/v5/transport/internet"
)

// Listener is a ShadowTLS listener
type Listener struct {
	listener net.Listener
	service  *shadowtls.Service
	config   *Config
	addConn  internet.ConnHandler
}

// Listen creates a ShadowTLS listener
func Listen(ctx context.Context, address net.Address, port net.Port, streamSettings *internet.MemoryStreamConfig, handler internet.ConnHandler) (internet.Listener, error) {
	stSettings := streamSettings.ProtocolSettings.(*Config)

	// Validate settings
	if stSettings.Version == 0 {
		stSettings.Version = 3 // Default to v3
	}
	if stSettings.Version < 1 || stSettings.Version > 3 {
		return nil, newError("invalid ShadowTLS version: ", stSettings.Version)
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

	// Prepare users for v3
	var users []shadowtls.User
	if stSettings.Version == 3 && len(stSettings.Users) > 0 {
		users = make([]shadowtls.User, len(stSettings.Users))
		for i, u := range stSettings.Users {
			users[i] = shadowtls.User{
				Name:     u.Name,
				Password: u.Password,
			}
		}
	}

	// Create v2ray dialer for handshake
	handshakeDialer := &v2rayDialerWrapper{
		ctx:            ctx,
		streamSettings: streamSettings,
	}

	// Create ShadowTLS service
	service, err := shadowtls.NewService(shadowtls.ServiceConfig{
		Version:  int(stSettings.Version),
		Password: stSettings.Password,
		Users:    users,
		Handshake: shadowtls.HandshakeConfig{
			Server: M.ParseSocksaddr(net.Destination{Address: net.ParseAddress(handshakeServer), Port: net.Port(handshakePort)}.NetAddr()),
			Dialer: handshakeDialer,
		},
		StrictMode: stSettings.StrictMode,
		Handler:    &shadowtlsHandler{handler: handler},
		Logger:     &stLogger{},
	})
	if err != nil {
		return nil, newError("failed to create ShadowTLS service").Base(err)
	}

	// Listen on TCP
	listener, err := internet.ListenSystem(ctx, &net.TCPAddr{
		IP:   address.IP(),
		Port: int(port),
	}, streamSettings.SocketSettings)
	if err != nil {
		return nil, newError("failed to listen on ", address, ":", port).Base(err)
	}

	stListener := &Listener{
		listener: listener,
		service:  service,
		config:   stSettings,
		addConn:  handler,
	}

	go stListener.acceptLoop()

	return stListener, nil
}

// acceptLoop accepts incoming connections
func (l *Listener) acceptLoop() {
	for {
		conn, err := l.listener.Accept()
		if err != nil {
			if errors := newError("failed to accept connection"); errors != nil {
				errors.Base(err).WriteToLog()
			}
			return
		}

		go l.handleConn(conn)
	}
}

// handleConn handles a single connection
func (l *Listener) handleConn(conn net.Conn) {
	ctx := context.Background()
	
	// Get source and destination addresses
	source := M.SocksaddrFromNet(conn.RemoteAddr())
	dest := M.SocksaddrFromNet(conn.LocalAddr())

	// Process connection through ShadowTLS service
	err := l.service.NewConnection(ctx, conn, source, dest, nil)
	if err != nil {
		newError("failed to process ShadowTLS connection").Base(err).WriteToLog()
		conn.Close()
	}
}

// Addr returns the listener's address
func (l *Listener) Addr() net.Addr {
	return l.listener.Addr()
}

// Close closes the listener
func (l *Listener) Close() error {
	return l.listener.Close()
}


// shadowtlsHandler implements shadowtls.Handler interface
type shadowtlsHandler struct {
	handler internet.ConnHandler
}

// NewConnectionEx handles new connections from ShadowTLS
func (h *shadowtlsHandler) NewConnectionEx(ctx context.Context, conn net.Conn, source M.Socksaddr, destination M.Socksaddr, onClose func(error)) {
	// Convert to v2ray types
	v2raySource := net.Destination{
		Network: net.Network_TCP,
		Address: net.ParseAddress(source.AddrString()),
		Port:    net.Port(source.Port),
	}

	// Create inbound context
	inbound := &session.Inbound{
		Source: v2raySource,
		Tag:    "shadowtls",
	}
	
	ctx = session.ContextWithInbound(ctx, inbound)
	
	// Handle the connection - wrap net.Conn as internet.Connection
	h.handler(internet.Connection(conn))
}

func init() {
	common.Must(internet.RegisterTransportListener(protocolName, Listen))
}
