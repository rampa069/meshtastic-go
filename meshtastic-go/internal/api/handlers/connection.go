package handlers

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/meshtastic/meshtastic-go/internal/radio"
	"github.com/meshtastic/meshtastic-go/internal/service"
)

// ConnectionHandler handles connection-related requests
type ConnectionHandler struct {
	meshService *service.MeshService
}

// NewConnectionHandler creates a new connection handler
func NewConnectionHandler(meshService *service.MeshService) *ConnectionHandler {
	return &ConnectionHandler{meshService: meshService}
}

// GetStatus returns the current connection status
func (h *ConnectionHandler) GetStatus(c *gin.Context) {
	connType, address, connected := h.meshService.GetConnectionInfo()
	state := h.meshService.GetConnectionState()

	c.JSON(http.StatusOK, gin.H{
		"connected": connected,
		"state":     state.String(),
		"type":      string(connType),
		"address":   address,
	})
}

// ConnectRequest represents a connection request
type ConnectRequest struct {
	Type    string `json:"type" binding:"required"`
	Address string `json:"address" binding:"required"`
}

// Connect connects to a radio device
func (h *ConnectionHandler) Connect(c *gin.Context) {
	var req ConnectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.meshService.Connect(req.Type, req.Address); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "connected"})
}

// Disconnect disconnects from the radio
func (h *ConnectionHandler) Disconnect(c *gin.Context) {
	if err := h.meshService.Disconnect(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "disconnected"})
}

// GetDevices returns available devices for a connection type
func (h *ConnectionHandler) GetDevices(c *gin.Context) {
	connType := c.Query("type")
	streaming := c.Query("streaming") == "true"

	if connType == "" {
		// Return all device types
		ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
		defer cancel()

		allDevices := make([]radio.DeviceInfo, 0)

		// Scan serial ports (fast)
		if devices, err := h.meshService.ScanDevices(ctx, "serial", 1*time.Second); err == nil {
			allDevices = append(allDevices, devices...)
		}

		// Scan TCP/mDNS (slower)
		if devices, err := h.meshService.ScanDevices(ctx, "tcp", 5*time.Second); err == nil {
			allDevices = append(allDevices, devices...)
		}

		c.JSON(http.StatusOK, gin.H{"devices": allDevices})
		return
	}

	// For BLE with streaming=true, start async scan and return immediately
	// Devices will be sent via WebSocket
	if connType == "ble" && streaming {
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			h.meshService.ScanDevicesStreaming(ctx, connType, 10*time.Second)
		}()
		c.JSON(http.StatusOK, gin.H{
			"status":  "scanning",
			"message": "BLE scan started. Devices will be sent via WebSocket (scan.device events)",
		})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	devices, err := h.meshService.ScanDevices(ctx, connType, 5*time.Second)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"devices": devices})
}

// CancelScan cancels any ongoing device scan
func (h *ConnectionHandler) CancelScan(c *gin.Context) {
	h.meshService.CancelScan()
	c.JSON(http.StatusOK, gin.H{"status": "cancelled"})
}
