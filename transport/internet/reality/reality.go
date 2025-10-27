package reality

//go:generate go run github.com/frogwall/f2ray-core/v5/common/errors/errorgen

import (
	"context"
	"crypto/ecdh"
	"crypto/ed25519"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/sha512"
	"crypto/x509"
	"encoding/binary"
	"errors"
	"time"

	coreCrypto "github.com/frogwall/f2ray-core/v5/common/crypto"
	coreNet "github.com/frogwall/f2ray-core/v5/common/net"
	utls "github.com/refraction-networking/utls"
	"golang.org/x/crypto/hkdf"
)

// UConn wraps a uTLS connection with REALITY specific state
type UConn struct {
	*utls.UConn
	Config     *Config
	ServerName string
	AuthKey    []byte
	Verified   bool
}

// NetConn returns the underlying net.Conn
func (c *UConn) NetConn() coreNet.Conn {
	return c.UConn.NetConn()
}

// Server handles incoming REALITY connections
// NOTE: Full server implementation would require decrypting the SessionId
// from ClientHello, extracting ShortId, and performing verification.
// This is a placeholder that returns the connection as-is.
func Server(c coreNet.Conn, _ *Config) (coreNet.Conn, error) {
	// TODO: Implement full REALITY server-side handshake
	// - Read ClientHello
	// - Decrypt SessionId[0:16] to extract ShortId
	// - Verify ShortId matches expected value
	// - Extract ephemeral key and derive AuthKey
	// - Perform certificate validation with derived AuthKey
	return c, nil
}

// UClient wraps an outbound TCP connection with REALITY handshake.
func UClient(ctx context.Context, c coreNet.Conn, config *Config) (coreNet.Conn, error) {
	// 1) Build uTLS client with fingerprint preset
	ch := clientHelloByName(config.GetFingerprint())
	if ch == nil {
		// default to Chrome Auto
		def := utls.HelloChrome_Auto
		ch = &def
	}

	uConn := &UConn{
		Config:     config,
		ServerName: config.ServerName,
	}

	ucfg := &utls.Config{
		VerifyPeerCertificate:  uConn.VerifyPeerCertificate,
		ServerName:             config.ServerName,
		InsecureSkipVerify:     true,
		SessionTicketsDisabled: true,
	}
	u := utls.UClient(c, ucfg, *ch)
	uConn.UConn = u

	// 2) Build handshake state and customize SessionID
	u.BuildHandshakeState()
	hello := u.HandshakeState.Hello

	// Customize SessionId with version and ShortId
	if len(hello.SessionId) < 32 {
		hello.SessionId = make([]byte, 32)
	}

	// Set version bytes (can be customized)
	hello.SessionId[0] = 0 // version
	hello.SessionId[1] = 0 // version
	hello.SessionId[2] = 0 // version
	hello.SessionId[3] = 0 // reserved

	// Set timestamp
	binary.BigEndian.PutUint32(hello.SessionId[4:8], uint32(time.Now().Unix()))

	// Set ShortId at offset 8-24
	sid := config.ShortId
	if len(sid) > 16 {
		sid = sid[:16]
	}
	copy(hello.SessionId[8:], sid)

	// Write SessionId back to raw ClientHello
	if len(hello.Raw) >= 39+len(hello.SessionId) {
		copy(hello.Raw[39:], hello.SessionId)
	}

	// 3) Derive shared key using X25519 public key
	pub, err := ecdh.X25519().NewPublicKey(config.PublicKey)
	if err != nil || pub == nil {
		return nil, errors.New("reality: invalid public key")
	}

	// utls exposes ephemeral ecdhe before handshake via HandshakeState.State13
	ecdhe := u.HandshakeState.State13.KeyShareKeys.Ecdhe
	if ecdhe == nil {
		ecdhe = u.HandshakeState.State13.KeyShareKeys.MlkemEcdhe
	}
	if ecdhe == nil {
		return nil, errors.New("reality: fingerprint does not use TLS 1.3 keyshare")
	}

	shared, err := ecdhe.ECDH(pub)
	if err != nil || shared == nil {
		return nil, errors.New("reality: failed to derive shared secret")
	}

	// Derive AuthKey using HKDF
	if _, err := hkdf.New(sha256.New, shared, hello.Random[:20], []byte("REALITY")).Read(shared); err != nil {
		return nil, err
	}
	uConn.AuthKey = shared

	// Encrypt SessionId[0:16] with AuthKey
	aead := coreCrypto.NewAesGcm(shared)
	aead.Seal(hello.SessionId[:0], hello.Random[20:], hello.SessionId[:16], hello.Raw)
	if len(hello.Raw) >= 39+len(hello.SessionId) {
		copy(hello.Raw[39:], hello.SessionId)
	}

	// 4) Perform handshake
	if err := u.HandshakeContext(ctx); err != nil {
		return nil, err
	}

	// Basic verification
	if u.ConnectionState().ServerName == "" {
		return nil, errors.New("reality: empty SNI")
	}

	// If verification failed, trigger spider behavior for camouflage
	if !uConn.Verified {
		if config.Show {
			newError("reality: certificate verification failed, triggering camouflage behavior").WriteToLog()
		}
		// Start spider behavior in background (commented out for now as it requires HTTP client)
		// go triggerSpiderBehavior(config)
	}

	return uConn, nil
}

// VerifyPeerCertificate verifies the peer certificate using REALITY's custom verification
func (c *UConn) VerifyPeerCertificate(rawCerts [][]byte, verifiedChains [][]*x509.Certificate) error {
	if len(rawCerts) == 0 {
		return errors.New("reality: no certificates provided")
	}

	// Parse certificates
	certs := make([]*x509.Certificate, 0, len(rawCerts))
	for _, rawCert := range rawCerts {
		cert, err := x509.ParseCertificate(rawCert)
		if err != nil {
			continue
		}
		certs = append(certs, cert)
	}

	if len(certs) == 0 {
		return errors.New("reality: failed to parse certificates")
	}

	// Check if using Ed25519 certificate (REALITY specific)
	if cert, ok := certs[0].PublicKey.(ed25519.PublicKey); ok {
		// Verify signature using HMAC-SHA512
		h := hmac.New(sha512.New, c.AuthKey)
		h.Write(cert)

		// Check if signature matches
		if len(certs[0].Signature) == 64 && hmac.Equal(h.Sum(nil), certs[0].Signature) {
			c.Verified = true
			if c.Config.Show {
				newError("reality: Ed25519 certificate verified successfully").WriteToLog()
			}
			return nil
		}
	}

	// Standard certificate verification
	opts := x509.VerifyOptions{
		DNSName:       c.ServerName,
		Intermediates: x509.NewCertPool(),
	}
	for _, cert := range certs[1:] {
		opts.Intermediates.AddCert(cert)
	}

	_, err := certs[0].Verify(opts)
	c.Verified = (err == nil)
	return err
}

func clientHelloByName(name string) *utls.ClientHelloID {
	switch name {
	case "chrome_auto", "chrome":
		v := utls.HelloChrome_Auto
		return &v
	case "chrome_102":
		v := utls.HelloChrome_102
		return &v
	case "chrome_100":
		v := utls.HelloChrome_100
		return &v
	case "firefox_auto", "firefox":
		v := utls.HelloFirefox_Auto
		return &v
	case "safari_auto", "safari":
		v := utls.HelloSafari_Auto
		return &v
	case "ios", "ios_auto":
		v := utls.HelloIOS_Auto
		return &v
	case "ios_11_1":
		v := utls.HelloIOS_11_1
		return &v
	case "ios_12_1":
		v := utls.HelloIOS_12_1
		return &v
	case "ios_13":
		v := utls.HelloIOS_13
		return &v
	case "ios_14":
		v := utls.HelloIOS_14
		return &v
	case "android_11_okhttp":
		v := utls.HelloAndroid_11_OkHttp
		return &v
	default:
		return nil
	}
}
