package hysteria2

import (
	"context"
	"io"
	"math/rand"

	"github.com/apernet/quic-go/quicvarint"
	core "github.com/frogwall/f2ray-core/v5"
	"github.com/frogwall/f2ray-core/v5/common"
	"github.com/frogwall/f2ray-core/v5/common/buf"
	"github.com/frogwall/f2ray-core/v5/common/net"
	"github.com/frogwall/f2ray-core/v5/common/net/packetaddr"
	"github.com/frogwall/f2ray-core/v5/common/protocol"
	"github.com/frogwall/f2ray-core/v5/common/retry"
	"github.com/frogwall/f2ray-core/v5/common/session"
	"github.com/frogwall/f2ray-core/v5/common/signal"
	"github.com/frogwall/f2ray-core/v5/common/task"
	"github.com/frogwall/f2ray-core/v5/features/policy"
	"github.com/frogwall/f2ray-core/v5/transport"
	"github.com/frogwall/f2ray-core/v5/transport/internet"
	hyTransport "github.com/frogwall/f2ray-core/v5/transport/internet/hysteria2"
	"github.com/frogwall/f2ray-core/v5/transport/internet/udp"
)

const (
	FrameTypeTCPRequest = 0x401
	MaxAddressLength    = 2048
	MaxMessageLength    = 2048
	MaxPaddingLength    = 4096
)

// Config represents the hysteria2 client configuration
type Config struct {
	Server []*protocol.ServerEndpoint
}

// Client is an enhanced inbound handler with full hysteria2 protocol support
type Client struct {
	serverPicker  protocol.ServerPicker
	policyManager policy.Manager
	config        *ClientConfig
}

// NewClient creates a new enhanced client.
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
	client := &Client{
		serverPicker:  protocol.NewRoundRobinServerPicker(serverList),
		policyManager: v.GetFeature(policy.ManagerType()).(policy.Manager),
		config:        config,
	}
	return client, nil
}

// Process implements OutboundHandler.Process().
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

		// Get password from user account
		var password string
		if user := server.PickUser(); user != nil {
			if account := user.Account; account != nil {
				if memoryAccount, ok := account.(*MemoryAccount); ok {
					password = memoryAccount.Password
				}
			}
		}

		// Pass password to transport layer via context
		ctxWithPassword := context.WithValue(ctx, "hysteria2_password", password)

		rawConn, err := dialer.Dial(ctxWithPassword, server.Destination())
		if err != nil {
			return err
		}

		conn = rawConn
		return nil
	})
	if err != nil {
		return newError("failed to find an available destination").AtWarning().Base(err)
	}
	newError("tunneling request to ", destination, " via ", server.Destination().NetAddr()).WriteToLog(session.ExportIDToError(ctx))

	defer conn.Close()

	// Perform hysteria2 authentication
	user := server.PickUser()
	if user == nil {
		return newError("no user found for server")
	}

	// Get password from user account (same as used for transport layer)
	var password string
	if account := user.Account; account != nil {
		if memoryAccount, ok := account.(*MemoryAccount); ok {
			password = memoryAccount.Password
		}
	}
	if password == "" {
		return newError("password not found in user account")
	}

	if len(password) > 0 {
		newError("using password for authentication: ", password[:min(8, len(password))]+"...").WriteToLog(session.ExportIDToError(ctx))
	} else {
		newError("using empty password for authentication").WriteToLog(session.ExportIDToError(ctx))
	}

	// Perform authentication
	authResp, err := c.authenticate(ctx, conn, server.Destination().NetAddr(), password)
	if err != nil {
		return newError("authentication failed").Base(err)
	}
	newError("hysteria2 authentication successful, UDP enabled: ", authResp.UDPEnabled).WriteToLog(session.ExportIDToError(ctx))

	iConn := conn
	if statConn, ok := conn.(*internet.StatCouterConnection); ok {
		iConn = statConn.Connection // will not count the UDP traffic.
	}
	hyConn, isHy2Transport := iConn.(*hyTransport.HyConn)

	if !isHy2Transport && network == net.Network_UDP {
		// hysteria2 need to use udp extension to proxy UDP.
		return newError(hyTransport.CanNotUseUDPExtension)
	}

	userLevel := uint32(0)
	if user != nil {
		userLevel = user.Level
	}
	sessionPolicy := c.policyManager.ForLevel(userLevel)
	ctx, cancel := context.WithCancel(ctx)
	timer := signal.CancelAfterInactivity(ctx, cancel, sessionPolicy.Timeouts.ConnectionIdle)

	if packetConn, err := packetaddr.ToPacketAddrConn(link, destination); err == nil {
		// UDP handling
		postRequest := func() error {
			defer timer.SetTimeout(sessionPolicy.Timeouts.DownlinkOnly)

			var buffer [2048]byte
			n, addr, err := packetConn.ReadFrom(buffer[:])
			if err != nil {
				return newError("failed to read a packet").Base(err)
			}
			dest := net.DestinationFromAddr(addr)

			bufferWriter := buf.NewBufferedWriter(buf.NewWriter(conn))
			connWriter := &ConnWriter{Writer: bufferWriter, Target: dest}
			packetWriter := &PacketWriter{Writer: connWriter, Target: dest, HyConn: hyConn}

			// write some request payload to buffer
			if _, err := packetWriter.WriteTo(buffer[:n], addr); err != nil {
				return newError("failed to write a request payload").Base(err)
			}

			// Flush; bufferWriter.WriteMultiBuffer now is bufferWriter.writer.WriteMultiBuffer
			if err = bufferWriter.SetBuffered(false); err != nil {
				return newError("failed to flush payload").Base(err).AtWarning()
			}

			return udp.CopyPacketConn(packetWriter, packetConn, udp.UpdateActivity(timer))
		}

		getResponse := func() error {
			defer timer.SetTimeout(sessionPolicy.Timeouts.UplinkOnly)

			packetReader := &PacketReader{Reader: conn, HyConn: hyConn}
			packetConnectionReader := &PacketConnectionReader{reader: packetReader}

			return udp.CopyPacketConn(packetConn, packetConnectionReader, udp.UpdateActivity(timer))
		}

		responseDoneAndCloseWriter := task.OnSuccess(getResponse, task.Close(link.Writer))
		if err := task.Run(ctx, postRequest, responseDoneAndCloseWriter); err != nil {
			return newError("connection ends").Base(err)
		}

		return nil
	}

	// TCP handling - hysteria2 transport layer handles the protocol
	postRequest := func() error {
		defer timer.SetTimeout(sessionPolicy.Timeouts.DownlinkOnly)

		// For hysteria2, we need to write the destination address and padding
		// The transport layer already handles the TCP frame type
		if err := c.writeDestinationWithPadding(conn, destination); err != nil {
			return newError("failed to write destination with padding").Base(err)
		}

		// Copy data
		return buf.Copy(link.Reader, buf.NewWriter(conn), buf.UpdateActivity(timer))
	}

	getResponse := func() error {
		defer timer.SetTimeout(sessionPolicy.Timeouts.UplinkOnly)

		// Read hysteria2 TCP response
		ok, msg, err := c.readTCPResponse(conn)
		if err != nil {
			return newError("failed to read TCP response").Base(err)
		}
		if !ok {
			return newError("server rejected connection: ", msg)
		}

		// Copy response data
		return buf.Copy(buf.NewReader(conn), link.Writer, buf.UpdateActivity(timer))
	}

	responseDoneAndCloseWriter := task.OnSuccess(getResponse, task.Close(link.Writer))
	if err := task.Run(ctx, postRequest, responseDoneAndCloseWriter); err != nil {
		return newError("connection ends").Base(err)
	}

	return nil
}

// authenticate performs hysteria2 authentication
func (c *Client) authenticate(ctx context.Context, conn internet.Connection, serverAddr string, password string) (*AuthResponse, error) {
	// The transport layer already handles authentication when creating the hysteria client
	// We just need to return a successful response since the connection is already authenticated
	return &AuthResponse{
		UDPEnabled: true,
		Rx:         0, // Unlimited
		RxAuto:     false,
	}, nil
}

// writeDestination writes the destination address for hysteria2
func (c *Client) writeDestination(conn internet.Connection, destination net.Destination) error {
	// Write destination address in hysteria2 format
	addr := destination.NetAddr()
	addrLen := len(addr)
	if addrLen > MaxAddressLength {
		return newError("address length too large: ", addrLen)
	}

	// Write address length and address
	buf := make([]byte, int(quicvarint.Len(uint64(addrLen)))+addrLen)
	i := varintPut(buf, uint64(addrLen))
	copy(buf[i:], addr)

	_, err := conn.Write(buf)
	return err
}

// writeDestinationWithPadding writes the destination address with padding for hysteria2
func (c *Client) writeDestinationWithPadding(conn internet.Connection, destination net.Destination) error {
	// Generate random padding
	paddingLen := 64 + rand.Intn(512-64)
	padding := make([]byte, paddingLen)
	for i := range padding {
		padding[i] = byte(rand.Intn(256))
	}

	addr := destination.NetAddr()
	addrLen := len(addr)
	if addrLen > MaxAddressLength {
		return newError("address length too large: ", addrLen)
	}

	// Write address length, address, padding length, and padding
	// The transport layer already wrote the TCP frame type
	buf := make([]byte,
		int(quicvarint.Len(uint64(addrLen)))+addrLen+
			int(quicvarint.Len(uint64(paddingLen)))+paddingLen)

	i := varintPut(buf, uint64(addrLen))
	i += copy(buf[i:], addr)
	i += varintPut(buf[i:], uint64(paddingLen))
	copy(buf[i:], padding)

	_, err := conn.Write(buf)
	return err
}

// writeTCPRequest writes a hysteria2 TCP request
func (c *Client) writeTCPRequest(conn internet.Connection, addr string) error {
	// Generate random padding
	paddingLen := 64 + rand.Intn(512-64)
	padding := make([]byte, paddingLen)
	for i := range padding {
		padding[i] = byte(rand.Intn(256))
	}

	addrLen := len(addr)
	if addrLen > MaxAddressLength {
		return newError("address length too large: ", addrLen)
	}

	// Calculate total size
	size := int(quicvarint.Len(FrameTypeTCPRequest)) +
		int(quicvarint.Len(uint64(addrLen))) + addrLen +
		int(quicvarint.Len(uint64(paddingLen))) + paddingLen

	buf := make([]byte, size)
	i := varintPut(buf, FrameTypeTCPRequest)
	i += varintPut(buf[i:], uint64(addrLen))
	i += copy(buf[i:], addr)
	i += varintPut(buf[i:], uint64(paddingLen))
	copy(buf[i:], padding)

	_, err := conn.Write(buf)
	return err
}

// readTCPResponse reads a hysteria2 TCP response
func (c *Client) readTCPResponse(conn internet.Connection) (bool, string, error) {
	// Read status byte
	var status [1]byte
	if _, err := io.ReadFull(conn, status[:]); err != nil {
		return false, "", err
	}

	// Read message length
	msgLen, err := quicvarint.Read(quicvarint.NewReader(conn))
	if err != nil {
		return false, "", err
	}

	if msgLen > MaxMessageLength {
		return false, "", newError("invalid message length")
	}

	// Read message
	var msg string
	if msgLen > 0 {
		msgBuf := make([]byte, msgLen)
		if _, err := io.ReadFull(conn, msgBuf); err != nil {
			return false, "", err
		}
		msg = string(msgBuf)
	}

	// Read padding length
	paddingLen, err := quicvarint.Read(quicvarint.NewReader(conn))
	if err != nil {
		return false, "", err
	}

	if paddingLen > MaxPaddingLength {
		return false, "", newError("invalid padding length")
	}

	// Skip padding
	if paddingLen > 0 {
		_, err = io.CopyN(io.Discard, conn, int64(paddingLen))
		if err != nil {
			return false, "", err
		}
	}

	return status[0] == 0, msg, nil
}

// varintPut writes a QUIC varint to the buffer
func varintPut(b []byte, i uint64) int {
	if i <= 63 {
		b[0] = uint8(i)
		return 1
	}
	if i <= 16383 {
		b[0] = uint8(i>>8) | 0x40
		b[1] = uint8(i)
		return 2
	}
	if i <= 1073741823 {
		b[0] = uint8(i>>24) | 0x80
		b[1] = uint8(i >> 16)
		b[2] = uint8(i >> 8)
		b[3] = uint8(i)
		return 4
	}
	if i <= 4611686018427387903 {
		b[0] = uint8(i>>56) | 0xc0
		b[1] = uint8(i >> 48)
		b[2] = uint8(i >> 40)
		b[3] = uint8(i >> 32)
		b[4] = uint8(i >> 24)
		b[5] = uint8(i >> 16)
		b[6] = uint8(i >> 8)
		b[7] = uint8(i)
		return 8
	}
	panic("varint too large")
}

func init() {
	common.Must(common.RegisterConfig((*ClientConfig)(nil), func(ctx context.Context, config interface{}) (interface{}, error) {
		return NewClient(ctx, config.(*ClientConfig))
	}))
}
