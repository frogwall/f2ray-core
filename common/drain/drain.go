package drain

import "io"

//go:generate go run github.com/frogwall/f2ray-core/v5/common/errors/errorgen

type Drainer interface {
	AcknowledgeReceive(size int)
	Drain(reader io.Reader) error
}
