package naive

import (
	"bufio"
	"context"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"sync"
	"time"

	utls "github.com/refraction-networking/utls"
	"golang.org/x/net/http2"

	core "github.com/frogwall/f2ray-core/v5"
	"github.com/frogwall/f2ray-core/v5/common"
	"github.com/frogwall/f2ray-core/v5/common/buf"
	v2net "github.com/frogwall/f2ray-core/v5/common/net"
	"github.com/frogwall/f2ray-core/v5/common/protocol"
	"github.com/frogwall/f2ray-core/v5/common/retry"
	"github.com/frogwall/f2ray-core/v5/common/session"
	"github.com/frogwall/f2ray-core/v5/common/task"
	"github.com/frogwall/f2ray-core/v5/features/policy"
	"github.com/frogwall/f2ray-core/v5/transport"
	"github.com/frogwall/f2ray-core/v5/transport/internet"
	"github.com/frogwall/f2ray-core/v5/transport/internet/security"
)

// Client implements a naive outbound with uTLS Chrome fingerprints
type Client struct {
	serverPicker       protocol.ServerPicker
	policyManager      policy.Manager
	h1SkipWaitForReply bool
}

// NewClient creates a new naive client with uTLS support
func NewClient(ctx context.Context, config *ClientConfig) (*Client, error) {
	fmt.Printf("[DEBUG] NewClient called with config: Server count=%d\n", len(config.Servers))
	sl := protocol.NewServerList()

	// Handle server configuration
	for _, naiveServer := range config.Servers {
		// Create destination
		dest := v2net.TCPDestination(v2net.ParseAddress(naiveServer.Address), v2net.Port(naiveServer.Port))

		// Create user account if credentials are provided
		var users []*protocol.MemoryUser
		if naiveServer.Username != "" {
			account := &Account{
				Username: naiveServer.Username,
				Password: naiveServer.Password,
			}
			user := &protocol.MemoryUser{
				Account: account,
				Level:   0,
			}
			users = append(users, user)
		}

		// Create ServerSpec using the correct API
		serverSpec := protocol.NewServerSpec(dest, protocol.AlwaysValid(), users...)
		sl.AddServer(serverSpec)
	}

	if sl.Size() == 0 {
		return nil, fmt.Errorf("0 server configured")
	}

	v := core.MustFromContext(ctx)
	return &Client{
		serverPicker:  protocol.NewRoundRobinServerPicker(sl),
		policyManager: v.GetFeature(policy.ManagerType()).(policy.Manager),
	}, nil
}

// createUTLSConn creates a uTLS connection that mimics Chrome
func (c *Client) createUTLSConn(rawConn net.Conn, serverName string) (*utls.UConn, error) {
	// Use Chrome 120 fingerprint for maximum compatibility with HTTP/2 support
	uConn := utls.UClient(rawConn, &utls.Config{
		ServerName:         serverName,
		InsecureSkipVerify: false,
		NextProtos:         []string{"h2", "http/1.1"}, // Support HTTP/2 and HTTP/1.1
	}, utls.HelloChrome_120)

	// Remove artificial delay for better performance

	if err := uConn.Handshake(); err != nil {
		return nil, fmt.Errorf("uTLS handshake failed: %w", err)
	}

	return uConn, nil
}

// addChromeHeaders adds headers that Chrome typically sends
func (c *Client) addChromeHeaders(req *http.Request) {
	// Chrome-like User-Agent (Chrome 120 on Linux)
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")

	// Chrome-like Accept headers
	req.Header.Set("Accept", "*/*")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	req.Header.Set("Accept-Encoding", "gzip, deflate, br")

	// Chrome connection preferences
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("Pragma", "no-cache")

	// Add the naive padding header to trigger server-side padding
	req.Header.Set("Padding", generatePaddingHeader())
}

// Process implements the outbound.Handler interface with uTLS Chrome simulation
func (c *Client) Process(ctx context.Context, link *transport.Link, dialer internet.Dialer) error {
	outbound := session.OutboundFromContext(ctx)
	if outbound == nil || !outbound.Target.IsValid() {
		return fmt.Errorf("target not specified")
	}

	target := outbound.Target
	targetAddr := target.NetAddr()

	var server *protocol.ServerSpec
	var conn net.Conn

	err := retry.ExponentialBackoff(5, 100).On(func() error {
		server = c.serverPicker.PickServer()
		dest := server.Destination()

		// Remove artificial delay for better performance

		rawConn, err := dialer.Dial(ctx, dest)
		if err != nil {
			return err
		}

		// For naive protocol with uTLS, we need to establish uTLS connection
		// Check if this is a TLS connection (port 443 or configured with TLS)
		if dest.Port == 443 {
			// Create uTLS connection that mimics Chrome
			serverName := dest.Address.String()
			if dest.Address.Family().IsDomain() {
				serverName = dest.Address.Domain()
			}

			uConn, err := c.createUTLSConn(rawConn, serverName)
			if err != nil {
				rawConn.Close()
				return fmt.Errorf("failed to create uTLS connection: %w", err)
			}
			conn = uConn
		} else {
			conn = rawConn
		}

		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to find an available destination: %w", err)
	}
	defer conn.Close()

	iConn := conn
	if statConn, ok := iConn.(*internet.StatCouterConnection); ok {
		iConn = statConn.Connection
	}

	user := server.PickUser()
	if user != nil {
		p := c.policyManager.ForLevel(user.Level)
		if p.Timeouts.Handshake > 0 {
			handshakeCtx, cancel := context.WithTimeout(ctx, p.Timeouts.Handshake)
			defer cancel()
			ctx = handshakeCtx
		}
	}

	// Create CONNECT request with Chrome-like headers
	req := &http.Request{
		Method: http.MethodConnect,
		URL:    &url.URL{Host: targetAddr},
		Header: make(http.Header),
		Host:   targetAddr,
	}

	// For CONNECT requests, only add essential headers
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")

	// Add the naive padding header to trigger server-side padding
	req.Header.Set("Padding", generatePaddingHeader())

	// Add authentication if available
	if user != nil && user.Account != nil {
		// Use interface-based approach to get credentials
		var username, password string
		if acc, ok := user.Account.(interface {
			GetUsername() string
			GetPassword() string
		}); ok {
			username = acc.GetUsername()
			password = acc.GetPassword()
		}
		if username != "" {
			auth := username + ":" + password
			req.Header.Set("Proxy-Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(auth)))
		} else {
			fmt.Printf("[DEBUG] No username found in account\n")
		}
	} else {
		fmt.Printf("[DEBUG] No user or account available\n")
	}

	// Check protocol negotiation for HTTP/2
	nextProto := ""
	if utlsConn, ok := iConn.(*utls.UConn); ok {
		// For uTLS connections, get the negotiated protocol
		state := utlsConn.ConnectionState()
		nextProto = state.NegotiatedProtocol
	} else if connALPNGetter, ok := iConn.(security.ConnectionApplicationProtocol); ok {
		var err error
		nextProto, err = connALPNGetter.GetConnectionApplicationProtocol()
		if err != nil {
			return fmt.Errorf("failed to get ALPN: %w", err)
		}
	} else if tlsConn, ok := iConn.(interface{ ConnectionState() tls.ConnectionState }); ok {
		state := tlsConn.ConnectionState()
		nextProto = state.NegotiatedProtocol
	}

	// Use HTTP/2 if negotiated, otherwise fallback to HTTP/1.1
	if nextProto == "h2" {
		return c.processHTTP2(ctx, req, iConn, link)
	}

	// Fallback to HTTP/1.1
	return c.processHTTP1(ctx, req, iConn, link)
}

// processHTTP2 handles HTTP/2 connections with Chrome-like behavior
func (c *Client) processHTTP2(ctx context.Context, req *http.Request, tlsConn net.Conn, link *transport.Link) error {

	// Create HTTP/2 client connection
	var t http2.Transport
	t.MaxHeaderListSize = 262144 // Chrome's default: 256KB
	t.AllowHTTP = false

	h2clientConn, err := t.NewClientConn(tlsConn)
	if err != nil {
		return fmt.Errorf("failed to create HTTP/2 client connection: %w", err)
	}

	// Create pipe for HTTP/2 CONNECT tunnel
	// The pipe allows us to write data that will be sent through the HTTP/2 stream
	pr, pw := io.Pipe()
	req.Body = pr

	resp, err := h2clientConn.RoundTrip(req)
	if err != nil {
		pw.Close() // Close pipe writer on error
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		pw.Close() // Close pipe writer on non-200 response
		return fmt.Errorf("proxy responded non-200: %s", resp.Status)
	}

	// Create HTTP/2 connection wrapper
	// pw is used for writing data to the server
	// resp.Body is used for reading data from the server
	proxyConn := &PaddingConn{Conn: newHTTP2Conn(tlsConn, pw, resp.Body)}

	// Start bidirectional data transfer
	requestFunc := func() error {
		return buf.Copy(link.Reader, buf.NewWriter(proxyConn))
	}
	responseFunc := func() error {
		return buf.Copy(buf.NewReader(proxyConn), link.Writer)
	}

	responseDonePost := task.OnSuccess(responseFunc, task.Close(link.Writer))
	if err := task.Run(ctx, requestFunc, responseDonePost); err != nil {
		return fmt.Errorf("connection ends: %w", err)
	}
	return nil
}

// processHTTP1 handles HTTP/1.1 connections with Chrome-like behavior
func (c *Client) processHTTP1(ctx context.Context, req *http.Request, conn net.Conn, link *transport.Link) error {
	// Remove artificial delay for better performance

	if err := req.Write(conn); err != nil {
		return err
	}

	bufferedReader := bufio.NewReader(conn)
	resp, err := http.ReadResponse(bufferedReader, req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("proxy responded non-200: %s", resp.Status)
	}

	var payload buf.MultiBuffer
	if bufferedReader.Buffered() > 0 {
		payload, err = buf.ReadFrom(io.LimitReader(bufferedReader, int64(bufferedReader.Buffered())))
		if err != nil {
			return fmt.Errorf("unable to drain buffer: %w", err)
		}
	}

	paddingConn := &PaddingConn{Conn: conn}
	return c.handleConnectionWithPayload(ctx, paddingConn, link, payload)
}

// handleConnection manages the bidirectional data transfer
func (c *Client) handleConnection(ctx context.Context, conn net.Conn, link *transport.Link, wg *sync.WaitGroup) error {
	defer conn.Close()

	requestDone := func() error {
		wg.Wait()
		return nil
	}

	responseDone := func() error {
		// Copy response data with random micro-delays
		for {
			buffer := buf.New()
			n, err := conn.Read(buffer.Bytes())
			if err != nil {
				buffer.Release()
				return err
			}

			buffer.Resize(0, int32(n))
			if err := link.Writer.WriteMultiBuffer(buf.MultiBuffer{buffer}); err != nil {
				buffer.Release()
				return err
			}

			// Remove random micro-delays for better performance
		}
	}

	if err := task.Run(ctx, requestDone, responseDone); err != nil {
		return fmt.Errorf("connection ends: %w", err)
	}

	return nil
}

// handleConnectionWithPayload handles connection with initial payload
func (c *Client) handleConnectionWithPayload(ctx context.Context, conn net.Conn, link *transport.Link, payload buf.MultiBuffer) error {
	defer conn.Close()

	if payload != nil {
		if err := link.Writer.WriteMultiBuffer(payload); err != nil {
			return err
		}
	}

	return c.handleConnection(ctx, conn, link, &sync.WaitGroup{})
}

// http2Conn implements net.Conn for HTTP/2 streams
type http2Conn struct {
	net.Conn
	in  *io.PipeWriter
	out io.ReadCloser
}

func newHTTP2Conn(c net.Conn, pipedReqBody *io.PipeWriter, respBody io.ReadCloser) net.Conn {
	return &http2Conn{Conn: c, in: pipedReqBody, out: respBody}
}

func (h *http2Conn) Read(p []byte) (n int, err error) {
	return h.out.Read(p)
}

func (h *http2Conn) Write(p []byte) (n int, err error) {
	return h.in.Write(p)
}

func (h *http2Conn) Close() error {
	var err1, err2 error
	if h.in != nil {
		err1 = h.in.Close()
	}
	if h.out != nil {
		err2 = h.out.Close()
	}
	// Return the first error encountered
	if err1 != nil {
		return err1
	}
	return err2
}

// http2StreamConn implements net.Conn for HTTP/2 CONNECT streams
type http2StreamConn struct {
	clientConn *http2.ClientConn
	stream     io.ReadCloser
}

func (h *http2StreamConn) Read(p []byte) (n int, err error) {
	return h.stream.Read(p)
}

func (h *http2StreamConn) Write(p []byte) (n int, err error) {
	// For HTTP/2 CONNECT streams, we can't write directly
	// This is a limitation of the current approach
	// We need to use a different method for writing
	return 0, fmt.Errorf("write not supported on HTTP/2 CONNECT stream")
}

func (h *http2StreamConn) Close() error {
	return h.stream.Close()
}

func (h *http2StreamConn) LocalAddr() net.Addr {
	return nil
}

func (h *http2StreamConn) RemoteAddr() net.Addr {
	return nil
}

func (h *http2StreamConn) SetDeadline(t time.Time) error {
	return nil
}

func (h *http2StreamConn) SetReadDeadline(t time.Time) error {
	return nil
}

func (h *http2StreamConn) SetWriteDeadline(t time.Time) error {
	return nil
}

func init() {
	common.Must(common.RegisterConfig((*ClientConfig)(nil), func(ctx context.Context, config interface{}) (interface{}, error) {
		return NewClient(ctx, config.(*ClientConfig))
	}))
}
