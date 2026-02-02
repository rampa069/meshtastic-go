package service

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/meshtastic/meshtastic-go/internal/database"
	"github.com/meshtastic/meshtastic-go/internal/database/dao"
	"github.com/meshtastic/meshtastic-go/internal/websocket"
	"github.com/rs/zerolog/log"
)

// MessageStatus represents the delivery status of a message
type MessageStatus int

const (
	MessageStatusUnknown MessageStatus = iota
	MessageStatusQueued
	MessageStatusEnRoute
	MessageStatusDelivered
	MessageStatusError
)

func (s MessageStatus) String() string {
	switch s {
	case MessageStatusQueued:
		return "queued"
	case MessageStatusEnRoute:
		return "enroute"
	case MessageStatusDelivered:
		return "delivered"
	case MessageStatusError:
		return "error"
	default:
		return "unknown"
	}
}

// Message represents a chat message
type Message struct {
	UUID         int64         `json:"uuid"`
	ContactKey   string        `json:"contactKey"`
	From         int32         `json:"from"`
	To           int32         `json:"to"`
	Channel      int32         `json:"channel"`
	Text         string        `json:"text"`
	Time         int64         `json:"time"`
	ReceivedTime int64         `json:"receivedTime"`
	Read         bool          `json:"read"`
	Status       MessageStatus `json:"status"`
	PacketID     int32         `json:"packetId"`
	ReplyTo      int32         `json:"replyTo,omitempty"` // Packet ID of message being replied to
	SNR          float32       `json:"snr"`
	RSSI         int32         `json:"rssi"`
	HopsAway     int32         `json:"hopsAway"`
	ViaMqtt      bool          `json:"viaMqtt"`
	Emojis       []Reaction    `json:"emojis,omitempty"`
}

// Reaction represents an emoji reaction to a message
type Reaction struct {
	UserID    string `json:"userId"`
	Emoji     string `json:"emoji"`
	Timestamp int64  `json:"timestamp"`
}

// Contact represents a conversation contact
type Contact struct {
	ContactKey     string   `json:"contactKey"`
	LastMessage    *Message `json:"lastMessage,omitempty"`
	UnreadCount    int      `json:"unreadCount"`
	MuteUntil      int64    `json:"muteUntil"`
	IsMuted        bool     `json:"isMuted"`
}

// MessageService manages messages and conversations
type MessageService struct {
	mu         sync.RWMutex
	db         *database.Database
	messageDAO *dao.MessageDAO
	wsHub      *websocket.Hub
	messages   map[string][]*Message // contactKey -> messages
	myNodeNum  uint32
	nextUUID   int64
}

// NewMessageService creates a new message service
func NewMessageService(db *database.Database, wsHub *websocket.Hub) *MessageService {
	return &MessageService{
		db:         db,
		messageDAO: dao.NewMessageDAO(db.DB()),
		wsHub:      wsHub,
		messages:   make(map[string][]*Message),
		nextUUID:   1,
	}
}

// ProcessTextMessage processes an incoming text message and returns the Message object
func (ms *MessageService) ProcessTextMessage(from, to, channel uint32, text string, rxTime, packetId uint32) *Message {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	// Generate contact key
	contactKey := fmt.Sprintf("!%x", from)
	if to != 0xFFFFFFFF { // Not broadcast
		contactKey = fmt.Sprintf("!%x", to)
	}

	// Create message
	msg := &Message{
		UUID:         ms.nextUUID,
		ContactKey:   contactKey,
		From:         int32(from),
		To:           int32(to),
		Channel:      int32(channel),
		Text:         text,
		Time:         int64(rxTime),
		ReceivedTime: time.Now().Unix(),
		Read:         false,
		Status:       MessageStatusDelivered,
		PacketID:     int32(packetId),
	}
	ms.nextUUID++

	// If rxTime is 0, use current time
	if msg.Time == 0 {
		msg.Time = time.Now().Unix()
	}

	// Persist to SQLite
	entity := messageToEntity(msg, ms.myNodeNum)
	if uuid, err := ms.messageDAO.Insert(entity); err != nil {
		log.Error().Err(err).Msg("failed to persist message")
	} else {
		msg.UUID = uuid
	}

	// Store in memory
	ms.messages[contactKey] = append(ms.messages[contactKey], msg)

	log.Info().
		Str("contactKey", contactKey).
		Int32("from", msg.From).
		Int32("to", msg.To).
		Str("text", text).
		Msg("processed text message")

	return msg
}

// SaveSentMessage saves a message that was sent by the local node
func (ms *MessageService) SaveSentMessage(to, channel, packetId uint32, text string) *Message {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	// Generate contact key based on destination
	contactKey := fmt.Sprintf("!%x", to)
	if to == 0xFFFFFFFF { // Broadcast
		contactKey = fmt.Sprintf("channel:%d", channel)
	}

	now := time.Now().Unix()
	msg := &Message{
		UUID:         ms.nextUUID,
		ContactKey:   contactKey,
		From:         int32(ms.myNodeNum),
		To:           int32(to),
		Channel:      int32(channel),
		Text:         text,
		Time:         now,
		ReceivedTime: now,
		Read:         true, // Sent messages are always "read"
		Status:       MessageStatusQueued,
		PacketID:     int32(packetId),
	}
	ms.nextUUID++

	// Persist to SQLite
	entity := messageToEntity(msg, ms.myNodeNum)
	if uuid, err := ms.messageDAO.Insert(entity); err != nil {
		log.Error().Err(err).Msg("failed to persist sent message")
	} else {
		msg.UUID = uuid
	}

	// Store in memory
	ms.messages[contactKey] = append(ms.messages[contactKey], msg)

	log.Info().
		Str("contactKey", contactKey).
		Int32("to", msg.To).
		Str("text", text).
		Uint32("packetId", packetId).
		Msg("saved sent message")

	return msg
}

// GetContacts returns all contacts with their last message
func (ms *MessageService) GetContacts() ([]*Contact, error) {
	// TODO: implement database query
	return nil, nil
}

// GetMessages returns messages for a contact
func (ms *MessageService) GetMessages(contactKey string, limit, offset int) ([]*Message, error) {
	ms.mu.RLock()
	defer ms.mu.RUnlock()

	msgs, ok := ms.messages[contactKey]
	if !ok {
		return []*Message{}, nil
	}

	// Apply offset and limit
	start := offset
	if start >= len(msgs) {
		return []*Message{}, nil
	}

	end := start + limit
	if end > len(msgs) || limit <= 0 {
		end = len(msgs)
	}

	return msgs[start:end], nil
}

// GetMessagesBefore returns messages before a specific timestamp (for pagination)
func (ms *MessageService) GetMessagesBefore(contactKey string, before int64, limit int) ([]*Message, error) {
	ms.mu.RLock()
	defer ms.mu.RUnlock()

	msgs, ok := ms.messages[contactKey]
	if !ok {
		return []*Message{}, nil
	}

	// Filter messages before the timestamp
	var result []*Message
	for _, msg := range msgs {
		if msg.Time < before {
			result = append(result, msg)
		}
	}

	// Sort by time descending and limit
	// Messages should already be roughly in order, but ensure we get the most recent ones before the cutoff
	if len(result) > limit && limit > 0 {
		// Take the last 'limit' messages (most recent before cutoff)
		result = result[len(result)-limit:]
	}

	return result, nil
}

// GetChannelMessages returns messages for a specific channel
func (ms *MessageService) GetChannelMessages(channelIndex, limit, offset int) ([]*Message, error) {
	ms.mu.RLock()
	defer ms.mu.RUnlock()

	// Collect all messages from this channel
	var channelMsgs []*Message
	for _, msgs := range ms.messages {
		for _, msg := range msgs {
			if int(msg.Channel) == channelIndex {
				channelMsgs = append(channelMsgs, msg)
			}
		}
	}

	// Sort by time (newest first for consistency)
	// Messages are already roughly in order, but let's ensure it

	// Apply offset and limit
	start := offset
	if start >= len(channelMsgs) {
		return []*Message{}, nil
	}

	end := start + limit
	if end > len(channelMsgs) || limit <= 0 {
		end = len(channelMsgs)
	}

	return channelMsgs[start:end], nil
}

// GetUnreadCount returns the number of unread messages for a contact
func (ms *MessageService) GetUnreadCount(contactKey string) (int, error) {
	// TODO: implement database query
	return 0, nil
}

// GetTotalUnreadCount returns the total number of unread messages
func (ms *MessageService) GetTotalUnreadCount() (int, error) {
	// TODO: implement database query
	return 0, nil
}

// MarkAsRead marks messages as read up to a timestamp
func (ms *MessageService) MarkAsRead(contactKey string, timestamp int64) error {
	// TODO: implement database update
	return nil
}

// MuteContact mutes a contact until the specified time
func (ms *MessageService) MuteContact(contactKey string, until int64) error {
	// TODO: implement database update
	return nil
}

// DeleteMessages deletes messages by UUID
func (ms *MessageService) DeleteMessages(uuids []int64) error {
	// TODO: implement database delete
	return nil
}

// DeleteContact deletes a contact and all its messages
func (ms *MessageService) DeleteContact(contactKey string) error {
	// TODO: implement database delete
	return nil
}

// AddReaction adds a reaction to a message
func (ms *MessageService) AddReaction(packetID int32, userID, emoji string) error {
	// TODO: implement database insert
	return nil
}

// SaveMessage saves a message to the database
func (ms *MessageService) SaveMessage(msg *Message) error {
	// TODO: implement database insert

	// Broadcast to websocket
	ms.wsHub.Broadcast(websocket.Event{
		Type: "message.received",
		Data: map[string]interface{}{
			"uuid":       msg.UUID,
			"contactKey": msg.ContactKey,
			"message":    msg,
		},
	})

	return nil
}

// UpdateMessageStatus updates a message's delivery status
func (ms *MessageService) UpdateMessageStatus(packetID int32, status MessageStatus) error {
	// TODO: implement database update

	// Broadcast to websocket
	ms.wsHub.Broadcast(websocket.Event{
		Type: "message.status",
		Data: map[string]interface{}{
			"id":     packetID,
			"status": status.String(),
		},
	})

	return nil
}

// SetMyNodeNum sets the local node number and loads historical messages
func (ms *MessageService) SetMyNodeNum(nodeNum uint32) error {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	ms.myNodeNum = nodeNum
	ms.messages = make(map[string][]*Message) // Clear memory

	// Load historical messages from SQLite for this node
	entities, err := ms.messageDAO.GetAllByNode(nodeNum, 10000)
	if err != nil {
		return fmt.Errorf("failed to load messages: %w", err)
	}

	// Convert and group by contactKey
	var maxUUID int64
	for _, entity := range entities {
		msg := entityToMessage(entity)
		ms.messages[msg.ContactKey] = append(ms.messages[msg.ContactKey], msg)
		if msg.UUID > maxUUID {
			maxUUID = msg.UUID
		}
	}

	// Set nextUUID to max+1 to avoid collisions
	ms.nextUUID = maxUUID + 1

	log.Info().
		Uint32("nodeNum", nodeNum).
		Int("messagesLoaded", len(entities)).
		Int("contacts", len(ms.messages)).
		Msg("loaded historical messages")

	return nil
}

// messageToEntity converts a Message to a MessageEntity for persistence
func messageToEntity(msg *Message, myNodeNum uint32) *dao.MessageEntity {
	// Store message data as JSON
	data, _ := json.Marshal(map[string]interface{}{
		"text":    msg.Text,
		"from":    msg.From,
		"to":      msg.To,
		"channel": msg.Channel,
		"time":    msg.Time,
		"replyTo": msg.ReplyTo,
		"status":  msg.Status,
		"viaMqtt": msg.ViaMqtt,
	})

	return &dao.MessageEntity{
		UUID:         msg.UUID,
		MyNodeNum:    myNodeNum,
		PortNum:      1, // TEXT_MESSAGE_APP
		ContactKey:   msg.ContactKey,
		ReceivedTime: msg.ReceivedTime,
		Read:         msg.Read,
		Data:         string(data),
		PacketId:     uint32(msg.PacketID),
		RoutingError: -1,
		SNR:          float64(msg.SNR),
		RSSI:         msg.RSSI,
		HopsAway:     msg.HopsAway,
		Filtered:     false,
	}
}

// entityToMessage converts a MessageEntity to a Message
func entityToMessage(entity *dao.MessageEntity) *Message {
	msg := &Message{
		UUID:         entity.UUID,
		ContactKey:   entity.ContactKey,
		ReceivedTime: entity.ReceivedTime,
		Read:         entity.Read,
		PacketID:     int32(entity.PacketId),
		SNR:          float32(entity.SNR),
		RSSI:         entity.RSSI,
		HopsAway:     entity.HopsAway,
	}

	// Parse JSON data
	var data map[string]interface{}
	if err := json.Unmarshal([]byte(entity.Data), &data); err == nil {
		if text, ok := data["text"].(string); ok {
			msg.Text = text
		}
		if from, ok := data["from"].(float64); ok {
			msg.From = int32(from)
		}
		if to, ok := data["to"].(float64); ok {
			msg.To = int32(to)
		}
		if channel, ok := data["channel"].(float64); ok {
			msg.Channel = int32(channel)
		}
		if t, ok := data["time"].(float64); ok {
			msg.Time = int64(t)
		}
		if replyTo, ok := data["replyTo"].(float64); ok {
			msg.ReplyTo = int32(replyTo)
		}
		if status, ok := data["status"].(float64); ok {
			msg.Status = MessageStatus(status)
		}
		if viaMqtt, ok := data["viaMqtt"].(bool); ok {
			msg.ViaMqtt = viaMqtt
		}
	}

	return msg
}
