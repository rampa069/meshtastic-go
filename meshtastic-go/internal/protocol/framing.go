package protocol

import (
	"errors"
)

const (
	START1        byte = 0x94
	START2        byte = 0xc3
	MaxPacketSize      = 512
)

var (
	ErrPacketTooLarge = errors.New("packet too large")
	ErrInvalidFrame   = errors.New("invalid frame")
)

// WakeSequence is sent before the first command to wake a sleeping device
var WakeSequence = []byte{START1, START1, START1, START1}

// StreamFramer handles framing for stream-based interfaces (Serial, TCP)
type StreamFramer struct {
	buffer    []byte
	state     int // 0=start1, 1=start2, 2=msb, 3=lsb, 4+=payload
	msb       byte
	lsb       byte
	packetLen int
}

// NewStreamFramer creates a new stream framer
func NewStreamFramer() *StreamFramer {
	return &StreamFramer{
		buffer: make([]byte, 0, MaxPacketSize),
	}
}

// Frame wraps data in the Meshtastic stream protocol frame
// Format: [START1][START2][MSB_LEN][LSB_LEN][PAYLOAD...]
func Frame(data []byte) ([]byte, error) {
	if len(data) > MaxPacketSize {
		return nil, ErrPacketTooLarge
	}

	frame := make([]byte, 4+len(data))
	frame[0] = START1
	frame[1] = START2
	frame[2] = byte(len(data) >> 8)   // MSB
	frame[3] = byte(len(data) & 0xff) // LSB
	copy(frame[4:], data)

	return frame, nil
}

// Parse processes a single byte and returns a complete packet if available
func (f *StreamFramer) Parse(b byte) ([]byte, bool) {
	switch f.state {
	case 0: // Waiting for START1
		if b == START1 {
			f.state = 1
		}
		return nil, false

	case 1: // Waiting for START2
		if b == START2 {
			f.state = 2
		} else if b == START1 {
			// Stay in state 1 (consecutive START1 bytes are ok)
		} else {
			f.state = 0
		}
		return nil, false

	case 2: // MSB of length
		f.msb = b
		f.state = 3
		return nil, false

	case 3: // LSB of length
		f.lsb = b
		f.packetLen = (int(f.msb) << 8) | int(f.lsb)
		if f.packetLen > MaxPacketSize || f.packetLen == 0 {
			f.state = 0
			return nil, false
		}
		f.buffer = make([]byte, 0, f.packetLen)
		f.state = 4
		return nil, false

	default: // Collecting payload
		f.buffer = append(f.buffer, b)
		if len(f.buffer) >= f.packetLen {
			packet := make([]byte, len(f.buffer))
			copy(packet, f.buffer)
			f.state = 0
			f.buffer = f.buffer[:0]
			return packet, true
		}
		return nil, false
	}
}

// Reset resets the framer state
func (f *StreamFramer) Reset() {
	f.state = 0
	f.buffer = f.buffer[:0]
	f.msb = 0
	f.lsb = 0
	f.packetLen = 0
}

// ParseBytes processes multiple bytes and returns all complete packets
func (f *StreamFramer) ParseBytes(data []byte) [][]byte {
	var packets [][]byte
	for _, b := range data {
		if packet, ok := f.Parse(b); ok {
			packets = append(packets, packet)
		}
	}
	return packets
}
