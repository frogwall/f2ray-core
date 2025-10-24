package tlstrafficgen

import "github.com/frogwall/f2ray-core/v5/common/errors"

type errPathObjHolder struct{}

func newError(values ...interface{}) *errors.Error {
	return errors.New(values...).WithPathObj(errPathObjHolder{})
}
