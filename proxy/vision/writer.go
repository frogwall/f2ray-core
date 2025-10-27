package vision

import (
	"bytes"
	"context"
	"crypto/rand"
	"log"
	"math/big"
	"net"

	"github.com/frogwall/f2ray-core/v5/common/buf"
	"github.com/frogwall/f2ray-core/v5/common/session"
	"github.com/frogwall/f2ray-core/v5/features/stats"
)

const (
	commandPaddingContinue byte = 0x00
	commandPaddingEnd      byte = 0x01
	commandPaddingDirect   byte = 0x02
)

type VisionWriter struct {
	w                  buf.Writer
	ctx                context.Context
	conn               net.Conn
	ob                 *session.Outbound
	writeOnceUserUUID  []byte
	isPadding          bool
	state              *TrafficState
	isUplink           bool
	directWriteCounter stats.Counter
}

func NewVisionWriter(w buf.Writer, ctx context.Context, conn net.Conn, ob *session.Outbound, state *TrafficState, isUplink bool) buf.Writer {
	u := make([]byte, len(state.UserUUID))
	copy(u, state.UserUUID)
	return &VisionWriter{
		w:                 w,
		ctx:               ctx,
		conn:              conn,
		ob:                ob,
		writeOnceUserUUID: u,
		isPadding:         true,
		state:             state,
		isUplink:          isUplink,
	}
}

func (vw *VisionWriter) WriteMultiBuffer(mb buf.MultiBuffer) error {
	var isPadding *bool
	var switchToDirectCopy *bool
	if vw.isUplink {
		isPadding = &vw.state.Outbound.IsPadding
		switchToDirectCopy = &vw.state.Outbound.UplinkWriterDirectCopy
	} else {
		isPadding = &vw.state.Inbound.IsPadding
		switchToDirectCopy = &vw.state.Inbound.DownlinkWriterDirectCopy
	}

	if *switchToDirectCopy {
		if inbound := session.InboundFromContext(vw.ctx); inbound != nil {
			if !vw.isUplink && inbound.CanSpliceCopy == 2 {
				inbound.CanSpliceCopy = 1
			}
			if vw.isUplink && vw.ob != nil && vw.ob.CanSpliceCopy == 2 {
				vw.ob.CanSpliceCopy = 1
			}
		}
		rawConn, _, writerCounter := UnwrapRawConn(vw.conn)
		vw.w = buf.NewWriter(rawConn)
		vw.directWriteCounter = writerCounter
		*switchToDirectCopy = false
	}
	if !mb.IsEmpty() && vw.directWriteCounter != nil {
		vw.directWriteCounter.Add(int64(mb.Len()))
	}

	if mb.IsEmpty() {
		return nil
	}
	defer buf.ReleaseMulti(mb)

	// Use outbound side for uplink in outbound handler
	if vw.state != nil {
		vw.isPadding = *isPadding
	}

	if vw.state != nil && vw.state.NumberOfPacketToFilter > 0 {
		XtlsFilterTls(mb, vw.state, vw.ctx)
	}

	if *isPadding {
		if len(mb) == 1 && mb[0] == nil {
			mb[0] = xtlsPadding(nil, commandPaddingContinue, &vw.writeOnceUserUUID, true, vw.ctx)
			return vw.w.WriteMultiBuffer(mb)
		}
		mb = ReshapeMultiBuffer(vw.ctx, mb)
		longPadding := vw.state != nil && vw.state.IsTLS
		for i, b := range mb {
			if vw.state != nil && vw.state.IsTLS && b.Len() >= 6 && bytes.Equal(TlsApplicationDataStart, b.BytesTo(3)) {
				if vw.state.EnableXtls {
					*switchToDirectCopy = true
				}
				cmd := commandPaddingContinue
				if i == len(mb)-1 {
					cmd = commandPaddingEnd
					if vw.state.EnableXtls {
						cmd = commandPaddingDirect
					}
				}
				log.Printf("Vision: detected TLS AppData, cmd=%d enableXtls=%v", cmd, vw.state.EnableXtls)
				mb[i] = xtlsPadding(b, cmd, &vw.writeOnceUserUUID, true, vw.ctx)
				*isPadding = false
				longPadding = false
				continue
			} else if vw.state != nil && !vw.state.IsTLS12orAbove && vw.state.NumberOfPacketToFilter <= 1 {
				// finish padding 1 packet early for pre‑TLS1.2
				log.Printf("Vision: finish padding early for pre-TLS1.2")
				*isPadding = false
				mb[i] = xtlsPadding(b, commandPaddingEnd, &vw.writeOnceUserUUID, longPadding, vw.ctx)
				break
			}
			cmd := commandPaddingContinue
			if i == len(mb)-1 && !*isPadding {
				cmd = commandPaddingEnd
				if vw.state != nil && vw.state.EnableXtls {
					cmd = commandPaddingDirect
				}
			}
			mb[i] = xtlsPadding(b, cmd, &vw.writeOnceUserUUID, longPadding, vw.ctx)
		}
	}
	return vw.w.WriteMultiBuffer(mb)
}

// xtlsPadding adds padding header and random padding similar to Xray Vision
func xtlsPadding(b *buf.Buffer, command byte, userUUID *[]byte, longPadding bool, ctx context.Context) *buf.Buffer {
	var contentLen int32
	if b != nil {
		contentLen = b.Len()
	}
	var paddingLen int32
	if contentLen < 900 && longPadding {
		l, _ := rand.Int(rand.Reader, big.NewInt(500))
		paddingLen = int32(l.Int64()) + 900 - contentLen
	} else {
		l, _ := rand.Int(rand.Reader, big.NewInt(256))
		paddingLen = int32(l.Int64())
	}
	if paddingLen > buf.Size-21-contentLen {
		paddingLen = buf.Size - 21 - contentLen
	}
	nb := buf.New()
	if userUUID != nil && len(*userUUID) > 0 {
		nb.Write(*userUUID)
		*userUUID = nil
		log.Printf("[WRITER DEBUG] Added UUID to padding: %x", nb.Bytes()[:16])
	}
	nb.Write([]byte{command, byte(contentLen >> 8), byte(contentLen), byte(paddingLen >> 8), byte(paddingLen)})
	if b != nil {
		nb.Write(b.Bytes())
		b.Release()
		b = nil
	}
	if paddingLen > 0 {
		nb.Extend(paddingLen)
	}
	log.Printf("XtlsPadding content=%d padding=%d cmd=%d", contentLen, paddingLen, command)
	log.Printf("[WRITER DEBUG] Padding header: %x", nb.Bytes()[:21])
	return nb
}
