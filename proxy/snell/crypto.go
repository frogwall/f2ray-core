package snell

import (
	"crypto/aes"
	"crypto/cipher"
	"io"

	"golang.org/x/crypto/argon2"
	"golang.org/x/crypto/chacha20poly1305"

	"github.com/frogwall/f2ray-core/v5/common"
	"github.com/frogwall/f2ray-core/v5/common/buf"
	"github.com/frogwall/f2ray-core/v5/common/crypto"
	"github.com/frogwall/f2ray-core/v5/common/protocol"
)

// SnellCipher implements the Shadowsocks AEAD cipher with Argon2id key derivation.
type SnellCipher struct {
	KeyBytes        int32
	IVBytes         int32
	AEADAuthCreator func(key []byte) cipher.AEAD
	Password        []byte
}

func (c *SnellCipher) KeySize() int32 { return c.KeyBytes }
func (c *SnellCipher) IVSize() int32  { return c.IVBytes }
func (c *SnellCipher) IsAEAD() bool   { return true }

func (c *SnellCipher) createAuthenticator(salt []byte) *crypto.AEADAuthenticator {
	// Argon2id parameters from reference implementation:
	// Time: 3, Memory: 8*1024 (if unit is KB in Rust, but wait...)
	// Rust `argon2::Params::new(8, 3, 1, Some(32))`
	// m_cost = 8. In Rust `argon2` crate, m_cost is in KiB.
	// So memory = 8 * 1024 bytes? No, `argon2.Key` takes memory in KiB.
	// So memory = 8.
	// Threads = 1.
	// KeyLen = c.KeyBytes.

	subkey := argon2.IDKey(c.Password, salt, 3, 8, 1, uint32(c.KeyBytes))

	nonce := crypto.GenerateInitialAEADNonce()
	return &crypto.AEADAuthenticator{
		AEAD:           c.AEADAuthCreator(subkey),
		NonceGenerator: nonce,
	}
}

func (c *SnellCipher) NewEncryptionWriter(iv []byte, writer io.Writer) (buf.Writer, error) {
	auth := c.createAuthenticator(iv)
	return crypto.NewAuthenticationWriter(auth, &crypto.AEADChunkSizeParser{
		Auth: auth,
	}, writer, protocol.TransferTypeStream, nil), nil
}

func (c *SnellCipher) NewDecryptionReader(iv []byte, reader io.Reader) (buf.Reader, error) {
	auth := c.createAuthenticator(iv)
	return crypto.NewAuthenticationReader(auth, &crypto.AEADChunkSizeParser{
		Auth: auth,
	}, reader, protocol.TransferTypeStream, nil), nil
}

func (c *SnellCipher) EncodePacket(b *buf.Buffer) error {
	ivLen := c.IVSize()
	payloadLen := b.Len()
	auth := c.createAuthenticator(b.BytesTo(ivLen))

	b.Extend(int32(auth.Overhead()))
	_, err := auth.Seal(b.BytesTo(ivLen), b.BytesRange(ivLen, payloadLen))
	return err
}

func (c *SnellCipher) DecodePacket(b *buf.Buffer) error {
	if b.Len() <= c.IVSize() {
		return newError("insufficient data: ", b.Len())
	}
	ivLen := c.IVSize()
	payloadLen := b.Len()
	auth := c.createAuthenticator(b.BytesTo(ivLen))

	bbb, err := auth.Open(b.BytesTo(ivLen), b.BytesRange(ivLen, payloadLen))
	if err != nil {
		return err
	}
	b.Resize(ivLen, int32(len(bbb)))
	return nil
}

func createAesGcm(key []byte) cipher.AEAD {
	block, err := aes.NewCipher(key)
	common.Must(err)
	gcm, err := cipher.NewGCM(block)
	common.Must(err)
	return gcm
}

func createChaCha20Poly1305(key []byte) cipher.AEAD {
	ChaChaPoly1305, err := chacha20poly1305.New(key)
	common.Must(err)
	return ChaChaPoly1305
}

func NewSnellCipher(name string, password string) (*SnellCipher, error) {
	switch name {
	case "aes-128-gcm":
		return &SnellCipher{
			KeyBytes:        16,
			IVBytes:         16,
			AEADAuthCreator: createAesGcm,
			Password:        []byte(password),
		}, nil
	case "aes-256-gcm":
		return &SnellCipher{
			KeyBytes:        32,
			IVBytes:         32,
			AEADAuthCreator: createAesGcm,
			Password:        []byte(password),
		}, nil
	case "chacha20-ietf-poly1305":
		return &SnellCipher{
			KeyBytes:        32,
			IVBytes:         32,
			AEADAuthCreator: createChaCha20Poly1305,
			Password:        []byte(password),
		}, nil
	default:
		return nil, newError("unsupported cipher: ", name)
	}
}
