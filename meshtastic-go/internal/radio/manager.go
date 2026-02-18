package radio

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/meshtastic/meshtastic-go/internal/radio/ble"
	"github.com/meshtastic/meshtastic-go/internal/radio/serial"
	"github.com/meshtastic/meshtastic-go/internal/radio/tcp"
	"github.com/rs/zerolog/log"
)

// ConnectionState represents the current connection state
type ConnectionState int

const (
	Disconnected ConnectionState = iota
	Connecting
	Connected
	DeviceSleep
)

func (s ConnectionState) String() string {
	switch s {
	case Disconnected:
		return "disconnected"
	case Connecting:
		return "connecting"
	case Connected:
		return "connected"
	case DeviceSleep:
		return "device_sleep"
	default:
		return "unknown"
	}
}

// ConnectionType represents the type of radio connection
type ConnectionType string

const (
	ConnectionBLE    ConnectionType = "ble"
	ConnectionSerial ConnectionType = "serial"
	ConnectionTCP    ConnectionType = "tcp"
)

// DeviceInfo represents a discovered device
type DeviceInfo struct {
	Type       ConnectionType `json:"type"`
	Address    string         `json:"address"`
	Name       string         `json:"name"`
	RSSI       int            `json:"rssi,omitempty"`
	MacAddress string         `json:"mac_address,omitempty"`
}

// DeviceFoundCallback is called when a device is discovered during scanning
type DeviceFoundCallback func(device DeviceInfo)

// IRadioInterface defines the contract for radio connections
type IRadioInterface interface {
	HandleSendToRadio(data []byte) error
	KeepAlive() error
	Close() error
	Type() string
	Address() string
	IsConnected() bool
}

// RadioCallbacks defines callbacks for radio events
type RadioCallbacks interface {
	OnConnect()
	OnDisconnect(isPermanent bool, err error)
	HandleFromRadio(data []byte)
}

// Manager manages radio connections
type Manager struct {
	mu            sync.RWMutex
	current       IRadioInterface
	state         ConnectionState
	callbacks     RadioCallbacks
	heartbeatStop chan struct{}

	// Scanners
	bleScanner    *ble.BLEScanner
	serialScanner *serial.SerialScanner
	tcpScanner    *tcp.MDNSScanner
}

// NewManager creates a new radio manager
func NewManager() *Manager {
	return &Manager{
		state:         Disconnected,
		bleScanner:    ble.NewBLEScanner(),
		serialScanner: serial.NewSerialScanner(),
		tcpScanner:    tcp.NewMDNSScanner(),
	}
}

// SetCallbacks sets the callbacks for radio events
func (m *Manager) SetCallbacks(callbacks RadioCallbacks) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.callbacks = callbacks
}

// Connect establishes a connection to a radio device
func (m *Manager) Connect(ctx context.Context, connType ConnectionType, address string) error {
	// Cancel any ongoing BLE scan first - this is critical because
	// the BLE adapter can't be used for scanning and connecting simultaneously
	if connType == ConnectionBLE && m.bleScanner != nil {
		if m.bleScanner.IsScanning() {
			log.Info().Msg("cancelling BLE scan before connecting")
			m.bleScanner.Cancel()
			// Cancel() already waits internally for scan to stop
		}
	}

	// Close existing connection first (with lock)
	m.mu.Lock()
	if m.current != nil {
		if err := m.current.Close(); err != nil {
			log.Warn().Err(err).Msg("failed to close existing connection")
		}
		m.current = nil
	}
	m.state = Connecting
	m.mu.Unlock()

	// Create and connect interface WITHOUT holding the lock
	// (callbacks may try to acquire the lock)
	var iface IRadioInterface
	var err error

	switch connType {
	case ConnectionBLE:
		iface, err = m.connectBLE(ctx, address)
	case ConnectionSerial:
		iface, err = m.connectSerial(ctx, address)
	case ConnectionTCP:
		iface, err = m.connectTCP(ctx, address)
	default:
		return fmt.Errorf("unknown connection type: %s", connType)
	}

	if err != nil {
		m.mu.Lock()
		m.state = Disconnected
		m.mu.Unlock()
		return fmt.Errorf("failed to connect: %w", err)
	}

	// Update state with lock
	m.mu.Lock()
	m.current = iface
	m.state = Connected
	m.mu.Unlock()

	// Start heartbeat
	m.startHeartbeat()

	log.Info().
		Str("type", string(connType)).
		Str("address", address).
		Msg("connected to radio")

	// Call OnConnect callback AFTER m.current is set
	// This allows the callback to use SendToRadio
	if m.callbacks != nil {
		m.callbacks.OnConnect()
	}

	return nil
}

// Disconnect closes the current connection
func (m *Manager) Disconnect() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.stopHeartbeat()

	if m.current == nil {
		return nil
	}

	err := m.current.Close()
	m.current = nil
	m.state = Disconnected

	if m.callbacks != nil {
		m.callbacks.OnDisconnect(true, nil)
	}

	return err
}

// SendToRadio sends data to the connected radio
func (m *Manager) SendToRadio(data []byte) error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.current == nil {
		return fmt.Errorf("not connected")
	}

	return m.current.HandleSendToRadio(data)
}

// State returns the current connection state
func (m *Manager) State() ConnectionState {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.state
}

// IsConnected returns true if connected
func (m *Manager) IsConnected() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.current != nil && m.current.IsConnected()
}

// GetConnectionInfo returns information about the current connection
func (m *Manager) GetConnectionInfo() (ConnectionType, string, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.current == nil {
		return "", "", false
	}

	return ConnectionType(m.current.Type()), m.current.Address(), true
}

// Scan discovers available devices of the specified type
func (m *Manager) Scan(ctx context.Context, connType ConnectionType, timeout time.Duration) ([]DeviceInfo, error) {
	switch connType {
	case ConnectionBLE:
		devices, err := m.bleScanner.Scan(ctx, timeout)
		if err != nil {
			return nil, err
		}
		// Convert to radio.DeviceInfo
		result := make([]DeviceInfo, len(devices))
		for i, d := range devices {
			result[i] = DeviceInfo{
				Type:       ConnectionType(d.Type),
				Address:    d.Address,
				Name:       d.Name,
				RSSI:       d.RSSI,
				MacAddress: d.MacAddress,
			}
		}
		return result, nil
	case ConnectionSerial:
		devices, err := m.serialScanner.Scan(ctx, timeout)
		if err != nil {
			return nil, err
		}
		// Convert to radio.DeviceInfo
		result := make([]DeviceInfo, len(devices))
		for i, d := range devices {
			result[i] = DeviceInfo{
				Type:    ConnectionType(d.Type),
				Address: d.Address,
				Name:    d.Name,
			}
		}
		return result, nil
	case ConnectionTCP:
		devices, err := m.tcpScanner.Scan(ctx, timeout)
		if err != nil {
			return nil, err
		}
		// Convert to radio.DeviceInfo
		result := make([]DeviceInfo, len(devices))
		for i, d := range devices {
			result[i] = DeviceInfo{
				Type:    ConnectionType(d.Type),
				Address: d.Address,
				Name:    d.Name,
			}
		}
		return result, nil
	default:
		return nil, fmt.Errorf("unknown connection type: %s", connType)
	}
}

func (m *Manager) startHeartbeat() {
	m.stopHeartbeat()
	m.heartbeatStop = make(chan struct{})

	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				m.mu.RLock()
				if m.current != nil {
					if err := m.current.KeepAlive(); err != nil {
						log.Warn().Err(err).Msg("keepalive failed")
					}
				}
				m.mu.RUnlock()
			case <-m.heartbeatStop:
				return
			}
		}
	}()
}

func (m *Manager) stopHeartbeat() {
	if m.heartbeatStop != nil {
		close(m.heartbeatStop)
		m.heartbeatStop = nil
	}
}

// Connection factory methods
func (m *Manager) connectSerial(ctx context.Context, address string) (IRadioInterface, error) {
	iface, err := serial.NewSerialInterface(m, address)
	if err != nil {
		return nil, err
	}
	if err := iface.Connect(ctx); err != nil {
		return nil, err
	}
	return iface, nil
}

func (m *Manager) connectTCP(ctx context.Context, address string) (IRadioInterface, error) {
	iface, err := tcp.NewTCPInterface(m, address)
	if err != nil {
		return nil, err
	}
	if err := iface.Connect(ctx); err != nil {
		return nil, err
	}
	return iface, nil
}

func (m *Manager) connectBLE(ctx context.Context, address string) (IRadioInterface, error) {
	iface, err := ble.NewBLEInterface(m, address)
	if err != nil {
		return nil, err
	}
	if err := iface.Connect(ctx); err != nil {
		return nil, err
	}
	return iface, nil
}

// CancelScan cancels any ongoing device scan
func (m *Manager) CancelScan() {
	if m.bleScanner != nil {
		m.bleScanner.Cancel()
	}
}

// ScanWithCallback discovers devices and calls the callback for each device found
func (m *Manager) ScanWithCallback(ctx context.Context, connType ConnectionType, timeout time.Duration, onDeviceFound DeviceFoundCallback) ([]DeviceInfo, error) {
	switch connType {
	case ConnectionBLE:
		// Wrap the callback to convert ble.DeviceInfo to radio.DeviceInfo
		var bleCallback ble.DeviceFoundCallback
		if onDeviceFound != nil {
			bleCallback = func(d ble.DeviceInfo) {
				onDeviceFound(DeviceInfo{
					Type:       ConnectionType(d.Type),
					Address:    d.Address,
					Name:       d.Name,
					RSSI:       d.RSSI,
					MacAddress: d.MacAddress,
				})
			}
		}
		devices, err := m.bleScanner.ScanWithCallback(ctx, timeout, bleCallback)
		if err != nil {
			return nil, err
		}
		// Convert to radio.DeviceInfo
		result := make([]DeviceInfo, len(devices))
		for i, d := range devices {
			result[i] = DeviceInfo{
				Type:       ConnectionType(d.Type),
				Address:    d.Address,
				Name:       d.Name,
				RSSI:       d.RSSI,
				MacAddress: d.MacAddress,
			}
		}
		return result, nil
	default:
		// For other types, fall back to regular scan (they're usually fast enough)
		return m.Scan(ctx, connType, timeout)
	}
}

// OnConnect is called when an interface connects
func (m *Manager) OnConnect() {
	m.mu.RLock()
	callbacks := m.callbacks
	m.mu.RUnlock()

	if callbacks != nil {
		callbacks.OnConnect()
	}
}

// OnDisconnect is called when an interface disconnects
func (m *Manager) OnDisconnect(isPermanent bool, err error) {
	m.mu.Lock()
	m.state = Disconnected
	m.mu.Unlock()

	m.mu.RLock()
	callbacks := m.callbacks
	m.mu.RUnlock()

	if callbacks != nil {
		callbacks.OnDisconnect(isPermanent, err)
	}
}

// HandleFromRadio is called when data is received from the radio
func (m *Manager) HandleFromRadio(data []byte) {
	m.mu.RLock()
	callbacks := m.callbacks
	m.mu.RUnlock()

	if callbacks != nil {
		callbacks.HandleFromRadio(data)
	}
}
