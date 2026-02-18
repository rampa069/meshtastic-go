package ble

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
	"tinygo.org/x/bluetooth"
)

// DeviceInfo represents a discovered device
type DeviceInfo struct {
	Type       string `json:"type"`
	Address    string `json:"address"`
	Name       string `json:"name"`
	RSSI       int    `json:"rssi,omitempty"`
	MacAddress string `json:"mac_address,omitempty"`
}

// DeviceFoundCallback is called when a device is discovered during scanning
type DeviceFoundCallback func(device DeviceInfo)

// BLEScanner discovers Meshtastic devices via Bluetooth Low Energy
type BLEScanner struct {
	adapter        *bluetooth.Adapter
	adapterEnabled bool
	mu             sync.Mutex
	scanning       bool
	cancelled      bool
	cancelChan     chan struct{}
}

// NewBLEScanner creates a new BLE scanner
func NewBLEScanner() *BLEScanner {
	return &BLEScanner{
		adapter: bluetooth.DefaultAdapter,
	}
}

// Scan discovers BLE devices with the Meshtastic service UUID
func (s *BLEScanner) Scan(ctx context.Context, timeout time.Duration) ([]DeviceInfo, error) {
	return s.ScanWithCallback(ctx, timeout, nil)
}

// ScanWithCallback discovers BLE devices and calls the callback for each device found
func (s *BLEScanner) ScanWithCallback(ctx context.Context, timeout time.Duration, onDeviceFound DeviceFoundCallback) ([]DeviceInfo, error) {
	s.mu.Lock()
	if s.scanning {
		s.mu.Unlock()
		return nil, fmt.Errorf("scan already in progress")
	}
	s.scanning = true
	s.cancelled = false
	s.cancelChan = make(chan struct{})
	cancelChan := s.cancelChan

	// Enable adapter only once
	adapter := s.adapter
	if !s.adapterEnabled {
		s.mu.Unlock()
		if err := adapter.Enable(); err != nil {
			s.mu.Lock()
			s.scanning = false
			s.mu.Unlock()
			log.Error().Err(err).Msg("failed to enable BLE adapter")
			return nil, err
		}
		s.mu.Lock()
		s.adapterEnabled = true
	}
	s.mu.Unlock()

	log.Info().Dur("timeout", timeout).Msg("starting BLE scan")

	// Parse the Meshtastic service UUID
	serviceUUID, err := bluetooth.ParseUUID(ServiceUUID)
	if err != nil {
		s.mu.Lock()
		s.scanning = false
		s.mu.Unlock()
		log.Error().Err(err).Str("uuid", ServiceUUID).Msg("failed to parse service UUID")
		return nil, err
	}

	var devices []DeviceInfo
	var devicesMu sync.Mutex
	seen := make(map[string]bool)

	// Channel to signal scan completion
	scanDone := make(chan error, 1)

	// Start scanning in a goroutine (adapter.Scan is blocking)
	go func() {
		err := adapter.Scan(func(adapter *bluetooth.Adapter, result bluetooth.ScanResult) {
			// Check if device has our service UUID or has Meshtastic in name
			hasMeshtasticService := false
			for _, uuid := range result.AdvertisementPayload.ServiceUUIDs() {
				if uuid == serviceUUID {
					hasMeshtasticService = true
					break
				}
			}

			name := result.LocalName()
			isMeshtastic := hasMeshtasticService || strings.Contains(strings.ToLower(name), "meshtastic")

			if !isMeshtastic {
				return
			}

			addr := result.Address.String()

			devicesMu.Lock()
			if seen[addr] {
				devicesMu.Unlock()
				return
			}
			seen[addr] = true
			devicesMu.Unlock()

			device := DeviceInfo{
				Type:       "ble",
				Address:    addr,
				Name:       name,
				RSSI:       int(result.RSSI),
				MacAddress: addr,
			}

			log.Info().
				Str("address", addr).
				Str("name", name).
				Int("rssi", int(result.RSSI)).
				Msg("found Meshtastic BLE device")

			// Call callback immediately when device is found
			if onDeviceFound != nil {
				onDeviceFound(device)
			}

			devicesMu.Lock()
			devices = append(devices, device)
			devicesMu.Unlock()
		})
		scanDone <- err
	}()

	// Wait for timeout, context cancellation, or cancel request
	select {
	case <-ctx.Done():
		log.Info().Msg("BLE scan context done")
	case <-cancelChan:
		log.Info().Msg("BLE scan cancelled by user")
	case <-time.After(timeout):
		log.Info().Msg("BLE scan timeout")
	case err := <-scanDone:
		// Scan finished on its own (error or adapter stopped)
		if err != nil {
			log.Warn().Err(err).Msg("BLE scan ended with error")
		}
	}

	// Stop scanning
	log.Debug().Msg("stopping BLE scan")
	if err := adapter.StopScan(); err != nil {
		log.Warn().Err(err).Msg("failed to stop BLE scan")
	}

	// Wait for scan goroutine to finish (with timeout)
	select {
	case <-scanDone:
		// Scan goroutine finished
	case <-time.After(1 * time.Second):
		log.Warn().Msg("scan goroutine did not finish in time")
	}

	// Mark as not scanning
	s.mu.Lock()
	s.scanning = false
	s.cancelled = false
	s.cancelChan = nil
	s.mu.Unlock()

	log.Info().Int("count", len(devices)).Msg("BLE scan complete")
	return devices, nil
}

// Cancel stops an ongoing scan and waits for it to complete
func (s *BLEScanner) Cancel() {
	s.mu.Lock()
	wasScanning := s.scanning
	adapter := s.adapter
	if s.scanning && !s.cancelled && s.cancelChan != nil {
		s.cancelled = true
		close(s.cancelChan)
		log.Info().Msg("BLE scan cancel requested")
	}
	s.mu.Unlock()

	if !wasScanning {
		return
	}

	// Stop the adapter scan directly - this is what actually stops the blocking Scan() call
	if adapter != nil {
		log.Debug().Msg("calling adapter.StopScan()")
		if err := adapter.StopScan(); err != nil {
			log.Warn().Err(err).Msg("failed to stop BLE scan from Cancel()")
		}
	}

	// Wait for scan to actually stop (up to 2 seconds)
	for i := 0; i < 40; i++ {
		s.mu.Lock()
		stillScanning := s.scanning
		s.mu.Unlock()
		if !stillScanning {
			log.Info().Msg("BLE scan stopped successfully")
			return
		}
		time.Sleep(50 * time.Millisecond)
	}

	log.Warn().Msg("BLE scan did not stop within timeout, proceeding anyway")
}

// IsScanning returns true if a scan is in progress
func (s *BLEScanner) IsScanning() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.scanning
}
