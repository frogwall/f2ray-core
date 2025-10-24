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
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"time"

	"golang.org/x/crypto/pbkdf2"
)

// Mieru protocol constants
const (
	// Protocol types
	OpenSessionRequest   = 2
	OpenSessionResponse  = 3
	CloseSessionRequest  = 4
	CloseSessionResponse = 5
	DataClientToServer   = 6
	DataServerToClient   = 7
	AckClientToServer    = 8
	AckServerToClient    = 9

	// Metadata length (moved to metadata.go). Keep alias for compatibility if needed.
	// MetadataLength = 32

	// Default values
	DefaultMTU       = 1400 // Same as mieru-main DefaultMTU
	DefaultNonceSize = 24
	DefaultOverhead  = 16
	KeySize          = 32
	MaxPDU           = 32768
)

// Protocol type definitions
type ProtocolType uint8

func (p ProtocolType) String() string {
	switch p {
	case OpenSessionRequest:
		return "OpenSessionRequest"
	case OpenSessionResponse:
		return "OpenSessionResponse"
	case CloseSessionRequest:
		return "CloseSessionRequest"
	case CloseSessionResponse:
		return "CloseSessionResponse"
	case DataClientToServer:
		return "DataClientToServer"
	case DataServerToClient:
		return "DataServerToClient"
	case AckClientToServer:
		return "AckClientToServer"
	case AckServerToClient:
		return "AckServerToClient"
	default:
		return "Unknown"
	}
}

// Check if protocol is session protocol
func IsSessionProtocol(protocol ProtocolType) bool {
	return protocol == OpenSessionRequest || protocol == OpenSessionResponse ||
		protocol == CloseSessionRequest || protocol == CloseSessionResponse
}

// Check if protocol is data/ack protocol
func IsDataAckProtocol(protocol ProtocolType) bool {
	return protocol == DataClientToServer || protocol == DataServerToClient ||
		protocol == AckClientToServer || protocol == AckServerToClient
}

// Key generation functions - Fixed to match mieru-main implementation
func GenerateKey(username, password string) ([]byte, error) {
	// Step 1: Generate hashed password exactly like mieru-main
	// hashedPassword = SHA256(password + "\x00" + username)
	hashedPassword := sha256.Sum256([]byte(password + "\x00" + username))

	// Step 2: Generate time salt exactly like mieru-main
	// Round current time to nearest 2 minutes
	unixTime := time.Now().Unix()
	roundedTime := (unixTime / 120) * 120

	// Convert to 8-byte big-endian
	timeBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(timeBytes, uint64(roundedTime))
	timeSalt := sha256.Sum256(timeBytes)

	// Step 3: Generate key using PBKDF2 exactly like mieru-main
	// Use golang.org/x/crypto/pbkdf2 for correct implementation
	key := pbkdf2.Key(hashedPassword[:], timeSalt[:], 64, 32, sha256.New)

	return key, nil
}

// Generate multiple keys for time tolerance exactly like mieru-main
func GenerateKeysWithTolerance(username, password string) ([][]byte, error) {
	// fmt.Printf("[MIERU V2RAY DEBUG] GenerateKeysWithTolerance called with username=%s, password=%s\n", username, password)
	var keys [][]byte

	// Generate keys for current time and tolerance range
	// Use exactly the same logic as mieru-main saltFromTime
	// CRITICAL FIX: Use rounded time exactly like mieru-main server
	baseTime := time.Now()
	// fmt.Printf("[MIERU V2RAY DEBUG] Base time: %s\n", baseTime.Format("2006-01-02 15:04:05"))

	// Use exactly the same saltFromTime function as mieru-main
	// This will round the time to the nearest 2-minute interval
	salts := saltFromTime(baseTime)
	// fmt.Printf("[MIERU V2RAY DEBUG] Generated %d salts from time %s\n", len(salts), baseTime.Format("2006-01-02 15:04:05"))

	// Generate keys using the same logic as mieru-main
	for _, salt := range salts {
		// fmt.Printf("[MIERU V2RAY DEBUG] Using salt %d: %x\n", i, salt[:8])

		// Generate hashed password exactly like mieru-main HashPassword function
		// Server uses: HashPassword([]byte(user.GetPassword()), []byte(user.GetName()))
		// Which does: sha256.Sum256(append(append(rawPassword, 0x00), uniqueValue...))
		p := append([]byte(password), 0x00) // 0x00 separates the password and username
		p = append(p, []byte(username)...)
		hashedPassword := sha256.Sum256(p)
		// fmt.Printf("[MIERU V2RAY DEBUG] Hashed password: %x\n", hashedPassword[:8])

		// Generate key using PBKDF2 exactly like mieru-main
		key := pbkdf2.Key(hashedPassword[:], salt, 64, 32, sha256.New)
		keys = append(keys, key)
		// fmt.Printf("[MIERU V2RAY DEBUG] Generated key %d: %x (full key: %x)\n", i, key[:8], key)
	}

	return keys, nil
}

// saltFromTime generates time-based salts exactly like mieru-main
// This is a direct copy from mieru-main/pkg/cipher/keygen.go
func saltFromTime(t time.Time) [][]byte {
	var times []time.Time
	// CRITICAL FIX: Use rounded time exactly like mieru-main server
	// Server uses: rounded := t.Round(KeyRefreshInterval)
	rounded := t.Round(KeyRefreshInterval)
	times = append(times, rounded.Add(-KeyRefreshInterval))
	times = append(times, rounded)
	times = append(times, rounded.Add(KeyRefreshInterval))

	b := make([]byte, 8) // 64 bits
	var salts [][]byte

	for _, t := range times {
		binary.BigEndian.PutUint64(b, uint64(t.Unix()))
		sha := sha256.Sum256(b)
		salts = append(salts, sha[:])
	}

	return salts
}

// KeyRefreshInterval is exactly the same as mieru-main
const KeyRefreshInterval = 2 * time.Minute

// Simple HMAC-SHA256 implementation
func hmacSha256(key, data []byte) []byte {
	// Pad key to block size (64 bytes)
	if len(key) > 64 {
		hash := sha256.Sum256(key)
		key = hash[:]
	}
	if len(key) < 64 {
		paddedKey := make([]byte, 64)
		copy(paddedKey, key)
		key = paddedKey
	}

	// Create inner and outer keys
	innerKey := make([]byte, 64)
	outerKey := make([]byte, 64)
	for i := 0; i < 64; i++ {
		innerKey[i] = key[i] ^ 0x36
		outerKey[i] = key[i] ^ 0x5c
	}

	// Inner hash
	innerData := append(innerKey, data...)
	innerHash := sha256.Sum256(innerData)

	// Outer hash
	outerData := append(outerKey, innerHash[:]...)
	outerHash := sha256.Sum256(outerData)

	return outerHash[:]
}

// Validate key
func ValidateKey(key []byte) error {
	if len(key) != KeySize {
		return fmt.Errorf("invalid key size: expected %d, got %d", KeySize, len(key))
	}
	return nil
}
