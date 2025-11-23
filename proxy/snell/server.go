package snell

import (
	"context"
	"crypto/rand" // Added for rand.Read
	"encoding/binary"
	"io"
	"time"

	core "github.com/frogwall/f2ray-core/v5"
	"github.com/frogwall/f2ray-core/v5/common"
	"github.com/frogwall/f2ray-core/v5/common/buf"
	"github.com/frogwall/f2ray-core/v5/common/log"
	"github.com/frogwall/f2ray-core/v5/common/net"
	udp_proto "github.com/frogwall/f2ray-core/v5/common/protocol/udp"
	"github.com/frogwall/f2ray-core/v5/common/session"
	"github.com/frogwall/f2ray-core/v5/common/task"
	"github.com/frogwall/f2ray-core/v5/features/policy"
	"github.com/frogwall/f2ray-core/v5/features/routing"
	"github.com/frogwall/f2ray-core/v5/transport/internet"
	"github.com/frogwall/f2ray-core/v5/transport/internet/udp"
)

type Server struct {
	config        *ServerConfig
	user          *MemoryAccount
	policyManager policy.Manager
}

func NewServer(ctx context.Context, config *ServerConfig) (*Server, error) {
	if config.User == nil {
		return nil, newError("user is not specified")
	}

	account, err := config.User.ToMemoryUser()
	if err != nil {
		return nil, newError("failed to parse user account").Base(err)
	}

	mUser, ok := account.Account.(*MemoryAccount)
	if !ok {
		return nil, newError("user account is not Snell account")
	}

	v := core.MustFromContext(ctx)
	s := &Server{
		config:        config,
		user:          mUser,
		policyManager: v.GetFeature(policy.ManagerType()).(policy.Manager),
	}

	return s, nil
}

func (s *Server) Network() []net.Network {
	return []net.Network{net.Network_TCP}
}

func (s *Server) Process(ctx context.Context, network net.Network, conn internet.Connection, dispatcher routing.Dispatcher) error {
	if network == net.Network_TCP {
		return s.handleConnection(ctx, conn, dispatcher)
	}
	return newError("unknown network: ", network)
}

func (s *Server) handleConnection(ctx context.Context, conn internet.Connection, dispatcher routing.Dispatcher) error {
	sessionPolicy := s.policyManager.ForLevel(0) // Default level
	conn.SetReadDeadline(time.Now().Add(sessionPolicy.Timeouts.Handshake))

	newError("snell server: start handshake").AtInfo().WriteToLog(session.ExportIDToError(ctx))
	// 1. Read IV
	iv := make([]byte, s.user.Cipher.IVSize())
	if _, err := io.ReadFull(conn, iv); err != nil {
		newError("snell server: read IV failed", err).AtError().WriteToLog(session.ExportIDToError(ctx))
		return newError("failed to read IV").Base(err)
	}
	newError("snell server: read IV ok size=", len(iv)).AtDebug().WriteToLog(session.ExportIDToError(ctx))

	// 2. Create Decryption Reader
	reader, err := s.user.Cipher.NewDecryptionReader(iv, conn)
	if err != nil {
		newError("snell server: new decrypt reader failed", err).AtError().WriteToLog(session.ExportIDToError(ctx))
		return newError("failed to create decryption reader").Base(err)
	}
	bufferedReader := &buf.BufferedReader{Reader: reader}

	// 3. Read Snell Request
	request, err := ReadSnellRequest(bufferedReader)
	if err != nil {
		newError("snell server: read snell request failed", err).AtError().WriteToLog(session.ExportIDToError(ctx))
		return newError("failed to read snell request").Base(err)
	}
	newError("snell server: request cmd=", request.Command, " addr=", request.Address, " port=", request.Port).AtInfo().WriteToLog(session.ExportIDToError(ctx))
	conn.SetReadDeadline(time.Time{})

	// 4. Create Encryption Writer
	// Generate response IV
	respIV := make([]byte, s.user.Cipher.IVSize())
	common.Must2(rand.Read(respIV)) // Use random source
	if _, err := conn.Write(respIV); err != nil {
		newError("snell server: write resp IV failed", err).AtError().WriteToLog(session.ExportIDToError(ctx))
		return newError("failed to write response IV").Base(err)
	}
	newError("snell server: write resp IV ok size=", len(respIV)).AtDebug().WriteToLog(session.ExportIDToError(ctx))

	// But first, handle the command.

	if request.Command == CommandPing {
		// Respond to ping?
		// Reference: `write_all(&mut server_stream, &[0x01]).await?;`
		// But we need to encrypt it.
		// So we set up encryption writer, write 0x01, flush, and close.
		return s.handlePing(conn, respIV)
	}

	if request.Command == CommandUDP {
		return s.handleUDP(ctx, conn, bufferedReader, respIV, dispatcher)
	}

	if request.Command == CommandConnect {
		return s.handleTCP(ctx, conn, bufferedReader, request, respIV, dispatcher, sessionPolicy)
	}

	return newError("unknown command: ", request.Command)
}

func (s *Server) handlePing(conn internet.Connection, iv []byte) error {
	if _, err := conn.Write(iv); err != nil {
		return err
	}
	writer, err := s.user.Cipher.NewEncryptionWriter(iv, conn)
	if err != nil {
		return err
	}
	// Ping response is 0x01?
	// Reference: `write_all(&mut server_stream, &[0x01]).await?;`
	// Yes.
	b := buf.New()
	b.WriteByte(0x01)
	return writer.WriteMultiBuffer(buf.MultiBuffer{b})
}

func (s *Server) handleTCP(ctx context.Context, conn internet.Connection, reader buf.Reader, request *SnellRequest, iv []byte, dispatcher routing.Dispatcher, sessionPolicy policy.Session) error {
	dest := net.Destination{
		Network: net.Network_TCP,
		Address: request.Address,
		Port:    request.Port,
	}

	newError("tunnelling request to ", dest).WriteToLog(session.ExportIDToError(ctx))

	// Write Response IV
	if _, err := conn.Write(iv); err != nil {
		return err
	}

	// Create Encryption Writer
	writer, err := s.user.Cipher.NewEncryptionWriter(iv, conn)
	if err != nil {
		return err
	}

	// Write Success Response (0x00)
	// Reference: `TCP_TUNNEL_RESPONSE: &[u8] = &[0x0];`
	b := buf.New()
	b.WriteByte(0x00)
	if err := writer.WriteMultiBuffer(buf.MultiBuffer{b}); err != nil {
		return err
	}

	// Dispatch to destination
	ctx = log.ContextWithAccessMessage(ctx, &log.AccessMessage{
		From:   conn.RemoteAddr(),
		To:     dest,
		Status: log.AccessAccepted,
		Reason: "",
	})

	link, err := dispatcher.Dispatch(ctx, dest)
	if err != nil {
		return err
	}

	// Forwarding
	requestDone := func() error {
		return buf.Copy(reader, link.Writer, buf.UpdateActivity(nil))
	}

	responseDone := func() error {
		return buf.Copy(link.Reader, writer, buf.UpdateActivity(nil))
	}

	if err := task.Run(ctx, requestDone, responseDone); err != nil {
		return newError("connection ends").Base(err)
	}

	return nil
}

func (s *Server) handleUDP(ctx context.Context, conn internet.Connection, reader buf.Reader, iv []byte, dispatcher routing.Dispatcher) error {
	// Write Response IV
	if _, err := conn.Write(iv); err != nil {
		return err
	}

	// Create Encryption Writer
	writer, err := s.user.Cipher.NewEncryptionWriter(iv, conn)
	if err != nil {
		return err
	}

	// Write UDP Ready Response (0x00)
	b := buf.New()
	b.WriteByte(0x00)
	if err := writer.WriteMultiBuffer(buf.MultiBuffer{b}); err != nil {
		return err
	}

	// UDP Dispatcher
	udpServer := udp.NewSplitDispatcher(dispatcher, func(ctx context.Context, packet *udp_proto.Packet) {
		// Write back to tunnel
		// Packet payload is the data.
		// Source address is packet.Source.
		// We need to write [Cmd] [IP] [Port] [Payload]

		// We need a writer that accepts []byte.
		// `writer` is `buf.Writer`. It has `WriteMultiBuffer`.
		// We can create a wrapper or just use a buffer.

		payload := packet.Payload.Bytes()
		dest := net.Destination{
			Network: net.Network_UDP,
			Address: packet.Source.Address,
			Port:    packet.Source.Port,
		}

		// Since `WriteUDPPacket` takes `io.Writer`, we need an adapter for `buf.Writer`.
		// Or we can just implement `WriteUDPPacket` logic here using `buf.Buffer`.

		buffer := buf.New()
		if dest.Address.Family().IsIPv4() {
			buffer.WriteByte(UDPCommandIPv4)
			buffer.Write(dest.Address.IP())
			portBytes := make([]byte, 2)
			binary.BigEndian.PutUint16(portBytes, dest.Port.Value())
			buffer.Write(portBytes)
		} else if dest.Address.Family().IsIPv6() {
			buffer.WriteByte(UDPCommandIPv6)
			buffer.Write(dest.Address.IP())
			portBytes := make([]byte, 2)
			binary.BigEndian.PutUint16(portBytes, dest.Port.Value())
			buffer.Write(portBytes)
		} else {
			// Drop domain
			packet.Payload.Release()
			return
		}
		buffer.Write(payload)
		packet.Payload.Release()

		newError("snell udp server write back size=", buffer.Len(), " src=", dest).AtDebug().WriteToLog(session.ExportIDToError(ctx))
		writer.WriteMultiBuffer(buf.MultiBuffer{buffer})
	})

	// Read Loop
	// `reader` is `buf.Reader`.
	// We need to read packets.
	// `ReadUDPPacket` takes `io.Reader`.
	// `buf.BufferedReader` implements `io.Reader`.
	// `reader` passed here is `buf.Reader` interface.
	// If it is `*buf.BufferedReader`, we can cast it.
	// Or we can wrap it.

	for {
		dest, payload, err := ReadClientUDPPacket(reader)
		if err != nil {
			newError("snell udp server read failed", err).AtError().WriteToLog(session.ExportIDToError(ctx))
			return err
		}
		newError("snell udp server read size=", payload.Len(), " dest=", dest).AtDebug().WriteToLog(session.ExportIDToError(ctx))

		currentPacketCtx := ctx
		if dest != nil {
			// Dispatch
			udpServer.Dispatch(currentPacketCtx, *dest, payload)
		} else {
			payload.Release()
		}
	}
}
