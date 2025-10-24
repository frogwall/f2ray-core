package naive

import (
	"encoding/binary"
	"io"
	"math/rand"
	"net"
)

const kFirstPaddings = 8

// generatePaddingHeader creates a non-indexed looking header value to trigger
// server-side padding negotiation fallback.
func generatePaddingHeader() string {
	paddingLen := rand.Intn(32) + 30
	padding := make([]byte, paddingLen)
	bits := rand.Uint64()
	charset := []byte("!#$()+<>?@[]^`{}") // non-Huffman-favored
	for i := 0; i < 16 && i < paddingLen; i++ {
		padding[i] = charset[int(bits&15)%len(charset)]
		bits >>= 4
	}
	for i := 16; i < paddingLen; i++ {
		padding[i] = '~'
	}
	return string(padding)
}

// PaddingConn wraps a net.Conn and applies naive variant1 padding for the first 8
// reads/writes to be compatible with naiveproxy server.
type PaddingConn struct {
	net.Conn

	readPadding      int
	writePadding     int
	readRemaining    int
	paddingRemaining int
}

func (c *PaddingConn) Read(p []byte) (n int, err error) {
	if c.readRemaining > 0 {
		if len(p) > c.readRemaining {
			p = p[:c.readRemaining]
		}
		n, err = c.Conn.Read(p)
		if err != nil {
			return
		}
		c.readRemaining -= n
		return
	}
	if c.paddingRemaining > 0 {
		// Skip remaining padding bytes from previous frame.
		buf := make([]byte, 1024)
		for c.paddingRemaining > 0 {
			toRead := c.paddingRemaining
			if toRead > len(buf) {
				toRead = len(buf)
			}
			m, e := io.ReadFull(c.Conn, buf[:toRead])
			if e != nil {
				return 0, e
			}
			c.paddingRemaining -= m
		}
	}
	if c.readPadding < kFirstPaddings {
		// Read 3-byte header: [len_hi len_lo pad_len]
		head := make([]byte, 3)
		if _, err = io.ReadFull(c.Conn, head); err != nil {
			return
		}
		original := int(binary.BigEndian.Uint16(head[:2]))
		pad := int(head[2])
		if len(p) > original {
			p = p[:original]
		}
		n, err = c.Conn.Read(p)
		if err != nil {
			return
		}
		c.readPadding++
		c.readRemaining = original - n
		c.paddingRemaining = pad
		return
	}
	return c.Conn.Read(p)
}

func (c *PaddingConn) Write(p []byte) (n int, err error) {
	if c.writePadding < kFirstPaddings {
		pad := rand.Intn(256)
		buf := make([]byte, 3+len(p)+pad)
		binary.BigEndian.PutUint16(buf[0:2], uint16(len(p)))
		buf[2] = byte(pad)
		copy(buf[3:], p)
		// padding bytes are zero-initialized by make
		if _, err = c.Conn.Write(buf); err != nil {
			return 0, err
		}
		c.writePadding++
		return len(p), nil
	}
	return c.Conn.Write(p)
}
