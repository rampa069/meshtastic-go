package service

import (
	"fmt"
	"sync"
	"time"

	"github.com/meshtastic/meshtastic-go/internal/database"
	"github.com/meshtastic/meshtastic-go/internal/util"
	"github.com/meshtastic/meshtastic-go/internal/websocket"
	"github.com/meshtastic/meshtastic-go/pkg/pb"
	"github.com/rs/zerolog/log"
)

// NodeManager manages node data and state
type NodeManager struct {
	mu    sync.RWMutex
	db    *database.Database
	wsHub *websocket.Hub
	nodes map[uint32]*Node
}

// Node represents a mesh node
type Node struct {
	Num              uint32  `json:"num"`
	LongName         string  `json:"longName"`
	ShortName        string  `json:"shortName"`
	HardwareModel    string  `json:"hardwareModel"`
	Role             string  `json:"role"`
	PublicKey        string  `json:"publicKey,omitempty"`
	IsLicensed       bool    `json:"isLicensed,omitempty"`

	// Position
	Latitude         float64 `json:"latitude"`
	Longitude        float64 `json:"longitude"`
	Altitude         int32   `json:"altitude"`
	PositionTime     int64   `json:"positionTime,omitempty"`
	PositionPrecision int32  `json:"positionPrecision,omitempty"`

	// Calculated position info (relative to local node)
	Distance         float64 `json:"distance,omitempty"`         // Distance in meters
	DistanceStr      string  `json:"distanceStr,omitempty"`      // Formatted distance
	Bearing          float64 `json:"bearing,omitempty"`          // Bearing in degrees
	BearingCardinal  string  `json:"bearingCardinal,omitempty"`  // Cardinal direction

	// Radio metrics
	SNR              float32 `json:"snr"`
	RSSI             int32   `json:"rssi"`
	LastHeard        int64   `json:"lastHeard"`
	Channel          int32   `json:"channel"`
	ViaMqtt          bool    `json:"viaMqtt"`
	HopsAway         int32   `json:"hopsAway"`

	// User preferences
	IsFavorite       bool    `json:"isFavorite"`
	IsIgnored        bool    `json:"isIgnored"`
	IsMuted          bool    `json:"isMuted"`
	Notes            string  `json:"notes"`

	// Device telemetry
	BatteryLevel     int32   `json:"batteryLevel,omitempty"`
	Voltage          float32 `json:"voltage,omitempty"`
	ChannelUtilization float32 `json:"channelUtilization,omitempty"`
	AirUtilTx        float32 `json:"airUtilTx,omitempty"`
	Uptime           int64   `json:"uptime,omitempty"`

	// Environment telemetry
	Temperature      float32 `json:"temperature,omitempty"`
	RelativeHumidity float32 `json:"relativeHumidity,omitempty"`
	BarometricPressure float32 `json:"barometricPressure,omitempty"`
	GasResistance    float32 `json:"gasResistance,omitempty"`
	Iaq              int32   `json:"iaq,omitempty"`

	// Neighbors
	Neighbors        []*NodeNeighbor `json:"neighbors,omitempty"`
}

// NodeNeighbor represents a direct neighbor of a node
type NodeNeighbor struct {
	NodeId     uint32  `json:"nodeId"`
	SNR        float32 `json:"snr"`
	LastRxTime int64   `json:"lastRxTime,omitempty"`
}

// NewNodeManager creates a new node manager
func NewNodeManager(db *database.Database, wsHub *websocket.Hub) *NodeManager {
	return &NodeManager{
		db:    db,
		wsHub: wsHub,
		nodes: make(map[uint32]*Node),
	}
}

// GetNode returns a node by number
func (nm *NodeManager) GetNode(num uint32) (*Node, bool) {
	nm.mu.RLock()
	defer nm.mu.RUnlock()
	node, ok := nm.nodes[num]
	return node, ok
}

// GetNodes returns all nodes
func (nm *NodeManager) GetNodes() []*Node {
	nm.mu.RLock()
	defer nm.mu.RUnlock()

	nodes := make([]*Node, 0, len(nm.nodes))
	for _, node := range nm.nodes {
		nodes = append(nodes, node)
	}
	return nodes
}

// GetNodesWithDistance returns all nodes with distance/bearing calculated from reference position
func (nm *NodeManager) GetNodesWithDistance(refLat, refLon float64) []*Node {
	nm.mu.RLock()
	defer nm.mu.RUnlock()

	nodes := make([]*Node, 0, len(nm.nodes))
	hasRef := refLat != 0 || refLon != 0

	for _, node := range nm.nodes {
		// Create a copy with position info
		nodeCopy := *node

		if hasRef && node.HasPosition() {
			nodeCopy.Distance = util.CalculateDistance(refLat, refLon, node.Latitude, node.Longitude)
			nodeCopy.DistanceStr = util.FormatDistance(nodeCopy.Distance)
			nodeCopy.Bearing = util.CalculateBearing(refLat, refLon, node.Latitude, node.Longitude)
			nodeCopy.BearingCardinal = util.BearingToCardinal8(nodeCopy.Bearing)
		}

		nodes = append(nodes, &nodeCopy)
	}
	return nodes
}

// HasPosition returns true if the node has valid GPS coordinates
func (n *Node) HasPosition() bool {
	return n.Latitude != 0 || n.Longitude != 0
}

// GetPositionInfo returns formatted position information
func (n *Node) GetPositionInfo() *util.PositionInfo {
	if !n.HasPosition() {
		return nil
	}
	return &util.PositionInfo{
		CoordDMS:     util.FormatCoordinateDMS(n.Latitude, n.Longitude),
		CoordDecimal: util.FormatCoordinateDecimal(n.Latitude, n.Longitude),
	}
}

// UpdateNode updates or adds a node
func (nm *NodeManager) UpdateNode(node *Node) {
	nm.mu.Lock()
	nm.nodes[node.Num] = node
	nm.mu.Unlock()

	// Broadcast update
	nm.wsHub.Broadcast(websocket.Event{
		Type: "node.updated",
		Data: map[string]interface{}{
			"num":  node.Num,
			"node": node,
		},
	})
}

// RemoveNode removes a node
func (nm *NodeManager) RemoveNode(num uint32) {
	nm.mu.Lock()
	delete(nm.nodes, num)
	nm.mu.Unlock()

	// Broadcast removal
	nm.wsHub.Broadcast(websocket.Event{
		Type: "node.removed",
		Data: map[string]interface{}{
			"num": num,
		},
	})
}

// SetFavorite sets a node's favorite status
func (nm *NodeManager) SetFavorite(num uint32, favorite bool) error {
	nm.mu.Lock()
	if node, ok := nm.nodes[num]; ok {
		node.IsFavorite = favorite
	}
	nm.mu.Unlock()
	// TODO: persist to database
	return nil
}

// SetIgnored sets a node's ignored status
func (nm *NodeManager) SetIgnored(num uint32, ignored bool) error {
	nm.mu.Lock()
	if node, ok := nm.nodes[num]; ok {
		node.IsIgnored = ignored
	}
	nm.mu.Unlock()
	// TODO: persist to database
	return nil
}

// SetMuted sets a node's muted status
func (nm *NodeManager) SetMuted(num uint32, muted bool) error {
	nm.mu.Lock()
	if node, ok := nm.nodes[num]; ok {
		node.IsMuted = muted
	}
	nm.mu.Unlock()
	// TODO: persist to database
	return nil
}

// SetNotes sets a node's notes
func (nm *NodeManager) SetNotes(num uint32, notes string) error {
	nm.mu.Lock()
	if node, ok := nm.nodes[num]; ok {
		node.Notes = notes
	}
	nm.mu.Unlock()
	// TODO: persist to database
	return nil
}

// ProcessNodeInfo processes a NodeInfo protobuf message and returns the updated Node
func (nm *NodeManager) ProcessNodeInfo(nodeInfo *pb.NodeInfo) *Node {
	if nodeInfo == nil {
		return nil
	}

	nm.mu.Lock()
	defer nm.mu.Unlock()

	num := nodeInfo.Num
	node, exists := nm.nodes[num]
	if !exists {
		node = &Node{
			Num:      num,
			HopsAway: -1,
		}
		nm.nodes[num] = node
	}

	// Update from user info
	if nodeInfo.User != nil {
		node.LongName = nodeInfo.User.LongName
		node.ShortName = nodeInfo.User.ShortName
		node.HardwareModel = nodeInfo.User.HwModel.String()
		node.IsLicensed = nodeInfo.User.IsLicensed
		if len(nodeInfo.User.PublicKey) > 0 {
			node.PublicKey = fmt.Sprintf("%x", nodeInfo.User.PublicKey)
		}
	}

	// Update from position
	if nodeInfo.Position != nil {
		node.Latitude = nodeInfo.Position.Latitude()
		node.Longitude = nodeInfo.Position.Longitude()
		node.Altitude = nodeInfo.Position.Altitude
	}

	// Update signal info
	if nodeInfo.Snr != 0 {
		node.SNR = nodeInfo.Snr
	}

	// Update last heard
	if nodeInfo.LastHeard != 0 {
		node.LastHeard = int64(nodeInfo.LastHeard)
	} else {
		node.LastHeard = time.Now().Unix()
	}

	// Update channel and other info
	node.Channel = int32(nodeInfo.Channel)
	node.ViaMqtt = nodeInfo.ViaMqtt
	node.IsFavorite = nodeInfo.IsFavorite

	// Set hops away from the NodeInfo
	// The frontend will use viaMqtt flag to determine display (MQTT vs Direct)
	node.HopsAway = int32(nodeInfo.Hops)

	// Update device metrics if available
	if nodeInfo.DeviceMetrics != nil {
		node.BatteryLevel = int32(nodeInfo.DeviceMetrics.BatteryLevel)
		node.Voltage = nodeInfo.DeviceMetrics.Voltage
		node.ChannelUtilization = nodeInfo.DeviceMetrics.ChannelUtilization
		node.AirUtilTx = nodeInfo.DeviceMetrics.AirUtilTx
	}

	log.Debug().
		Uint32("num", num).
		Str("longName", node.LongName).
		Str("shortName", node.ShortName).
		Bool("viaMqtt", node.ViaMqtt).
		Bool("nodeInfoViaMqtt", nodeInfo.ViaMqtt).
		Msg("processed node info")

	return node
}

// UpdateNodePosition updates a node's position from a Position protobuf
func (nm *NodeManager) UpdateNodePosition(nodeNum uint32, position *pb.Position, viaMqtt bool) {
	if position == nil {
		return
	}

	nm.mu.Lock()
	defer nm.mu.Unlock()

	node, exists := nm.nodes[nodeNum]
	if !exists {
		node = &Node{
			Num:      nodeNum,
			HopsAway: -1,
		}
		nm.nodes[nodeNum] = node
	}

	node.Latitude = position.Latitude()
	node.Longitude = position.Longitude()
	node.Altitude = position.Altitude
	node.LastHeard = time.Now().Unix()
	node.ViaMqtt = viaMqtt

	log.Debug().
		Uint32("num", nodeNum).
		Float64("lat", node.Latitude).
		Float64("lon", node.Longitude).
		Bool("viaMqtt", viaMqtt).
		Msg("updated node position")
}

// UpdateNodeTelemetry updates a node's telemetry from a Telemetry protobuf
func (nm *NodeManager) UpdateNodeTelemetry(nodeNum uint32, telemetry *pb.Telemetry, viaMqtt bool) {
	if telemetry == nil {
		return
	}

	nm.mu.Lock()
	defer nm.mu.Unlock()

	node, exists := nm.nodes[nodeNum]
	if !exists {
		node = &Node{
			Num:      nodeNum,
			HopsAway: -1,
		}
		nm.nodes[nodeNum] = node
	}

	// Update device metrics - only update non-zero values to preserve existing data
	if dm := telemetry.GetDeviceMetrics(); dm != nil {
		if dm.BatteryLevel != 0 {
			node.BatteryLevel = int32(dm.BatteryLevel)
		}
		if dm.Voltage != 0 {
			node.Voltage = dm.Voltage
		}
		if dm.ChannelUtilization != 0 {
			node.ChannelUtilization = dm.ChannelUtilization
		}
		if dm.AirUtilTx != 0 {
			node.AirUtilTx = dm.AirUtilTx
		}
		if dm.UptimeSeconds != 0 {
			node.Uptime = int64(dm.UptimeSeconds)
		}
	}

	// Update environment metrics - only update non-zero values to preserve existing data
	if em := telemetry.GetEnvironmentMetrics(); em != nil {
		if em.Temperature != 0 {
			node.Temperature = em.Temperature
		}
		if em.RelativeHumidity != 0 {
			node.RelativeHumidity = em.RelativeHumidity
		}
		if em.BarometricPressure != 0 {
			node.BarometricPressure = em.BarometricPressure
		}
		if em.GasResistance != 0 {
			node.GasResistance = em.GasResistance
		}
		if em.Iaq != 0 {
			node.Iaq = int32(em.Iaq)
		}
	}

	node.LastHeard = time.Now().Unix()
	node.ViaMqtt = viaMqtt

	log.Debug().
		Uint32("num", nodeNum).
		Bool("viaMqtt", viaMqtt).
		Msg("updated node telemetry")
}

// UpdateNodeNeighbors updates a node's neighbor list from a NeighborInfo protobuf
func (nm *NodeManager) UpdateNodeNeighbors(nodeNum uint32, neighborInfo *pb.NeighborInfo) {
	if neighborInfo == nil {
		return
	}

	nm.mu.Lock()
	defer nm.mu.Unlock()

	node, exists := nm.nodes[nodeNum]
	if !exists {
		node = &Node{
			Num:      nodeNum,
			HopsAway: -1,
		}
		nm.nodes[nodeNum] = node
	}

	// Convert protobuf neighbors to our format
	neighbors := make([]*NodeNeighbor, 0, len(neighborInfo.Neighbors))
	for _, n := range neighborInfo.Neighbors {
		neighbors = append(neighbors, &NodeNeighbor{
			NodeId:     n.NodeId,
			SNR:        n.Snr,
			LastRxTime: int64(n.LastRxTime),
		})
	}
	node.Neighbors = neighbors
	node.LastHeard = time.Now().Unix()

	log.Debug().
		Uint32("num", nodeNum).
		Int("neighborCount", len(neighbors)).
		Msg("updated node neighbors")
}
