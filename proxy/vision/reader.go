package vision

import (
	"bytes"
	"context"
	"log"
	"net"

	"github.com/frogwall/f2ray-core/v5/common/buf"
	"github.com/frogwall/f2ray-core/v5/common/session"
	"github.com/frogwall/f2ray-core/v5/features/stats"
	"github.com/frogwall/f2ray-core/v5/proxy/vless/encryption"
	"github.com/pires/go-proxyproto"
)

type reader struct {
	r                 buf.Reader
	state             *TrafficState
	isUplink          bool
	ctx               context.Context
	conn              net.Conn
	input             *bytes.Reader
	rawInput          *bytes.Buffer
	ob                *session.Outbound
	directReadCounter stats.Counter
}

func (vr *reader) ReadMultiBuffer() (buf.MultiBuffer, error) {
	buffer, err := vr.r.ReadMultiBuffer()
	if buffer.IsEmpty() {
		return buffer, err
	}

	// Determine pointers to state variables (matching Xray exactly)
	var withinPaddingBuffers *bool
	var remainingContent *int32
	var remainingPadding *int32
	var currentCommand *int
	var switchToDirectCopy *bool

	if vr.isUplink {
		withinPaddingBuffers = &vr.state.Inbound.WithinPaddingBuffers
		remainingContent = &vr.state.Inbound.RemainingContent
		remainingPadding = &vr.state.Inbound.RemainingPadding
		currentCommand = &vr.state.Inbound.CurrentCommand
		switchToDirectCopy = &vr.state.Inbound.UplinkReaderDirectCopy
	} else {
		withinPaddingBuffers = &vr.state.Outbound.WithinPaddingBuffers
		remainingContent = &vr.state.Outbound.RemainingContent
		remainingPadding = &vr.state.Outbound.RemainingPadding
		currentCommand = &vr.state.Outbound.CurrentCommand
		switchToDirectCopy = &vr.state.Outbound.DownlinkReaderDirectCopy
	}

	if *switchToDirectCopy {
		if vr.directReadCounter != nil {
			vr.directReadCounter.Add(int64(buffer.Len()))
		}
		return buffer, err
	}

	// Check if we should process Vision format
	if *withinPaddingBuffers || vr.state.NumberOfPacketToFilter > 0 {
		mb2 := make(buf.MultiBuffer, 0, len(buffer))
		for i, b := range buffer {
			log.Printf("[VISION DEBUG] Before xtlsUnpadding[%d], buffer len=%d", i, b.Len())
			newbuffer := vr.xtlsUnpadding(b)
			log.Printf("[VISION DEBUG] After xtlsUnpadding[%d], result len=%d", i, newbuffer.Len())
			if newbuffer.Len() > 0 {
				mb2 = append(mb2, newbuffer)
			}
		}
		buffer = mb2

		// Update withinPaddingBuffers based on remaining state (matching Xray behavior)
		if *remainingContent > 0 || *remainingPadding > 0 || *currentCommand == 0 {
			*withinPaddingBuffers = true
		} else if *currentCommand == 1 {
			*withinPaddingBuffers = false
		} else if *currentCommand == 2 {
			*withinPaddingBuffers = false
			*switchToDirectCopy = true
		}
	} else {
		log.Printf("[VISION DEBUG] Skipping Vision processing, withinPaddingBuffers=%v, NumberOfPacketToFilter=%d", *withinPaddingBuffers, vr.state.NumberOfPacketToFilter)
	}

	// Call XtlsFilterTls if NumberOfPacketToFilter > 0 (matching Xray behavior)
	if vr.state.NumberOfPacketToFilter > 0 {
		XtlsFilterTls(buffer, vr.state, vr.ctx)
	}

	if *switchToDirectCopy {
		// XTLS Vision processes TLS-like conn's input and rawInput
		if vr.input != nil {
			if inputBuffer, err := buf.ReadFrom(vr.input); err == nil && !inputBuffer.IsEmpty() {
				buffer, _ = buf.MergeMulti(buffer, inputBuffer)
			}
			*vr.input = bytes.Reader{} // release memory
			vr.input = nil
		}
		if vr.rawInput != nil {
			if rawInputBuffer, err := buf.ReadFrom(vr.rawInput); err == nil && !rawInputBuffer.IsEmpty() {
				buffer, _ = buf.MergeMulti(buffer, rawInputBuffer)
			}
			*vr.rawInput = bytes.Buffer{} // release memory
			vr.rawInput = nil
		}

		if vr.conn != nil {
			// TODO: Implement UnwrapRawConn similar to Xray
			// This requires additional infrastructure to unwrap the connection and get the read counter
			// For now, we skip switching to raw conn
			readerConn, readCounter, _ := UnwrapRawConn(vr.conn)
			vr.directReadCounter = readCounter
			vr.r = buf.NewReader(readerConn)
		}
	}

	return buffer, err
}

func (vr *reader) xtlsUnpadding(b *buf.Buffer) *buf.Buffer {
	// Debug: log when xtlsUnpadding is called
	log.Printf("[VISION DEBUG] xtlsUnpadding called, buffer len=%d", b.Len())
	if vr.state == nil {
		log.Printf("[VISION DEBUG] xtlsUnpadding: state is nil, returning raw buffer")
		return b
	}
	// Use appropriate state based on isUplink
	var remainingCommand *int32
	var remainingContent *int32
	var remainingPadding *int32
	var currentCommand *int

	if vr.isUplink {
		remainingCommand = &vr.state.Inbound.RemainingCommand
		remainingContent = &vr.state.Inbound.RemainingContent
		remainingPadding = &vr.state.Inbound.RemainingPadding
		currentCommand = &vr.state.Inbound.CurrentCommand
	} else {
		remainingCommand = &vr.state.Outbound.RemainingCommand
		remainingContent = &vr.state.Outbound.RemainingContent
		remainingPadding = &vr.state.Outbound.RemainingPadding
		currentCommand = &vr.state.Outbound.CurrentCommand
	}

	if *remainingCommand == -1 && *remainingContent == -1 && *remainingPadding == -1 { // initial state
		log.Printf("[F2RAY VISION DEBUG] xtlsUnpadding: initial state, buffer len=%d", b.Len())
		log.Printf("[F2RAY VISION DEBUG] First 16 bytes=%x", b.BytesTo(min(16, b.Len())))
		log.Printf("[F2RAY VISION DEBUG] UserUUID (raw)=%x", vr.state.UserUUID)
		first16Bytes := b.BytesTo(min(16, b.Len()))
		log.Printf("[F2RAY VISION DEBUG] UUID comparison: expected=%x, actual=%x, match=%v", vr.state.UserUUID, first16Bytes, bytes.Equal(vr.state.UserUUID, first16Bytes))
		// Xray's strict logic: Only parse Vision if UUID matches, otherwise return raw buffer
		// This prevents parsing non-Vision data as Vision
		if b.Len() >= 21 && bytes.Equal(vr.state.UserUUID, b.BytesTo(16)) {
			log.Printf("[F2RAY VISION DEBUG] UUID matched! Advancing 16 bytes")
			b.Advance(16)
			*remainingCommand = 5
		} else {
			log.Printf("[F2RAY VISION DEBUG] UUID mismatch. Expected UUID bytes 6-7=%x %x", vr.state.UserUUID[6], vr.state.UserUUID[7])
			if len(first16Bytes) > 7 {
				log.Printf("[F2RAY VISION DEBUG] Actual UUID bytes 6-7=%x %x", first16Bytes[6], first16Bytes[7])
			}
			log.Printf("[F2RAY VISION DEBUG] Returning raw buffer (Xray behavior)")
			return b
		}
	}
	newbuffer := buf.New()
	for b.Len() > 0 {
		if *remainingCommand > 0 {
			data, err := b.ReadByte()
			if err != nil {
				return newbuffer
			}
			switch *remainingCommand {
			case 5:
				*currentCommand = int(data)
				log.Printf("[F2RAY VISION DEBUG] Parsed command: %d", data)
			case 4:
				*remainingContent = int32(data) << 8
			case 3:
				*remainingContent = *remainingContent | int32(data)
				log.Printf("[F2RAY VISION DEBUG] Parsed content length: %d", *remainingContent)
			case 2:
				*remainingPadding = int32(data) << 8
			case 1:
				*remainingPadding = *remainingPadding | int32(data)
				log.Printf("[F2RAY VISION DEBUG] Parsed padding length: %d, command=%d", *remainingPadding, *currentCommand)
			}
			*remainingCommand--
		} else if *remainingContent > 0 {
			len := *remainingContent
			if b.Len() < len {
				len = b.Len()
			}
			data, err := b.ReadBytes(len)
			if err != nil {
				return newbuffer
			}
			newbuffer.Write(data)
			*remainingContent -= len
		} else { // remainingPadding > 0
			len := *remainingPadding
			if b.Len() < len {
				len = b.Len()
			}
			b.Advance(len)
			*remainingPadding -= len
		}
		if *remainingCommand <= 0 && *remainingContent <= 0 && *remainingPadding <= 0 { // this block done
			if *currentCommand == 0 {
				*remainingCommand = 5
			} else {
				*remainingCommand = -1 // set to initial state
				*remainingContent = -1
				*remainingPadding = -1
				if b.Len() > 0 { // shouldn't happen
					newbuffer.Write(b.Bytes())
				}
				break
			}
		}
	}
	b.Release()
	b = nil
	return newbuffer
}

func NewReader(r buf.Reader, ctx context.Context, conn net.Conn, input *bytes.Reader, rawInput *bytes.Buffer, ob *session.Outbound, state *TrafficState, isUplink bool) buf.Reader {
	return &reader{
		r:        r,
		state:    state,
		isUplink: isUplink,
		ctx:      ctx,
		conn:     conn,
		input:    input,
		rawInput: rawInput,
		ob:       ob,
	}
}

// UnwrapRawConn support unwrap encryption, stats, tls, utls, reality, proxyproto, uds-wrapper conn and get raw tcp/uds conn from it
func UnwrapRawConn(conn net.Conn) (net.Conn, stats.Counter, stats.Counter) {
	var readCounter, writerCounter stats.Counter
	if conn != nil {
		isEncryption := false
		if commonConn, ok := conn.(*encryption.CommonConn); ok {
			conn = commonConn.Conn
			isEncryption = true
		}
		if xorConn, ok := conn.(*encryption.XorConn); ok {
			return xorConn, nil, nil // full-random xorConn should not be penetrated
		}
		// TODO: Add support for CounterConnection when implemented in f2ray-core
		// if statConn, ok := conn.(*stat.CounterConnection); ok {
		// 	conn = statConn.Connection
		// 	readCounter = statConn.ReadCounter
		// 	writerCounter = statConn.WriteCounter
		// }
		if !isEncryption { // avoids double penetration
			// Unwrap REALITY/uTLS connection
			type RealityConn interface {
				NetConn() net.Conn
			}
			if realityConn, ok := conn.(RealityConn); ok {
				conn = realityConn.NetConn()
			}

			// Unwrap uTLS connection
			type UConn interface {
				NetConn() net.Conn
			}
			if utlsConn, ok := conn.(UConn); ok {
				conn = utlsConn.NetConn()
			}
		}
		// Unwrap proxyproto
		if pc, ok := conn.(*proxyproto.Conn); ok {
			conn = pc.Raw()
		}

		// Unwrap Unix connection wrapper
		type UnixConnWrapper interface {
			UnixConn() net.Conn
		}
		if uc, ok := conn.(UnixConnWrapper); ok {
			conn = uc.UnixConn()
		}
	}
	return conn, readCounter, writerCounter
}
