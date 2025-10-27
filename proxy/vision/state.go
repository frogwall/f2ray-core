package vision

import (
	"bytes"
	"context"
	"log"

	"github.com/frogwall/f2ray-core/v5/common/buf"
)

var (
	Tls13SupportedVersions  = []byte{0x00, 0x2b, 0x00, 0x02, 0x03, 0x04}
	TlsClientHandShakeStart = []byte{0x16, 0x03}
	TlsServerHandShakeStart = []byte{0x16, 0x03, 0x03}
	TlsApplicationDataStart = []byte{0x17, 0x03, 0x03}
	Tls13CipherSuiteDic     = map[uint16]string{
		0x1301: "TLS_AES_128_GCM_SHA256",
		0x1302: "TLS_AES_256_GCM_SHA384",
		0x1303: "TLS_CHACHA20_POLY1305_SHA256",
		0x1304: "TLS_AES_128_CCM_SHA256",
		0x1305: "TLS_AES_128_CCM_8_SHA256",
	}
)

type InboundState struct {
	WithinPaddingBuffers     bool
	UplinkReaderDirectCopy   bool
	RemainingCommand         int32
	RemainingContent         int32
	RemainingPadding         int32
	CurrentCommand           int
	IsPadding                bool
	DownlinkWriterDirectCopy bool
}

type OutboundState struct {
	WithinPaddingBuffers     bool
	DownlinkReaderDirectCopy bool
	RemainingCommand         int32
	RemainingContent         int32
	RemainingPadding         int32
	CurrentCommand           int
	IsPadding                bool
	UplinkWriterDirectCopy   bool
	PaddedBatches            int
}

type TrafficState struct {
	UserUUID               []byte
	NumberOfPacketToFilter int
	EnableXtls             bool
	IsTLS12orAbove         bool
	IsTLS                  bool
	Cipher                 uint16
	RemainingServerHello   int32
	Inbound                InboundState
	Outbound               OutboundState
}

func NewTrafficState(userUUID []byte) *TrafficState {
	return &TrafficState{
		UserUUID:               userUUID,
		NumberOfPacketToFilter: 8,
		EnableXtls:             false,
		IsTLS12orAbove:         false,
		IsTLS:                  false,
		Cipher:                 0,
		RemainingServerHello:   -1,
		Inbound: InboundState{
			WithinPaddingBuffers:     true,
			UplinkReaderDirectCopy:   false,
			RemainingCommand:         -1,
			RemainingContent:         -1,
			RemainingPadding:         -1,
			CurrentCommand:           0,
			IsPadding:                true,
			DownlinkWriterDirectCopy: false,
		},
		Outbound: OutboundState{
			WithinPaddingBuffers:     true,
			DownlinkReaderDirectCopy: false,
			RemainingCommand:         -1,
			RemainingContent:         -1,
			RemainingPadding:         -1,
			CurrentCommand:           0,
			IsPadding:                true,
			UplinkWriterDirectCopy:   false,
		},
	}
}

// ReshapeMultiBuffer prepares buffers to ensure room for padding headers
func ReshapeMultiBuffer(ctx context.Context, buffer buf.MultiBuffer) buf.MultiBuffer {
	need := 0
	for _, b := range buffer {
		if b.Len() >= buf.Size-21 {
			need++
		}
	}
	if need == 0 {
		return buffer
	}
	mb2 := make(buf.MultiBuffer, 0, len(buffer)+need)
	for i, b1 := range buffer {
		if b1.Len() >= buf.Size-21 {
			idx := int32(bytes.LastIndex(b1.Bytes(), TlsApplicationDataStart))
			if idx < 21 || idx > buf.Size-21 {
				idx = buf.Size / 2
			}
			b2 := buf.New()
			b2.Write(b1.BytesFrom(idx))
			b1.Resize(0, idx)
			mb2 = append(mb2, b1, b2)
		} else {
			mb2 = append(mb2, b1)
		}
		buffer[i] = nil
	}
	buffer = buffer[:0]
	return mb2
}

// XtlsFilterTls inspects early packets to detect TLS and stop filtering when determined
func XtlsFilterTls(buffer buf.MultiBuffer, s *TrafficState, ctx context.Context) {
	for _, b := range buffer {
		if b == nil {
			continue
		}
		s.NumberOfPacketToFilter--
		if b.Len() >= 6 {
			starts := b.BytesTo(6)
			if bytes.Equal(TlsServerHandShakeStart, starts[:3]) && starts[5] == 0x02 { // ServerHello
				s.RemainingServerHello = (int32(starts[3])<<8 | int32(starts[4])) + 5
				s.IsTLS12orAbove = true
				s.IsTLS = true
				if b.Len() >= 79 && s.RemainingServerHello >= 79 {
					sessionIdLen := int32(b.Byte(43))
					cipherSuite := b.BytesRange(43+sessionIdLen+1, 43+sessionIdLen+3)
					s.Cipher = uint16(cipherSuite[0])<<8 | uint16(cipherSuite[1])
				}
			} else if bytes.Equal(TlsClientHandShakeStart, starts[:2]) && starts[5] == 0x01 { // ClientHello
				s.IsTLS = true
			}
		}
		if s.RemainingServerHello > 0 {
			end := s.RemainingServerHello
			if end > b.Len() {
				end = b.Len()
			}
			s.RemainingServerHello -= b.Len()
			if bytes.Contains(b.BytesTo(end), Tls13SupportedVersions) {
				v, ok := Tls13CipherSuiteDic[s.Cipher]
				if !ok {
					// Add log for old cipher
					log.Printf("XtlsFilterTls: Old cipher: %x", s.Cipher)
				} else if v != "TLS_AES_128_CCM_8_SHA256" {
					s.EnableXtls = true
				}
				log.Printf("XtlsFilterTls found tls 1.3! %d %s", b.Len(), v)
				s.NumberOfPacketToFilter = 0
				return
			} else if s.RemainingServerHello <= 0 {
				log.Printf("XtlsFilterTls found tls 1.2! %d", b.Len())
				s.NumberOfPacketToFilter = 0
				return
			}
			log.Printf("XtlsFilterTls inconclusive server hello %d %d", b.Len(), s.RemainingServerHello)
		}
		if s.NumberOfPacketToFilter <= 0 {
			return
		}
	}
}
