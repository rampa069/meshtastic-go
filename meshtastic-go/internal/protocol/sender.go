package protocol

import (
	"fmt"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"

	"github.com/meshtastic/meshtastic-go/pkg/pb"
	"github.com/rs/zerolog/log"
)

// RadioSender is a function that sends data to the radio
type RadioSender func(data []byte) error

// Sender handles sending commands to the radio
type Sender struct {
	mu         sync.Mutex
	send       RadioSender
	myNodeNum  uint32
	packetId   uint32
	configId   uint32
}

// NewSender creates a new command sender
func NewSender(sendFunc RadioSender) *Sender {
	return &Sender{
		send:     sendFunc,
		packetId: uint32(rand.Int31()),
	}
}

// SetMyNodeNum sets the local node number
func (s *Sender) SetMyNodeNum(num uint32) {
	atomic.StoreUint32(&s.myNodeNum, num)
}

// GetMyNodeNum returns the local node number
func (s *Sender) GetMyNodeNum() uint32 {
	return atomic.LoadUint32(&s.myNodeNum)
}

// nextPacketId returns the next packet ID
func (s *Sender) nextPacketId() uint32 {
	return atomic.AddUint32(&s.packetId, 1)
}

// RequestConfig sends a request for the device configuration
func (s *Sender) RequestConfig() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.configId = uint32(rand.Int31())

	toRadio := &pb.ToRadio{
		PayloadVariant: &pb.ToRadio_WantConfigId{
			WantConfigId: s.configId,
		},
	}

	data, err := pb.MarshalToRadio(toRadio)
	if err != nil {
		return fmt.Errorf("failed to marshal ToRadio: %w", err)
	}

	log.Debug().
		Uint32("configId", s.configId).
		Msg("requesting config")

	return s.sendFramed(data)
}

// SendHeartbeat sends a heartbeat to the radio
func (s *Sender) SendHeartbeat() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	toRadio := &pb.ToRadio{
		PayloadVariant: &pb.ToRadio_Heartbeat{
			Heartbeat: &pb.Heartbeat{},
		},
	}

	data, err := pb.MarshalToRadio(toRadio)
	if err != nil {
		return fmt.Errorf("failed to marshal heartbeat: %w", err)
	}

	return s.sendFramed(data)
}

// SendDisconnect sends a disconnect message to the radio
func (s *Sender) SendDisconnect() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	toRadio := &pb.ToRadio{
		PayloadVariant: &pb.ToRadio_Disconnect{
			Disconnect: true,
		},
	}

	data, err := pb.MarshalToRadio(toRadio)
	if err != nil {
		return fmt.Errorf("failed to marshal disconnect: %w", err)
	}

	return s.sendFramed(data)
}

// SendTextMessage sends a text message to a specific node or broadcast
func (s *Sender) SendTextMessage(to uint32, channel uint32, text string, wantAck bool) (uint32, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	packetId := s.nextPacketId()
	myNodeNum := atomic.LoadUint32(&s.myNodeNum)

	if myNodeNum == 0 {
		return 0, fmt.Errorf("myNodeNum not set - not properly connected")
	}

	packet := &pb.MeshPacket{
		From:     myNodeNum,
		To:       to,
		Channel:  channel,
		Id:       packetId,
		WantAck:  wantAck,
		HopLimit: 3,
		HopStart: 3,
		PayloadVariant: &pb.MeshPacket_Decoded{
			Decoded: &pb.Data{
				Portnum: pb.PortNum_TEXT_MESSAGE_APP,
				Payload: []byte(text),
			},
		},
	}

	toRadio := &pb.ToRadio{
		PayloadVariant: &pb.ToRadio_Packet{
			Packet: packet,
		},
	}

	data, err := pb.MarshalToRadio(toRadio)
	if err != nil {
		return 0, fmt.Errorf("failed to marshal text message: %w", err)
	}

	log.Info().
		Uint32("from", myNodeNum).
		Uint32("to", to).
		Uint32("channel", channel).
		Uint32("packetId", packetId).
		Int("dataLen", len(data)).
		Hex("dataHex", data).
		Str("text", text).
		Msg("sending text message")

	if err := s.sendFramed(data); err != nil {
		return 0, err
	}

	return packetId, nil
}

// SendPosition sends a position update
func (s *Sender) SendPosition(lat, lon float64, altitude int32) (uint32, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	packetId := s.nextPacketId()
	myNodeNum := atomic.LoadUint32(&s.myNodeNum)

	position := &pb.Position{
		Time: uint32(time.Now().Unix()),
	}
	position.SetLatitude(lat)
	position.SetLongitude(lon)
	position.Altitude = altitude

	posData, err := pb.MarshalPosition(position)
	if err != nil {
		return 0, fmt.Errorf("failed to marshal position: %w", err)
	}

	packet := &pb.MeshPacket{
		From:     myNodeNum,
		To:       0xFFFFFFFF, // Broadcast
		Channel:  0,
		Id:       packetId,
		HopLimit: 3,
		HopStart: 3,
		PayloadVariant: &pb.MeshPacket_Decoded{
			Decoded: &pb.Data{
				Portnum: pb.PortNum_POSITION_APP,
				Payload: posData,
			},
		},
	}

	toRadio := &pb.ToRadio{
		PayloadVariant: &pb.ToRadio_Packet{
			Packet: packet,
		},
	}

	data, err := pb.MarshalToRadio(toRadio)
	if err != nil {
		return 0, fmt.Errorf("failed to marshal position packet: %w", err)
	}

	log.Info().
		Float64("lat", lat).
		Float64("lon", lon).
		Int32("alt", altitude).
		Msg("sending position")

	if err := s.sendFramed(data); err != nil {
		return 0, err
	}

	return packetId, nil
}

// SendTraceroute initiates a traceroute to a node
func (s *Sender) SendTraceroute(destNode uint32) (uint32, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	packetId := s.nextPacketId()
	myNodeNum := atomic.LoadUint32(&s.myNodeNum)

	// Create a RouteDiscovery message with our node in the route
	routeDiscovery := &pb.RouteDiscovery{
		Route: []uint32{myNodeNum},
	}

	// Marshal it manually (simple encoding)
	var routeData []byte
	tmp := make([]byte, 10)
	for _, nodeId := range routeDiscovery.Route {
		n := pb.EncodeVarint(tmp, pb.EncodeTag(1, pb.WireFixed32))
		routeData = append(routeData, tmp[:n]...)
		b := make([]byte, 4)
		b[0] = byte(nodeId)
		b[1] = byte(nodeId >> 8)
		b[2] = byte(nodeId >> 16)
		b[3] = byte(nodeId >> 24)
		routeData = append(routeData, b...)
	}

	packet := &pb.MeshPacket{
		From:     myNodeNum,
		To:       destNode,
		Channel:  0,
		Id:       packetId,
		WantAck:  true,
		HopLimit: 7,
		HopStart: 7,
		PayloadVariant: &pb.MeshPacket_Decoded{
			Decoded: &pb.Data{
				Portnum:      pb.PortNum_TRACEROUTE_APP,
				Payload:      routeData,
				WantResponse: true,
			},
		},
	}

	toRadio := &pb.ToRadio{
		PayloadVariant: &pb.ToRadio_Packet{
			Packet: packet,
		},
	}

	data, err := pb.MarshalToRadio(toRadio)
	if err != nil {
		return 0, fmt.Errorf("failed to marshal traceroute: %w", err)
	}

	log.Info().
		Uint32("dest", destNode).
		Uint32("packetId", packetId).
		Msg("sending traceroute request")

	if err := s.sendFramed(data); err != nil {
		return 0, err
	}

	return packetId, nil
}

// RequestPosition requests position from a remote node
func (s *Sender) RequestPosition(destNode uint32) (uint32, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	packetId := s.nextPacketId()
	myNodeNum := atomic.LoadUint32(&s.myNodeNum)

	packet := &pb.MeshPacket{
		From:     myNodeNum,
		To:       destNode,
		Channel:  0,
		Id:       packetId,
		WantAck:  true,
		HopLimit: 3,
		HopStart: 3,
		PayloadVariant: &pb.MeshPacket_Decoded{
			Decoded: &pb.Data{
				Portnum:      pb.PortNum_POSITION_APP,
				Payload:      []byte{}, // Empty payload to request position
				WantResponse: true,
			},
		},
	}

	toRadio := &pb.ToRadio{
		PayloadVariant: &pb.ToRadio_Packet{
			Packet: packet,
		},
	}

	data, err := pb.MarshalToRadio(toRadio)
	if err != nil {
		return 0, fmt.Errorf("failed to marshal position request: %w", err)
	}

	log.Info().
		Uint32("dest", destNode).
		Uint32("packetId", packetId).
		Msg("requesting position from node")

	if err := s.sendFramed(data); err != nil {
		return 0, err
	}

	return packetId, nil
}

// RequestNodeInfo requests node info from a remote node
func (s *Sender) RequestNodeInfo(destNode uint32) (uint32, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	packetId := s.nextPacketId()
	myNodeNum := atomic.LoadUint32(&s.myNodeNum)

	packet := &pb.MeshPacket{
		From:     myNodeNum,
		To:       destNode,
		Channel:  0,
		Id:       packetId,
		WantAck:  true,
		HopLimit: 3,
		HopStart: 3,
		PayloadVariant: &pb.MeshPacket_Decoded{
			Decoded: &pb.Data{
				Portnum:      pb.PortNum_NODEINFO_APP,
				Payload:      []byte{}, // Empty payload to request info
				WantResponse: true,
			},
		},
	}

	toRadio := &pb.ToRadio{
		PayloadVariant: &pb.ToRadio_Packet{
			Packet: packet,
		},
	}

	data, err := pb.MarshalToRadio(toRadio)
	if err != nil {
		return 0, fmt.Errorf("failed to marshal node info request: %w", err)
	}

	log.Info().
		Uint32("dest", destNode).
		Uint32("packetId", packetId).
		Msg("requesting node info")

	if err := s.sendFramed(data); err != nil {
		return 0, err
	}

	return packetId, nil
}

// RequestTelemetry requests telemetry from a remote node
func (s *Sender) RequestTelemetry(destNode uint32) (uint32, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	packetId := s.nextPacketId()
	myNodeNum := atomic.LoadUint32(&s.myNodeNum)

	packet := &pb.MeshPacket{
		From:     myNodeNum,
		To:       destNode,
		Channel:  0,
		Id:       packetId,
		WantAck:  true,
		HopLimit: 3,
		HopStart: 3,
		PayloadVariant: &pb.MeshPacket_Decoded{
			Decoded: &pb.Data{
				Portnum:      pb.PortNum_TELEMETRY_APP,
				Payload:      []byte{}, // Empty payload to request telemetry
				WantResponse: true,
			},
		},
	}

	toRadio := &pb.ToRadio{
		PayloadVariant: &pb.ToRadio_Packet{
			Packet: packet,
		},
	}

	data, err := pb.MarshalToRadio(toRadio)
	if err != nil {
		return 0, fmt.Errorf("failed to marshal telemetry request: %w", err)
	}

	log.Info().
		Uint32("dest", destNode).
		Uint32("packetId", packetId).
		Msg("requesting telemetry from node")

	if err := s.sendFramed(data); err != nil {
		return 0, err
	}

	return packetId, nil
}

// SetChannel sends a channel configuration to the local node
func (s *Sender) SetChannel(channel *pb.Channel) (uint32, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	myNodeNum := atomic.LoadUint32(&s.myNodeNum)
	if myNodeNum == 0 {
		return 0, fmt.Errorf("not connected")
	}

	// Create the AdminMessage with SetChannel
	adminMsg := &pb.AdminMessage{
		PayloadVariant: &pb.AdminMessage_SetChannel{
			SetChannel: channel,
		},
	}

	adminData, err := pb.MarshalAdminMessage(adminMsg)
	if err != nil {
		return 0, fmt.Errorf("failed to marshal admin message: %w", err)
	}

	packetId := s.nextPacketId()

	packet := &pb.MeshPacket{
		From:     myNodeNum,
		To:       myNodeNum, // Send to self for local config changes
		Channel:  0,
		Id:       packetId,
		WantAck:  true,
		HopLimit: 3,
		HopStart: 3,
		PayloadVariant: &pb.MeshPacket_Decoded{
			Decoded: &pb.Data{
				Portnum:      pb.PortNum_ADMIN_APP,
				Payload:      adminData,
				WantResponse: true,
			},
		},
	}

	toRadio := &pb.ToRadio{
		PayloadVariant: &pb.ToRadio_Packet{
			Packet: packet,
		},
	}

	data, err := pb.MarshalToRadio(toRadio)
	if err != nil {
		return 0, fmt.Errorf("failed to marshal ToRadio: %w", err)
	}

	log.Info().
		Uint32("index", channel.Index).
		Str("role", channel.Role.String()).
		Msg("setting channel configuration")

	if err := s.sendFramed(data); err != nil {
		return 0, err
	}

	return packetId, nil
}

// SetConfig sends a configuration to the local node
func (s *Sender) SetConfig(config *pb.Config) (uint32, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	myNodeNum := atomic.LoadUint32(&s.myNodeNum)
	if myNodeNum == 0 {
		return 0, fmt.Errorf("not connected")
	}

	// Create the AdminMessage with SetConfig
	adminMsg := &pb.AdminMessage{
		PayloadVariant: &pb.AdminMessage_SetConfig{
			SetConfig: config,
		},
	}

	adminData, err := pb.MarshalAdminMessage(adminMsg)
	if err != nil {
		return 0, fmt.Errorf("failed to marshal admin message: %w", err)
	}

	packetId := s.nextPacketId()

	packet := &pb.MeshPacket{
		From:     myNodeNum,
		To:       myNodeNum, // Send to self for local config changes
		Channel:  0,
		Id:       packetId,
		WantAck:  true,
		HopLimit: 3,
		HopStart: 3,
		PayloadVariant: &pb.MeshPacket_Decoded{
			Decoded: &pb.Data{
				Portnum:      pb.PortNum_ADMIN_APP,
				Payload:      adminData,
				WantResponse: true,
			},
		},
	}

	toRadio := &pb.ToRadio{
		PayloadVariant: &pb.ToRadio_Packet{
			Packet: packet,
		},
	}

	data, err := pb.MarshalToRadio(toRadio)
	if err != nil {
		return 0, fmt.Errorf("failed to marshal ToRadio: %w", err)
	}

	log.Info().
		Uint32("packetId", packetId).
		Msg("setting device configuration")

	if err := s.sendFramed(data); err != nil {
		return 0, err
	}

	return packetId, nil
}

// SetModuleConfig sends a module configuration to the local node
func (s *Sender) SetModuleConfig(moduleConfig *pb.ModuleConfig) (uint32, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	myNodeNum := atomic.LoadUint32(&s.myNodeNum)
	if myNodeNum == 0 {
		return 0, fmt.Errorf("not connected")
	}

	// Create the AdminMessage with SetModuleConfig
	adminMsg := &pb.AdminMessage{
		PayloadVariant: &pb.AdminMessage_SetModuleConfig{
			SetModuleConfig: moduleConfig,
		},
	}

	adminData, err := pb.MarshalAdminMessage(adminMsg)
	if err != nil {
		return 0, fmt.Errorf("failed to marshal admin message: %w", err)
	}

	packetId := s.nextPacketId()

	packet := &pb.MeshPacket{
		From:     myNodeNum,
		To:       myNodeNum, // Send to self for local config changes
		Channel:  0,
		Id:       packetId,
		WantAck:  true,
		HopLimit: 3,
		HopStart: 3,
		PayloadVariant: &pb.MeshPacket_Decoded{
			Decoded: &pb.Data{
				Portnum:      pb.PortNum_ADMIN_APP,
				Payload:      adminData,
				WantResponse: true,
			},
		},
	}

	toRadio := &pb.ToRadio{
		PayloadVariant: &pb.ToRadio_Packet{
			Packet: packet,
		},
	}

	data, err := pb.MarshalToRadio(toRadio)
	if err != nil {
		return 0, fmt.Errorf("failed to marshal ToRadio: %w", err)
	}

	log.Info().
		Uint32("packetId", packetId).
		Msg("setting module configuration")

	if err := s.sendFramed(data); err != nil {
		return 0, err
	}

	return packetId, nil
}

// SendAdminMessage sends an admin message to a node
func (s *Sender) SendAdminMessage(destNode uint32, adminData []byte) (uint32, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	packetId := s.nextPacketId()
	myNodeNum := atomic.LoadUint32(&s.myNodeNum)

	packet := &pb.MeshPacket{
		From:     myNodeNum,
		To:       destNode,
		Channel:  0,
		Id:       packetId,
		WantAck:  true,
		HopLimit: 3,
		HopStart: 3,
		PayloadVariant: &pb.MeshPacket_Decoded{
			Decoded: &pb.Data{
				Portnum:      pb.PortNum_ADMIN_APP,
				Payload:      adminData,
				WantResponse: true,
			},
		},
	}

	toRadio := &pb.ToRadio{
		PayloadVariant: &pb.ToRadio_Packet{
			Packet: packet,
		},
	}

	data, err := pb.MarshalToRadio(toRadio)
	if err != nil {
		return 0, fmt.Errorf("failed to marshal admin message: %w", err)
	}

	log.Debug().
		Uint32("dest", destNode).
		Msg("sending admin message")

	if err := s.sendFramed(data); err != nil {
		return 0, err
	}

	return packetId, nil
}

// SendWaypoint sends a waypoint to the mesh
func (s *Sender) SendWaypoint(waypoint *pb.Waypoint, channel uint32) (uint32, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	packetId := s.nextPacketId()
	myNodeNum := atomic.LoadUint32(&s.myNodeNum)

	waypointData, err := pb.MarshalWaypoint(waypoint)
	if err != nil {
		return 0, fmt.Errorf("failed to marshal waypoint: %w", err)
	}

	packet := &pb.MeshPacket{
		From:     myNodeNum,
		To:       0xFFFFFFFF, // Broadcast
		Channel:  channel,
		Id:       packetId,
		HopLimit: 3,
		HopStart: 3,
		PayloadVariant: &pb.MeshPacket_Decoded{
			Decoded: &pb.Data{
				Portnum: pb.PortNum_WAYPOINT_APP,
				Payload: waypointData,
			},
		},
	}

	toRadio := &pb.ToRadio{
		PayloadVariant: &pb.ToRadio_Packet{
			Packet: packet,
		},
	}

	data, err := pb.MarshalToRadio(toRadio)
	if err != nil {
		return 0, fmt.Errorf("failed to marshal waypoint packet: %w", err)
	}

	log.Info().
		Uint32("waypointId", waypoint.Id).
		Str("name", waypoint.Name).
		Uint32("packetId", packetId).
		Msg("sending waypoint")

	if err := s.sendFramed(data); err != nil {
		return 0, err
	}

	return packetId, nil
}

// sendFramed sends data to the radio
// Note: The actual framing is done by the transport layer (HandleSendToRadio)
func (s *Sender) sendFramed(data []byte) error {
	return s.send(data)
}
