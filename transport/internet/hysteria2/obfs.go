//go:build !confonly
// +build !confonly

package hysteria2

import (
	"fmt"
	"math/rand"
	"net"
	"sync"
	"time"

	"golang.org/x/crypto/blake2b"
)

const (
	obfsSaltLen = 8
	obfsKeyLen  = blake2b.Size256
	obfsMinPSK  = 4
)

// SalamanderObfuscator implements the Salamander obfuscation algorithm
// This is based on hysteria's implementation for obfs-password functionality
type SalamanderObfuscator struct {
	PSK     []byte
	RandSrc *rand.Rand
	mu      sync.Mutex
}

// NewSalamanderObfuscator creates a new Salamander obfuscator
func NewSalamanderObfuscator(password string) (*SalamanderObfuscator, error) {
	psk := []byte(password)
	if len(psk) < obfsMinPSK {
		return nil, fmt.Errorf("obfs password must be at least %d bytes", obfsMinPSK)
	}

	return &SalamanderObfuscator{
		PSK:     psk,
		RandSrc: rand.New(rand.NewSource(time.Now().UnixNano())),
	}, nil
}

// Obfuscate obfuscates the input data using Salamander algorithm
// Format: [8-byte salt][obfuscated payload]
func (o *SalamanderObfuscator) Obfuscate(in, out []byte) int {
	outLen := len(in) + obfsSaltLen
	if len(out) < outLen {
		return 0
	}

	o.mu.Lock()
	_, _ = o.RandSrc.Read(out[:obfsSaltLen])
	o.mu.Unlock()

	key := o.key(out[:obfsSaltLen])
	for i, c := range in {
		out[i+obfsSaltLen] = c ^ key[i%obfsKeyLen]
	}

	return outLen
}

// Deobfuscate deobfuscates the input data using Salamander algorithm
func (o *SalamanderObfuscator) Deobfuscate(in, out []byte) int {
	outLen := len(in) - obfsSaltLen
	if outLen <= 0 || len(out) < outLen {
		return 0
	}

	key := o.key(in[:obfsSaltLen])
	for i, c := range in[obfsSaltLen:] {
		out[i] = c ^ key[i%obfsKeyLen]
	}

	return outLen
}

// key generates the obfuscation key using BLAKE2b-256(PSK + salt)
func (o *SalamanderObfuscator) key(salt []byte) [obfsKeyLen]byte {
	return blake2b.Sum256(append(o.PSK, salt...))
}

// obfsPacketConn wraps a net.PacketConn to apply obfuscation
type obfsPacketConn struct {
	net.PacketConn
	obfs *SalamanderObfuscator

	readBuf    []byte
	readMutex  sync.Mutex
	writeBuf   []byte
	writeMutex sync.Mutex
}

// WrapPacketConn enables obfuscation on a net.PacketConn
func WrapPacketConn(conn net.PacketConn, obfs *SalamanderObfuscator) net.PacketConn {
	return &obfsPacketConn{
		PacketConn: conn,
		obfs:       obfs,
		readBuf:    make([]byte, 2048),
		writeBuf:   make([]byte, 2048),
	}
}

func (c *obfsPacketConn) ReadFrom(p []byte) (n int, addr net.Addr, err error) {
	for {
		c.readMutex.Lock()
		n, addr, err = c.PacketConn.ReadFrom(c.readBuf)
		if n <= 0 {
			c.readMutex.Unlock()
			return n, addr, err
		}
		n = c.obfs.Deobfuscate(c.readBuf[:n], p)
		c.readMutex.Unlock()
		if n > 0 || err != nil {
			return n, addr, err
		}
		// Invalid packet, try again
	}
}

func (c *obfsPacketConn) WriteTo(p []byte, addr net.Addr) (n int, err error) {
	c.writeMutex.Lock()
	nn := c.obfs.Obfuscate(p, c.writeBuf)
	_, err = c.PacketConn.WriteTo(c.writeBuf[:nn], addr)
	c.writeMutex.Unlock()
	if err == nil {
		n = len(p)
	}
	return n, err
}

func (c *obfsPacketConn) Close() error {
	return c.PacketConn.Close()
}

func (c *obfsPacketConn) LocalAddr() net.Addr {
	return c.PacketConn.LocalAddr()
}

func (c *obfsPacketConn) SetDeadline(t time.Time) error {
	return c.PacketConn.SetDeadline(t)
}

func (c *obfsPacketConn) SetReadDeadline(t time.Time) error {
	return c.PacketConn.SetReadDeadline(t)
}

func (c *obfsPacketConn) SetWriteDeadline(t time.Time) error {
	return c.PacketConn.SetWriteDeadline(t)
}
