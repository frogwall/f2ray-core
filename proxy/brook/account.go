package brook

import (
	"github.com/frogwall/v2ray-core/v5/common/protocol"
)

// Equals implements protocol.Account.Equals().
func (a *Account) Equals(another protocol.Account) bool {
	if account, ok := another.(*Account); ok {
		return a.Password == account.Password
	}
	return false
}

// AsAccount implements protocol.AsAccount.
func (a *Account) AsAccount() (protocol.Account, error) {
	return a, nil
}
