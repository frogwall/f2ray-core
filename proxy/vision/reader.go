package vision

import (
    "bytes"
    "context"
    "net"
    "log"

    "github.com/frogwall/f2ray-core/v5/common/buf"
    "github.com/frogwall/f2ray-core/v5/common/session"
)

type reader struct {
    r         buf.Reader
    state     *TrafficState
    isUplink  bool
}

func (vr *reader) ReadMultiBuffer() (buf.MultiBuffer, error) {
    buffer, err := vr.r.ReadMultiBuffer()
    if buffer.IsEmpty() {
        return buffer, err
    }
    mb2 := make(buf.MultiBuffer, 0, len(buffer))
    for _, b := range buffer {
        nb := vr.xtlsUnpadding(b)
        if nb.Len() > 0 {
            mb2 = append(mb2, nb)
        }
    }
    return mb2, err
}

func (vr *reader) xtlsUnpadding(b *buf.Buffer) *buf.Buffer {
    if vr.state == nil {
        return b
    }
    // Use outbound side for downlink in this outbound handler
    s := &vr.state.Outbound
    if s.RemainingCommand == -1 && s.RemainingContent == -1 && s.RemainingPadding == -1 { // initial state
        if b.Len() >= 21 && len(vr.state.UserUUID) == 16 && bytes.Equal(b.BytesTo(16), vr.state.UserUUID) {
            b.Advance(16)
            s.RemainingCommand = 5
            vr.state.Outbound.WithinPaddingBuffers = true
            log.Printf("VisionReader: initial UUID matched, start header parse")
        } else {
            if b.Len() >= 16 {
                log.Printf("VisionReader: initial UUID mismatch, got=%x expected=%x", b.BytesTo(16), vr.state.UserUUID)
            } else {
                log.Printf("VisionReader: initial buffer too short=%d", b.Len())
            }
            return b
        }
    }
    newbuffer := buf.New()
    for b.Len() > 0 {
        if s.RemainingCommand > 0 {
            data, err := b.ReadByte()
            if err != nil {
                return newbuffer
            }
            switch s.RemainingCommand {
            case 5:
                s.CurrentCommand = int(data)
            case 4:
                s.RemainingContent = int32(data) << 8
            case 3:
                s.RemainingContent |= int32(data)
            case 2:
                s.RemainingPadding = int32(data) << 8
            case 1:
                s.RemainingPadding |= int32(data)
                log.Printf("VisionReader: new block cmd=%d content=%d padding=%d", s.CurrentCommand, s.RemainingContent, s.RemainingPadding)
            }
            s.RemainingCommand--
        } else if s.RemainingContent > 0 {
            l := s.RemainingContent
            if b.Len() < l {
                l = b.Len()
            }
            data, err := b.ReadBytes(l)
            if err != nil {
                return newbuffer
            }
            newbuffer.Write(data)
            s.RemainingContent -= l
        } else { // padding
            l := s.RemainingPadding
            if b.Len() < l {
                l = b.Len()
            }
            b.Advance(l)
            s.RemainingPadding -= l
        }
        if s.RemainingCommand <= 0 && s.RemainingContent <= 0 && s.RemainingPadding <= 0 { // block done
            if s.CurrentCommand == 0 { // continue
                s.RemainingCommand = 5
                vr.state.Outbound.WithinPaddingBuffers = true
                log.Printf("VisionReader: block done, continue")
            } else { // end or direct
                s.RemainingCommand = -1
                s.RemainingContent = -1
                s.RemainingPadding = -1
                vr.state.Outbound.WithinPaddingBuffers = false
                if s.CurrentCommand == 2 { // direct
                    vr.state.Outbound.DownlinkReaderDirectCopy = true
                    log.Printf("VisionReader: block done, direct switch")
                } else {
                    log.Printf("VisionReader: block done, end")
                }
                if b.Len() > 0 {
                    newbuffer.Write(b.Bytes())
                }
                break
            }
        }
    }
    b.Release()
    return newbuffer
}

func NewReader(r buf.Reader, _ context.Context, _ net.Conn, _ *session.Outbound, state *TrafficState, isUplink bool) buf.Reader {
    return &reader{
        r:     r,
        state: state,
        isUplink: isUplink,
    }
}
