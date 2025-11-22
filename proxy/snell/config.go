package snell

import (
	"context"

	"github.com/frogwall/f2ray-core/v5/common"
	"github.com/frogwall/f2ray-core/v5/common/protocol"
)

// MemoryAccount is an account type converted from Account.
type MemoryAccount struct {
	Cipher *SnellCipher
}

// Equals implements protocol.Account.Equals().
func (a *MemoryAccount) Equals(another protocol.Account) bool {
	if account, ok := another.(*MemoryAccount); ok {
		return string(a.Cipher.Password) == string(account.Cipher.Password)
	}
	return false
}

// AsAccount implements protocol.AsAccount.
func (a *Account) AsAccount() (protocol.Account, error) {
	cipher, err := NewSnellCipher(a.Cipher, a.Password)
	if err != nil {
		return nil, newError("failed to create cipher").Base(err)
	}
	return &MemoryAccount{
		Cipher: cipher,
	}, nil
}

func init() {
	common.Must(common.RegisterConfig((*ServerConfig)(nil), func(ctx context.Context, config interface{}) (interface{}, error) {
		return NewServer(ctx, config.(*ServerConfig))
	}))
	common.Must(common.RegisterConfig((*ClientConfig)(nil), func(ctx context.Context, config interface{}) (interface{}, error) {
		return NewClient(ctx, config.(*ClientConfig))
	}))
}
