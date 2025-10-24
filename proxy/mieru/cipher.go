// Copyright (C) 2024  v2ray-core authors
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

package mieru

import (
	"crypto/cipher"
	"crypto/rand"
	"fmt"
	"sync"

	"golang.org/x/crypto/chacha20poly1305"
)

// BlockContext contains optional context associated to a cipher block
type BlockContext struct {
	UserName string
}

// Cipher interface for mieru encryption
type Cipher interface {
	Encrypt(plaintext []byte) ([]byte, error)
	Decrypt(ciphertext []byte) ([]byte, error)
	EncryptWithNonce(plaintext, nonce []byte) ([]byte, error)
	DecryptWithNonce(ciphertext, nonce []byte) ([]byte, error)
	NonceSize() int
	Overhead() int
	Clone() Cipher
	BlockContext() BlockContext
	SetBlockContext(bc BlockContext)
	SetImplicitNonceMode(enable bool)
	GetKey() []byte
	GetCurrentNonce() []byte
}

// XChaCha20Poly1305Cipher implements mieru cipher using XChaCha20-Poly1305
type XChaCha20Poly1305Cipher struct {
	aead                cipher.AEAD
	key                 []byte
	ctx                 BlockContext
	enableImplicitNonce bool
	implicitNonce       []byte     // Same field for both encrypt and decrypt (like mieru-main)
	mu                  sync.Mutex // CRITICAL: Add mutex for thread safety like mieru-main
}

// NewXChaCha20Poly1305Cipher creates a new cipher instance
func NewXChaCha20Poly1305Cipher(key []byte) (*XChaCha20Poly1305Cipher, error) {
	if len(key) != KeySize {
		return nil, fmt.Errorf("invalid key size: expected %d, got %d", KeySize, len(key))
	}

	aead, err := chacha20poly1305.NewX(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create XChaCha20-Poly1305 AEAD: %w", err)
	}

	keyCopy := make([]byte, len(key))
	copy(keyCopy, key)
	return &XChaCha20Poly1305Cipher{
		aead: aead,
		key:  keyCopy,
	}, nil
}

// Encrypt encrypts plaintext with nonce exactly like mieru-main
func (c *XChaCha20Poly1305Cipher) Encrypt(plaintext []byte) ([]byte, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	var nonce []byte
	var err error
	needSendNonce := true

	// fmt.Printf("[MIERU V2RAY DEBUG] Encrypt called with %d bytes, enableImplicitNonce=%v\n", len(plaintext), c.enableImplicitNonce)
	if c.enableImplicitNonce {
		if len(c.implicitNonce) == 0 {
			// First encryption: generate random nonce
			c.implicitNonce, err = c.newNonce()
			if err != nil {
				return nil, fmt.Errorf("newNonce() failed: %w", err)
			}
			// Must create a copy because nonce will be extended
			nonce = make([]byte, len(c.implicitNonce))
			copy(nonce, c.implicitNonce)
			// fmt.Printf("[MIERU V2RAY DEBUG] First encryption, generated nonce: %x\n", nonce)
		} else {
			// Subsequent encryptions: increment nonce
			c.increaseNonce()
			nonce = c.implicitNonce
			needSendNonce = false
			// fmt.Printf("[MIERU V2RAY DEBUG] Subsequent encryption, using nonce: %x\n", nonce)
		}
	} else {
		// Stateless mode: generate random nonce each time
		nonce, err = c.newNonce()
		if err != nil {
			return nil, fmt.Errorf("newNonce() failed: %w", err)
		}
		// fmt.Printf("[MIERU V2RAY DEBUG] Stateless encryption, generated nonce: %x\n", nonce)
	}

	ciphertext := c.aead.Seal(nil, nonce, plaintext, nil)
	// fmt.Printf("[MIERU V2RAY DEBUG] Encrypted %d bytes to %d bytes, needSendNonce=%v\n", len(plaintext), len(ciphertext), needSendNonce)
	if needSendNonce {
		result := append(nonce, ciphertext...)
		// fmt.Printf("[MIERU V2RAY DEBUG] Returning %d bytes (nonce=%d + ciphertext=%d)\n", len(result), len(nonce), len(ciphertext))
		return result, nil
	}
	// fmt.Printf("[MIERU V2RAY DEBUG] Returning %d bytes (ciphertext only)\n", len(ciphertext))
	return ciphertext, nil
}

// Decrypt decrypts ciphertext with nonce exactly like mieru-main
func (c *XChaCha20Poly1305Cipher) Decrypt(ciphertext []byte) ([]byte, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	var nonce []byte
	// fmt.Printf("[MIERU V2RAY DEBUG] Decrypt called with %d bytes, enableImplicitNonce=%v\n", len(ciphertext), c.enableImplicitNonce)
	if c.enableImplicitNonce {
		// ImplicitNonceMode: use the same implicitNonce field as Encrypt (like mieru-main)
		if len(c.implicitNonce) == 0 {
			// First decryption: extract nonce from ciphertext
			if len(ciphertext) < c.NonceSize() {
				return nil, fmt.Errorf("ciphertext is smaller than nonce size")
			}
			c.implicitNonce = make([]byte, c.NonceSize())
			copy(c.implicitNonce, ciphertext[:c.NonceSize()])
			ciphertext = ciphertext[c.NonceSize():]
			// fmt.Printf("[MIERU V2RAY DEBUG] First Decrypt, extracted nonce: %x\n", c.implicitNonce)
		} else {
			// Subsequent decryptions: increment nonce
			c.increaseNonce()
			// fmt.Printf("[MIERU V2RAY DEBUG] Subsequent Decrypt, using nonce+1: %x\n", c.implicitNonce)
		}
		nonce = c.implicitNonce
	} else {
		// Stateless mode: nonce is always in ciphertext
		if len(ciphertext) < c.NonceSize() {
			return nil, fmt.Errorf("ciphertext is smaller than nonce size")
		}
		nonce = ciphertext[:c.NonceSize()]
		ciphertext = ciphertext[c.NonceSize():]
		// fmt.Printf("[MIERU V2RAY DEBUG] Decrypt enableImplicitNonce false, using nonce: %x\n", nonce)
	}

	plaintext, err := c.aead.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("cipher.AEAD.Open() failed: %w", err)
	}
	// fmt.Printf("[MIERU V2RAY DEBUG] Decrypted %d bytes to %d bytes\n", len(ciphertext), len(plaintext))
	return plaintext, nil
}

// EncryptWithNonce encrypts plaintext with provided nonce
func (c *XChaCha20Poly1305Cipher) EncryptWithNonce(plaintext, nonce []byte) ([]byte, error) {
	if len(nonce) != c.NonceSize() {
		return nil, fmt.Errorf("invalid nonce size: expected %d, got %d", c.NonceSize(), len(nonce))
	}

	ciphertext := c.aead.Seal(nil, nonce, plaintext, nil)
	return ciphertext, nil
}

// DecryptWithNonce decrypts ciphertext with provided nonce
func (c *XChaCha20Poly1305Cipher) DecryptWithNonce(ciphertext, nonce []byte) ([]byte, error) {
	if len(nonce) != c.NonceSize() {
		return nil, fmt.Errorf("invalid nonce size: expected %d, got %d", c.NonceSize(), len(nonce))
	}

	plaintext, err := c.aead.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt: %w", err)
	}

	return plaintext, nil
}

// NonceSize returns the nonce size
func (c *XChaCha20Poly1305Cipher) NonceSize() int {
	return c.aead.NonceSize()
}

// Overhead returns the authentication tag size
func (c *XChaCha20Poly1305Cipher) Overhead() int {
	return c.aead.Overhead()
}

// Clone creates a copy of the cipher
func (c *XChaCha20Poly1305Cipher) Clone() Cipher {
	c.mu.Lock()
	defer c.mu.Unlock()

	clone, err := NewXChaCha20Poly1305Cipher(c.key)
	if err != nil {
		panic(fmt.Sprintf("failed to clone cipher: %v", err))
	}
	clone.ctx = c.ctx
	clone.enableImplicitNonce = c.enableImplicitNonce

	// CRITICAL: Copy the implicitNonce state like original mieru
	if len(c.implicitNonce) != 0 {
		clone.implicitNonce = make([]byte, len(c.implicitNonce))
		copy(clone.implicitNonce, c.implicitNonce)
	}

	return clone
}

// BlockContext returns the block context
func (c *XChaCha20Poly1305Cipher) BlockContext() BlockContext {
	return c.ctx
}

// SetBlockContext sets the block context
func (c *XChaCha20Poly1305Cipher) SetBlockContext(bc BlockContext) {
	c.ctx = bc
}

// SetImplicitNonceMode enables or disables implicit nonce mode
func (c *XChaCha20Poly1305Cipher) SetImplicitNonceMode(enable bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.enableImplicitNonce = enable
	if !enable {
		c.implicitNonce = nil
	}
}

// GetKey returns the cipher key
func (c *XChaCha20Poly1305Cipher) GetKey() []byte {
	return c.key
}

// incrementNonce increments the nonce by 1 (treating it as a big-endian integer)
// increaseNonce increments the implicit nonce by 1 exactly like mieru-main
func (c *XChaCha20Poly1305Cipher) increaseNonce() {
	if !c.enableImplicitNonce || len(c.implicitNonce) == 0 {
		panic("implicit nonce mode is not enabled")
	}
	oldNonce := make([]byte, len(c.implicitNonce))
	copy(oldNonce, c.implicitNonce)

	for i := range c.implicitNonce {
		j := len(c.implicitNonce) - 1 - i
		c.implicitNonce[j] += 1
		if c.implicitNonce[j] != 0 {
			break
		}
	}

	// fmt.Printf("[MIERU V2RAY DEBUG] increaseNonce: %x -> %x\n", oldNonce, c.implicitNonce)
}

// newNonce generates a new random nonce with ToCommon64Set applied
func (c *XChaCha20Poly1305Cipher) newNonce() ([]byte, error) {
	nonce := make([]byte, c.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, fmt.Errorf("failed to generate nonce: %w", err)
	}

	// Adjust the nonce such that the first 8 bytes are printable ASCII characters
	// This is exactly like mieru-main's ToCommon64Set function
	common64Set := "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz_-"
	rewriteLen := 8
	if rewriteLen > len(nonce) {
		rewriteLen = len(nonce)
	}
	for i := 0; i < rewriteLen; i++ {
		setIdx := nonce[i] & 0x3f
		nonce[i] = common64Set[setIdx]
	}

	return nonce, nil
}

// increaseNonce increments the implicit nonce by 1

// TryDecrypt attempts to decrypt with multiple keys
func TryDecrypt(ciphertext []byte, keys [][]byte) ([]byte, Cipher, error) {
	if len(ciphertext) < DefaultNonceSize {
		return nil, nil, fmt.Errorf("ciphertext too short")
	}

	nonce := ciphertext[:DefaultNonceSize]
	encryptedData := ciphertext[DefaultNonceSize:]

	for _, key := range keys {
		cipher, err := NewXChaCha20Poly1305Cipher(key)
		if err != nil {
			continue
		}

		plaintext, err := cipher.DecryptWithNonce(encryptedData, nonce)
		if err == nil {
			return plaintext, cipher, nil
		}
	}

	return nil, nil, fmt.Errorf("failed to decrypt with any key")
}

// SelectDecrypt tries to decrypt with multiple ciphers
func SelectDecrypt(ciphertext []byte, ciphers []Cipher) ([]byte, Cipher, error) {
	// Match original mieru implementation: use Decrypt() directly
	// This ensures proper nonce state management
	for _, cipher := range ciphers {
		plaintext, err := cipher.Decrypt(ciphertext)
		if err == nil {
			return plaintext, cipher, nil
		}
	}

	return nil, nil, fmt.Errorf("failed to decrypt with any cipher")
}

// GetCurrentNonce returns the current nonce state
func (c *XChaCha20Poly1305Cipher) GetCurrentNonce() []byte {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.enableImplicitNonce && c.implicitNonce != nil {
		return append([]byte{}, c.implicitNonce...)
	}
	return nil
}
