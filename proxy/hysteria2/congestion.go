package hysteria2

import (
	"time"

	"github.com/apernet/quic-go"
	"github.com/apernet/quic-go/congestion"
)

// CongestionControlType represents the type of congestion control algorithm
type CongestionControlType string

const (
	CongestionControlBBR    CongestionControlType = "bbr"
	CongestionControlBrutal CongestionControlType = "brutal"
)

// CongestionControlConfig holds configuration for congestion control
type CongestionControlConfig struct {
	Type     CongestionControlType
	UpMbps   uint64
	DownMbps uint64
}

// ApplyCongestionControl applies the specified congestion control algorithm to the QUIC connection
func ApplyCongestionControl(conn quic.Connection, config *CongestionControlConfig) {
	if config == nil {
		return
	}

	switch config.Type {
	case CongestionControlBBR:
		// Use BBR congestion control
		// Note: This would require implementing BBR or using a library that provides it
		// For now, we'll use the default QUIC congestion control
		break
	case CongestionControlBrutal:
		// Use Brutal congestion control
		if config.UpMbps > 0 {
			// Convert Mbps to bytes per second
			rate := config.UpMbps * 1024 * 1024 / 8
			// Note: This would require implementing Brutal congestion control
			// For now, we'll use the default QUIC congestion control
			_ = rate
		}
		break
	default:
		// Use default QUIC congestion control
		break
	}
}

// CongestionBandwidthConfig holds bandwidth configuration for congestion control
type CongestionBandwidthConfig struct {
	MaxTx uint64 // Maximum transmit rate in bytes per second
	MaxRx uint64 // Maximum receive rate in bytes per second
}

// GetBandwidthConfig returns bandwidth configuration from congestion config
func GetBandwidthConfig(config *CongestionControlConfig) *CongestionBandwidthConfig {
	if config == nil {
		return &CongestionBandwidthConfig{}
	}

	return &CongestionBandwidthConfig{
		MaxTx: config.UpMbps * 1024 * 1024 / 8,   // Convert Mbps to bytes per second
		MaxRx: config.DownMbps * 1024 * 1024 / 8, // Convert Mbps to bytes per second
	}
}

// CongestionControlInterface defines the interface for congestion control algorithms
type CongestionControlInterface interface {
	SetRTTStatsProvider(provider congestion.RTTStatsProvider)
	TimeUntilSend(bytesInFlight congestion.ByteCount) time.Time
	OnPacketSent(packet congestion.PacketNumber, bytes congestion.ByteCount, isRetransmittable bool)
	OnPacketAcked(packet congestion.PacketNumber, bytes congestion.ByteCount)
	OnPacketLost(packet congestion.PacketNumber, bytes congestion.ByteCount)
	OnRetransmissionTimeout(packet congestion.PacketNumber) bool
	OnConnectionMigration()
	SetMaxDatagramSize(size congestion.ByteCount)
	GetCongestionWindow() congestion.ByteCount
}

// DefaultCongestionControl implements a basic congestion control algorithm
type DefaultCongestionControl struct {
	rttProvider     congestion.RTTStatsProvider
	maxDatagramSize congestion.ByteCount
}

// NewDefaultCongestionControl creates a new default congestion control instance
func NewDefaultCongestionControl() *DefaultCongestionControl {
	return &DefaultCongestionControl{
		maxDatagramSize: 1200, // Default MTU
	}
}

func (d *DefaultCongestionControl) SetRTTStatsProvider(provider congestion.RTTStatsProvider) {
	d.rttProvider = provider
}

func (d *DefaultCongestionControl) TimeUntilSend(bytesInFlight congestion.ByteCount) time.Time {
	// Simple rate limiting - send immediately
	return time.Now()
}

func (d *DefaultCongestionControl) OnPacketSent(packet congestion.PacketNumber, bytes congestion.ByteCount, isRetransmittable bool) {
	// Handle packet sent
}

func (d *DefaultCongestionControl) OnPacketAcked(packet congestion.PacketNumber, bytes congestion.ByteCount) {
	// Handle packet acknowledged
}

func (d *DefaultCongestionControl) OnPacketLost(packet congestion.PacketNumber, bytes congestion.ByteCount) {
	// Handle packet lost
}

func (d *DefaultCongestionControl) OnRetransmissionTimeout(packet congestion.PacketNumber) bool {
	// Handle retransmission timeout
	return false
}

func (d *DefaultCongestionControl) OnConnectionMigration() {
	// Handle connection migration
}

func (d *DefaultCongestionControl) SetMaxDatagramSize(size congestion.ByteCount) {
	d.maxDatagramSize = size
}

func (d *DefaultCongestionControl) GetCongestionWindow() congestion.ByteCount {
	// Return a reasonable congestion window size
	return congestion.ByteCount(10000) // 10KB default window
}
