package log

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/frogwall/v2ray-core/v5/common/serial"
)

type logKey int

const (
	accessMessageKey logKey = iota
)

type AccessStatus string

const (
	AccessAccepted = AccessStatus("accepted")
	AccessRejected = AccessStatus("rejected")
)

type AccessMessage struct {
	From      interface{}
	To        interface{}
	Status    AccessStatus
	Reason    interface{}
	Email     string
	Detour    string
	// Extended fields
	Method    string        // HTTP method or protocol command
	Duration  time.Duration // Connection duration
	Upload    int64         // Uploaded bytes
	Download  int64         // Downloaded bytes
	Protocol  string        // Protocol name (http, socks, vmess, etc.)
}

func (m *AccessMessage) String() string {
	builder := strings.Builder{}
	builder.WriteString(serial.ToString(m.From))
	builder.WriteByte(' ')
	builder.WriteString(string(m.Status))
	builder.WriteByte(' ')
	builder.WriteString(serial.ToString(m.To))

	// Add protocol info
	if len(m.Protocol) > 0 {
		builder.WriteString(" [")
		builder.WriteString(m.Protocol)
		if len(m.Method) > 0 {
			builder.WriteByte(':')
			builder.WriteString(m.Method)
		}
		builder.WriteByte(']')
	} else if len(m.Method) > 0 {
		builder.WriteString(" [")
		builder.WriteString(m.Method)
		builder.WriteByte(']')
	}

	if len(m.Detour) > 0 {
		builder.WriteString(" via:")
		builder.WriteString(m.Detour)
	}

	// Add traffic stats
	if m.Upload > 0 || m.Download > 0 {
		builder.WriteString(" traffic:")
		if m.Upload > 0 {
			builder.WriteString(fmt.Sprintf("↑%s", formatBytes(m.Upload)))
		}
		if m.Download > 0 {
			if m.Upload > 0 {
				builder.WriteByte('/')
			}
			builder.WriteString(fmt.Sprintf("↓%s", formatBytes(m.Download)))
		}
	}

	// Add duration
	if m.Duration > 0 {
		builder.WriteString(fmt.Sprintf(" duration:%s", m.Duration.Round(time.Millisecond)))
	}

	if reason := serial.ToString(m.Reason); len(reason) > 0 {
		builder.WriteString(" reason:")
		builder.WriteString(reason)
	}

	if len(m.Email) > 0 {
		builder.WriteString(" email:")
		builder.WriteString(m.Email)
	}

	return builder.String()
}

func ContextWithAccessMessage(ctx context.Context, accessMessage *AccessMessage) context.Context {
	return context.WithValue(ctx, accessMessageKey, accessMessage)
}

func AccessMessageFromContext(ctx context.Context) *AccessMessage {
	if accessMessage, ok := ctx.Value(accessMessageKey).(*AccessMessage); ok {
		return accessMessage
	}
	return nil
}

// formatBytes formats bytes to human readable format
func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%dB", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f%cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// ShouldFilter checks if this access message should be filtered out
func (m *AccessMessage) ShouldFilter() bool {
	// Filter by tag - skip "api" tag (used for local services)
	if m.Detour == "api" {
		return true
	}
	
	// Get destination string
	dest := serial.ToString(m.To)
	
	// Filter rules for Telegram API requests
	filterPatterns := []string{
	}
	
	// Check if destination matches any filter pattern
	for _, pattern := range filterPatterns {
		if strings.Contains(dest, pattern) {
			// Also check if it's an API request
			if strings.Contains(dest, "/api") || strings.Contains(dest, ":80") {
				return true
			}
		}
	}
	
	return false
}
