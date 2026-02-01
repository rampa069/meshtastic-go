package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/meshtastic/meshtastic-go/internal/service"
)

// MessageHandler handles message-related requests
type MessageHandler struct {
	meshService *service.MeshService
}

// NewMessageHandler creates a new message handler
func NewMessageHandler(meshService *service.MeshService) *MessageHandler {
	return &MessageHandler{meshService: meshService}
}

// GetContacts returns all contacts
func (h *MessageHandler) GetContacts(c *gin.Context) {
	contacts, err := h.meshService.MessageService().GetContacts()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"contacts": contacts})
}

// GetMessages returns messages for a contact
func (h *MessageHandler) GetMessages(c *gin.Context) {
	contactKey := c.Param("contactKey")
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	before, _ := strconv.ParseInt(c.DefaultQuery("before", "0"), 10, 64)

	var messages []*service.Message
	var err error

	if before > 0 {
		// Load messages before a specific timestamp (for infinite scroll)
		messages, err = h.meshService.MessageService().GetMessagesBefore(contactKey, before, limit)
	} else {
		messages, err = h.meshService.MessageService().GetMessages(contactKey, limit, offset)
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"messages": messages})
}

// GetChannelMessages returns messages for a specific channel
func (h *MessageHandler) GetChannelMessages(c *gin.Context) {
	channelIndex, err := strconv.Atoi(c.Param("channelIndex"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid channel index"})
		return
	}
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	messages, err := h.meshService.MessageService().GetChannelMessages(channelIndex, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"messages": messages})
}

// SendMessageRequest represents a message send request
type SendMessageRequest struct {
	To      interface{} `json:"to" binding:"required"` // Can be string "!hex" or number
	Channel int         `json:"channel"`
	Text    string      `json:"text" binding:"required"`
	ReplyTo *uint32     `json:"replyTo,omitempty"` // Packet ID of message being replied to
}

// SendMessage sends a message
func (h *MessageHandler) SendMessage(c *gin.Context) {
	var req SendMessageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Parse the destination node number
	var destNode uint32
	switch v := req.To.(type) {
	case float64:
		destNode = uint32(v)
	case string:
		// Parse hex string like "!1234abcd"
		if len(v) > 1 && v[0] == '!' {
			parsed, err := strconv.ParseUint(v[1:], 16, 32)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "invalid destination format"})
				return
			}
			destNode = uint32(parsed)
		} else {
			parsed, err := strconv.ParseUint(v, 10, 32)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "invalid destination format"})
				return
			}
			destNode = uint32(parsed)
		}
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid destination type"})
		return
	}

	// Send message via MeshService
	packetId, err := h.meshService.SendTextMessage(destNode, uint32(req.Channel), req.Text)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"status":   "queued",
		"packetId": packetId,
	})
}

// AddReaction adds a reaction to a message
func (h *MessageHandler) AddReaction(c *gin.Context) {
	packetId, err := strconv.ParseInt(c.Param("packetId"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid packet id"})
		return
	}

	var req struct {
		Emoji string `json:"emoji" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// TODO: send reaction via radio
	if err := h.meshService.MessageService().AddReaction(int32(packetId), "", req.Emoji); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// DeleteMessage deletes a message
func (h *MessageHandler) DeleteMessage(c *gin.Context) {
	uuid, err := strconv.ParseInt(c.Param("uuid"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid uuid"})
		return
	}

	if err := h.meshService.MessageService().DeleteMessages([]int64{uuid}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// MarkAsRead marks messages as read
func (h *MessageHandler) MarkAsRead(c *gin.Context) {
	contactKey := c.Param("contactKey")

	var req struct {
		Timestamp int64 `json:"timestamp"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.meshService.MessageService().MarkAsRead(contactKey, req.Timestamp); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// MuteContact mutes a contact
func (h *MessageHandler) MuteContact(c *gin.Context) {
	contactKey := c.Param("contactKey")

	var req struct {
		Until int64 `json:"until"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.meshService.MessageService().MuteContact(contactKey, req.Until); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// DeleteContact deletes a contact and all its messages
func (h *MessageHandler) DeleteContact(c *gin.Context) {
	contactKey := c.Param("contactKey")

	if err := h.meshService.MessageService().DeleteContact(contactKey); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}
