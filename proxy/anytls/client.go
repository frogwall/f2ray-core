package anytls

import (
	"context"
	stdnet "net"
	"time"

	anytls "github.com/anytls/sing-anytls"
	"github.com/sagernet/sing/common/metadata"
	core "github.com/frogwall/v2ray-core/v5"
	"github.com/frogwall/v2ray-core/v5/common"
	"github.com/frogwall/v2ray-core/v5/common/buf"
	"github.com/frogwall/v2ray-core/v5/common/net"
	"github.com/frogwall/v2ray-core/v5/common/retry"
	"github.com/frogwall/v2ray-core/v5/common/session"
	"github.com/frogwall/v2ray-core/v5/common/signal"
	"github.com/frogwall/v2ray-core/v5/common/task"
	"github.com/frogwall/v2ray-core/v5/features/policy"
	"github.com/frogwall/v2ray-core/v5/transport"
	"github.com/frogwall/v2ray-core/v5/transport/internet"
)

// Client is an outbound handler for AnyTLS protocol
type Client struct {
	serverPicker  ServerPicker
	policyManager policy.Manager
	config        *ClientConfig
}

// ServerPicker is a simple interface for picking servers
type ServerPicker interface {
	PickServer() *ServerEndpoint
}

// roundRobinServerPicker implements ServerPicker with round-robin selection
type roundRobinServerPicker struct {
	servers []*ServerEndpoint
	current int
}

func newRoundRobinServerPicker(servers []*ServerEndpoint) *roundRobinServerPicker {
	return &roundRobinServerPicker{
		servers: servers,
		current: 0,
	}
}

func (p *roundRobinServerPicker) PickServer() *ServerEndpoint {
	if len(p.servers) == 0 {
		return nil
	}
	server := p.servers[p.current]
	p.current = (p.current + 1) % len(p.servers)
	return server
}

// NewClient creates a new AnyTLS client
func NewClient(ctx context.Context, config *ClientConfig) (*Client, error) {
	if len(config.Servers) == 0 {
		return nil, newError("no server configured")
	}

	v := core.MustFromContext(ctx)
	client := &Client{
		serverPicker:  newRoundRobinServerPicker(config.Servers),
		policyManager: v.GetFeature(policy.ManagerType()).(policy.Manager),
		config:        config,
	}
	return client, nil
}

// Process implements OutboundHandler.Process()
func (c *Client) Process(ctx context.Context, link *transport.Link, dialer internet.Dialer) error {
	outbound := session.OutboundFromContext(ctx)
	if outbound == nil || !outbound.Target.IsValid() {
		return newError("target not specified")
	}
	destination := outbound.Target

	// Pick a server
	server := c.serverPicker.PickServer()
	if server == nil {
		return newError("no server available")
	}

	// Build server destination
	serverDest := net.Destination{
		Network: net.Network_TCP,
		Address: net.ParseAddress(server.Address),
		Port:    net.Port(server.Port),
	}

	newError("tunneling request to ", destination, " via ", serverDest).WriteToLog(session.ExportIDToError(ctx))

	// Create AnyTLS client instance
	idleCheckInterval := time.Duration(c.config.IdleSessionCheckInterval) * time.Second
	if idleCheckInterval == 0 {
		idleCheckInterval = 30 * time.Second
	}
	idleTimeout := time.Duration(c.config.IdleSessionTimeout) * time.Second
	if idleTimeout == 0 {
		idleTimeout = 30 * time.Second
	}

	anytlsClient, err := anytls.NewClient(ctx, anytls.ClientConfig{
		Password:                 server.Password,
		IdleSessionCheckInterval: idleCheckInterval,
		IdleSessionTimeout:       idleTimeout,
		MinIdleSession:           int(c.config.MinIdleSession),
		DialOut: func(ctx context.Context) (stdnet.Conn, error) {
			return c.dialTLS(ctx, serverDest, dialer)
		},
		Logger: newAnytlsLogger(ctx),
	})
	if err != nil {
		return newError("failed to create anytls client").Base(err)
	}
	defer anytlsClient.Close()

	// Create proxy connection through AnyTLS
	var conn stdnet.Conn
	err = retry.ExponentialBackoff(5, 100).On(func() error {
		// Convert v2ray destination to sing socksaddr
		socksAddr := toSingSocksaddr(destination)
		conn, err = anytlsClient.CreateProxy(ctx, socksAddr)
		return err
	})
	if err != nil {
		return newError("failed to create proxy connection").Base(err)
	}
	defer conn.Close()

	// Connection is ready to use

	// Setup policy and timeout
	sessionPolicy := c.policyManager.ForLevel(0)
	ctx, cancel := context.WithCancel(ctx)
	timer := signal.CancelAfterInactivity(ctx, cancel, sessionPolicy.Timeouts.ConnectionIdle)

	// Handle data transfer
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

// dialTLS establishes a TLS connection to the server
func (c *Client) dialTLS(ctx context.Context, dest net.Destination, dialer internet.Dialer) (stdnet.Conn, error) {
	rawConn, err := dialer.Dial(ctx, dest)
	if err != nil {
		return nil, err
	}

	// Simply return the connection - TLS should be configured via streamSettings
	// The dialer will handle TLS if configured properly
	return rawConn, nil
}

// anytlsLogger implements logger interface for AnyTLS
type anytlsLogger struct {
	ctx context.Context
}

func newAnytlsLogger(ctx context.Context) *anytlsLogger {
	return &anytlsLogger{ctx: ctx}
}

func (l *anytlsLogger) Trace(args ...interface{}) {}
func (l *anytlsLogger) Debug(args ...interface{}) {}
func (l *anytlsLogger) Info(args ...interface{}) {}
func (l *anytlsLogger) Warn(args ...interface{}) {}
func (l *anytlsLogger) Error(args ...interface{}) {
	newError(args...).AtError().WriteToLog(session.ExportIDToError(l.ctx))
}
func (l *anytlsLogger) Fatal(args ...interface{}) {
	newError(args...).AtError().WriteToLog(session.ExportIDToError(l.ctx))
}
func (l *anytlsLogger) Panic(args ...interface{}) {
	newError(args...).AtError().WriteToLog(session.ExportIDToError(l.ctx))
}

func (l *anytlsLogger) TraceContext(ctx context.Context, args ...interface{}) {}
func (l *anytlsLogger) DebugContext(ctx context.Context, args ...interface{}) {}
func (l *anytlsLogger) InfoContext(ctx context.Context, args ...interface{}) {}
func (l *anytlsLogger) WarnContext(ctx context.Context, args ...interface{}) {}
func (l *anytlsLogger) ErrorContext(ctx context.Context, args ...interface{}) {
	newError(args...).AtError().WriteToLog(session.ExportIDToError(ctx))
}
func (l *anytlsLogger) FatalContext(ctx context.Context, args ...interface{}) {
	newError(args...).AtError().WriteToLog(session.ExportIDToError(ctx))
}
func (l *anytlsLogger) PanicContext(ctx context.Context, args ...interface{}) {
	newError(args...).AtError().WriteToLog(session.ExportIDToError(ctx))
}

// toSingSocksaddr converts v2ray destination to sing socksaddr
func toSingSocksaddr(dest net.Destination) metadata.Socksaddr {
	// Use ParseSocksaddrHostPort to create the socksaddr
	hostStr := dest.Address.String()
	port := dest.Port.Value()
	return metadata.ParseSocksaddrHostPort(hostStr, port)
}

func init() {
	common.Must(common.RegisterConfig((*ClientConfig)(nil), func(ctx context.Context, config interface{}) (interface{}, error) {
		return NewClient(ctx, config.(*ClientConfig))
	}))
}
