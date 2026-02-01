package serial

import (
	"context"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/meshtastic/meshtastic-go/internal/protocol"
	"github.com/rs/zerolog/log"
	"go.bug.st/serial"
)

const (
	BaudRate           = 115200
	ConnectionTypeSerial = "serial"
)

// RadioCallbacks defines callbacks for radio events
type RadioCallbacks interface {
	OnConnect()
	OnDisconnect(isPermanent bool, err error)
	HandleFromRadio(data []byte)
}

// SerialInterface implements IRadioInterface for USB Serial connections
type SerialInterface struct {
	mu        sync.RWMutex
	address   string
	callbacks RadioCallbacks
	port      serial.Port
	connected bool
	framer    *protocol.StreamFramer
	stopChan  chan struct{}
}

// NewSerialInterface creates a new Serial interface
func NewSerialInterface(callbacks RadioCallbacks, address string) (*SerialInterface, error) {
	return &SerialInterface{
		address:   address,
		callbacks: callbacks,
		framer:    protocol.NewStreamFramer(),
		stopChan:  make(chan struct{}),
	}, nil
}

// Connect establishes a serial connection
func (s *SerialInterface) Connect(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	mode := &serial.Mode{
		BaudRate: BaudRate,
		DataBits: 8,
		Parity:   serial.NoParity,
		StopBits: serial.OneStopBit,
	}

	port, err := serial.Open(s.address, mode)
	if err != nil {
		return fmt.Errorf("failed to open serial port: %w", err)
	}

	s.port = port
	s.connected = true

	// Send wake sequence
	if _, err := port.Write(protocol.WakeSequence); err != nil {
		port.Close()
		s.connected = false
		return fmt.Errorf("failed to send wake sequence: %w", err)
	}

	// Start read loop
	go s.readLoop()

	// Note: We do NOT call OnConnect here - the Manager will call it
	// after setting up m.current, so SendToRadio will work in the callback

	log.Info().Str("address", s.address).Msg("Serial connected")
	return nil
}

// HandleSendToRadio sends data to the radio via Serial
func (s *SerialInterface) HandleSendToRadio(data []byte) error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if !s.connected || s.port == nil {
		return fmt.Errorf("not connected")
	}

	frame, err := protocol.Frame(data)
	if err != nil {
		return err
	}

	_, err = s.port.Write(frame)
	return err
}

// KeepAlive sends a heartbeat packet
func (s *SerialInterface) KeepAlive() error {
	// TODO: Send ToRadio.heartbeat protobuf
	return nil
}

// Close closes the serial connection
func (s *SerialInterface) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	select {
	case <-s.stopChan:
		// Already closed
	default:
		close(s.stopChan)
	}
	s.connected = false

	if s.port != nil {
		err := s.port.Close()
		s.port = nil
		return err
	}
	return nil
}

// Type returns the connection type
func (s *SerialInterface) Type() string {
	return ConnectionTypeSerial
}

// Address returns the device address
func (s *SerialInterface) Address() string {
	return s.address
}

// IsConnected returns true if connected
func (s *SerialInterface) IsConnected() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.connected
}

func (s *SerialInterface) readLoop() {
	buf := make([]byte, 256)

	for {
		select {
		case <-s.stopChan:
			return
		default:
		}

		s.mu.RLock()
		port := s.port
		s.mu.RUnlock()

		if port == nil {
			return
		}

		// Set read timeout
		port.SetReadTimeout(100 * time.Millisecond)

		n, err := port.Read(buf)
		if err != nil {
			if err == io.EOF || !s.IsConnected() {
				return
			}
			// Timeout is ok, just retry
			continue
		}

		if n == 0 {
			continue
		}

		// Parse received data through framer
		packets := s.framer.ParseBytes(buf[:n])
		for _, packet := range packets {
			if s.callbacks != nil {
				s.callbacks.HandleFromRadio(packet)
			}
		}
	}
}
