package service

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/meshtastic/meshtastic-go/internal/config"
	"github.com/meshtastic/meshtastic-go/internal/websocket"
	"github.com/meshtastic/meshtastic-go/pkg/pb"
	"github.com/rs/zerolog/log"
)

// MQTTBridge handles bidirectional MQTT communication
type MQTTBridge struct {
	mu          sync.RWMutex
	config      config.MQTTConfig
	client      mqtt.Client
	wsHub       *websocket.Hub
	meshService *MeshService
	connected   bool
	channels    []string // Channel names to subscribe to
}

// MQTTMessage represents a JSON message from MQTT
type MQTTMessage struct {
	From      uint32                 `json:"from"`
	To        uint32                 `json:"to"`
	Channel   uint32                 `json:"channel"`
	ID        uint32                 `json:"id"`
	Type      string                 `json:"type"`
	Sender    string                 `json:"sender,omitempty"`
	Timestamp int64                  `json:"timestamp,omitempty"`
	HopLimit  uint32                 `json:"hopLimit,omitempty"`
	HopStart  uint32                 `json:"hopStart,omitempty"`
	Payload   map[string]interface{} `json:"payload,omitempty"`
}

// MQTTServiceEnvelope wraps protobuf messages for MQTT
type MQTTServiceEnvelope struct {
	Packet       []byte `json:"packet,omitempty"`
	ChannelID    string `json:"channelId,omitempty"`
	GatewayID    string `json:"gatewayId,omitempty"`
}

// NewMQTTBridge creates a new MQTT bridge
func NewMQTTBridge(cfg config.MQTTConfig, wsHub *websocket.Hub) *MQTTBridge {
	return &MQTTBridge{
		config:   cfg,
		wsHub:    wsHub,
		channels: []string{"LongFast"}, // Default channel
	}
}

// SetMeshService sets the mesh service for sending messages
func (b *MQTTBridge) SetMeshService(ms *MeshService) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.meshService = ms
}

// SetChannels sets the channel names to subscribe to
func (b *MQTTBridge) SetChannels(channels []string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.channels = channels
}

// Connect connects to the MQTT broker
func (b *MQTTBridge) Connect() error {
	if !b.config.Enabled {
		log.Info().Msg("MQTT bridge disabled")
		return nil
	}

	broker := fmt.Sprintf("tcp://%s:%d", b.config.Server, b.config.Port)

	opts := mqtt.NewClientOptions()
	opts.AddBroker(broker)
	opts.SetClientID(fmt.Sprintf("meshtastic-go-%d", time.Now().UnixNano()))

	if b.config.Username != "" {
		opts.SetUsername(b.config.Username)
		opts.SetPassword(b.config.Password)
	}

	// TLS config for secure connections
	if b.config.Port == 8883 {
		opts.SetTLSConfig(&tls.Config{
			InsecureSkipVerify: false,
		})
	}

	opts.SetAutoReconnect(true)
	opts.SetConnectRetry(true)
	opts.SetConnectRetryInterval(5 * time.Second)
	opts.SetKeepAlive(60 * time.Second)

	opts.SetOnConnectHandler(b.onConnect)
	opts.SetConnectionLostHandler(b.onConnectionLost)
	opts.SetDefaultPublishHandler(b.onMessage)

	b.client = mqtt.NewClient(opts)

	log.Info().
		Str("broker", broker).
		Str("rootTopic", b.config.RootTopic).
		Msg("connecting to MQTT broker")

	token := b.client.Connect()
	if token.Wait() && token.Error() != nil {
		return fmt.Errorf("failed to connect to MQTT broker: %w", token.Error())
	}

	return nil
}

// Disconnect disconnects from the MQTT broker
func (b *MQTTBridge) Disconnect() {
	if b.client != nil && b.client.IsConnected() {
		b.client.Disconnect(1000)
		log.Info().Msg("disconnected from MQTT broker")
	}
	b.mu.Lock()
	b.connected = false
	b.mu.Unlock()
}

// IsConnected returns true if connected to MQTT broker
func (b *MQTTBridge) IsConnected() bool {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.connected
}

// onConnect handles successful connection
func (b *MQTTBridge) onConnect(client mqtt.Client) {
	b.mu.Lock()
	b.connected = true
	channels := b.channels
	b.mu.Unlock()

	log.Info().Msg("connected to MQTT broker")

	// Subscribe to JSON topics for each channel
	for _, channel := range channels {
		jsonTopic := fmt.Sprintf("%s%s/#", b.config.JSONTopic, channel)
		token := client.Subscribe(jsonTopic, 0, nil)
		if token.Wait() && token.Error() != nil {
			log.Error().Err(token.Error()).Str("topic", jsonTopic).Msg("failed to subscribe")
		} else {
			log.Info().Str("topic", jsonTopic).Msg("subscribed to MQTT topic")
		}

		// Also subscribe to protobuf topic
		protoTopic := fmt.Sprintf("%s%s/#", b.config.RootTopic, channel)
		token = client.Subscribe(protoTopic, 0, nil)
		if token.Wait() && token.Error() != nil {
			log.Error().Err(token.Error()).Str("topic", protoTopic).Msg("failed to subscribe")
		} else {
			log.Info().Str("topic", protoTopic).Msg("subscribed to MQTT topic")
		}
	}

	// Broadcast connection status
	b.wsHub.Broadcast(websocket.Event{
		Type: "mqtt.connected",
		Data: map[string]interface{}{
			"server": b.config.Server,
		},
	})
}

// onConnectionLost handles connection loss
func (b *MQTTBridge) onConnectionLost(client mqtt.Client, err error) {
	b.mu.Lock()
	b.connected = false
	b.mu.Unlock()

	log.Warn().Err(err).Msg("MQTT connection lost")

	b.wsHub.Broadcast(websocket.Event{
		Type: "mqtt.disconnected",
		Data: map[string]interface{}{
			"error": err.Error(),
		},
	})
}

// onMessage handles incoming MQTT messages
func (b *MQTTBridge) onMessage(client mqtt.Client, msg mqtt.Message) {
	topic := msg.Topic()
	payload := msg.Payload()

	log.Debug().
		Str("topic", topic).
		Int("payloadLen", len(payload)).
		Msg("received MQTT message")

	// Determine if it's a JSON or protobuf message
	if strings.Contains(topic, "/json/") {
		b.handleJSONMessage(topic, payload)
	} else {
		b.handleProtobufMessage(topic, payload)
	}
}

// handleJSONMessage processes JSON format MQTT messages
func (b *MQTTBridge) handleJSONMessage(topic string, payload []byte) {
	var msg MQTTMessage
	if err := json.Unmarshal(payload, &msg); err != nil {
		log.Error().Err(err).Str("topic", topic).Msg("failed to parse JSON message")
		return
	}

	log.Debug().
		Uint32("from", msg.From).
		Uint32("to", msg.To).
		Str("type", msg.Type).
		Msg("received MQTT JSON message")

	// Broadcast to websocket clients
	b.wsHub.Broadcast(websocket.Event{
		Type: "mqtt.message",
		Data: map[string]interface{}{
			"topic":   topic,
			"message": msg,
		},
	})

	// Process based on message type
	switch msg.Type {
	case "text":
		b.handleTextMessage(msg)
	case "position":
		b.handlePositionMessage(msg)
	case "telemetry":
		b.handleTelemetryMessage(msg)
	case "nodeinfo":
		b.handleNodeInfoMessage(msg)
	}
}

// handleProtobufMessage processes protobuf format MQTT messages
func (b *MQTTBridge) handleProtobufMessage(topic string, payload []byte) {
	// Parse ServiceEnvelope
	envelope, err := pb.UnmarshalServiceEnvelope(payload)
	if err != nil {
		log.Debug().Err(err).Msg("failed to parse protobuf service envelope")
		return
	}

	if envelope.Packet == nil {
		return
	}

	log.Debug().
		Uint32("from", envelope.Packet.From).
		Uint32("to", envelope.Packet.To).
		Str("channelId", envelope.ChannelId).
		Msg("received MQTT protobuf message")

	// Process the packet similar to local radio messages
	// This would be handled by the mesh service
}

// handleTextMessage processes text messages from MQTT
func (b *MQTTBridge) handleTextMessage(msg MQTTMessage) {
	text, ok := msg.Payload["text"].(string)
	if !ok {
		return
	}

	log.Info().
		Uint32("from", msg.From).
		Str("text", text).
		Bool("viaMqtt", true).
		Msg("received text message via MQTT")

	// Create a message for the UI
	b.wsHub.Broadcast(websocket.Event{
		Type: "message.received",
		Data: map[string]interface{}{
			"from":     msg.From,
			"to":       msg.To,
			"channel":  msg.Channel,
			"text":     text,
			"time":     msg.Timestamp,
			"viaMqtt":  true,
			"sender":   msg.Sender,
		},
	})
}

// handlePositionMessage processes position messages from MQTT
func (b *MQTTBridge) handlePositionMessage(msg MQTTMessage) {
	lat, _ := msg.Payload["latitude_i"].(float64)
	lon, _ := msg.Payload["longitude_i"].(float64)
	alt, _ := msg.Payload["altitude"].(float64)

	// Convert from integer format if needed
	if lat > 1000 || lat < -1000 {
		lat = lat * 1e-7
	}
	if lon > 1000 || lon < -1000 {
		lon = lon * 1e-7
	}

	log.Debug().
		Uint32("from", msg.From).
		Float64("lat", lat).
		Float64("lon", lon).
		Msg("received position via MQTT")

	b.wsHub.Broadcast(websocket.Event{
		Type: "position.updated",
		Data: map[string]interface{}{
			"nodeNum":   msg.From,
			"latitude":  lat,
			"longitude": lon,
			"altitude":  int32(alt),
			"viaMqtt":   true,
		},
	})
}

// handleTelemetryMessage processes telemetry messages from MQTT
func (b *MQTTBridge) handleTelemetryMessage(msg MQTTMessage) {
	log.Debug().
		Uint32("from", msg.From).
		Interface("payload", msg.Payload).
		Msg("received telemetry via MQTT")

	b.wsHub.Broadcast(websocket.Event{
		Type: "telemetry.received",
		Data: map[string]interface{}{
			"nodeNum": msg.From,
			"data":    msg.Payload,
			"viaMqtt": true,
		},
	})
}

// handleNodeInfoMessage processes node info messages from MQTT
func (b *MQTTBridge) handleNodeInfoMessage(msg MQTTMessage) {
	log.Debug().
		Uint32("from", msg.From).
		Interface("payload", msg.Payload).
		Msg("received nodeinfo via MQTT")

	// Extract user info
	longName, _ := msg.Payload["longname"].(string)
	shortName, _ := msg.Payload["shortname"].(string)
	hwModel, _ := msg.Payload["hardware"].(string)

	b.wsHub.Broadcast(websocket.Event{
		Type: "node.updated",
		Data: map[string]interface{}{
			"num":           msg.From,
			"longName":      longName,
			"shortName":     shortName,
			"hardwareModel": hwModel,
			"viaMqtt":       true,
		},
	})
}

// PublishMessage publishes a message to MQTT
func (b *MQTTBridge) PublishMessage(channelName string, packet *pb.MeshPacket) error {
	if !b.IsConnected() {
		return fmt.Errorf("not connected to MQTT")
	}

	b.mu.RLock()
	meshService := b.meshService
	b.mu.RUnlock()

	// Get gateway ID (our node number)
	var gatewayID string
	if meshService != nil {
		nodeNum := meshService.GetMyNodeNum()
		if nodeNum != 0 {
			gatewayID = fmt.Sprintf("!%08x", nodeNum)
		}
	}
	if gatewayID == "" {
		gatewayID = "!unknown"
	}

	// Publish to JSON topic
	jsonTopic := fmt.Sprintf("%s%s/%s", b.config.JSONTopic, channelName, gatewayID)

	msg := MQTTMessage{
		From:      packet.From,
		To:        packet.To,
		Channel:   packet.Channel,
		ID:        packet.Id,
		HopLimit:  packet.HopLimit,
		HopStart:  packet.HopStart,
		Timestamp: time.Now().Unix(),
	}

	// Determine message type and payload from decoded data
	if decoded := packet.GetDecoded(); decoded != nil {
		switch decoded.Portnum {
		case pb.PortNum_TEXT_MESSAGE_APP:
			msg.Type = "text"
			msg.Payload = map[string]interface{}{
				"text": string(decoded.Payload),
			}
		case pb.PortNum_POSITION_APP:
			msg.Type = "position"
			if pos, err := pb.UnmarshalPosition(decoded.Payload); err == nil {
				msg.Payload = map[string]interface{}{
					"latitude_i":  pos.LatitudeI,
					"longitude_i": pos.LongitudeI,
					"altitude":    pos.Altitude,
					"time":        pos.Time,
				}
			}
		case pb.PortNum_TELEMETRY_APP:
			msg.Type = "telemetry"
			// Would need to unmarshal telemetry data
		case pb.PortNum_NODEINFO_APP:
			msg.Type = "nodeinfo"
			// Would need to unmarshal nodeinfo data
		default:
			msg.Type = decoded.Portnum.String()
		}
	}

	jsonData, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	token := b.client.Publish(jsonTopic, 0, false, jsonData)
	if token.Wait() && token.Error() != nil {
		return fmt.Errorf("failed to publish: %w", token.Error())
	}

	log.Debug().
		Str("topic", jsonTopic).
		Str("type", msg.Type).
		Msg("published message to MQTT")

	return nil
}

// PublishProtobuf publishes a protobuf message to MQTT
func (b *MQTTBridge) PublishProtobuf(channelName string, packet *pb.MeshPacket) error {
	if !b.IsConnected() {
		return fmt.Errorf("not connected to MQTT")
	}

	b.mu.RLock()
	meshService := b.meshService
	b.mu.RUnlock()

	// Get gateway ID
	var gatewayID string
	if meshService != nil {
		nodeNum := meshService.GetMyNodeNum()
		if nodeNum != 0 {
			gatewayID = fmt.Sprintf("!%08x", nodeNum)
		}
	}
	if gatewayID == "" {
		gatewayID = "!unknown"
	}

	// Create ServiceEnvelope
	packetData, err := pb.MarshalMeshPacket(packet)
	if err != nil {
		return fmt.Errorf("failed to marshal packet: %w", err)
	}

	envelope := &pb.ServiceEnvelope{
		Packet:    packet,
		ChannelId: channelName,
		GatewayId: gatewayID,
	}

	envelopeData, err := pb.MarshalServiceEnvelope(envelope)
	if err != nil {
		return fmt.Errorf("failed to marshal envelope: %w", err)
	}

	// Publish to protobuf topic
	protoTopic := fmt.Sprintf("%s%s/%s", b.config.RootTopic, channelName, gatewayID)

	token := b.client.Publish(protoTopic, 0, false, envelopeData)
	if token.Wait() && token.Error() != nil {
		return fmt.Errorf("failed to publish: %w", token.Error())
	}

	log.Debug().
		Str("topic", protoTopic).
		Int("size", len(packetData)).
		Msg("published protobuf to MQTT")

	return nil
}

// GetStatus returns the current MQTT bridge status
func (b *MQTTBridge) GetStatus() map[string]interface{} {
	b.mu.RLock()
	defer b.mu.RUnlock()

	return map[string]interface{}{
		"enabled":   b.config.Enabled,
		"connected": b.connected,
		"server":    b.config.Server,
		"port":      b.config.Port,
		"channels":  b.channels,
	}
}
