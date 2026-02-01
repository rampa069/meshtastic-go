package service

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/meshtastic/meshtastic-go/internal/database"
	"github.com/meshtastic/meshtastic-go/internal/database/dao"
	"github.com/meshtastic/meshtastic-go/internal/protocol"
	"github.com/meshtastic/meshtastic-go/internal/radio"
	"github.com/meshtastic/meshtastic-go/internal/websocket"
	"github.com/meshtastic/meshtastic-go/pkg/pb"
	"github.com/rs/zerolog/log"
)

// MeshService coordinates all mesh operations
type MeshService struct {
	mu           sync.RWMutex
	db           *database.Database
	radioManager *radio.Manager
	wsHub        *websocket.Hub
	myNodeNum    uint32

	// Protocol handlers
	processor *protocol.Processor
	sender    *protocol.Sender

	// Sub-services
	nodeManager      *NodeManager
	messageService   *MessageService
	configManager    *ConfigManager
	telemetryService *TelemetryService
}

// NewMeshService creates a new mesh service
func NewMeshService(db *database.Database, radioManager *radio.Manager, wsHub *websocket.Hub) *MeshService {
	ms := &MeshService{
		db:           db,
		radioManager: radioManager,
		wsHub:        wsHub,
	}

	// Set radio callbacks
	radioManager.SetCallbacks(ms)

	// Initialize protocol handlers
	ms.processor = protocol.NewProcessor()
	ms.sender = protocol.NewSender(func(data []byte) error {
		return radioManager.SendToRadio(data)
	})

	// Set up processor handlers
	ms.setupProcessorHandlers()

	// Initialize sub-services
	ms.nodeManager = NewNodeManager(db, wsHub)
	ms.messageService = NewMessageService(db, wsHub)
	ms.configManager = NewConfigManager(wsHub)
	ms.telemetryService = NewTelemetryService(dao.NewTelemetryDAO(db.DB()), wsHub)

	return ms
}

// setupProcessorHandlers configures the handlers for incoming messages
func (ms *MeshService) setupProcessorHandlers() {
	// Handle my node info
	ms.processor.SetMyInfoHandler(func(myInfo *pb.MyNodeInfo) {
		ms.mu.Lock()
		ms.myNodeNum = myInfo.MyNodeNum
		ms.mu.Unlock()
		ms.sender.SetMyNodeNum(myInfo.MyNodeNum)

		log.Info().
			Uint32("nodeNum", myInfo.MyNodeNum).
			Msg("my node number set")
	})

	// Handle node info updates
	ms.processor.SetNodeHandler(func(nodeInfo *pb.NodeInfo) {
		node := ms.nodeManager.ProcessNodeInfo(nodeInfo)
		if node != nil {
			ms.wsHub.Broadcast(websocket.Event{
				Type: "node.updated",
				Data: map[string]interface{}{
					"node": node,
				},
			})

			// If this is our own node, also broadcast mynode.updated
			ms.mu.RLock()
			isMyNode := ms.myNodeNum != 0 && nodeInfo.Num == ms.myNodeNum
			ms.mu.RUnlock()
			if isMyNode {
				ms.wsHub.Broadcast(websocket.Event{
					Type: "mynode.updated",
					Data: map[string]interface{}{
						"node": node,
					},
				})
			}
		}
	})

	// Handle text messages
	ms.processor.SetMessageHandler(func(from, to, channel uint32, text string, rxTime, packetId uint32) {
		msg := ms.messageService.ProcessTextMessage(from, to, channel, text, rxTime, packetId)
		if msg != nil {
			ms.wsHub.Broadcast(websocket.Event{
				Type: "message.received",
				Data: map[string]interface{}{
					"message": msg,
				},
			})
		}
	})

	// Handle position updates
	ms.processor.SetPositionHandler(func(from uint32, position *pb.Position, viaMqtt bool) {
		ms.nodeManager.UpdateNodePosition(from, position, viaMqtt)
	})

	// Handle telemetry updates
	ms.processor.SetTelemetryHandler(func(from uint32, telemetry *pb.Telemetry, viaMqtt bool) {
		// Update node with latest telemetry
		ms.nodeManager.UpdateNodeTelemetry(from, telemetry, viaMqtt)
		// Store telemetry in history and broadcast
		ms.telemetryService.ProcessTelemetry(from, telemetry, viaMqtt)
	})

	// Handle config complete
	ms.processor.SetConfigCompleteHandler(func(configId uint32) {
		log.Info().Uint32("configId", configId).Msg("config download complete")
	})

	// Handle channel updates
	ms.processor.SetChannelHandler(func(channel *pb.Channel) {
		ms.configManager.ProcessChannel(channel)
	})

	// Handle config updates
	ms.processor.SetConfigHandler(func(config *pb.Config) {
		ms.configManager.ProcessConfig(config)
	})

	// Handle metadata updates
	ms.processor.SetMetadataHandler(func(metadata *pb.DeviceMetadata) {
		ms.configManager.ProcessMetadata(metadata)
	})

	// Handle traceroute responses
	ms.processor.SetTracerouteHandler(func(from uint32, route *pb.RouteDiscovery) {
		// Convert route to node IDs for the frontend
		routeNodes := make([]uint32, 0, len(route.Route)+1)
		routeNodes = append(routeNodes, from) // Add destination node
		routeNodes = append(routeNodes, route.Route...)

		routeBackNodes := make([]uint32, 0, len(route.RouteBack))
		routeBackNodes = append(routeBackNodes, route.RouteBack...)

		// Store traceroute in database
		tracerouteDAO := dao.NewTracerouteDAO(ms.db.DB())
		entity := &dao.TracerouteEntity{
			FromNode:   ms.myNodeNum,
			ToNode:     from,
			Route:      routeNodes,
			RouteBack:  routeBackNodes,
			SnrTowards: route.SnrTowards,
			SnrBack:    route.SnrBack,
			HopCount:   len(routeNodes),
			Timestamp:  time.Now().Unix(),
			Success:    true,
		}
		if id, err := tracerouteDAO.Insert(entity); err != nil {
			log.Error().Err(err).Msg("failed to save traceroute")
		} else {
			log.Debug().Int64("id", id).Uint32("to", from).Msg("saved traceroute")
		}

		ms.wsHub.Broadcast(websocket.Event{
			Type: "traceroute.response",
			Data: map[string]interface{}{
				"from":       from,
				"route":      routeNodes,
				"routeBack":  routeBackNodes,
				"snrTowards": route.SnrTowards,
				"snrBack":    route.SnrBack,
			},
		})
	})

	// Handle neighbor info updates
	ms.processor.SetNeighborInfoHandler(func(from uint32, neighborInfo *pb.NeighborInfo) {
		// Convert neighbors to frontend format
		neighbors := make([]map[string]interface{}, 0, len(neighborInfo.Neighbors))
		for _, n := range neighborInfo.Neighbors {
			neighbors = append(neighbors, map[string]interface{}{
				"nodeId":     n.NodeId,
				"snr":        n.Snr,
				"lastRxTime": n.LastRxTime,
			})
		}

		ms.wsHub.Broadcast(websocket.Event{
			Type: "neighborinfo.updated",
			Data: map[string]interface{}{
				"from":      from,
				"nodeId":    neighborInfo.NodeId,
				"neighbors": neighbors,
			},
		})

		// Also update the node's neighbor list in the node manager
		ms.nodeManager.UpdateNodeNeighbors(from, neighborInfo)
	})

	// Handle waypoint updates
	ms.processor.SetWaypointHandler(func(from uint32, waypoint *pb.Waypoint, viaMqtt bool) {
		// Store waypoint in database
		waypointDAO := dao.NewWaypointDAO(ms.db.DB())
		waypointEntity := &dao.WaypointEntity{
			ID:          waypoint.Id,
			LatitudeI:   waypoint.LatitudeI,
			LongitudeI:  waypoint.LongitudeI,
			Expire:      waypoint.Expire,
			LockedTo:    waypoint.LockedTo,
			Name:        waypoint.Name,
			Description: waypoint.Description,
			Icon:        waypoint.Icon,
			FromNode:    from,
			ViaMqtt:     viaMqtt,
		}
		if err := waypointDAO.Upsert(waypointEntity); err != nil {
			log.Error().Err(err).Msg("failed to save waypoint")
		}

		// Broadcast to websocket clients
		ms.wsHub.Broadcast(websocket.Event{
			Type: "waypoint.received",
			Data: map[string]interface{}{
				"id":          waypoint.Id,
				"latitude":    float64(waypoint.LatitudeI) * 1e-7,
				"longitude":   float64(waypoint.LongitudeI) * 1e-7,
				"expire":      waypoint.Expire,
				"lockedTo":    waypoint.LockedTo,
				"name":        waypoint.Name,
				"description": waypoint.Description,
				"icon":        waypoint.Icon,
				"from":        from,
				"viaMqtt":     viaMqtt,
			},
		})
	})
}

// Connect connects to a radio device
func (ms *MeshService) Connect(connType string, address string) error {
	ct := radio.ConnectionType(connType)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return ms.radioManager.Connect(ctx, ct, address)
}

// Disconnect disconnects from the radio
func (ms *MeshService) Disconnect() error {
	return ms.radioManager.Disconnect()
}

// IsConnected returns true if connected to a radio
func (ms *MeshService) IsConnected() bool {
	return ms.radioManager.IsConnected()
}

// GetConnectionState returns the current connection state
func (ms *MeshService) GetConnectionState() radio.ConnectionState {
	return ms.radioManager.State()
}

// GetConnectionInfo returns connection details
func (ms *MeshService) GetConnectionInfo() (radio.ConnectionType, string, bool) {
	return ms.radioManager.GetConnectionInfo()
}

// ScanDevices scans for available devices of the specified type
func (ms *MeshService) ScanDevices(ctx context.Context, connType string, timeout time.Duration) ([]radio.DeviceInfo, error) {
	ct := radio.ConnectionType(connType)
	return ms.radioManager.Scan(ctx, ct, timeout)
}

// CancelScan cancels any ongoing device scan
func (ms *MeshService) CancelScan() {
	ms.radioManager.CancelScan()
}

// ScanDevicesStreaming scans for devices and broadcasts each one via WebSocket as found
func (ms *MeshService) ScanDevicesStreaming(ctx context.Context, connType string, timeout time.Duration) {
	ct := radio.ConnectionType(connType)

	// Broadcast scan started
	ms.wsHub.Broadcast(websocket.Event{
		Type: "scan.started",
		Data: map[string]interface{}{
			"type":    connType,
			"timeout": timeout.Seconds(),
		},
	})

	// Scan with callback that broadcasts each device
	devices, err := ms.radioManager.ScanWithCallback(ctx, ct, timeout, func(device radio.DeviceInfo) {
		ms.wsHub.Broadcast(websocket.Event{
			Type: "scan.device",
			Data: map[string]interface{}{
				"device": device,
			},
		})
	})

	// Broadcast scan complete
	if err != nil {
		ms.wsHub.Broadcast(websocket.Event{
			Type: "scan.error",
			Data: map[string]interface{}{
				"error": err.Error(),
			},
		})
	} else {
		ms.wsHub.Broadcast(websocket.Event{
			Type: "scan.complete",
			Data: map[string]interface{}{
				"type":    connType,
				"count":   len(devices),
				"devices": devices,
			},
		})
	}
}

// SendToRadio sends data to the radio
func (ms *MeshService) SendToRadio(data []byte) error {
	return ms.radioManager.SendToRadio(data)
}

// GetMyNodeNum returns the local node number
func (ms *MeshService) GetMyNodeNum() uint32 {
	ms.mu.RLock()
	defer ms.mu.RUnlock()
	return ms.myNodeNum
}

// SendTextMessage sends a text message
func (ms *MeshService) SendTextMessage(to uint32, channel uint32, text string) (uint32, error) {
	if ms.sender == nil {
		return 0, fmt.Errorf("not connected")
	}
	return ms.sender.SendTextMessage(to, channel, text, true)
}

// SendPosition sends a position update
func (ms *MeshService) SendPosition(lat, lon float64, altitude int32) (uint32, error) {
	if ms.sender == nil {
		return 0, fmt.Errorf("not connected")
	}
	return ms.sender.SendPosition(lat, lon, altitude)
}

// SendTraceroute initiates a traceroute to a node
func (ms *MeshService) SendTraceroute(destNode uint32) (uint32, error) {
	if ms.sender == nil {
		return 0, fmt.Errorf("not connected")
	}
	return ms.sender.SendTraceroute(destNode)
}

// RequestPosition requests position from a remote node
func (ms *MeshService) RequestPosition(destNode uint32) (uint32, error) {
	if ms.sender == nil {
		return 0, fmt.Errorf("not connected")
	}
	return ms.sender.RequestPosition(destNode)
}

// RequestNodeInfo requests node info from a remote node
func (ms *MeshService) RequestNodeInfo(destNode uint32) (uint32, error) {
	if ms.sender == nil {
		return 0, fmt.Errorf("not connected")
	}
	return ms.sender.RequestNodeInfo(destNode)
}

// RequestTelemetry requests telemetry from a remote node
func (ms *MeshService) RequestTelemetry(destNode uint32) (uint32, error) {
	if ms.sender == nil {
		return 0, fmt.Errorf("not connected")
	}
	return ms.sender.RequestTelemetry(destNode)
}

// SendWaypoint sends a waypoint to the mesh
func (ms *MeshService) SendWaypoint(waypoint *pb.Waypoint, channel uint32) error {
	if ms.sender == nil {
		return fmt.Errorf("not connected")
	}
	_, err := ms.sender.SendWaypoint(waypoint, channel)
	return err
}

// RequestConfig requests device configuration
func (ms *MeshService) RequestConfig() error {
	if ms.sender == nil {
		return fmt.Errorf("not connected")
	}
	return ms.sender.RequestConfig()
}

// SetChannel updates a channel configuration on the device
func (ms *MeshService) SetChannel(channel *pb.Channel) (uint32, error) {
	if ms.sender == nil {
		return 0, fmt.Errorf("not connected")
	}
	return ms.sender.SetChannel(channel)
}

// SetConfig updates device configuration
func (ms *MeshService) SetConfig(config *pb.Config) (uint32, error) {
	if ms.sender == nil {
		return 0, fmt.Errorf("not connected")
	}
	return ms.sender.SetConfig(config)
}

// SetModuleConfig updates module configuration
func (ms *MeshService) SetModuleConfig(moduleConfig *pb.ModuleConfig) (uint32, error) {
	if ms.sender == nil {
		return 0, fmt.Errorf("not connected")
	}
	return ms.sender.SetModuleConfig(moduleConfig)
}

// NodeManager returns the node manager
func (ms *MeshService) NodeManager() *NodeManager {
	return ms.nodeManager
}

// MessageService returns the message service
func (ms *MeshService) MessageService() *MessageService {
	return ms.messageService
}

// ConfigManager returns the config manager
func (ms *MeshService) ConfigManager() *ConfigManager {
	return ms.configManager
}

// TelemetryService returns the telemetry service
func (ms *MeshService) TelemetryService() *TelemetryService {
	return ms.telemetryService
}

// RadioCallbacks implementation

// OnConnect is called when the radio connects
func (ms *MeshService) OnConnect() {
	log.Info().Msg("radio connected")
	ms.wsHub.Broadcast(websocket.Event{
		Type: "connection.state",
		Data: map[string]interface{}{
			"state": "connected",
		},
	})

	// Request configuration from the device
	if err := ms.RequestConfig(); err != nil {
		log.Warn().Err(err).Msg("failed to request config on connect")
	}
}

// OnDisconnect is called when the radio disconnects
func (ms *MeshService) OnDisconnect(isPermanent bool, err error) {
	log.Info().Bool("permanent", isPermanent).Err(err).Msg("radio disconnected")
	ms.wsHub.Broadcast(websocket.Event{
		Type: "connection.state",
		Data: map[string]interface{}{
			"state": "disconnected",
		},
	})
}

// HandleFromRadio processes data received from the radio
func (ms *MeshService) HandleFromRadio(data []byte) {
	if err := ms.processor.ProcessFromRadio(data); err != nil {
		log.Warn().Err(err).Int("bytes", len(data)).Msg("failed to process FromRadio")
	}
}
