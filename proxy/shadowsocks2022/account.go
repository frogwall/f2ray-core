package shadowsocks2022

import (
	"bytes"

	"github.com/frogwall/f2ray-core/v5/common/protocol"
)

// Equals implements protocol.Account.Equals()
func (a *Account) Equals(another protocol.Account) bool {
	if account, ok := another.(*Account); ok {
		return bytes.Equal(a.UserPsk, account.UserPsk)
	}
	return false
}

// AsAccount implements protocol.AsAccount
func (a *Account) AsAccount() (protocol.Account, error) {
	return a, nil
}
