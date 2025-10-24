package hysteria2

import (
	"errors"
	"io"
	"math/rand"
	"sync"

	"github.com/apernet/quic-go"

	hyProtocol "github.com/v2fly/hysteria/core/v2/international/protocol"
)

const (
	udpMessageChanSize = 1024
)

// UDPConn represents a UDP connection interface
type UDPConn interface {
	Receive() ([]byte, string, error)
	Send([]byte, string) error
	Close() error
}

// UDPIO defines the interface for UDP message I/O
type UDPIO interface {
	ReceiveMessage() (*hyProtocol.UDPMessage, error)
	SendMessage([]byte, *hyProtocol.UDPMessage) error
}

// udpIO is an alias for UDPIO to avoid naming conflicts
type udpIO = UDPIO

// UDPConnImpl implements UDPConn interface
type UDPConnImpl struct {
	ID        uint32
	D         *Defragger
	ReceiveCh chan *hyProtocol.UDPMessage
	SendBuf   []byte
	SendFunc  func([]byte, *hyProtocol.UDPMessage) error
	CloseFunc func()
	Closed    bool
}

// Receive receives UDP data
func (u *UDPConnImpl) Receive() ([]byte, string, error) {
	for {
		msg := <-u.ReceiveCh
		if msg == nil {
			// Closed
			return nil, "", io.EOF
		}
		dfMsg := u.D.Feed(msg)
		if dfMsg == nil {
			// Incomplete message, wait for more
			continue
		}
		return dfMsg.Data, dfMsg.Addr, nil
	}
}

// Send sends UDP data
func (u *UDPConnImpl) Send(data []byte, addr string) error {
	// Try no frag first
	msg := &hyProtocol.UDPMessage{
		SessionID: u.ID,
		PacketID:  0,
		FragID:    0,
		FragCount: 1,
		Addr:      addr,
		Data:      data,
	}
	err := u.SendFunc(u.SendBuf, msg)
	var errTooLarge *quic.DatagramTooLargeError
	if errors.As(err, &errTooLarge) {
		// Message too large, try fragmentation
		msg.PacketID = uint16(rand.Intn(0xFFFF)) + 1
		fMsgs := FragUDPMessage(msg, 1200) // Use default MTU size
		for _, fMsg := range fMsgs {
			err := u.SendFunc(u.SendBuf, &fMsg)
			if err != nil {
				return err
			}
		}
		return nil
	} else {
		return err
	}
}

// Close closes the UDP connection
func (u *UDPConnImpl) Close() error {
	u.CloseFunc()
	return nil
}

// UDPSessionManager manages UDP sessions
type UDPSessionManager struct {
	io UDPIO

	mutex  sync.RWMutex
	m      map[uint32]*UDPConnImpl
	nextID uint32

	closed bool
}

// NewUDPSessionManager creates a new UDP session manager
func NewUDPSessionManager(io UDPIO) *UDPSessionManager {
	m := &UDPSessionManager{
		io:     io,
		m:      make(map[uint32]*UDPConnImpl),
		nextID: 1,
	}
	go m.run()
	return m
}

// run runs the session manager main loop
func (m *UDPSessionManager) run() error {
	defer m.closeCleanup()
	for {
		msg, err := m.io.ReceiveMessage()
		if err != nil {
			return err
		}
		m.feed(msg)
	}
}

// closeCleanup cleans up all sessions
func (m *UDPSessionManager) closeCleanup() {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	for _, conn := range m.m {
		m.close(conn)
	}
	m.closed = true
}

// feed feeds a UDP message to the appropriate session
func (m *UDPSessionManager) feed(msg *hyProtocol.UDPMessage) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	conn, ok := m.m[msg.SessionID]
	if !ok {
		// Ignore message from unknown session
		return
	}

	select {
	case conn.ReceiveCh <- msg:
		// OK
	default:
		// Channel full, drop the message
	}
}

// NewUDP creates a new UDP session
func (m *UDPSessionManager) NewUDP() (UDPConn, error) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if m.closed {
		return nil, errors.New("session manager closed")
	}

	id := m.nextID
	m.nextID++

	conn := &UDPConnImpl{
		ID:        id,
		D:         NewDefragger(),
		ReceiveCh: make(chan *hyProtocol.UDPMessage, udpMessageChanSize),
		SendBuf:   make([]byte, hyProtocol.MaxUDPSize),
		SendFunc:  m.io.SendMessage,
	}
	conn.CloseFunc = func() {
		m.mutex.Lock()
		defer m.mutex.Unlock()
		m.close(conn)
	}
	m.m[id] = conn

	return conn, nil
}

// close closes a UDP connection
func (m *UDPSessionManager) close(conn *UDPConnImpl) {
	if !conn.Closed {
		conn.Closed = true
		close(conn.ReceiveCh)
		delete(m.m, conn.ID)
	}
}

// Count returns the number of active sessions
func (m *UDPSessionManager) Count() int {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	return len(m.m)
}

// Defragger handles UDP message defragmentation
type Defragger struct {
	// Implementation would go here
	// This is a simplified version
}

// NewDefragger creates a new defragger
func NewDefragger() *Defragger {
	return &Defragger{}
}

// Feed feeds a fragmented message to the defragger
func (d *Defragger) Feed(msg *hyProtocol.UDPMessage) *hyProtocol.UDPMessage {
	// Simplified implementation - just return the message as-is
	// In a real implementation, this would handle fragmentation
	return msg
}

// FragUDPMessage fragments a UDP message if it's too large
func FragUDPMessage(msg *hyProtocol.UDPMessage, maxSize int) []hyProtocol.UDPMessage {
	// Simplified implementation - just return the original message
	// In a real implementation, this would split large messages
	return []hyProtocol.UDPMessage{*msg}
}

// UDPIOImpl implements UDPIO interface for v2ray-core
type UDPIOImpl struct {
	Conn quic.Connection
}

// ReceiveMessage receives a UDP message from the QUIC connection
func (io *UDPIOImpl) ReceiveMessage() (*hyProtocol.UDPMessage, error) {
	for {
		msg, err := io.Conn.ReceiveDatagram(nil)
		if err != nil {
			// Connection error, this will stop the session manager
			return nil, err
		}
		udpMsg, err := hyProtocol.ParseUDPMessage(msg)
		if err != nil {
			// Invalid message, this is fine - just wait for the next
			continue
		}
		return udpMsg, nil
	}
}

// SendMessage sends a UDP message over the QUIC connection
func (io *UDPIOImpl) SendMessage(buf []byte, msg *hyProtocol.UDPMessage) error {
	msgN := msg.Serialize(buf)
	if msgN < 0 {
		// Message larger than buffer, silent drop
		return nil
	}
	return io.Conn.SendDatagram(buf[:msgN])
}
