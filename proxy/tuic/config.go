package tuic

import (
	"context"

	"github.com/frogwall/f2ray-core/v5/common"
	"github.com/frogwall/f2ray-core/v5/common/protocol"
	"github.com/google/uuid"
)

// MemoryAccount is an account type converted from Account.
type MemoryAccount struct {
	UUID     uuid.UUID
	Password string
}

// Equals implements protocol.Account.Equals().
func (a *MemoryAccount) Equals(another protocol.Account) bool {
	if account, ok := another.(*MemoryAccount); ok {
		return a.UUID == account.UUID && a.Password == account.Password
	}
	return false
}

// AsAccount implements protocol.AsAccount.
func (a *Account) AsAccount() (protocol.Account, error) {
	id, err := uuid.Parse(a.Uuid)
	if err != nil {
		return nil, newError("invalid UUID").Base(err)
	}
	return &MemoryAccount{
		UUID:     id,
		Password: a.Password,
	}, nil
}

func init() {
	common.Must(common.RegisterConfig((*ClientConfig)(nil), func(ctx context.Context, config interface{}) (interface{}, error) {
		return NewClient(ctx, config.(*ClientConfig))
	}))
}
