package hysteria2

import (
	"context"
	"crypto/tls"
	"net"
	"net/http"
	"net/url"

	"github.com/apernet/quic-go"
	"github.com/apernet/quic-go/http3"
	hyProtocol "github.com/v2fly/hysteria/core/v2/international/protocol"
)

const (
	closeErrCodeOK            = 0x100 // HTTP3 ErrCodeNoError
	closeErrCodeProtocolError = 0x101 // HTTP3 ErrCodeGeneralProtocolError
)

// AuthRequest is what client sends to server for authentication.
type AuthRequest struct {
	Auth string
	Rx   uint64 // 0 = unknown, client asks server to use bandwidth detection
}

// AuthResponse is what server sends to client when authentication is passed.
type AuthResponse struct {
	UDPEnabled bool
	Rx         uint64 // 0 = unlimited
	RxAuto     bool   // true = server asks client to use bandwidth detection
}

// HandshakeInfo contains information from the handshake
type HandshakeInfo struct {
	UDPEnabled bool
	Tx         uint64 // 0 if using BBR
}

// authenticate performs HTTP/3 authentication with the server
func authenticate(ctx context.Context, pktConn net.PacketConn, serverAddr string, tlsConfig *tls.Config, quicConfig *quic.Config, auth string, maxRx uint64) (quic.Connection, *AuthResponse, error) {
	// Parse server address
	addr, err := net.ResolveUDPAddr("udp", serverAddr)
	if err != nil {
		return nil, nil, newError("invalid server address").Base(err)
	}

	// Prepare RoundTripper
	var conn quic.Connection
	rt := &http3.Transport{
		TLSClientConfig: tlsConfig,
		QUICConfig:      quicConfig,
		Dial: func(ctx context.Context, _ string, tlsCfg *tls.Config, cfg *quic.Config) (quic.EarlyConnection, error) {
			qc, err := quic.DialEarly(ctx, pktConn, addr, tlsCfg, cfg)
			if err != nil {
				return nil, err
			}
			conn = qc
			return qc, nil
		},
	}

	// Send auth HTTP request
	req := &http.Request{
		Method: http.MethodPost,
		URL: &url.URL{
			Scheme: "https",
			Host:   hyProtocol.URLHost,
			Path:   hyProtocol.URLPath,
		},
		Header: make(http.Header),
	}
	hyProtocol.AuthRequestToHeader(req.Header, hyProtocol.AuthRequest{
		Auth: auth,
		Rx:   maxRx,
	})

	resp, err := rt.RoundTrip(req)
	if err != nil {
		if conn != nil {
			_ = conn.CloseWithError(closeErrCodeProtocolError, "")
		}
		_ = pktConn.Close()
		return nil, nil, newError("authentication failed").Base(err)
	}

	if resp.StatusCode != hyProtocol.StatusAuthOK {
		_ = conn.CloseWithError(closeErrCodeProtocolError, "")
		_ = pktConn.Close()
		return nil, nil, newError("authentication failed with status: %d", resp.StatusCode)
	}

	// Auth OK
	authResp := hyProtocol.AuthResponseFromHeader(resp.Header)
	_ = resp.Body.Close()

	return conn, &AuthResponse{
		UDPEnabled: authResp.UDPEnabled,
		Rx:         authResp.Rx,
		RxAuto:     authResp.RxAuto,
	}, nil
}
