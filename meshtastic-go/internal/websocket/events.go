package websocket

// Event represents a WebSocket event
type Event struct {
	Type string      `json:"type"`
	Data interface{} `json:"data,omitempty"`
}

// Event types
const (
	// Connection events
	EventConnectionState = "connection.state"
	EventConnectionError = "connection.error"

	// Node events
	EventNodeUpdated   = "node.updated"
	EventNodeRemoved   = "node.removed"
	EventMyNodeUpdated = "mynode.updated"

	// Message events
	EventMessageReceived = "message.received"
	EventMessageSent     = "message.sent"
	EventMessageStatus   = "message.status"
	EventMessageReaction = "message.reaction"

	// Channel events
	EventChannelUpdated = "channel.updated"
	EventChannelsSet    = "channels.set"

	// Config events
	EventConfigUpdated = "config.updated"
	EventModuleUpdated = "module.updated"

	// Telemetry events
	EventTelemetryDevice      = "telemetry.device"
	EventTelemetryEnvironment = "telemetry.environment"
	EventTelemetryPosition    = "telemetry.position"

	// Diagnostic events
	EventTracerouteUpdate   = "traceroute.update"
	EventTracerouteComplete = "traceroute.complete"
	EventNeighborInfo       = "neighbor-info.received"

	// Debug events
	EventPacketFromRadio = "packet.from_radio"
	EventPacketToRadio   = "packet.to_radio"
)
