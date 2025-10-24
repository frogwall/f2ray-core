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
	"fmt"
	"net"
	"time"

	core "github.com/frogwall/v2ray-core/v5"
	"github.com/frogwall/v2ray-core/v5/common"
	"github.com/frogwall/v2ray-core/v5/common/buf"
	v2net "github.com/frogwall/v2ray-core/v5/common/net"
	"github.com/frogwall/v2ray-core/v5/common/session"
	"github.com/frogwall/v2ray-core/v5/common/signal"
	"github.com/frogwall/v2ray-core/v5/common/task"
	"github.com/frogwall/v2ray-core/v5/features/policy"
	"github.com/frogwall/v2ray-core/v5/transport"
	"github.com/frogwall/v2ray-core/v5/transport/internet"
)

// Handler implements mieru outbound handler
type Handler struct {
	config        *ClientConfig
	policyManager policy.Manager
}

// New creates a new mieru outbound handler
func New(ctx context.Context, config *ClientConfig) (*Handler, error) {
	v := core.MustFromContext(ctx)

	handler := &Handler{
		config:        config,
		policyManager: v.GetFeature(policy.ManagerType()).(policy.Manager),
	}

	return handler, nil
}

// Process implements proxy.Outbound.Process()
func (h *Handler) Process(ctx context.Context, link *transport.Link, dialer internet.Dialer) error {
	outbound := session.OutboundFromContext(ctx)
	if outbound == nil || !outbound.Target.IsValid() {
		return newError("target not specified")
	}

	destination := outbound.Target

	// Use the first server (simplified single server logic)
	if len(h.config.Servers) == 0 {
		return newError("no server configured")
	}
	server := h.config.Servers[0]

	newError("tunneling request to ", destination, " via mieru server ", server.Address).WriteToLog(session.ExportIDToError(ctx))

	// Create direct connection bypassing v2ray transport layer
	conn, err := h.dialDirect(ctx, server, destination)
	if err != nil {
		return newError("failed to dial").Base(err)
	}
	defer conn.Close()

	// Process data
	return h.processData(ctx, link, conn)
}

// dialDirect establishes a direct connection with Mieru protocol handshake
func (h *Handler) dialDirect(ctx context.Context, server *Server, destination v2net.Destination) (net.Conn, error) {
	serverAddr := fmt.Sprintf("%s:%d", server.Address, server.Port)

	// Use v2ray's transport layer network setting
	// This will be determined by the streamSettings.network configuration
	network := "tcp" // Default to TCP, will be overridden by transport layer

	// Create direct connection with timeout
	dialer := &net.Dialer{
		Timeout: 30 * time.Second,
	}

	// fmt.Printf("[MIERU DEBUG] Attempting to connect to %s using %s\n", serverAddr, network)
	conn, err := dialer.DialContext(ctx, network, serverAddr)
	if err != nil {
		return nil, fmt.Errorf("failed to dial %s: %w", serverAddr, err)
	}
	// fmt.Printf("[MIERU DEBUG] Successfully connected to %s\n", serverAddr)

	// Implement Mieru protocol handshake
	session, err := h.establishMieruSession(conn, server, destination)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to establish mieru session: %w", err)
	}

	return session, nil
}

// establishMieruSession establishes a Mieru session with the server
// Fixed to match mieru-main implementation exactly
func (h *Handler) establishMieruSession(conn net.Conn, server *Server, destination v2net.Destination) (net.Conn, error) {
	// CRITICAL FIX: Follow mieru-main client pattern exactly
	// Client generates keys for time tolerance, but only uses ONE cipher
	// Server uses SelectDecrypt to try all keys for time tolerance

	// Generate keys for time tolerance exactly like mieru-main client
	keys, err := GenerateKeysWithTolerance(server.Username, server.Password)
	if err != nil {
		return nil, fmt.Errorf("failed to generate keys: %w", err)
	}

	// CRITICAL FIX: Client uses the SECOND key (current time window)
	// This matches mieru-main behavior where client uses cipherList[1] (current time)
	// Server will use SelectDecrypt to try all keys for time tolerance
	if len(keys) < 2 {
		return nil, fmt.Errorf("insufficient keys generated: %d", len(keys))
	}

	// Use the second key (current time window) - exactly like mieru-main client
	// keys[0] = past time window, keys[1] = current time window, keys[2] = future time window
	primaryKey := keys[1]
	// fmt.Printf("[MIERU V2RAY DEBUG] Client using current time key (keys[1]): %x\n", primaryKey[:8])

	// Create single cipher exactly like mieru-main client
	cipher, err := NewXChaCha20Poly1305Cipher(primaryKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	// CRITICAL FIX: Set ImplicitNonceMode for client like mieru-main
	// In mieru-main, client DOES call SetImplicitNonceMode(true) in newBlockCipherList
	// This is needed for continuous nonce sequence
	cipher.SetImplicitNonceMode(true)

	// Set BlockContext exactly like mieru-main client
	cipher.SetBlockContext(BlockContext{
		UserName: server.Username,
	})

	// Create session with single cipher (like mieru-main client)
	session := NewMieruSessionWithCiphers(conn, []Cipher{cipher}, destination)

	// Perform handshake exactly like mieru-main
	if err := session.Handshake(); err != nil {
		return nil, fmt.Errorf("handshake failed: %w", err)
	}

	return session, nil
}

// processData processes data between v2ray and mieru connection
func (h *Handler) processData(ctx context.Context, link *transport.Link, conn net.Conn) error {
	policy := h.policyManager.ForLevel(0)

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	timer := signal.CancelAfterInactivity(ctx, cancel, policy.Timeouts.ConnectionIdle)

	// Start data processing
	requestDone := func() error {
		defer timer.SetTimeout(policy.Timeouts.DownlinkOnly)

		return task.Run(ctx, func() error {
			return h.copyDataFromReader(link.Reader, conn)
		})
	}

	responseDone := func() error {
		defer timer.SetTimeout(policy.Timeouts.UplinkOnly)

		return task.Run(ctx, func() error {
			return h.copyDataToWriter(conn, link.Writer)
		})
	}

	var responseDonePost = task.OnSuccess(responseDone, task.Close(link.Writer))
	if err := task.Run(ctx, requestDone, responseDonePost); err != nil {
		common.Interrupt(link.Reader)
		common.Interrupt(link.Writer)
		return newError("connection ends").Base(err)
	}

	return nil
}

// copyDataFromReader copies data from buf.Reader to net.Conn
func (h *Handler) copyDataFromReader(src buf.Reader, dst net.Conn) error {
	return buf.Copy(src, buf.NewWriter(dst))
}

// copyDataToWriter copies data from net.Conn to buf.Writer
func (h *Handler) copyDataToWriter(src net.Conn, dst buf.Writer) error {
	return buf.Copy(buf.NewReader(src), dst)
}
