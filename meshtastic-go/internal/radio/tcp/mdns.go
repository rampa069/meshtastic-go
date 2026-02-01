package tcp

import (
	"context"
	"fmt"
	"time"

	"github.com/grandcat/zeroconf"
	"github.com/rs/zerolog/log"
)

const (
	// MeshtasticServiceType is the mDNS service type for Meshtastic devices
	MeshtasticServiceType = "_meshtastic._tcp"
)

// DeviceInfo represents a discovered device
type DeviceInfo struct {
	Type       string `json:"type"`
	Address    string `json:"address"`
	Name       string `json:"name"`
	RSSI       int    `json:"rssi,omitempty"`
	MacAddress string `json:"mac_address,omitempty"`
}

// MDNSScanner discovers Meshtastic devices via mDNS/Bonjour
type MDNSScanner struct{}

// NewMDNSScanner creates a new mDNS scanner
func NewMDNSScanner() *MDNSScanner {
	return &MDNSScanner{}
}

// Scan discovers Meshtastic devices on the local network
func (s *MDNSScanner) Scan(ctx context.Context, timeout time.Duration) ([]DeviceInfo, error) {
	resolver, err := zeroconf.NewResolver(nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create mDNS resolver: %w", err)
	}

	entries := make(chan *zeroconf.ServiceEntry, 10)
	devices := make([]DeviceInfo, 0)

	// Create timeout context
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Start browsing
	err = resolver.Browse(ctx, MeshtasticServiceType, "local.", entries)
	if err != nil {
		return nil, fmt.Errorf("failed to browse mDNS: %w", err)
	}

	// Collect results
	for {
		select {
		case entry := <-entries:
			if entry == nil {
				continue
			}

			// Get the first IPv4 address
			var addr string
			for _, ip := range entry.AddrIPv4 {
				addr = ip.String()
				break
			}
			if addr == "" {
				for _, ip := range entry.AddrIPv6 {
					addr = fmt.Sprintf("[%s]", ip.String())
					break
				}
			}

			if addr != "" {
				device := DeviceInfo{
					Type:    ConnectionTypeTCP,
					Address: fmt.Sprintf("%s:%d", addr, entry.Port),
					Name:    entry.Instance,
				}
				devices = append(devices, device)
				log.Debug().
					Str("name", entry.Instance).
					Str("address", device.Address).
					Msg("discovered Meshtastic device via mDNS")
			}

		case <-ctx.Done():
			return devices, nil
		}
	}
}
