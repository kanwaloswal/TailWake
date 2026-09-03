// Package status provides mechanisms to check if a machine is online or sleeping.
package status

import (
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/kanwaloswal/TailWake/pkg/config"
)

// DeviceStatus represents the current online/awake state of a device.
type DeviceStatus string

const (
	StatusOnline  DeviceStatus = "online"
	StatusOffline DeviceStatus = "offline"
	StatusWaking  DeviceStatus = "waking"
	StatusUnknown DeviceStatus = "unknown"
)

// DeviceState contains the calculated status and timestamp.
type DeviceState struct {
	DeviceID   string       `json:"device_id"`
	Status     DeviceStatus `json:"status"`
	LastSeen   *time.Time   `json:"last_seen,omitempty"`
	CheckedAt  time.Time    `json:"checked_at"`
	CheckError string       `json:"check_error,omitempty"`
}

// Monitor checks the status of devices configured in TailWake.
type Monitor struct {
	mu     sync.RWMutex
	cache  map[string]DeviceState
	ttl    time.Duration
}

// NewMonitor initializes a new status monitor.
func NewMonitor(cacheTTL time.Duration) *Monitor {
	if cacheTTL <= 0 {
		cacheTTL = 5 * time.Second
	}
	return &Monitor{
		cache: make(map[string]DeviceState),
		ttl:   cacheTTL,
	}
}

// CheckDevice probes a device to determine if it is currently awake.
func (m *Monitor) CheckDevice(dev config.Device) DeviceState {
	m.mu.RLock()
	cached, exists := m.cache[dev.ID]
	m.mu.RUnlock()

	// Return cached result if still fresh
	if exists && time.Since(cached.CheckedAt) < m.ttl {
		return cached
	}

	host := dev.PingHost
	if host == "" {
		// Fallback to checking default LAN IP if configured
		host = dev.BroadcastIP
	}

	port := dev.PingPort
	if port <= 0 {
		port = 22 // Default SSH port for macOS / Linux check
	}

	state := DeviceState{
		DeviceID:  dev.ID,
		CheckedAt: time.Now(),
	}

	// Fast TCP dial probe with 1.5 second timeout
	targetAddr := fmt.Sprintf("%s:%d", host, port)
	conn, err := net.DialTimeout("tcp", targetAddr, 1500*time.Millisecond)

	if err == nil {
		_ = conn.Close()
		state.Status = StatusOnline
		now := time.Now()
		state.LastSeen = &now
	} else {
		// Check if previously marked as 'waking'
		if exists && cached.Status == StatusWaking && time.Since(cached.CheckedAt) < 45*time.Second {
			state.Status = StatusWaking
		} else {
			state.Status = StatusOffline
		}
		state.CheckError = err.Error()
	}

	// Update cache
	m.mu.Lock()
	m.cache[dev.ID] = state
	m.mu.Unlock()

	return state
}

// SetWaking Status temporarily flags a device as waking up after sending a WoL packet.
func (m *Monitor) SetWaking(deviceID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.cache[deviceID] = DeviceState{
		DeviceID:  deviceID,
		Status:    StatusWaking,
		CheckedAt: time.Now(),
	}
}
