package ble

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
	"tinygo.org/x/bluetooth"
)

// ConnectionType for BLE
const ConnectionTypeBLE = "ble"

// RadioCallbacks defines callbacks for radio events
type RadioCallbacks interface {
	OnConnect()
	OnDisconnect(isPermanent bool, err error)
	HandleFromRadio(data []byte)
}

// BLEInterface implements IRadioInterface for Bluetooth Low Energy
type BLEInterface struct {
	mu        sync.RWMutex
	address   string
	callbacks RadioCallbacks
	connected bool

	adapter    *bluetooth.Adapter
	device     bluetooth.Device
	toRadio    bluetooth.DeviceCharacteristic
	fromRadio  bluetooth.DeviceCharacteristic
	fromNum    bluetooth.DeviceCharacteristic

	stopChan chan struct{}
}

// NewBLEInterface creates a new BLE interface
func NewBLEInterface(callbacks RadioCallbacks, address string) (*BLEInterface, error) {
	return &BLEInterface{
		address:   address,
		callbacks: callbacks,
		stopChan:  make(chan struct{}),
	}, nil
}

// Connect establishes a BLE connection
func (b *BLEInterface) Connect(ctx context.Context) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.connected {
		return fmt.Errorf("already connected")
	}

	log.Info().Str("address", b.address).Msg("connecting to BLE device")

	// Get the default adapter
	b.adapter = bluetooth.DefaultAdapter

	// Try to enable the adapter - it may already be enabled by the scanner
	if err := b.adapter.Enable(); err != nil {
		// "already calling Enable function" means it's already enabled, which is fine
		if err.Error() != "already calling Enable function" {
			log.Warn().Err(err).Msg("failed to enable BLE adapter")
			return fmt.Errorf("failed to enable BLE adapter: %w", err)
		}
		log.Debug().Msg("BLE adapter already enabled")
	}

	// On macOS, we always need to scan to find the device
	// because addresses are UUIDs, not MAC addresses
	return b.connectByScanning(ctx)
}

// connectByScanning finds and connects to a device by scanning
func (b *BLEInterface) connectByScanning(ctx context.Context) error {
	log.Info().Str("address", b.address).Msg("scanning for device")

	var foundDevice bluetooth.ScanResult
	found := false

	scanCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	err := b.adapter.Scan(func(adapter *bluetooth.Adapter, result bluetooth.ScanResult) {
		addr := result.Address.String()
		name := result.LocalName()

		// Match by address or name
		if addr == b.address || name == b.address {
			foundDevice = result
			found = true
			adapter.StopScan()
		}
	})

	if err != nil {
		return fmt.Errorf("failed to scan: %w", err)
	}

	// Wait for scan to find device or timeout
	select {
	case <-scanCtx.Done():
		b.adapter.StopScan()
	case <-time.After(10 * time.Second):
		b.adapter.StopScan()
	}

	if !found {
		return fmt.Errorf("device not found: %s", b.address)
	}

	log.Info().
		Str("address", foundDevice.Address.String()).
		Str("name", foundDevice.LocalName()).
		Msg("found device, connecting")

	// Connect to the found device
	device, err := b.adapter.Connect(foundDevice.Address, bluetooth.ConnectionParams{})
	if err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}

	return b.setupDevice(device)
}

// setupDevice discovers services and sets up characteristics
func (b *BLEInterface) setupDevice(device bluetooth.Device) error {
	b.device = device

	log.Info().Msg("discovering services")

	// Discover services
	services, err := device.DiscoverServices(nil)
	if err != nil {
		device.Disconnect()
		return fmt.Errorf("failed to discover services: %w", err)
	}

	// Parse UUIDs
	serviceUUID, _ := bluetooth.ParseUUID(ServiceUUID)
	toRadioUUID, _ := bluetooth.ParseUUID(ToRadioUUID)
	fromRadioUUID, _ := bluetooth.ParseUUID(FromRadioUUID)
	fromNumUUID, _ := bluetooth.ParseUUID(FromNumUUID)

	// Find Meshtastic service
	var meshtasticService bluetooth.DeviceService
	foundService := false
	for _, service := range services {
		if service.UUID() == serviceUUID {
			meshtasticService = service
			foundService = true
			break
		}
	}

	if !foundService {
		device.Disconnect()
		return fmt.Errorf("Meshtastic service not found")
	}

	log.Info().Msg("found Meshtastic service, discovering characteristics")

	// Discover characteristics
	chars, err := meshtasticService.DiscoverCharacteristics(nil)
	if err != nil {
		device.Disconnect()
		return fmt.Errorf("failed to discover characteristics: %w", err)
	}

	// Find required characteristics
	foundToRadio := false
	foundFromRadio := false
	foundFromNum := false

	for _, char := range chars {
		uuid := char.UUID()
		if uuid == toRadioUUID {
			b.toRadio = char
			foundToRadio = true
			log.Debug().Msg("found ToRadio characteristic")
		} else if uuid == fromRadioUUID {
			b.fromRadio = char
			foundFromRadio = true
			log.Debug().Msg("found FromRadio characteristic")
		} else if uuid == fromNumUUID {
			b.fromNum = char
			foundFromNum = true
			log.Debug().Msg("found FromNum characteristic")
		}
	}

	if !foundToRadio || !foundFromRadio || !foundFromNum {
		device.Disconnect()
		return fmt.Errorf("missing required characteristics: toRadio=%v, fromRadio=%v, fromNum=%v",
			foundToRadio, foundFromRadio, foundFromNum)
	}

	// Enable notifications on FromNum
	log.Info().Msg("enabling notifications")
	err = b.fromNum.EnableNotifications(b.handleFromNumNotification)
	if err != nil {
		log.Warn().Err(err).Msg("failed to enable FromNum notifications, will use polling")
	}

	b.connected = true
	b.stopChan = make(chan struct{})

	// Start read loop
	go b.readLoop()

	log.Info().Str("address", b.address).Msg("BLE connected")

	if b.callbacks != nil {
		b.callbacks.OnConnect()
	}

	return nil
}

// handleFromNumNotification is called when FromNum changes (data available)
func (b *BLEInterface) handleFromNumNotification(buf []byte) {
	// FromNum notification means data is available to read
	b.readFromRadio()
}

// readLoop continuously reads from the radio
func (b *BLEInterface) readLoop() {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-b.stopChan:
			return
		case <-ticker.C:
			b.readFromRadio()
		}
	}
}

// readFromRadio reads available data from the FromRadio characteristic
func (b *BLEInterface) readFromRadio() {
	b.mu.RLock()
	if !b.connected {
		b.mu.RUnlock()
		return
	}
	b.mu.RUnlock()

	// Read from FromRadio characteristic
	buf := make([]byte, 512)
	n, err := b.fromRadio.Read(buf)
	if err != nil {
		// Read errors are common when no data is available
		return
	}

	if n == 0 {
		return
	}

	data := buf[:n]
	log.Debug().Int("bytes", n).Msg("received BLE data")

	if b.callbacks != nil {
		b.callbacks.HandleFromRadio(data)
	}
}

// HandleSendToRadio sends data to the radio via BLE
func (b *BLEInterface) HandleSendToRadio(data []byte) error {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if !b.connected {
		return fmt.Errorf("not connected")
	}

	log.Debug().Int("bytes", len(data)).Msg("sending BLE data")

	// Write to ToRadio characteristic
	_, err := b.toRadio.WriteWithoutResponse(data)
	if err != nil {
		return fmt.Errorf("failed to write to ToRadio: %w", err)
	}

	return nil
}

// KeepAlive performs a keepalive check
func (b *BLEInterface) KeepAlive() error {
	// BLE uses connection supervision timeout, no explicit keepalive needed
	return nil
}

// Close closes the BLE connection
func (b *BLEInterface) Close() error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if !b.connected {
		return nil
	}

	log.Info().Msg("closing BLE connection")

	// Stop read loop
	close(b.stopChan)

	// Disconnect
	b.device.Disconnect()
	b.connected = false

	if b.callbacks != nil {
		b.callbacks.OnDisconnect(true, nil)
	}

	return nil
}

// Type returns the connection type
func (b *BLEInterface) Type() string {
	return ConnectionTypeBLE
}

// Address returns the device address
func (b *BLEInterface) Address() string {
	return b.address
}

// IsConnected returns true if connected
func (b *BLEInterface) IsConnected() bool {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.connected
}
