package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/meshtastic/meshtastic-go/internal/service"
)

// NodeHandler handles node-related requests
type NodeHandler struct {
	meshService *service.MeshService
}

// NewNodeHandler creates a new node handler
func NewNodeHandler(meshService *service.MeshService) *NodeHandler {
	return &NodeHandler{meshService: meshService}
}

// GetNodes returns all nodes with distance/bearing from local node
func (h *NodeHandler) GetNodes(c *gin.Context) {
	// Get local node position for distance calculations
	var refLat, refLon float64
	myNodeNum := h.meshService.GetMyNodeNum()
	if myNodeNum != 0 {
		if myNode, ok := h.meshService.NodeManager().GetNode(myNodeNum); ok {
			refLat = myNode.Latitude
			refLon = myNode.Longitude
		}
	}

	nodes := h.meshService.NodeManager().GetNodesWithDistance(refLat, refLon)
	c.JSON(http.StatusOK, gin.H{"nodes": nodes})
}

// GetNode returns a single node
func (h *NodeHandler) GetNode(c *gin.Context) {
	nodeNum, err := strconv.ParseUint(c.Param("nodeNum"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid node number"})
		return
	}

	node, ok := h.meshService.NodeManager().GetNode(uint32(nodeNum))
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "node not found"})
		return
	}

	c.JSON(http.StatusOK, node)
}

// GetMyNode returns the local node
func (h *NodeHandler) GetMyNode(c *gin.Context) {
	myNodeNum := h.meshService.GetMyNodeNum()
	if myNodeNum == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "not connected"})
		return
	}

	node, ok := h.meshService.NodeManager().GetNode(myNodeNum)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "node not found"})
		return
	}

	c.JSON(http.StatusOK, node)
}

// SetFavorite sets a node's favorite status
func (h *NodeHandler) SetFavorite(c *gin.Context) {
	nodeNum, err := strconv.ParseUint(c.Param("nodeNum"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid node number"})
		return
	}

	var req struct {
		Favorite bool `json:"favorite"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.meshService.NodeManager().SetFavorite(uint32(nodeNum), req.Favorite); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// SetIgnore sets a node's ignored status
func (h *NodeHandler) SetIgnore(c *gin.Context) {
	nodeNum, err := strconv.ParseUint(c.Param("nodeNum"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid node number"})
		return
	}

	var req struct {
		Ignored bool `json:"ignored"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.meshService.NodeManager().SetIgnored(uint32(nodeNum), req.Ignored); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// SetMute sets a node's muted status
func (h *NodeHandler) SetMute(c *gin.Context) {
	nodeNum, err := strconv.ParseUint(c.Param("nodeNum"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid node number"})
		return
	}

	var req struct {
		Muted bool `json:"muted"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.meshService.NodeManager().SetMuted(uint32(nodeNum), req.Muted); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// SetNotes sets a node's notes
func (h *NodeHandler) SetNotes(c *gin.Context) {
	nodeNum, err := strconv.ParseUint(c.Param("nodeNum"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid node number"})
		return
	}

	var req struct {
		Notes string `json:"notes"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.meshService.NodeManager().SetNotes(uint32(nodeNum), req.Notes); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// DeleteNode deletes a node
func (h *NodeHandler) DeleteNode(c *gin.Context) {
	nodeNum, err := strconv.ParseUint(c.Param("nodeNum"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid node number"})
		return
	}

	h.meshService.NodeManager().RemoveNode(uint32(nodeNum))
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// RequestInfo requests node info from a remote node
func (h *NodeHandler) RequestInfo(c *gin.Context) {
	nodeNum, err := strconv.ParseUint(c.Param("nodeNum"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid node number"})
		return
	}

	packetId, err := h.meshService.RequestNodeInfo(uint32(nodeNum))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "ok", "requestId": packetId})
}

// RequestPosition requests position from a remote node
func (h *NodeHandler) RequestPosition(c *gin.Context) {
	nodeNum, err := strconv.ParseUint(c.Param("nodeNum"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid node number"})
		return
	}

	packetId, err := h.meshService.RequestPosition(uint32(nodeNum))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "ok", "requestId": packetId})
}

// RequestTelemetry requests telemetry from a remote node
func (h *NodeHandler) RequestTelemetry(c *gin.Context) {
	nodeNum, err := strconv.ParseUint(c.Param("nodeNum"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid node number"})
		return
	}

	packetId, err := h.meshService.RequestTelemetry(uint32(nodeNum))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "ok", "requestId": packetId})
}
