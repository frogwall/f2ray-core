//go:build !confonly
// +build !confonly

package naive

import (
	"github.com/frogwall/f2ray-core/v5/common/protocol"
)

// Equals implements protocol.Account.Equals().
func (a *Account) Equals(account protocol.Account) bool {
	naiveAccount, ok := account.(*Account)
	if !ok {
		return false
	}
	return a.Username == naiveAccount.Username && a.Password == naiveAccount.Password
}
