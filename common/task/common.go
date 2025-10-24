package task

import "github.com/frogwall/f2ray-core/v5/common"

// Close returns a func() that closes v.
func Close(v interface{}) func() error {
	return func() error {
		return common.Close(v)
	}
}
