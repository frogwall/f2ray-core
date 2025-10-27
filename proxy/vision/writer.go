package vision

import (
    "bytes"
    "context"
    "crypto/rand"
    "math/big"
    "net"
    "log"

    "github.com/frogwall/f2ray-core/v5/common/buf"
    "github.com/frogwall/f2ray-core/v5/common/session"
)

const (
    commandPaddingContinue byte = 0x00
    commandPaddingEnd      byte = 0x01
    commandPaddingDirect   byte = 0x02
)

type writer struct {
    w                  buf.Writer
    ctx                context.Context
    conn               net.Conn
    ob                 *session.Outbound
    writeOnceUserUUID  []byte
    isPadding          bool
    state              *TrafficState
    isUplink           bool
}

func NewWriter(w buf.Writer, ctx context.Context, conn net.Conn, ob *session.Outbound, state *TrafficState, isUplink bool) buf.Writer {
    u := make([]byte, len(state.UserUUID))
    copy(u, state.UserUUID)
    return &writer{
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

func (vw *writer) WriteMultiBuffer(mb buf.MultiBuffer) error {
    if mb.IsEmpty() {
        return nil
    }
    defer buf.ReleaseMulti(mb)

    // Use outbound side for uplink in outbound handler
    if vw.state != nil {
        vw.isPadding = vw.state.Outbound.IsPadding
        if vw.state.Outbound.UplinkWriterDirectCopy {
            // switch to direct copy: replace underlying writer with raw conn writer
            // We skip raw unwrap optimization; just proceed writing via existing writer.
            vw.state.Outbound.UplinkWriterDirectCopy = false
        }
    }

    if vw.isPadding {
        // Reshape and filter TLS like Xray
        mb = ReshapeMultiBuffer(vw.ctx, mb)
        if vw.state != nil {
            XtlsFilterTls(mb, vw.state, vw.ctx)
        }
        longPadding := vw.state != nil && vw.state.IsTLS
        if len(mb) == 1 && mb[0] == nil {
            mb[0] = xtlsPadding(nil, commandPaddingContinue, &vw.writeOnceUserUUID, true, vw.ctx)
            return vw.w.WriteMultiBuffer(mb)
        }
        for i, b := range mb {
            if vw.state != nil && vw.state.IsTLS && b.Len() >= 6 && bytes.Equal(TlsApplicationDataStart, b.BytesTo(3)) {
                if vw.state.EnableXtls {
                    vw.state.Outbound.UplinkWriterDirectCopy = true
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
                vw.isPadding = false
                if vw.state != nil {
                    vw.state.Outbound.IsPadding = false
                }
                longPadding = false
                continue
            } else if vw.state != nil && !vw.state.IsTLS12orAbove && vw.state.NumberOfPacketToFilter <= 1 {
                // finish padding 1 packet early for pre‑TLS1.2
                log.Printf("Vision: finish padding early for pre-TLS1.2")
                vw.isPadding = false
                if vw.state != nil {
                    vw.state.Outbound.IsPadding = false
                }
                mb[i] = xtlsPadding(b, commandPaddingEnd, &vw.writeOnceUserUUID, longPadding, vw.ctx)
                break
            }
            cmd := commandPaddingContinue
            if i == len(mb)-1 && !vw.isPadding {
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
    }
    nb.Write([]byte{command, byte(contentLen >> 8), byte(contentLen), byte(paddingLen >> 8), byte(paddingLen)})
    if b != nil {
        nb.Write(b.Bytes())
        b.Release()
    }
    if paddingLen > 0 {
        pad := make([]byte, paddingLen)
        if _, err := rand.Read(pad); err == nil {
            nb.Write(pad)
        } else {
            // fallback to zero padding on randomness failure
            nb.Write(make([]byte, paddingLen))
        }
    }
    log.Printf("XtlsPadding content=%d padding=%d cmd=%d", contentLen, paddingLen, command)
    return nb
}
