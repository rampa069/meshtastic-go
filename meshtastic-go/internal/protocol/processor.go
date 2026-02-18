package protocol

import (
	"fmt"
	"sync"
	"time"

	"github.com/meshtastic/meshtastic-go/pkg/pb"
	"github.com/rs/zerolog/log"
)

// PacketHandler is called when a packet is received
type PacketHandler func(packet *pb.MeshPacket)

// NodeHandler is called when node info is received
type NodeHandler func(nodeInfo *pb.NodeInfo, meta PacketMeta)

// MyInfoHandler is called when my node info is received
type MyInfoHandler func(myInfo *pb.MyNodeInfo)

// ConfigCompleteHandler is called when config download is complete
type ConfigCompleteHandler func(configId uint32)

// MessageHandler handles decoded text messages
type MessageHandler func(from uint32, to uint32, channel uint32, text string, rxTime uint32, packetId uint32)

// PacketMeta carries radio metadata from a received mesh packet
type PacketMeta struct {
	ViaMqtt  bool
	HopsAway int32 // -1 = unknown, 0 = direct, >0 = relayed
	SNR      float32
	RSSI     int32
}

// packetMeta extracts radio metadata from a mesh packet
func packetMeta(packet *pb.MeshPacket) PacketMeta {
	meta := PacketMeta{
		ViaMqtt:  packet.ViaMqtt,
		HopsAway: -1,
		SNR:      packet.RxSnr,
		RSSI:     packet.RxRssi,
	}
	// Only interpret radio metadata when HopStart > 0
	// HopStart=0 means the packet came from serial (config dump, local node)
	// and has no meaningful radio metrics
	if packet.HopStart > 0 {
		meta.HopsAway = int32(packet.HopStart - packet.HopLimit)
		// RSSI=0 on a radio packet indicates it was relayed via MQTT
		// (real radio reception always has non-zero RSSI)
		if packet.RxRssi == 0 && !packet.ViaMqtt {
			meta.ViaMqtt = true
		}
	}
	return meta
}

// PositionHandler handles position updates
type PositionHandler func(from uint32, position *pb.Position, meta PacketMeta)

// TelemetryHandler handles telemetry data
type TelemetryHandler func(from uint32, telemetry *pb.Telemetry, meta PacketMeta)

// ChannelHandler handles channel info
type ChannelHandler func(channel *pb.Channel)

// ConfigHandler handles config info
type ConfigHandler func(config *pb.Config)

// MetadataHandler handles device metadata
type MetadataHandler func(metadata *pb.DeviceMetadata)

// TracerouteHandler handles traceroute responses
type TracerouteHandler func(from uint32, route *pb.RouteDiscovery)

// NeighborInfoHandler handles neighbor info updates
type NeighborInfoHandler func(from uint32, neighborInfo *pb.NeighborInfo)

// WaypointHandler handles waypoint updates
type WaypointHandler func(from uint32, waypoint *pb.Waypoint, meta PacketMeta)

// ModuleConfigHandler handles module config updates
type ModuleConfigHandler func(moduleConfig *pb.ModuleConfig)

// Processor handles incoming radio packets
type Processor struct {
	mu sync.RWMutex

	// Handlers
	onPacket         PacketHandler
	onNode           NodeHandler
	onMyInfo         MyInfoHandler
	onConfigComplete ConfigCompleteHandler
	onMessage        MessageHandler
	onPosition       PositionHandler
	onTelemetry      TelemetryHandler
	onChannel        ChannelHandler
	onConfig         ConfigHandler
	onModuleConfig   ModuleConfigHandler
	onMetadata       MetadataHandler
	onTraceroute     TracerouteHandler
	onNeighborInfo   NeighborInfoHandler
	onWaypoint       WaypointHandler

	// State
	myNodeNum     uint32
	configId      uint32
	configStarted bool
}

// NewProcessor creates a new packet processor
func NewProcessor() *Processor {
	return &Processor{}
}

// SetPacketHandler sets the packet handler
func (p *Processor) SetPacketHandler(h PacketHandler) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.onPacket = h
}

// SetNodeHandler sets the node info handler
func (p *Processor) SetNodeHandler(h NodeHandler) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.onNode = h
}

// SetMyInfoHandler sets the my node info handler
func (p *Processor) SetMyInfoHandler(h MyInfoHandler) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.onMyInfo = h
}

// SetConfigCompleteHandler sets the config complete handler
func (p *Processor) SetConfigCompleteHandler(h ConfigCompleteHandler) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.onConfigComplete = h
}

// SetMessageHandler sets the text message handler
func (p *Processor) SetMessageHandler(h MessageHandler) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.onMessage = h
}

// SetPositionHandler sets the position handler
func (p *Processor) SetPositionHandler(h PositionHandler) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.onPosition = h
}

// SetTelemetryHandler sets the telemetry handler
func (p *Processor) SetTelemetryHandler(h TelemetryHandler) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.onTelemetry = h
}

// SetChannelHandler sets the channel handler
func (p *Processor) SetChannelHandler(h ChannelHandler) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.onChannel = h
}

// SetConfigHandler sets the config handler
func (p *Processor) SetConfigHandler(h ConfigHandler) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.onConfig = h
}

// SetModuleConfigHandler sets the module config handler
func (p *Processor) SetModuleConfigHandler(h ModuleConfigHandler) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.onModuleConfig = h
}

// SetMetadataHandler sets the metadata handler
func (p *Processor) SetMetadataHandler(h MetadataHandler) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.onMetadata = h
}

// SetTracerouteHandler sets the traceroute response handler
func (p *Processor) SetTracerouteHandler(h TracerouteHandler) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.onTraceroute = h
}

// SetNeighborInfoHandler sets the neighbor info handler
func (p *Processor) SetNeighborInfoHandler(h NeighborInfoHandler) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.onNeighborInfo = h
}

// SetWaypointHandler sets the waypoint handler
func (p *Processor) SetWaypointHandler(h WaypointHandler) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.onWaypoint = h
}

// GetMyNodeNum returns the local node number
func (p *Processor) GetMyNodeNum() uint32 {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.myNodeNum
}

// ProcessFromRadio processes a FromRadio message
func (p *Processor) ProcessFromRadio(data []byte) error {
	fromRadio, err := pb.UnmarshalFromRadio(data)
	if err != nil {
		return fmt.Errorf("failed to unmarshal FromRadio: %w", err)
	}

	log.Debug().
		Uint32("id", fromRadio.Id).
		Msg("processing FromRadio message")

	switch v := fromRadio.PayloadVariant.(type) {
	case *pb.FromRadio_Packet:
		if v.Packet != nil {
			p.handlePacket(v.Packet)
		}

	case *pb.FromRadio_MyInfo:
		if v.MyInfo != nil {
			p.handleMyInfo(v.MyInfo)
		}

	case *pb.FromRadio_NodeInfo:
		if v.NodeInfo != nil {
			p.handleNodeInfo(v.NodeInfo)
		}

	case *pb.FromRadio_ConfigCompleteId:
		p.handleConfigComplete(v.ConfigCompleteId)

	case *pb.FromRadio_Rebooted:
		if v.Rebooted {
			log.Info().Msg("device rebooted, requesting config")
			p.configStarted = false
		}

	case *pb.FromRadio_Config:
		if v.Config != nil {
			p.handleConfig(v.Config)
		}

	case *pb.FromRadio_ModuleConfig:
		if v.ModuleConfig != nil {
			p.handleModuleConfig(v.ModuleConfig)
		}

	case *pb.FromRadio_Channel:
		if v.Channel != nil {
			p.handleChannel(v.Channel)
		}

	case *pb.FromRadio_LogRecord:
		if v.LogRecord != nil {
			log.Debug().
				Str("source", v.LogRecord.Source).
				Str("message", v.LogRecord.Message).
				Msg("device log")
		}

	case *pb.FromRadio_QueueStatus:
		if v.QueueStatus != nil {
			log.Debug().
				Uint32("free", v.QueueStatus.Free).
				Uint32("maxlen", v.QueueStatus.Maxlen).
				Msg("queue status")
		}

	case *pb.FromRadio_Metadata:
		if v.Metadata != nil {
			p.handleMetadata(v.Metadata)
		}

	default:
		log.Debug().Msg("received unknown FromRadio variant")
	}

	return nil
}

func (p *Processor) handleMyInfo(myInfo *pb.MyNodeInfo) {
	p.mu.Lock()
	p.myNodeNum = myInfo.MyNodeNum
	p.mu.Unlock()

	log.Info().
		Uint32("nodeNum", myInfo.MyNodeNum).
		Msg("received my node info")

	p.mu.RLock()
	handler := p.onMyInfo
	p.mu.RUnlock()

	if handler != nil {
		handler(myInfo)
	}
}

func (p *Processor) handleNodeInfo(nodeInfo *pb.NodeInfo) {
	log.Debug().
		Uint32("num", nodeInfo.Num).
		Msg("received node info")

	if nodeInfo.User != nil {
		log.Debug().
			Str("longName", nodeInfo.User.LongName).
			Str("shortName", nodeInfo.User.ShortName).
			Msg("node user info")
	}

	p.mu.RLock()
	handler := p.onNode
	p.mu.RUnlock()

	if handler != nil {
		// Config dump from serial — no real radio metadata
		handler(nodeInfo, PacketMeta{HopsAway: -1})
	}
}

func (p *Processor) handleConfigComplete(configId uint32) {
	p.mu.Lock()
	p.configId = configId
	p.configStarted = false
	p.mu.Unlock()

	log.Info().
		Uint32("configId", configId).
		Msg("config download complete")

	p.mu.RLock()
	handler := p.onConfigComplete
	p.mu.RUnlock()

	if handler != nil {
		handler(configId)
	}
}

func (p *Processor) handlePacket(packet *pb.MeshPacket) {
	log.Debug().
		Uint32("from", packet.From).
		Uint32("to", packet.To).
		Uint32("id", packet.Id).
		Uint32("channel", packet.Channel).
		Float32("snr", packet.RxSnr).
		Int32("rssi", packet.RxRssi).
		Bool("viaMqtt", packet.ViaMqtt).
		Msg("received mesh packet")

	// Call the general packet handler first
	p.mu.RLock()
	packetHandler := p.onPacket
	p.mu.RUnlock()

	if packetHandler != nil {
		packetHandler(packet)
	}

	// Process decoded data if available
	if decoded := packet.GetDecoded(); decoded != nil {
		p.handleDecodedData(packet, decoded)
	}
}

func (p *Processor) handleDecodedData(packet *pb.MeshPacket, data *pb.Data) {
	log.Info().
		Int32("portnum", int32(data.Portnum)).
		Str("portnumName", data.Portnum.String()).
		Int("payloadLen", len(data.Payload)).
		Uint32("from", packet.From).
		Uint32("to", packet.To).
		Msg("processing decoded data")

	switch data.Portnum {
	case pb.PortNum_TEXT_MESSAGE_APP:
		p.handleTextMessage(packet, data)

	case pb.PortNum_POSITION_APP:
		p.handlePositionApp(packet, data)

	case pb.PortNum_NODEINFO_APP:
		p.handleNodeInfoApp(packet, data)

	case pb.PortNum_ROUTING_APP:
		p.handleRoutingApp(packet, data)

	case pb.PortNum_ADMIN_APP:
		p.handleAdminApp(packet, data)

	case pb.PortNum_TELEMETRY_APP:
		p.handleTelemetryApp(packet, data)

	case pb.PortNum_WAYPOINT_APP:
		p.handleWaypointApp(packet, data)

	case pb.PortNum_NEIGHBORINFO_APP:
		p.handleNeighborInfoApp(packet, data)

	case pb.PortNum_TRACEROUTE_APP:
		p.handleTracerouteApp(packet, data)

	default:
		log.Debug().
			Int32("portnum", int32(data.Portnum)).
			Msg("unhandled portnum")
	}
}

func (p *Processor) handleTextMessage(packet *pb.MeshPacket, data *pb.Data) {
	text := string(data.Payload)

	log.Info().
		Uint32("from", packet.From).
		Uint32("to", packet.To).
		Uint32("channel", packet.Channel).
		Str("text", text).
		Msg("received text message")

	p.mu.RLock()
	handler := p.onMessage
	p.mu.RUnlock()

	if handler != nil {
		handler(packet.From, packet.To, packet.Channel, text, packet.RxTime, packet.Id)
	}
}

func (p *Processor) handlePositionApp(packet *pb.MeshPacket, data *pb.Data) {
	position, err := pb.UnmarshalPosition(data.Payload)
	if err != nil {
		log.Warn().Err(err).Msg("failed to unmarshal position")
		return
	}

	log.Debug().
		Uint32("from", packet.From).
		Float64("lat", position.Latitude()).
		Float64("lon", position.Longitude()).
		Int32("alt", position.Altitude).
		Msg("received position")

	p.mu.RLock()
	handler := p.onPosition
	p.mu.RUnlock()

	if handler != nil {
		handler(packet.From, position, packetMeta(packet))
	}
}

func (p *Processor) handleNodeInfoApp(packet *pb.MeshPacket, data *pb.Data) {
	user, err := pb.UnmarshalUser(data.Payload)
	if err != nil {
		log.Warn().Err(err).Msg("failed to unmarshal user")
		return
	}

	// Create a NodeInfo from the user data
	meta := packetMeta(packet)
	var hops uint32
	if meta.HopsAway > 0 {
		hops = uint32(meta.HopsAway)
	}
	nodeInfo := &pb.NodeInfo{
		Num:       packet.From,
		User:      user,
		LastHeard: uint32(time.Now().Unix()),
		Snr:       packet.RxSnr,
		Channel:   packet.Channel,
		ViaMqtt:   meta.ViaMqtt,
		Hops:      hops,
	}

	log.Debug().
		Uint32("from", packet.From).
		Str("longName", user.LongName).
		Str("shortName", user.ShortName).
		Msg("received user info broadcast")

	p.mu.RLock()
	handler := p.onNode
	p.mu.RUnlock()

	if handler != nil {
		handler(nodeInfo, meta)
	}
}

func (p *Processor) handleRoutingApp(packet *pb.MeshPacket, data *pb.Data) {
	// Routing messages contain ACKs, NAKs, and route discovery
	log.Debug().
		Uint32("from", packet.From).
		Uint32("requestId", data.RequestId).
		Msg("received routing message")
}

func (p *Processor) handleAdminApp(packet *pb.MeshPacket, data *pb.Data) {
	log.Debug().
		Uint32("from", packet.From).
		Msg("received admin message")
}

func (p *Processor) handleTelemetryApp(packet *pb.MeshPacket, data *pb.Data) {
	telemetry, err := pb.UnmarshalTelemetry(data.Payload)
	if err != nil {
		log.Warn().Err(err).Msg("failed to unmarshal telemetry")
		return
	}

	log.Debug().
		Uint32("from", packet.From).
		Msg("received telemetry")

	p.mu.RLock()
	handler := p.onTelemetry
	p.mu.RUnlock()

	if handler != nil {
		handler(packet.From, telemetry, packetMeta(packet))
	}
}

func (p *Processor) handleWaypointApp(packet *pb.MeshPacket, data *pb.Data) {
	waypoint, err := pb.UnmarshalWaypoint(data.Payload)
	if err != nil {
		log.Warn().Err(err).Msg("failed to unmarshal waypoint")
		return
	}

	log.Info().
		Uint32("from", packet.From).
		Uint32("id", waypoint.Id).
		Str("name", waypoint.Name).
		Int32("lat", waypoint.LatitudeI).
		Int32("lon", waypoint.LongitudeI).
		Msg("received waypoint")

	p.mu.RLock()
	handler := p.onWaypoint
	p.mu.RUnlock()

	if handler != nil {
		handler(packet.From, waypoint, packetMeta(packet))
	}
}

func (p *Processor) handleNeighborInfoApp(packet *pb.MeshPacket, data *pb.Data) {
	neighborInfo, err := pb.UnmarshalNeighborInfo(data.Payload)
	if err != nil {
		log.Warn().Err(err).Msg("failed to unmarshal neighbor info")
		return
	}

	log.Info().
		Uint32("from", packet.From).
		Uint32("nodeId", neighborInfo.NodeId).
		Int("neighborCount", len(neighborInfo.Neighbors)).
		Msg("received neighbor info")

	// Log each neighbor
	for _, neighbor := range neighborInfo.Neighbors {
		log.Debug().
			Uint32("neighborNodeId", neighbor.NodeId).
			Float32("snr", neighbor.Snr).
			Msg("neighbor")
	}

	p.mu.RLock()
	handler := p.onNeighborInfo
	p.mu.RUnlock()

	if handler != nil {
		handler(packet.From, neighborInfo)
	}
}

func (p *Processor) handleTracerouteApp(packet *pb.MeshPacket, data *pb.Data) {
	log.Info().
		Uint32("from", packet.From).
		Uint32("to", packet.To).
		Uint32("requestId", data.RequestId).
		Int("payloadLen", len(data.Payload)).
		Hex("payload", data.Payload).
		Msg("received traceroute packet")

	route, err := pb.UnmarshalRouteDiscovery(data.Payload)
	if err != nil {
		log.Warn().Err(err).Msg("failed to unmarshal traceroute response")
		return
	}

	log.Info().
		Uint32("from", packet.From).
		Uint32("requestId", data.RequestId).
		Uints32("route", route.Route).
		Uints32("routeBack", route.RouteBack).
		Ints32("snrTowards", route.SnrTowards).
		Ints32("snrBack", route.SnrBack).
		Msg("parsed traceroute response")

	p.mu.RLock()
	handler := p.onTraceroute
	p.mu.RUnlock()

	if handler != nil {
		handler(packet.From, route)
	}
}

func (p *Processor) handleChannel(channel *pb.Channel) {
	log.Debug().
		Uint32("index", channel.Index).
		Int32("role", int32(channel.Role)).
		Msg("received channel")

	if channel.Settings != nil {
		log.Debug().
			Str("name", channel.Settings.Name).
			Msg("channel settings")
	}

	p.mu.RLock()
	handler := p.onChannel
	p.mu.RUnlock()

	if handler != nil {
		handler(channel)
	}
}

func (p *Processor) handleConfig(config *pb.Config) {
	log.Debug().Msg("received config")

	p.mu.RLock()
	handler := p.onConfig
	p.mu.RUnlock()

	if handler != nil {
		handler(config)
	}
}

func (p *Processor) handleModuleConfig(moduleConfig *pb.ModuleConfig) {
	log.Debug().Msg("received module config")

	p.mu.RLock()
	handler := p.onModuleConfig
	p.mu.RUnlock()

	if handler != nil {
		handler(moduleConfig)
	}
}

func (p *Processor) handleMetadata(metadata *pb.DeviceMetadata) {
	log.Info().
		Str("firmware", metadata.FirmwareVersion).
		Str("hwModel", metadata.HwModel.String()).
		Msg("device metadata")

	p.mu.RLock()
	handler := p.onMetadata
	p.mu.RUnlock()

	if handler != nil {
		handler(metadata)
	}
}
