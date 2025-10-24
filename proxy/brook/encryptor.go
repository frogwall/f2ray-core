//go:build !confonly
// +build !confonly

package brook

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"io"

	"golang.org/x/crypto/hkdf"
)

const (
	// Brook protocol constants
	ClientHKDFInfo  = "brook"
	ServerHKDFInfo  = "brook"
	NonceSize       = 12
	KeySize         = 32
	MaxFragmentSize = 2048
)

// BrookEncryptor handles brook protocol encryption
type BrookEncryptor struct {
	password          []byte
	nonce             []byte
	aead              cipher.AEAD
	isFirstEncryption bool
}

// NewBrookEncryptor creates a new brook encryptor
func NewBrookEncryptor(password string) (*BrookEncryptor, error) {
	encryptor := &BrookEncryptor{
		password:          []byte(password),
		nonce:             make([]byte, NonceSize),
		isFirstEncryption: true,
	}

	// Generate random nonce
	if _, err := io.ReadFull(rand.Reader, encryptor.nonce); err != nil {
		return nil, err
	}

	// Derive key using HKDF
	key := make([]byte, KeySize)
	_, err := hkdf.New(sha256.New, encryptor.password, encryptor.nonce, []byte(ClientHKDFInfo)).Read(key)
	if err != nil {
		return nil, err
	}

	// Create AES-GCM cipher
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	encryptor.aead, err = cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	return encryptor, nil
}

// Encrypt encrypts data using brook protocol
func (e *BrookEncryptor) Encrypt(data []byte) ([]byte, error) {
	if len(data) > MaxFragmentSize {
		return nil, errors.New("data too large")
	}

	// Create fragment with length prefix
	fragment := make([]byte, 2+len(data))
	binary.BigEndian.PutUint16(fragment[:2], uint16(len(data)))
	copy(fragment[2:], data)

	// Encrypt fragment
	encrypted := e.aead.Seal(nil, e.nonce, fragment, nil)

	// Increment nonce for next use
	e.incrementNonce()

	// For all encryptions, only send the encrypted data (nonce is sent separately)
	return encrypted, nil
}

// incrementNonce increments the nonce for next encryption
func (e *BrookEncryptor) incrementNonce() {
	// Increment first 8 bytes as little endian 64-bit integer (same as brook's NextNonce)
	val := binary.LittleEndian.Uint64(e.nonce[:8])
	val++
	binary.LittleEndian.PutUint64(e.nonce[:8], val)
}

// BrookDecryptor handles brook protocol decryption
type BrookDecryptor struct {
	password []byte
	nonce    []byte
	aead     cipher.AEAD
}

// NewBrookDecryptor creates a new brook decryptor
func NewBrookDecryptor(password string) (*BrookDecryptor, error) {
	decryptor := &BrookDecryptor{
		password: []byte(password),
		nonce:    make([]byte, NonceSize),
	}

	// We'll set the nonce when we receive the first encrypted data
	return decryptor, nil
}

// Decrypt decrypts data using brook protocol
func (d *BrookDecryptor) Decrypt(data []byte) ([]byte, error) {
	if len(data) < NonceSize {
		return nil, errors.New("data too short")
	}

	// Extract nonce from data
	nonce := data[:NonceSize]
	encrypted := data[NonceSize:]

	// If this is the first decryption, derive the key
	if d.aead == nil {
		key := make([]byte, KeySize)
		_, err := hkdf.New(sha256.New, d.password, nonce, []byte(ServerHKDFInfo)).Read(key)
		if err != nil {
			return nil, err
		}

		// Create AES-GCM cipher
		block, err := aes.NewCipher(key)
		if err != nil {
			return nil, err
		}

		d.aead, err = cipher.NewGCM(block)
		if err != nil {
			return nil, err
		}

		copy(d.nonce, nonce)
	}

	// Decrypt data
	decrypted, err := d.aead.Open(nil, nonce, encrypted, nil)
	if err != nil {
		return nil, err
	}

	// Increment nonce for next use
	d.incrementNonce()

	// Extract fragment length and data
	if len(decrypted) < 2 {
		return nil, errors.New("invalid decrypted data")
	}

	fragmentLength := binary.BigEndian.Uint16(decrypted[:2])
	if len(decrypted) < int(2+fragmentLength) {
		return nil, errors.New("fragment length mismatch")
	}

	return decrypted[2 : 2+fragmentLength], nil
}

// incrementNonce increments the nonce for next decryption
func (d *BrookDecryptor) incrementNonce() {
	// Increment first 8 bytes as little endian 64-bit integer (same as brook's NextNonce)
	val := binary.LittleEndian.Uint64(d.nonce[:8])
	val++
	binary.LittleEndian.PutUint64(d.nonce[:8], val)
}
