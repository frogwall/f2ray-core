package internet

import (
	"net"

	"github.com/frogwall/f2ray-core/v5/common"
	"github.com/frogwall/f2ray-core/v5/features/stats"
)

type Connection interface {
	net.Conn
}

type AbstractPacketConnReader interface {
	ReadFrom(p []byte) (n int, addr net.Addr, err error)
}

type AbstractPacketConnWriter interface {
	WriteTo(p []byte, addr net.Addr) (n int, err error)
}

type AbstractPacketConn interface {
	AbstractPacketConnReader
	AbstractPacketConnWriter
	common.Closable
}

type PacketConn interface {
	AbstractPacketConn
	net.PacketConn
}

type StatCouterConnection struct {
	Connection
	ReadCounter  stats.Counter
	WriteCounter stats.Counter
}

func (c *StatCouterConnection) Read(b []byte) (int, error) {
	nBytes, err := c.Connection.Read(b)
	if c.ReadCounter != nil {
		c.ReadCounter.Add(int64(nBytes))
	}

	return nBytes, err
}

func (c *StatCouterConnection) Write(b []byte) (int, error) {
	nBytes, err := c.Connection.Write(b)
	if c.WriteCounter != nil {
		c.WriteCounter.Add(int64(nBytes))
	}
	return nBytes, err
}

// UnixConnWrapper wraps a net.UnixConn to provide UnwrapRawConn support
type UnixConnWrapper struct {
	*net.UnixConn
}

// GetUnixConn returns the underlying UnixConn
func (c *UnixConnWrapper) GetUnixConn() *net.UnixConn {
	return c.UnixConn
}
