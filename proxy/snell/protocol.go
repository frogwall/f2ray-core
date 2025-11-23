package snell

import (
	"encoding/binary"
	"io"

	"github.com/frogwall/f2ray-core/v5/common/buf"
	"github.com/frogwall/f2ray-core/v5/common/errors"
	"github.com/frogwall/f2ray-core/v5/common/net"
	"github.com/frogwall/f2ray-core/v5/common/protocol"
)

const (
	Version = 1

	CommandPing      = 0
	CommandConnect   = 1
	CommandConnectV2 = 5 // Used by Snell v2, but we focus on v1/v3 (Command 1)
	CommandUDP       = 6

	// UDP packet commands for writing to tunnel
	UDPCommandIPv4 = 4
	UDPCommandIPv6 = 6
)

var addrParser = protocol.NewAddressParser(
	protocol.AddressFamilyByte(0x01, net.AddressFamilyIPv4),
	protocol.AddressFamilyByte(0x04, net.AddressFamilyIPv6),
	protocol.AddressFamilyByte(0x03, net.AddressFamilyDomain),
)

// SnellRequest represents a Snell protocol request
type SnellRequest struct {
	Version byte
	Command byte
	Address net.Address
	Port    net.Port
}

// ReadSnellRequest reads a Snell request from the given reader.
// Format: [Version] [Command] [ClientIDLen] [ClientID] [HostLen] [Host] [Port]
// Note: ClientID is currently ignored/skipped as per reference implementation.
func ReadSnellRequest(reader io.Reader) (*SnellRequest, error) {
	buffer := buf.New()
	defer buffer.Release()

	// Read Version and Command
	if _, err := buffer.ReadFullFrom(reader, 2); err != nil {
		return nil, newError("failed to read version and command").Base(err)
	}
	version := buffer.Byte(0)
	command := buffer.Byte(1)

	if version != Version {
		return nil, newError("unexpected version").Base(newError("version: ", version))
	}

	// Read ClientID Length
	if _, err := buffer.ReadFullFrom(reader, 1); err != nil {
		return nil, newError("failed to read client id length").Base(err)
	}
	clientIDLen := int(buffer.Byte(2))
	buffer.Clear()

	// Skip ClientID
	if clientIDLen > 0 {
		if _, err := buffer.ReadFullFrom(reader, int32(clientIDLen)); err != nil {
			return nil, newError("failed to read client id").Base(err)
		}
		buffer.Clear()
	}

	req := &SnellRequest{
		Version: version,
		Command: command,
	}

	if command == CommandUDP {
		return req, nil
	}

	if command == CommandPing {
		return req, nil
	}

	// Read Hostname Length
	if _, err := buffer.ReadFullFrom(reader, 1); err != nil {
		return nil, newError("failed to read host length").Base(err)
	}
	hostLen := int(buffer.Byte(0))
	buffer.Clear()

	// Read Hostname and Port
	if _, err := buffer.ReadFullFrom(reader, int32(hostLen+2)); err != nil {
		return nil, newError("failed to read host and port").Base(err)
	}

	hostBytes := buffer.BytesTo(int32(hostLen))
	portBytes := buffer.BytesFrom(int32(hostLen))

	req.Address = net.DomainAddress(string(hostBytes))
	req.Port = net.PortFromBytes(portBytes)

	return req, nil
}

// WriteSnellRequest writes a Snell request to the writer.
func WriteSnellRequest(writer io.Writer, req *SnellRequest) error {
	buffer := buf.New()
	defer buffer.Release()

	buffer.WriteByte(req.Version)
	buffer.WriteByte(req.Command)
	buffer.WriteByte(0) // ClientID Length = 0

	if req.Command == CommandUDP || req.Command == CommandPing {
		return buf.WriteAllBytes(writer, buffer.Bytes())
	}

	hostStr := req.Address.String()
	hostLen := len(hostStr)
	if hostLen > 255 {
		return newError("hostname too long")
	}

	buffer.WriteByte(byte(hostLen))
	buffer.WriteString(hostStr)
	portBytes := make([]byte, 2)
	binary.BigEndian.PutUint16(portBytes, req.Port.Value())
	buffer.Write(portBytes)

	return buf.WriteAllBytes(writer, buffer.Bytes())
}

// ReadUDPPacket reads a UDP packet from the Snell tunnel.
// Format: [Cmd=1] [AddrLen] [Addr] [Payload]
// Note: AddrLen=0 means IP address follows (1 byte type + 4/16 bytes IP + 2 bytes Port)
// ReadUDPPacket reads a UDP packet from the Snell tunnel.
// Format: [Cmd=1] [AddrLen] [Addr] [Payload]
// Note: AddrLen=0 means IP address follows (1 byte type + 4/16 bytes IP + 2 bytes Port)
func ReadUDPPacket(reader buf.Reader) (*net.Destination, *buf.Buffer, error) {
	mb, err := reader.ReadMultiBuffer()
	if err != nil {
		return nil, nil, newError("failed to read UDP packet chunk").Base(err)
	}

	// The chunk IS the packet.
	// We need to parse it.
	// Since MultiBuffer is a list of buffers, and we expect one packet per chunk,
	// we can merge them into a single buffer for parsing.
	// Usually AEAD chunk is one buffer.
	buffer := buf.New()
	if _, err := buf.WriteMultiBuffer(buffer, mb); err != nil {
		buffer.Release()
		return nil, nil, newError("failed to write multi buffer").Base(err)
	}

	// Read Header: Cmd(1) + AddrLen(1)
	if buffer.Len() < 2 {
		buffer.Release()
		return nil, nil, newError("packet too short")
	}

	cmd := buffer.Byte(0)
	if cmd != 1 {
		buffer.Release()
		return nil, nil, newError("invalid snell udp command: ", cmd)
	}

	addrLen := int(buffer.Byte(1))
	buffer.Advance(2) // Skip Cmd and AddrLen

	var dest net.Destination

	if addrLen == 0 {
		// Read IP Type
		if buffer.Len() < 1 {
			buffer.Release()
			return nil, nil, newError("packet too short for IP type")
		}
		ipType := buffer.Byte(0)
		buffer.Advance(1)

		if ipType == 4 { // IPv4
			if buffer.Len() < 6 { // 4 IP + 2 Port
				buffer.Release()
				return nil, nil, newError("packet too short for IPv4")
			}
			ip := net.IPAddress(buffer.BytesTo(4))
			port := net.PortFromBytes(buffer.BytesRange(4, 6))
			dest = net.UDPDestination(ip, port)
			buffer.Advance(6)
		} else if ipType == 6 { // IPv6
			if buffer.Len() < 18 { // 16 IP + 2 Port
				buffer.Release()
				return nil, nil, newError("packet too short for IPv6")
			}
			ip := net.IPAddress(buffer.BytesTo(16))
			port := net.PortFromBytes(buffer.BytesRange(16, 18))
			dest = net.UDPDestination(ip, port)
			buffer.Advance(18)
		} else {
			buffer.Release()
			return nil, nil, newError("unknown IP type: ", ipType)
		}
	} else {
		// Domain
		if buffer.Len() < int32(addrLen+2) {
			buffer.Release()
			return nil, nil, newError("packet too short for domain")
		}
		domain := string(buffer.BytesTo(int32(addrLen)))
		port := net.PortFromBytes(buffer.BytesRange(int32(addrLen), int32(addrLen+2)))
		dest = net.UDPDestination(net.DomainAddress(domain), port)
		buffer.Advance(int32(addrLen + 2))
	}

	// The remaining buffer is the payload.
	return &dest, buffer, nil
}

// ReadClientUDPPacket is an alias for client-originated UDP packet reader (Cmd=1)
func ReadClientUDPPacket(reader buf.Reader) (*net.Destination, *buf.Buffer, error) {
	return ReadUDPPacket(reader)
}

// WriteUDPPacket writes a UDP packet to the tunnel.
// Format: [Cmd=4/6] [IP] [Port] [Payload]
// Note: Cmd=4 for IPv4, Cmd=6 for IPv6. Domain is NOT supported for writing to tunnel in reference implementation?
// Rust `poll_write_sourced_message`:
// Match source address:
// V4 -> Cmd=4, IP(4), Port(2)
// V6 -> Cmd=6, IP(16), Port(2)
// It seems it only supports IP addresses for outgoing UDP packets (from server to client, or client to server?).
// Usually this is for "Sourced Message", i.e. carrying the source address.
func WriteUDPPacket(writer io.Writer, payload []byte, dest net.Destination) error {
	buffer := buf.New()
	defer buffer.Release()

	if dest.Address.Family().IsIPv4() {
		buffer.WriteByte(UDPCommandIPv4)
		buffer.Write(dest.Address.IP())
		portBytes := make([]byte, 2)
		binary.BigEndian.PutUint16(portBytes, dest.Port.Value())
		buffer.Write(portBytes)
	} else if dest.Address.Family().IsIPv6() {
		buffer.WriteByte(UDPCommandIPv6)
		buffer.Write(dest.Address.IP())
		portBytes := make([]byte, 2)
		binary.BigEndian.PutUint16(portBytes, dest.Port.Value())
		buffer.Write(portBytes)
	} else {
		return newError("domain address not supported in UDP response")
	}

	buffer.Write(payload)
	return buf.WriteAllBytes(writer, buffer.Bytes())
}

// WriteClientUDPPacket writes a client-originated UDP packet (Cmd=1)
func WriteClientUDPPacket(writer io.Writer, payload []byte, dest net.Destination) error {
	buffer := buf.New()
	defer buffer.Release()

	buffer.WriteByte(1)

	if dest.Address.Family().IsDomain() {
		host := dest.Address.Domain()
		if len(host) > 255 {
			return newError("hostname too long")
		}
		buffer.WriteByte(byte(len(host)))
		buffer.WriteString(host)
		portBytes := make([]byte, 2)
		binary.BigEndian.PutUint16(portBytes, dest.Port.Value())
		buffer.Write(portBytes)
	} else {
		buffer.WriteByte(0)
		if dest.Address.Family().IsIPv4() {
			buffer.WriteByte(4)
			buffer.Write(dest.Address.IP())
			portBytes := make([]byte, 2)
			binary.BigEndian.PutUint16(portBytes, dest.Port.Value())
			buffer.Write(portBytes)
		} else if dest.Address.Family().IsIPv6() {
			buffer.WriteByte(6)
			buffer.Write(dest.Address.IP())
			portBytes := make([]byte, 2)
			binary.BigEndian.PutUint16(portBytes, dest.Port.Value())
			buffer.Write(portBytes)
		} else {
			return newError("unknown address family")
		}
	}

	buffer.Write(payload)
	return buf.WriteAllBytes(writer, buffer.Bytes())
}

// ReadServerUDPPacket reads a server-originated UDP packet (Cmd=4/6)
func ReadServerUDPPacket(reader buf.Reader) (*net.Destination, *buf.Buffer, error) {
	mb, err := reader.ReadMultiBuffer()
	if err != nil {
		return nil, nil, newError("failed to read UDP packet chunk").Base(err)
	}

	buffer := buf.New()
	if _, err := buf.WriteMultiBuffer(buffer, mb); err != nil {
		buffer.Release()
		return nil, nil, newError("failed to write multi buffer").Base(err)
	}

	if buffer.Len() < 1 {
		buffer.Release()
		return nil, nil, newError("packet too short")
	}

	cmd := buffer.Byte(0)
	buffer.Advance(1)

	var dest net.Destination
	if cmd == UDPCommandIPv4 {
		if buffer.Len() < 6 {
			buffer.Release()
			return nil, nil, newError("packet too short for IPv4")
		}
		ip := net.IPAddress(buffer.BytesTo(4))
		port := net.PortFromBytes(buffer.BytesRange(4, 6))
		dest = net.UDPDestination(ip, port)
		buffer.Advance(6)
	} else if cmd == UDPCommandIPv6 {
		if buffer.Len() < 18 {
			buffer.Release()
			return nil, nil, newError("packet too short for IPv6")
		}
		ip := net.IPAddress(buffer.BytesTo(16))
		port := net.PortFromBytes(buffer.BytesRange(16, 18))
		dest = net.UDPDestination(ip, port)
		buffer.Advance(18)
	} else {
		buffer.Release()
		return nil, nil, newError("invalid snell udp command: ", cmd)
	}

	return &dest, buffer, nil
}

func newError(values ...interface{}) *errors.Error {
	return errors.New(values...).AtWarning()
}
