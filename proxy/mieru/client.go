// Copyright (C) 2024  v2ray-core authors
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

package mieru

import (
	"context"

	core "github.com/frogwall/f2ray-core/v5"
	"github.com/frogwall/f2ray-core/v5/common"
	"github.com/frogwall/f2ray-core/v5/features/policy"
	"github.com/frogwall/f2ray-core/v5/transport"
	"github.com/frogwall/f2ray-core/v5/transport/internet"
)

// Client is a Mieru client.
type Client struct {
	config        *ClientConfig
	policyManager policy.Manager
}

// NewClient creates a new Mieru client based on the given config.
func NewClient(ctx context.Context, config *ClientConfig) (*Client, error) {
	if len(config.Servers) == 0 {
		return nil, newError("no server configured")
	}

	v := core.MustFromContext(ctx)
	c := &Client{
		config:        config,
		policyManager: v.GetFeature(policy.ManagerType()).(policy.Manager),
	}

	return c, nil
}

// Process implements proxy.Outbound.Process().
func (c *Client) Process(ctx context.Context, link *transport.Link, dialer internet.Dialer) error {
	// Use the handler for actual processing
	handler, err := New(ctx, c.config)
	if err != nil {
		return newError("failed to create handler").Base(err)
	}

	return handler.Process(ctx, link, dialer)
}

func init() {
	common.Must(common.RegisterConfig((*ClientConfig)(nil), func(ctx context.Context, config interface{}) (interface{}, error) {
		return NewClient(ctx, config.(*ClientConfig))
	}))
}
