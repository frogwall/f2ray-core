//go:build !confonly
// +build !confonly

package vless

import (
	"strings"

	"github.com/frogwall/f2ray-core/v5/common/protocol"
	"github.com/frogwall/f2ray-core/v5/common/uuid"
)

// AsAccount implements protocol.Account.AsAccount().
func (a *Account) AsAccount() (protocol.Account, error) {
	id, err := uuid.ParseString(a.Id)
	if err != nil {
		return nil, newError("failed to parse ID").Base(err).AtError()
	}

	// Parse encryption string to extract configuration
	xorMode, seconds, padding := parseEncryption(a.Encryption)

	return &MemoryAccount{
		ID:         protocol.NewID(id),
		Flow:       a.Flow,
		Encryption: a.Encryption,
		XorMode:    xorMode,
		Seconds:    seconds,
		Padding:    padding,
	}, nil
}

// MemoryAccount is an in-memory form of VLess account.
type MemoryAccount struct {
	// ID of the account.
	ID *protocol.ID
	// Flow of the account.
	Flow string
	// Encryption of the account.
	Encryption string
	// XOR mode for encryption (0=none, 1=xorpub, 2=random)
	XorMode uint32
	// Seconds configuration for 0-RTT
	Seconds uint32
	// Padding configuration
	Padding string
	// Reverse configuration (for future use)
	Reverse *Reverse
}

// Equals implements protocol.Account.Equals().
func (a *MemoryAccount) Equals(account protocol.Account) bool {
	vlessAccount, ok := account.(*MemoryAccount)
	if !ok {
		return false
	}
	return a.ID.Equals(vlessAccount.ID)
}

// parseEncryption parses the encryption string to extract XorMode, Seconds, and Padding.
// Format: "mlkem768x25519plus.<mode>.<rtt-mode>.<padding>"
// Example: "mlkem768x25519plus.native.1rtt.padding"
func parseEncryption(encryption string) (uint32, uint32, string) {
	xorMode := uint32(0)
	seconds := uint32(0)
	padding := ""

	if encryption == "" || encryption == "none" {
		return xorMode, seconds, padding
	}

	parts := strings.Split(encryption, ".")
	if len(parts) < 2 {
		return xorMode, seconds, padding
	}

	// Parse XOR mode (native, xorpub, random)
	modeStr := parts[1]
	switch modeStr {
	case "native":
		xorMode = 0
	case "xorpub":
		xorMode = 1
	case "random":
		xorMode = 2
	}

	// Parse RTT mode (0rtt, 1rtt)
	if len(parts) >= 3 {
		rttStr := parts[2]
		if strings.Contains(rttStr, "1rtt") {
			seconds = 3600 // Default 1 hour for 0-RTT session cache
		}
	}

	// Parse padding
	if len(parts) >= 4 {
		padding = parts[3]
	}

	return xorMode, seconds, padding
}
