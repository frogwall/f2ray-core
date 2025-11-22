package snell_test

import (
	"bytes"
	"crypto/rand"
	"encoding/binary"
	"testing"

	"github.com/frogwall/f2ray-core/v5/common"
	"github.com/frogwall/f2ray-core/v5/common/buf"
	"github.com/frogwall/f2ray-core/v5/common/net"
	"github.com/frogwall/f2ray-core/v5/proxy/snell"
	"github.com/stretchr/testify/assert"
)

func TestSnellRequest(t *testing.T) {
	req := &snell.SnellRequest{
		Version: snell.Version,
		Command: snell.CommandConnect,
		Address: net.DomainAddress("example.com"),
		Port:    net.Port(80),
	}

	buffer := bytes.NewBuffer(nil)
	err := snell.WriteSnellRequest(buffer, req)
	common.Must(err)

	decodedReq, err := snell.ReadSnellRequest(buffer)
	common.Must(err)

	assert.Equal(t, req.Version, decodedReq.Version)
	assert.Equal(t, req.Command, decodedReq.Command)
	assert.Equal(t, req.Address.String(), decodedReq.Address.String())
	assert.Equal(t, req.Port, decodedReq.Port)
}

func TestReadUDPPacket(t *testing.T) {
	payload := []byte("hello world")
	dest := net.UDPDestination(net.LocalHostIP, net.Port(1234))

	buffer := bytes.NewBuffer(nil)
	// Manually construct Client->Server packet
	buffer.WriteByte(1) // Cmd = 1 (Targeted Message)
	buffer.WriteByte(0) // AddrLen = 0 (IP)
	buffer.WriteByte(4) // IPVersion = 4 (IPv4)
	buffer.Write(dest.Address.IP().To4())
	portBytes := make([]byte, 2)
	binary.BigEndian.PutUint16(portBytes, dest.Port.Value())
	buffer.Write(portBytes)
	buffer.Write(payload)

	decodedDest, decodedPayload, err := snell.ReadUDPPacket(buf.NewReader(buffer))
	assert.NoError(t, err)
	if err != nil {
		return
	}

	assert.Equal(t, dest.String(), decodedDest.String())
	assert.Equal(t, payload, decodedPayload.Bytes())
}

func TestWriteUDPPacket(t *testing.T) {
	payload := []byte("hello world")
	dest := net.UDPDestination(net.LocalHostIP, net.Port(1234))

	buffer := bytes.NewBuffer(nil)
	err := snell.WriteUDPPacket(buffer, payload, dest)
	common.Must(err)

	// Verify Server->Client packet
	data := buffer.Bytes()
	assert.Equal(t, byte(snell.UDPCommandIPv4), data[0]) // Cmd = 4
	assert.Equal(t, dest.Address.IP(), net.IP(data[1:5]))
	// Port is at 5, 6
	port := binary.BigEndian.Uint16(data[5:7])
	assert.Equal(t, dest.Port.Value(), port)
	assert.Equal(t, payload, data[7:])
}

func TestCipher(t *testing.T) {
	cipher, err := snell.NewSnellCipher("chacha20-ietf-poly1305", "password")
	common.Must(err)

	iv := make([]byte, cipher.IVSize())
	common.Must2(rand.Read(iv))

	buffer := buf.New()
	buffer.Write(iv)
	buffer.Write([]byte("test payload"))

	err = cipher.EncodePacket(buffer)
	common.Must(err)

	err = cipher.DecodePacket(buffer)
	common.Must(err)

	assert.Equal(t, "test payload", buffer.String())
}
