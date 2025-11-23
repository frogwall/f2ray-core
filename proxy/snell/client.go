package snell

import (
	"context"
	"crypto/rand"
	"io"

	core "github.com/frogwall/f2ray-core/v5"
	"github.com/frogwall/f2ray-core/v5/common"
	"github.com/frogwall/f2ray-core/v5/common/buf"
	"github.com/frogwall/f2ray-core/v5/common/net"
	"github.com/frogwall/f2ray-core/v5/common/protocol"
	"github.com/frogwall/f2ray-core/v5/common/retry"
	"github.com/frogwall/f2ray-core/v5/common/session"
	"github.com/frogwall/f2ray-core/v5/common/signal"
	"github.com/frogwall/f2ray-core/v5/common/task"
	"github.com/frogwall/f2ray-core/v5/features/policy"
	"github.com/frogwall/f2ray-core/v5/transport"
	"github.com/frogwall/f2ray-core/v5/transport/internet"
)

type Client struct {
	serverPicker  protocol.ServerPicker
	policyManager policy.Manager
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
		dest.Network = net.Network_TCP

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
		return newError("user account is not Snell account")
	}

	sessionPolicy := c.policyManager.ForLevel(user.Level)
	ctx, cancel := context.WithCancel(ctx)
	timer := signal.CancelAfterInactivity(ctx, cancel, sessionPolicy.Timeouts.ConnectionIdle)

	// Handshake

	newError("snell client: start handshake dest=", destination).AtInfo().WriteToLog(session.ExportIDToError(ctx))
	// 1. Write IV
	iv := make([]byte, account.Cipher.IVSize())
	common.Must2(rand.Read(iv))
	if _, err := conn.Write(iv); err != nil {
		newError("snell client: write IV failed", err).AtError().WriteToLog(session.ExportIDToError(ctx))
		return newError("failed to write IV").Base(err)
	}
	newError("snell client: write IV ok size=", len(iv)).AtDebug().WriteToLog(session.ExportIDToError(ctx))

	// 2. Create Encryption Writer
	writer, err := account.Cipher.NewEncryptionWriter(iv, conn)
	if err != nil {
		newError("snell client: new encrypt writer failed", err).AtError().WriteToLog(session.ExportIDToError(ctx))
		return newError("failed to create encryption writer").Base(err)
	}

	// 3. Write Snell Request
	req := &SnellRequest{
		Version: Version,
		Address: destination.Address,
		Port:    destination.Port,
	}
	if network == net.Network_TCP {
		req.Command = CommandConnect
	} else {
		req.Command = CommandUDP
	}

	// Use a buffer to write request
	// Since `WriteSnellRequest` takes `io.Writer`, we need to adapt `writer` (buf.Writer).
	// `buf.NewBufferedWriter` implements `io.Writer`.
	bufferedWriter := buf.NewBufferedWriter(writer)
	if err := WriteSnellRequest(bufferedWriter, req); err != nil {
		newError("snell client: write request failed", err).AtError().WriteToLog(session.ExportIDToError(ctx))
		return newError("failed to write snell request").Base(err)
	}
	// Flush request
	if err := bufferedWriter.SetBuffered(false); err != nil {
		newError("snell client: flush request failed", err).AtError().WriteToLog(session.ExportIDToError(ctx))
		return err
	}
	newError("snell client: request cmd=", req.Command, " addr=", req.Address, " port=", req.Port).AtInfo().WriteToLog(session.ExportIDToError(ctx))

	// 4. Read Response IV
	respIV := make([]byte, account.Cipher.IVSize())
	if _, err := io.ReadFull(conn, respIV); err != nil {
		newError("snell client: read resp IV failed", err).AtError().WriteToLog(session.ExportIDToError(ctx))
		return newError("failed to read response IV").Base(err)
	}
	newError("snell client: read resp IV ok size=", len(respIV)).AtDebug().WriteToLog(session.ExportIDToError(ctx))

	// 5. Create Decryption Reader
	reader, err := account.Cipher.NewDecryptionReader(respIV, conn)
	if err != nil {
		newError("snell client: new decrypt reader failed", err).AtError().WriteToLog(session.ExportIDToError(ctx))
		return newError("failed to create decryption reader").Base(err)
	}
	bufferedReader := &buf.BufferedReader{Reader: reader}

	// 6. Read Response (0x00)
	b, err := bufferedReader.ReadByte()
	if err != nil {
		newError("snell client: read response failed", err).AtError().WriteToLog(session.ExportIDToError(ctx))
		return newError("failed to read response").Base(err)
	}
	if b != 0x00 {
		newError("snell client: unexpected response=", b).AtError().WriteToLog(session.ExportIDToError(ctx))
		return newError("unexpected response: ", b)
	}
	newError("snell client: handshake ok").AtInfo().WriteToLog(session.ExportIDToError(ctx))

	if network == net.Network_TCP {
		return c.handleTCP(ctx, link, bufferedReader, writer, timer)
	} else {
		return c.handleUDP(ctx, link, bufferedReader, writer, timer)
	}
}

func (c *Client) handleTCP(ctx context.Context, link *transport.Link, reader buf.Reader, writer buf.Writer, timer *signal.ActivityTimer) error {
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

func (c *Client) handleUDP(ctx context.Context, link *transport.Link, reader buf.Reader, writer buf.Writer, timer *signal.ActivityTimer) error {
	// UDP Client Loop

	// Writer Wrapper
	// `writer` is `buf.Writer`. `WriteUDPPacket` needs `io.Writer`.
	// `buf.NewBufferedWriter` implements `io.Writer`.
	udpWriter := buf.NewBufferedWriter(writer)

	requestDone := func() error {
		for {
			mb, err := link.Reader.ReadMultiBuffer()
			if err != nil {
				return err
			}
			timer.Update()

			for _, buffer := range mb {
				// Each buffer is a UDP payload?
				// In f2ray, `link.Reader` for UDP contains payload.
				// But we don't know the destination here?
				// `link` is a stream.
				// Wait, `Process` is called for ONE outbound connection.
				// `outbound.Target` is the destination.
				// So all packets in `link.Reader` are for `outbound.Target`.
				// BUT, `link` in `Process` is usually a stream.
				// For UDP, `f2ray` usually uses `PacketDispatcher`?
				// No, `Process` interface takes `link *transport.Link`.
				// If it's UDP, `link` contains the payload.
				// The destination is fixed for this `Process` call.

				// So we write [Cmd] [IP] [Port] [Payload]
				// Where IP/Port is `outbound.Target`.

				// But wait, `outbound.Target` is the destination of the packet.
				// Snell UDP tunnel expects the destination address in the packet header.
				// So yes, we use `outbound.Target`.

				// However, `link.Reader` might contain multiple packets merged?
				// Usually `transport.Link` carries stream data.
				// For UDP, `f2ray` usually creates a new `Process` call for each flow (src, dest).
				// So `link` corresponds to one flow.
				// We just need to encapsulate every chunk read from `link.Reader` as a UDP packet.
				// Is `link.Reader` preserving packet boundaries?
				// `buf.MultiBuffer` is a list of buffers.
				// We can treat each `Buffer` in `MultiBuffer` as a packet?
				// Or the whole `MultiBuffer`?
				// Usually `MultiBuffer` is just data.
				// If `f2ray` passes UDP via `link`, it usually means it's a stream of packets?
				// Actually, `f2ray` handles UDP outbound by creating a "session" for the flow.
				// The `link` carries the payload.
				// We should assume `link.Reader` provides the payload stream.
				// But UDP is message-based.
				// If we treat the stream as one big message, it might be wrong.
				// But usually `link.Reader.ReadMultiBuffer()` returns what was written.
				// If the source wrote one packet, we get one MultiBuffer (or part of it).
				// We will treat each `ReadMultiBuffer` result as ONE or MORE payloads?
				// Let's assume each `Buffer` inside `MultiBuffer` is a packet payload?
				// Or just write the whole thing as one packet?
				// If the payload is large, it might be split.
				// But UDP packets are small.
				// I'll iterate over `mb` and write each `Buffer` as a packet.

				// Wait, `outbound.Target` is available in `Process`.
				// I need to pass it to `requestDone`.
				// I'll capture it in closure.

				// Actually, `outbound.Target` is `destination`.

				payload := buffer.Bytes()
				newError("snell udp client write size=", len(payload), " dest=", session.OutboundFromContext(ctx).Target).AtDebug().WriteToLog(session.ExportIDToError(ctx))
				if err := WriteClientUDPPacket(udpWriter, payload, session.OutboundFromContext(ctx).Target); err != nil {
					buffer.Release()
					newError("snell udp client write failed", err).AtError().WriteToLog(session.ExportIDToError(ctx))
					return err
				}
				buffer.Release()
			}

			// Flush?
			if err := udpWriter.SetBuffered(false); err != nil {
				return err
			}
		}
	}

	responseDone := func() error {
		for {
			dest, payload, err := ReadServerUDPPacket(reader)
			if err != nil {
				newError("snell udp client read failed", err).AtError().WriteToLog(session.ExportIDToError(ctx))
				return err
			}
			newError("snell udp client read size=", payload.Len(), " src=", dest).AtDebug().WriteToLog(session.ExportIDToError(ctx))
			timer.Update()

			// We got a packet from tunnel.
			// We need to write it to `link.Writer`.
			// `link.Writer` expects stream data.
			// We just write the payload.
			// But wait, `link` is associated with `destination`.
			// If `dest` (from packet) doesn't match `destination`, what happens?
			// In standard UDP proxy, the response should come from the target.
			// If Snell sends data from another address, we might need to handle it?
			// But `Process` is for a specific flow.
			// We assume `dest` matches or we ignore it.
			// Actually, we just write the payload to `link.Writer`.
			// The caller (dispatcher) handles it.

			if dest != nil {
				// Verify dest?
				// For now, just write payload.
			}

			if err := link.Writer.WriteMultiBuffer(buf.MultiBuffer{payload}); err != nil {
				return err
			}
		}
	}

	if err := task.Run(ctx, requestDone, responseDone); err != nil {
		return newError("connection ends").Base(err)
	}
	return nil
}
