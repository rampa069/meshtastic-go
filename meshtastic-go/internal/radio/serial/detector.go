package serial

import (
	"context"
	"runtime"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
	"go.bug.st/serial"
)

// DeviceInfo represents a discovered device
type DeviceInfo struct {
	Type       string `json:"type"`
	Address    string `json:"address"`
	Name       string `json:"name"`
	RSSI       int    `json:"rssi,omitempty"`
	MacAddress string `json:"mac_address,omitempty"`
}

// SerialScanner discovers serial ports
type SerialScanner struct{}

// NewSerialScanner creates a new serial port scanner
func NewSerialScanner() *SerialScanner {
	return &SerialScanner{}
}

// Scan lists available serial ports
func (s *SerialScanner) Scan(ctx context.Context, timeout time.Duration) ([]DeviceInfo, error) {
	ports, err := serial.GetPortsList()
	if err != nil {
		return nil, err
	}

	devices := make([]DeviceInfo, 0)

	for _, port := range ports {
		// Filter by platform-specific patterns
		if !isLikelyMeshtastic(port) {
			continue
		}

		device := DeviceInfo{
			Type:    ConnectionTypeSerial,
			Address: port,
			Name:    guessDeviceName(port),
		}
		devices = append(devices, device)
		log.Debug().Str("port", port).Msg("discovered serial port")
	}

	return devices, nil
}

// isLikelyMeshtastic checks if a port is likely a Meshtastic device
func isLikelyMeshtastic(port string) bool {
	switch runtime.GOOS {
	case "darwin":
		// macOS: /dev/cu.usbserial-*, /dev/cu.usbmodem*, /dev/cu.SLAB_USBtoUART
		if strings.HasPrefix(port, "/dev/cu.usb") ||
			strings.HasPrefix(port, "/dev/cu.SLAB") ||
			strings.HasPrefix(port, "/dev/cu.wchusbserial") {
			return true
		}
	case "linux":
		// Linux: /dev/ttyUSB*, /dev/ttyACM*
		if strings.HasPrefix(port, "/dev/ttyUSB") ||
			strings.HasPrefix(port, "/dev/ttyACM") {
			return true
		}
	case "windows":
		// Windows: COM*
		if strings.HasPrefix(port, "COM") {
			return true
		}
	}
	return false
}

// guessDeviceName tries to identify the device type from the port name
func guessDeviceName(port string) string {
	portLower := strings.ToLower(port)

	if strings.Contains(portLower, "ch340") || strings.Contains(portLower, "wchusbserial") {
		return "Meshtastic (CH340)"
	}
	if strings.Contains(portLower, "cp210") || strings.Contains(portLower, "slab") {
		return "Meshtastic (CP210x)"
	}
	if strings.Contains(portLower, "ftdi") || strings.Contains(portLower, "ft232") {
		return "Meshtastic (FTDI)"
	}
	if strings.Contains(portLower, "acm") {
		return "Meshtastic (CDC ACM)"
	}
	if strings.Contains(portLower, "usbmodem") {
		return "Meshtastic (USB Modem)"
	}

	return "Meshtastic Device"
}
