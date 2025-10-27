// Package proxy contains all proxies used by V2Ray.
//
// To implement an inbound or outbound proxy, one needs to do the following:
// 1. Implement the interface(s) below.
// 2. Register a config creator through common.RegisterConfig.
package proxy

import (
	"context"
	"io"
	"log"
	"runtime"
	"time"

	"github.com/frogwall/f2ray-core/v5/common/buf"
	"github.com/frogwall/f2ray-core/v5/common/errors"
	"github.com/frogwall/f2ray-core/v5/common/net"
	"github.com/frogwall/f2ray-core/v5/common/protocol"
	"github.com/frogwall/f2ray-core/v5/common/session"
	"github.com/frogwall/f2ray-core/v5/common/signal"
	"github.com/frogwall/f2ray-core/v5/features/routing"
	"github.com/frogwall/f2ray-core/v5/features/stats"
	"github.com/frogwall/f2ray-core/v5/proxy/vless/encryption"
	"github.com/frogwall/f2ray-core/v5/transport"
	"github.com/frogwall/f2ray-core/v5/transport/internet"
	"github.com/frogwall/f2ray-core/v5/transport/internet/reality"
	"github.com/pires/go-proxyproto"
)

// A timeout for reading the first payload from the client, used in 0-RTT optimizations.
const FirstPayloadTimeout = 100 * time.Millisecond

// An Inbound processes inbound connections.
type Inbound interface {
	// Network returns a list of networks that this inbound supports. Connections with not-supported networks will not be passed into Process().
	Network() []net.Network

	// Process processes a connection of given network. If necessary, the Inbound can dispatch the connection to an Outbound.
	Process(context.Context, net.Network, internet.Connection, routing.Dispatcher) error
}

// An Outbound process outbound connections.
type Outbound interface {
	// Process processes the given connection. The given dialer may be used to dial a system outbound connection.
	Process(context.Context, *transport.Link, internet.Dialer) error
}

// UserManager is the interface for Inbounds and Outbounds that can manage their users.
type UserManager interface {
	// AddUser adds a new user.
	AddUser(context.Context, *protocol.MemoryUser) error

	// RemoveUser removes a user by email.
	RemoveUser(context.Context, string) error
}

type GetInbound interface {
	GetInbound() Inbound
}

type GetOutbound interface {
	GetOutbound() Outbound
}

// UnwrapRawConn support unwrap encryption, stats, tls, utls, reality, proxyproto, uds-wrapper conn and get raw tcp/uds conn from it
func UnwrapRawConn(conn net.Conn) (net.Conn, stats.Counter, stats.Counter) {
	var readCounter, writerCounter stats.Counter
	if conn != nil {
		log.Printf("[UnwrapRawConn] Input conn type: %T", conn)
		isEncryption := false
		if commonConn, ok := conn.(*encryption.CommonConn); ok {
			log.Printf("[UnwrapRawConn] Unwrapping CommonConn")
			conn = commonConn.Conn
			isEncryption = true
		}
		if xorConn, ok := conn.(*encryption.XorConn); ok {
			log.Printf("[UnwrapRawConn] XorConn detected, returning as-is")
			return xorConn, nil, nil // full-random xorConn should not be penetrated
		}
		if statConn, ok := conn.(*internet.StatCouterConnection); ok {
			log.Printf("[UnwrapRawConn] Unwrapping StatCouterConnection")
			conn = statConn.Connection
			readCounter = statConn.ReadCounter
			writerCounter = statConn.WriteCounter
		}
		if !isEncryption { // avoids double penetration
			// Check for REALITY UConn
			if realityUConn, ok := conn.(*reality.UConn); ok {
				log.Printf("[UnwrapRawConn] Unwrapping REALITY UConn")
				conn = realityUConn.NetConn()
				log.Printf("[UnwrapRawConn] After unwrapping REALITY, conn type: %T", conn)
			}
		}
		if pc, ok := conn.(*proxyproto.Conn); ok {
			log.Printf("[UnwrapRawConn] Unwrapping proxyproto.Conn")
			conn = pc.Raw()
			// 8192 > 4096, there is no need to process pc's bufReader
		}
		if uc, ok := conn.(*internet.UnixConnWrapper); ok {
			log.Printf("[UnwrapRawConn] Unwrapping UnixConnWrapper")
			conn = uc.GetUnixConn()
		}
		log.Printf("[UnwrapRawConn] Output conn type: %T", conn)
	}
	return conn, readCounter, writerCounter
}

// CopyRawConnIfExist use the most efficient copy method.
// - If caller don't want to turn on splice, do not pass in both reader conn and writer conn
// - writer are from *transport.Link
func CopyRawConnIfExist(ctx context.Context, readerConn net.Conn, writerConn net.Conn, writer buf.Writer, timer *signal.ActivityTimer, inTimer *signal.ActivityTimer) error {
	readerConn, readCounter, _ := UnwrapRawConn(readerConn)
	writerConn, _, writeCounter := UnwrapRawConn(writerConn)
	reader := buf.NewReader(readerConn)
	if runtime.GOOS != "linux" && runtime.GOOS != "android" {
		return readV(ctx, reader, writer, timer, readCounter)
	}
	tc, ok := writerConn.(*net.TCPConn)
	if !ok || readerConn == nil || writerConn == nil {
		// Cannot use splice copy because writerConn is nil or not TCP
		return readV(ctx, reader, writer, timer, readCounter)
	}
	inbound := session.InboundFromContext(ctx)
	if inbound != nil {
		log.Printf("[CopyRawConnIfExist] inbound=%p, inbound.CanSpliceCopy=%d, inbound.Conn=%p", inbound, inbound.CanSpliceCopy, inbound.Conn)
	}
	if inbound == nil || inbound.CanSpliceCopy == 3 {
		log.Printf("[CopyRawConnIfExist] Early return: inbound=%v, CanSpliceCopy=%d", inbound == nil, func() int {
			if inbound != nil {
				return inbound.CanSpliceCopy
			}
			return -1
		}())
		return readV(ctx, reader, writer, timer, readCounter)
	}
	outbounds := session.OutboundsFromContext(ctx)
	if len(outbounds) == 0 {
		return readV(ctx, reader, writer, timer, readCounter)
	}
	for _, ob := range outbounds {
		if ob.CanSpliceCopy == 3 {
			return readV(ctx, reader, writer, timer, readCounter)
		}
	}

	for {
		inbound := session.InboundFromContext(ctx)
		outbounds := session.OutboundsFromContext(ctx)
		var splice = inbound.CanSpliceCopy == 1
		for _, ob := range outbounds {
			if ob.CanSpliceCopy != 1 {
				splice = false
			}
		}
		if splice {
			log.Printf("[CopyRawConnIfExist] Using splice copy, inbound.CanSpliceCopy=%d", inbound.CanSpliceCopy)
			// Note: For f2ray, we don't have SizeStatWriter, so we'll skip that optimization
			time.Sleep(time.Millisecond)     // without this, there will be a rare ssl error for freedom splice
			timer.SetTimeout(24 * time.Hour) // prevent leak, just in case
			if inTimer != nil {
				inTimer.SetTimeout(24 * time.Hour)
			}
			w, err := tc.ReadFrom(readerConn)
			if readCounter != nil {
				readCounter.Add(w) // outbound stats
			}
			if writeCounter != nil {
				writeCounter.Add(w) // inbound stats
			}
			if err != nil && errors.Cause(err) != io.EOF {
				return err
			}
			return nil
		}
		buffer, err := reader.ReadMultiBuffer()
		if !buffer.IsEmpty() {
			if readCounter != nil {
				readCounter.Add(int64(buffer.Len()))
			}
			timer.Update()
			if werr := writer.WriteMultiBuffer(buffer); werr != nil {
				return werr
			}
		}
		if err != nil {
			if errors.Cause(err) == io.EOF {
				return nil
			}
			return err
		}
	}
}

func readV(ctx context.Context, reader buf.Reader, writer buf.Writer, timer signal.ActivityUpdater, readCounter stats.Counter) error {
	log.Printf("[CopyRawConnIfExist] Using readv copy")
	if err := buf.Copy(reader, writer, buf.UpdateActivity(timer), buf.AddToStatCounter(readCounter)); err != nil {
		return errors.New("failed to process response").Base(err)
	}
	return nil
}

func IsRAWTransportWithoutSecurity(conn internet.Connection) bool {
	iConn := conn
	if statConn, ok := iConn.(*internet.StatCouterConnection); ok {
		iConn = statConn.Connection
	}
	_, ok1 := iConn.(*proxyproto.Conn)
	_, ok2 := iConn.(*net.TCPConn)
	return ok1 || ok2
}
