package tcp

import (
	"context"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/meshtastic/meshtastic-go/internal/protocol"
	"github.com/rs/zerolog/log"
)

const (
	DefaultPort       = 4403
	SocketTimeout     = 5 * time.Second
	MaxRetries        = 18
	MinBackoff        = 1 * time.Second
	MaxBackoff        = 5 * time.Minute
	ConnectionTypeTCP = "tcp"
)

// RadioCallbacks defines callbacks for radio events
type RadioCallbacks interface {
	OnConnect()
	OnDisconnect(isPermanent bool, err error)
	HandleFromRadio(data []byte)
}

// TCPInterface implements IRadioInterface for TCP connections
type TCPInterface struct {
	mu        sync.RWMutex
	address   string
	callbacks RadioCallbacks
	conn      net.Conn
	connected bool
	framer    *protocol.StreamFramer
	stopChan  chan struct{}
}

// NewTCPInterface creates a new TCP interface
func NewTCPInterface(callbacks RadioCallbacks, address string) (*TCPInterface, error) {
	// Add default port if not specified
	if !strings.Contains(address, ":") {
		address = address + ":" + strconv.Itoa(DefaultPort)
	}

	return &TCPInterface{
		address:   address,
		callbacks: callbacks,
		framer:    protocol.NewStreamFramer(),
		stopChan:  make(chan struct{}),
	}, nil
}

// Connect establishes a TCP connection
func (t *TCPInterface) Connect(ctx context.Context) error {
	log.Debug().Str("address", t.address).Msg("TCP connecting...")

	dialer := net.Dialer{Timeout: SocketTimeout}
	conn, err := dialer.DialContext(ctx, "tcp", t.address)
	if err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}

	log.Debug().Str("address", t.address).Msg("TCP dial successful")

	// Enable TCP keepalive
	if tcpConn, ok := conn.(*net.TCPConn); ok {
		tcpConn.SetKeepAlive(true)
		tcpConn.SetKeepAlivePeriod(30 * time.Second)
		tcpConn.SetNoDelay(true)
	}

	// Send wake sequence
	if _, err := conn.Write(protocol.WakeSequence); err != nil {
		conn.Close()
		return fmt.Errorf("failed to send wake sequence: %w", err)
	}

	log.Debug().Str("address", t.address).Msg("TCP wake sequence sent")

	// Update state with lock
	t.mu.Lock()
	t.conn = conn
	t.connected = true
	t.mu.Unlock()

	// Start read loop (outside lock)
	go t.readLoop()

	// Note: We do NOT call OnConnect here - the Manager will call it
	// after setting up m.current, so SendToRadio will work in the callback

	log.Info().Str("address", t.address).Msg("TCP connected")
	return nil
}

// HandleSendToRadio sends data to the radio via TCP
func (t *TCPInterface) HandleSendToRadio(data []byte) error {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if !t.connected || t.conn == nil {
		return fmt.Errorf("not connected")
	}

	frame, err := protocol.Frame(data)
	if err != nil {
		return err
	}

	t.conn.SetWriteDeadline(time.Now().Add(SocketTimeout))
	_, err = t.conn.Write(frame)
	return err
}

// KeepAlive performs a keepalive check
func (t *TCPInterface) KeepAlive() error {
	// TCP keepalive is handled by the OS
	// We could send a heartbeat packet here if needed
	return nil
}

// Close closes the TCP connection
func (t *TCPInterface) Close() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	select {
	case <-t.stopChan:
		// Already closed
	default:
		close(t.stopChan)
	}
	t.connected = false

	if t.conn != nil {
		err := t.conn.Close()
		t.conn = nil
		return err
	}
	return nil
}

// Type returns the connection type
func (t *TCPInterface) Type() string {
	return ConnectionTypeTCP
}

// Address returns the device address
func (t *TCPInterface) Address() string {
	return t.address
}

// IsConnected returns true if connected
func (t *TCPInterface) IsConnected() bool {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.connected
}

func (t *TCPInterface) readLoop() {
	buf := make([]byte, 1024)

	for {
		select {
		case <-t.stopChan:
			return
		default:
		}

		t.mu.RLock()
		conn := t.conn
		t.mu.RUnlock()

		if conn == nil {
			return
		}

		conn.SetReadDeadline(time.Now().Add(SocketTimeout))
		n, err := conn.Read(buf)
		if err != nil {
			if err == io.EOF || !t.IsConnected() {
				return
			}
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				continue // Timeout is ok, just retry
			}
			log.Error().Err(err).Msg("TCP read error")
			t.handleDisconnect(err)
			return
		}

		// Parse received data through framer
		packets := t.framer.ParseBytes(buf[:n])
		for _, packet := range packets {
			if t.callbacks != nil {
				t.callbacks.HandleFromRadio(packet)
			}
		}
	}
}

func (t *TCPInterface) handleDisconnect(err error) {
	t.mu.Lock()
	t.connected = false
	if t.conn != nil {
		t.conn.Close()
		t.conn = nil
	}
	t.mu.Unlock()

	if t.callbacks != nil {
		t.callbacks.OnDisconnect(false, err)
	}
}
