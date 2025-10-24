//go:build !confonly
// +build !confonly

package brook

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha256"
	"encoding/binary"
	"io"
	"time"

	"github.com/frogwall/f2ray-core/v5/common"
	"github.com/frogwall/f2ray-core/v5/common/buf"
	"github.com/frogwall/f2ray-core/v5/common/errors"
	"github.com/frogwall/f2ray-core/v5/common/protocol"
	"github.com/frogwall/f2ray-core/v5/common/session"
	"github.com/frogwall/f2ray-core/v5/common/task"
	"github.com/frogwall/f2ray-core/v5/features/policy"
	"github.com/frogwall/f2ray-core/v5/transport"
	"github.com/frogwall/f2ray-core/v5/transport/internet"
	"golang.org/x/crypto/hkdf"
)

type errPathObjHolder struct{}

func newError(values ...interface{}) *errors.Error {
	return errors.New(values...).WithPathObj(errPathObjHolder{})
}

// Client is a brook client
type Client struct {
	serverPicker  protocol.ServerPicker
	policyManager policy.Manager
	config        *ClientConfig
}

// NewClient creates a new brook client
func NewClient(ctx context.Context, config *ClientConfig) (*Client, error) {
	serverList := protocol.NewServerList()
	for _, rec := range config.Server {
		s, err := protocol.NewServerSpecFromPB(rec)
		if err != nil {
			return nil, newError("failed to parse server spec").Base(err)
		}
		serverList.AddServer(s)
	}

	if serverList.Size() == 0 {
		return nil, newError("0 server configured")
	}

	return &Client{
		serverPicker:  protocol.NewRoundRobinServerPicker(serverList),
		policyManager: nil, // TODO: Get from context
		config:        config,
	}, nil
}

// Process implements proxy.Outbound.Process
func (c *Client) Process(ctx context.Context, link *transport.Link, dialer internet.Dialer) error {
	outbound := session.OutboundFromContext(ctx)
	if outbound == nil || !outbound.Target.IsValid() {
		return newError("target not specified")
	}

	destination := outbound.Target
	network := destination.Network

	var server *protocol.ServerSpec
	var conn internet.Connection

	if err := task.Run(ctx, func() error {
		server = c.serverPicker.PickServer()
		dest := server.Destination()
		dest.Network = network

		// For brook protocol, we need to handle transport layer differently
		// Brook protocol itself handles the transport (TCP/WebSocket/QUIC)
		// So we should use a direct TCP connection without v2ray's transport layer
		rawConn, err := dialer.Dial(ctx, dest)
		if err != nil {
			return err
		}
		conn = rawConn
		return nil
	}); err != nil {
		return newError("failed to find an available destination").Base(err)
	}

	defer func() {
		if err := conn.Close(); err != nil {
			newError("failed to close connection").Base(err).WriteToLog(session.ExportIDToError(ctx))
		}
	}()

	// Extract password from server's user account
	var password string
	user := server.PickUser()
	if user != nil {
		if acc, ok := user.Account.(*Account); ok {
			password = acc.Password
		}
	}
	
	// Fallback to global password if server password is not set
	if password == "" {
		password = c.config.Password
	}
	
	if password == "" {
		return newError("no password configured")
	}

	account := &Account{Password: password}

	request := &protocol.RequestHeader{
		Version: 0x05, // SOCKS5 version
		Command: 0x01, // CONNECT command
		Address: destination.Address,
		Port:    destination.Port,
	}

	return c.handleConnection(ctx, conn, link, request, account, server.Method())
}

func (c *Client) handleConnection(ctx context.Context, conn internet.Connection, link *transport.Link, request *protocol.RequestHeader, account *Account, method string) error {
	// Create brook stream client based on method
	var streamClient StreamClient
	var err error

	switch method {
	case "tcp":
		streamClient, err = NewTCPStreamClient(conn, account.Password, request)
	case "ws", "wss":
		streamClient, err = NewWSStreamClient(conn, account.Password, request, c.config)
	case "quic":
		streamClient, err = NewQUICStreamClient(conn, account.Password, request, c.config)
	default:
		return newError("unsupported method: " + method)
	}

	if err != nil {
		return newError("failed to create stream client").Base(err)
	}

	// Handle data exchange
	return c.exchangeData(ctx, link, streamClient)
}

func (c *Client) exchangeData(ctx context.Context, link *transport.Link, streamClient StreamClient) error {
	defer streamClient.Close()

	requestDone := func() error {
		return buf.Copy(link.Reader, streamClient)
	}

	responseDone := func() error {
		return buf.Copy(streamClient, link.Writer)
	}

	var responseDonePost = task.OnSuccess(responseDone, task.Close(link.Writer))
	if err := task.Run(ctx, requestDone, responseDonePost); err != nil {
		return newError("connection ends").Base(err)
	}

	return nil
}

// StreamClient interface for different brook transport methods
type StreamClient interface {
	io.ReadWriteCloser
	buf.Reader
	buf.Writer
}

// TCPStreamClient implements brook TCP protocol
type TCPStreamClient struct {
	conn      internet.Connection
	password  []byte
	request   *protocol.RequestHeader
	encryptor *BrookEncryptor
	decryptor *BrookDecryptor
}

func NewTCPStreamClient(conn internet.Connection, password string, request *protocol.RequestHeader) (*TCPStreamClient, error) {
	client := &TCPStreamClient{
		conn:     conn,
		password: []byte(password),
		request:  request,
	}

	// Initialize encryption
	encryptor, err := NewBrookEncryptor(password)
	if err != nil {
		return nil, err
	}
	client.encryptor = encryptor

	decryptor, err := NewBrookDecryptor(password)
	if err != nil {
		return nil, err
	}
	client.decryptor = decryptor

	// Send initial request
	if err := client.sendRequest(); err != nil {
		return nil, err
	}

	// Wait for server nonce response
	if err := client.waitForServerNonce(); err != nil {
		return nil, err
	}

	return client, nil
}

func (c *TCPStreamClient) sendRequest() error { // First, send the nonce to server
	_, err := c.conn.Write(c.encryptor.nonce)
	if err != nil {
		return err
	}

	// Create destination address according to brook protocol
	var dst []byte

	if c.request.Address.Family().IsIP() {
		// IP address
		ip := c.request.Address.IP()
		dst = make([]byte, 0, 1+len(ip)+2)
		if len(ip) == 4 {
			dst = append(dst, byte(0x01)) // IPv4
		} else {
			dst = append(dst, byte(0x04)) // IPv6
		}
		dst = append(dst, ip...)
	} else {
		// Domain name
		domain := c.request.Address.Domain()
		dst = make([]byte, 0, 1+1+len(domain)+2)
		dst = append(dst, byte(0x03))        // Domain
		dst = append(dst, byte(len(domain))) // Domain length
		dst = append(dst, []byte(domain)...)
	}

	dst = append(dst, byte(c.request.Port>>8), byte(c.request.Port))

	// Create timestamp - must be even for TCP
	timestamp := uint32(time.Now().Unix())
	if timestamp%2 != 0 {
		timestamp += 1
	}

	// Create request data: timestamp + destination address
	requestData := make([]byte, 4+len(dst))
	binary.BigEndian.PutUint32(requestData[:4], timestamp)
	copy(requestData[4:], dst)

	// Send request data using brook protocol format
	err = c.sendBrookData(requestData)
	if err != nil {
		return err
	}
	return nil
}

func (c *TCPStreamClient) sendBrookData(data []byte) error {
	// Brook protocol has a maximum data size limit: 2048-2-16-4-16 = 2010 bytes
	maxDataSize := 2010

	// If data is too large, we need to split it into fragments
	if len(data) > maxDataSize {
		return c.sendBrookDataFragmented(data, maxDataSize)
	}

	return c.sendBrookDataSingle(data)
}

func (c *TCPStreamClient) sendBrookDataSingle(data []byte) error {
	// Create buffer for brook protocol format (same as brook's WB buffer)
	buffer := make([]byte, 2048) // Use same size as brook's BP2048

	// Put length prefix (2 bytes) - same as brook's Write method
	binary.BigEndian.PutUint16(buffer[:2], uint16(len(data)))

	// Encrypt length prefix - same as brook's Write method
	c.encryptor.aead.Seal(buffer[:0], c.encryptor.nonce, buffer[:2], nil)
	c.encryptor.incrementNonce()

	// Copy data to buffer at position 2+16 (same as brook's WB[2+16:2+16+l])
	copy(buffer[2+16:2+16+len(data)], data)

	// Encrypt data - same as brook's Write method
	c.encryptor.aead.Seal(buffer[2+16:2+16], c.encryptor.nonce, buffer[2+16:2+16+len(data)], nil)
	c.encryptor.incrementNonce()

	// Send encrypted length + encrypted data - same as brook's Write method
	totalLength := 2 + 16 + len(data) + 16
	_, err := c.conn.Write(buffer[:totalLength])
	return err
}

func (c *TCPStreamClient) sendBrookDataFragmented(data []byte, maxSize int) error {
	// Send data in fragments
	for offset := 0; offset < len(data); offset += maxSize {
		end := offset + maxSize
		if end > len(data) {
			end = len(data)
		}

		fragment := data[offset:end]
		if err := c.sendBrookDataSingle(fragment); err != nil {
			return err
		}
	}
	return nil
}

func (c *TCPStreamClient) waitForServerNonce() error {
	// Read server nonce (12 bytes)
	serverNonce := make([]byte, 12)
	if _, err := io.ReadFull(c.conn, serverNonce); err != nil {
		return err
	}

	// Initialize decryptor with server nonce
	var err error
	key := make([]byte, 32)
	_, err = hkdf.New(sha256.New, c.password, serverNonce, []byte(ServerHKDFInfo)).Read(key)
	if err != nil {
		return err
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return err
	}

	c.decryptor.aead, err = cipher.NewGCM(block)
	if err != nil {
		return err
	}

	copy(c.decryptor.nonce, serverNonce)

	return nil
}

func (c *TCPStreamClient) Read(b []byte) (int, error) {
	// Create buffer for brook protocol format (same as brook's RB buffer)
	buffer := make([]byte, 2048) // Use same size as brook's BP2048

	// Read encrypted length (2+16 bytes) - same as brook's Read method
	_, err := io.ReadFull(c.conn, buffer[:2+16])
	if err != nil {
		return 0, err
	}

	// Decrypt length - same as brook's Read method
	_, err = c.decryptor.aead.Open(buffer[:0], c.decryptor.nonce, buffer[:2+16], nil)
	if err != nil {
		return 0, err
	}
	c.decryptor.incrementNonce()

	dataLength := int(binary.BigEndian.Uint16(buffer[:2]))

	// Read encrypted data - same as brook's Read method
	_, err = io.ReadFull(c.conn, buffer[2+16:2+16+dataLength+16])
	if err != nil {
		return 0, err
	}

	// Decrypt data - same as brook's Read method
	_, err = c.decryptor.aead.Open(buffer[2+16:2+16], c.decryptor.nonce, buffer[2+16:2+16+dataLength+16], nil)
	if err != nil {
		return 0, err
	}
	c.decryptor.incrementNonce()

	// Copy to output buffer - same as brook's RB[2+16:2+16+l]
	copyLen := dataLength
	if copyLen > len(b) {
		copyLen = len(b)
	}
	copy(b, buffer[2+16:2+16+copyLen])
	return copyLen, nil
}

func (c *TCPStreamClient) Write(b []byte) (int, error) {
	// Send data using brook protocol format
	err := c.sendBrookData(b)
	if err != nil {
		return 0, err
	}
	return len(b), nil
}

func (c *TCPStreamClient) Close() error {
	return c.conn.Close()
}

// ReadMultiBuffer implements buf.Reader
func (c *TCPStreamClient) ReadMultiBuffer() (buf.MultiBuffer, error) {
	b := buf.New()
	b.Extend(2048) // Ensure buffer has enough space
	n, err := c.Read(b.Bytes())
	if err != nil {
		b.Release()
		return nil, err
	}
	b.Resize(0, int32(n)) // Set actual size
	return buf.MultiBuffer{b}, nil
}

// WriteMultiBuffer implements buf.Writer
func (c *TCPStreamClient) WriteMultiBuffer(mb buf.MultiBuffer) error {
	defer buf.ReleaseMulti(mb)
	for _, b := range mb {
		_, err := c.Write(b.Bytes())
		if err != nil {
			return err
		}
	}
	return nil
}

// WSStreamClient implements brook WebSocket protocol
type WSStreamClient struct {
	conn      internet.Connection
	password  []byte
	request   *protocol.RequestHeader
	config    *ClientConfig
	encryptor *BrookEncryptor
	decryptor *BrookDecryptor
}

func NewWSStreamClient(conn internet.Connection, password string, request *protocol.RequestHeader, config *ClientConfig) (*WSStreamClient, error) {
	client := &WSStreamClient{
		conn:     conn,
		password: []byte(password),
		request:  request,
		config:   config,
	}

	// Initialize encryption
	encryptor, err := NewBrookEncryptor(password)
	if err != nil {
		return nil, err
	}
	client.encryptor = encryptor

	decryptor, err := NewBrookDecryptor(password)
	if err != nil {
		return nil, err
	}
	client.decryptor = decryptor

	// Send WebSocket handshake and initial request
	if err := client.sendWebSocketRequest(); err != nil {
		return nil, err
	}

	return client, nil
}

func (c *WSStreamClient) sendWebSocketRequest() error {
	// TODO: Implement WebSocket handshake and brook protocol over WebSocket
	// This is a simplified implementation
	return c.sendRequest()
}

func (c *WSStreamClient) sendRequest() error {
	// Similar to TCP implementation but over WebSocket
	dst := make([]byte, 0, 1+len(c.request.Address.IP())+2)
	dst = append(dst, byte(0x01)) // IPv4
	dst = append(dst, c.request.Address.IP()...)
	dst = append(dst, byte(c.request.Port>>8), byte(c.request.Port))

	timestamp := uint32(time.Now().Unix())
	if timestamp%2 != 0 {
		timestamp += 1
	}

	requestData := make([]byte, 4+len(dst))
	binary.BigEndian.PutUint32(requestData[:4], timestamp)
	copy(requestData[4:], dst)

	encrypted, err := c.encryptor.Encrypt(requestData)
	if err != nil {
		return err
	}

	_, err = c.conn.Write(encrypted)
	return err
}

func (c *WSStreamClient) Read(b []byte) (int, error) {
	// TODO: Implement WebSocket frame reading and decryption
	encrypted := make([]byte, len(b)+32)
	n, err := c.conn.Read(encrypted)
	if err != nil {
		return 0, err
	}

	decrypted, err := c.decryptor.Decrypt(encrypted[:n])
	if err != nil {
		return 0, err
	}

	copy(b, decrypted)
	return len(decrypted), nil
}

func (c *WSStreamClient) Write(b []byte) (int, error) {
	// TODO: Implement WebSocket frame writing and encryption
	encrypted, err := c.encryptor.Encrypt(b)
	if err != nil {
		return 0, err
	}

	return c.conn.Write(encrypted)
}

func (c *WSStreamClient) Close() error {
	return c.conn.Close()
}

// ReadMultiBuffer implements buf.Reader
func (c *WSStreamClient) ReadMultiBuffer() (buf.MultiBuffer, error) {
	b := buf.New()
	_, err := c.Read(b.Bytes())
	if err != nil {
		b.Release()
		return nil, err
	}
	return buf.MultiBuffer{b}, nil
}

// WriteMultiBuffer implements buf.Writer
func (c *WSStreamClient) WriteMultiBuffer(mb buf.MultiBuffer) error {
	defer buf.ReleaseMulti(mb)
	for _, b := range mb {
		_, err := c.Write(b.Bytes())
		if err != nil {
			return err
		}
	}
	return nil
}

// QUICStreamClient implements brook QUIC protocol
type QUICStreamClient struct {
	conn      internet.Connection
	password  []byte
	request   *protocol.RequestHeader
	config    *ClientConfig
	encryptor *BrookEncryptor
	decryptor *BrookDecryptor
}

func NewQUICStreamClient(conn internet.Connection, password string, request *protocol.RequestHeader, config *ClientConfig) (*QUICStreamClient, error) {
	client := &QUICStreamClient{
		conn:     conn,
		password: []byte(password),
		request:  request,
		config:   config,
	}

	// Initialize encryption
	encryptor, err := NewBrookEncryptor(password)
	if err != nil {
		return nil, err
	}
	client.encryptor = encryptor

	decryptor, err := NewBrookDecryptor(password)
	if err != nil {
		return nil, err
	}
	client.decryptor = decryptor

	// Send initial request over QUIC
	if err := client.sendRequest(); err != nil {
		return nil, err
	}

	return client, nil
}

func (c *QUICStreamClient) sendRequest() error {
	// Similar to TCP implementation but over QUIC
	dst := make([]byte, 0, 1+len(c.request.Address.IP())+2)
	dst = append(dst, byte(0x01)) // IPv4
	dst = append(dst, c.request.Address.IP()...)
	dst = append(dst, byte(c.request.Port>>8), byte(c.request.Port))

	timestamp := uint32(time.Now().Unix())
	if timestamp%2 != 0 {
		timestamp += 1
	}

	requestData := make([]byte, 4+len(dst))
	binary.BigEndian.PutUint32(requestData[:4], timestamp)
	copy(requestData[4:], dst)

	encrypted, err := c.encryptor.Encrypt(requestData)
	if err != nil {
		return err
	}

	_, err = c.conn.Write(encrypted)
	return err
}

func (c *QUICStreamClient) Read(b []byte) (int, error) {
	// TODO: Implement QUIC stream reading and decryption
	encrypted := make([]byte, len(b)+32)
	n, err := c.conn.Read(encrypted)
	if err != nil {
		return 0, err
	}

	decrypted, err := c.decryptor.Decrypt(encrypted[:n])
	if err != nil {
		return 0, err
	}

	copy(b, decrypted)
	return len(decrypted), nil
}

func (c *QUICStreamClient) Write(b []byte) (int, error) {
	// TODO: Implement QUIC stream writing and encryption
	encrypted, err := c.encryptor.Encrypt(b)
	if err != nil {
		return 0, err
	}

	return c.conn.Write(encrypted)
}

func (c *QUICStreamClient) Close() error {
	return c.conn.Close()
}

// ReadMultiBuffer implements buf.Reader
func (c *QUICStreamClient) ReadMultiBuffer() (buf.MultiBuffer, error) {
	b := buf.New()
	_, err := c.Read(b.Bytes())
	if err != nil {
		b.Release()
		return nil, err
	}
	return buf.MultiBuffer{b}, nil
}

// WriteMultiBuffer implements buf.Writer
func (c *QUICStreamClient) WriteMultiBuffer(mb buf.MultiBuffer) error {
	defer buf.ReleaseMulti(mb)
	for _, b := range mb {
		_, err := c.Write(b.Bytes())
		if err != nil {
			return err
		}
	}
	return nil
}

func init() {
	common.Must(common.RegisterConfig((*ClientConfig)(nil), func(ctx context.Context, config interface{}) (interface{}, error) {
		return NewClient(ctx, config.(*ClientConfig))
	}))
}
