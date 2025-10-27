package vless

import (
	"log"
	"strings"
	"sync"

	"github.com/frogwall/f2ray-core/v5/common/protocol"
	"github.com/frogwall/f2ray-core/v5/common/uuid"
)

// ProcessUUID processes UUID for VLESS protocol by zeroing out bytes 6 and 7
// This is part of the VLESS protocol specification
func ProcessUUID(id [16]byte) [16]byte {
	originalUUID := uuid.UUID(id)
	originalID := originalUUID.String()
	id[6] = 0
	id[7] = 0
	processedUUID := uuid.UUID(id)
	processedID := processedUUID.String()
	log.Printf("VLESS ProcessUUID: %s -> %s", originalID, processedID)
	return id
}

// Validator stores valid VLESS users.
type Validator struct {
	// Considering email's usage here, map + sync.Mutex/RWMutex may have better performance.
	email sync.Map
	users sync.Map
}

// Add a VLESS user, Email must be empty or unique.
func (v *Validator) Add(u *protocol.MemoryUser) error {
	if u.Email != "" {
		_, loaded := v.email.LoadOrStore(strings.ToLower(u.Email), u)
		if loaded {
			return newError("User ", u.Email, " already exists.")
		}
	}
	processedUUID := ProcessUUID(u.Account.(*MemoryAccount).ID.UUID())
	v.users.Store(processedUUID, u)
	processedUUIDObj := uuid.UUID(processedUUID)
	log.Printf("VLESS user added: %s UUID: %s", u.Email, processedUUIDObj.String())
	return nil
}

// Del a VLESS user with a non-empty Email.
func (v *Validator) Del(e string) error {
	if e == "" {
		return newError("Email must not be empty.")
	}
	le := strings.ToLower(e)
	u, _ := v.email.Load(le)
	if u == nil {
		return newError("User ", e, " not found.")
	}
	v.email.Delete(le)
	v.users.Delete(ProcessUUID(u.(*protocol.MemoryUser).Account.(*MemoryAccount).ID.UUID()))
	return nil
}

// Get a VLESS user with UUID, nil if user doesn't exist.
func (v *Validator) Get(id uuid.UUID) *protocol.MemoryUser {
	processedID := ProcessUUID(id)
	u, _ := v.users.Load(processedID)
	if u != nil {
		user := u.(*protocol.MemoryUser)
		idObj := uuid.UUID(id)
		log.Printf("VLESS UUID matched successfully: %s -> %s", idObj.String(), user.Email)
		return user
	}
	idObj := uuid.UUID(id)
	log.Printf("VLESS UUID not found: %s", idObj.String())
	return nil
}
