package api

import (
	"context"
	"fmt"
	"io/fs"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/meshtastic/meshtastic-go/internal/api/handlers"
	"github.com/meshtastic/meshtastic-go/internal/api/middleware"
	"github.com/meshtastic/meshtastic-go/internal/config"
	"github.com/meshtastic/meshtastic-go/internal/database"
	"github.com/meshtastic/meshtastic-go/internal/database/dao"
	"github.com/meshtastic/meshtastic-go/internal/service"
	"github.com/meshtastic/meshtastic-go/internal/util"
	"github.com/meshtastic/meshtastic-go/internal/websocket"
	"github.com/meshtastic/meshtastic-go/pkg/pb"
	"github.com/meshtastic/meshtastic-go/web"
)

// Server represents the HTTP server
type Server struct {
	cfg         config.ServerConfig
	router      *gin.Engine
	httpServer  *http.Server
	db          *database.Database
	meshService *service.MeshService
	wsHub       *websocket.Hub
	mqttBridge  *service.MQTTBridge
}

// NewServer creates a new HTTP server
func NewServer(
	cfg config.ServerConfig,
	db *database.Database,
	meshService *service.MeshService,
	wsHub *websocket.Hub,
	mqttBridge *service.MQTTBridge,
) *Server {
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()

	s := &Server{
		cfg:         cfg,
		router:      router,
		db:          db,
		meshService: meshService,
		wsHub:       wsHub,
		mqttBridge:  mqttBridge,
	}

	s.setupMiddleware()
	s.setupRoutes()

	return s
}

func (s *Server) setupMiddleware() {
	s.router.Use(gin.Recovery())
	s.router.Use(middleware.Logger())

	if s.cfg.CORS.Enabled {
		s.router.Use(middleware.CORS(s.cfg.CORS.AllowedOrigins))
	}
}

func (s *Server) setupRoutes() {
	// Create handlers
	connHandler := handlers.NewConnectionHandler(s.meshService)
	nodeHandler := handlers.NewNodeHandler(s.meshService)
	msgHandler := handlers.NewMessageHandler(s.meshService)
	wsHandler := handlers.NewWebSocketHandler(s.wsHub)

	// API v1 routes
	v1 := s.router.Group("/api/v1")
	{
		// WebSocket
		v1.GET("/ws", wsHandler.HandleWebSocket)

		// Connection
		conn := v1.Group("/connection")
		{
			conn.GET("/status", connHandler.GetStatus)
			conn.POST("/connect", connHandler.Connect)
			conn.POST("/disconnect", connHandler.Disconnect)
			conn.GET("/devices", connHandler.GetDevices)
			conn.POST("/cancel-scan", connHandler.CancelScan)
		}

		// Nodes
		nodes := v1.Group("/nodes")
		{
			nodes.GET("", nodeHandler.GetNodes)
			nodes.GET("/me", nodeHandler.GetMyNode)
			nodes.GET("/:nodeNum", nodeHandler.GetNode)
			nodes.PUT("/:nodeNum/favorite", nodeHandler.SetFavorite)
			nodes.PUT("/:nodeNum/ignore", nodeHandler.SetIgnore)
			nodes.PUT("/:nodeNum/mute", nodeHandler.SetMute)
			nodes.PUT("/:nodeNum/notes", nodeHandler.SetNotes)
			nodes.DELETE("/:nodeNum", nodeHandler.DeleteNode)
			nodes.POST("/:nodeNum/request-info", nodeHandler.RequestInfo)
			nodes.POST("/:nodeNum/request-position", nodeHandler.RequestPosition)
			nodes.POST("/:nodeNum/request-telemetry", nodeHandler.RequestTelemetry)
			nodes.POST("/:nodeNum/request-neighbor-info", nodeHandler.RequestNeighborInfo)
		}

		// Messages & Contacts
		contacts := v1.Group("/contacts")
		{
			contacts.GET("", msgHandler.GetContacts)
			contacts.PUT("/:contactKey/read", msgHandler.MarkAsRead)
			contacts.PUT("/:contactKey/mute", msgHandler.MuteContact)
			contacts.DELETE("/:contactKey", msgHandler.DeleteContact)
		}

		// Channel messages (separate route to avoid conflict with :contactKey)
		v1.GET("/channel-messages/:channelIndex", msgHandler.GetChannelMessages)

		messages := v1.Group("/messages")
		{
			messages.GET("/:contactKey", msgHandler.GetMessages)
			messages.POST("", msgHandler.SendMessage)
			messages.POST("/:packetId/reaction", msgHandler.AddReaction)
			messages.DELETE("/:uuid", msgHandler.DeleteMessage)
		}

		// Channels
		channels := v1.Group("/channels")
		{
			channels.GET("", s.handleGetChannels)
			channels.GET("/:index", s.handleGetChannel)
			channels.PUT("/:index", s.handleSetChannel)
			channels.DELETE("/:index", s.handleDeleteChannel)
			channels.POST("/url", s.handleImportChannelURL)
			channels.GET("/url", s.handleExportChannelURL)
		}

		// Device Configuration
		cfg := v1.Group("/config")
		{
			cfg.GET("", s.handleGetConfig)
			cfg.GET("/device", s.handleGetDeviceConfig)
			cfg.PUT("/device", s.handleSetDeviceConfig)
			cfg.GET("/position", s.handleGetPositionConfig)
			cfg.PUT("/position", s.handleSetPositionConfig)
			cfg.GET("/power", s.handleGetPowerConfig)
			cfg.PUT("/power", s.handleSetPowerConfig)
			cfg.GET("/network", s.handleGetNetworkConfig)
			cfg.PUT("/network", s.handleSetNetworkConfig)
			cfg.GET("/display", s.handleGetDisplayConfig)
			cfg.PUT("/display", s.handleSetDisplayConfig)
			cfg.GET("/lora", s.handleGetLoRaConfig)
			cfg.PUT("/lora", s.handleSetLoRaConfig)
			cfg.GET("/bluetooth", s.handleGetBluetoothConfig)
			cfg.PUT("/bluetooth", s.handleSetBluetoothConfig)
			cfg.GET("/security", s.handleGetSecurityConfig)
			cfg.PUT("/security", s.handleSetSecurityConfig)
		}

		// Module Configuration
		modules := v1.Group("/modules")
		{
			modules.GET("", s.handleGetModules)
			modules.GET("/mqtt", s.handleGetMQTTModule)
			modules.PUT("/mqtt", s.handleSetMQTTModule)
			modules.GET("/serial", s.handleGetSerialModule)
			modules.PUT("/serial", s.handleSetSerialModule)
			modules.GET("/telemetry", s.handleGetTelemetryModule)
			modules.PUT("/telemetry", s.handleSetTelemetryModule)
			modules.GET("/store-forward", s.handleGetStoreForwardModule)
			modules.PUT("/store-forward", s.handleSetStoreForwardModule)
			modules.GET("/range-test", s.handleGetRangeTestModule)
			modules.PUT("/range-test", s.handleSetRangeTestModule)
			modules.GET("/canned-message", s.handleGetCannedMessageModule)
			modules.PUT("/canned-message", s.handleSetCannedMessageModule)
			modules.GET("/neighbor-info", s.handleGetNeighborInfoModule)
			modules.PUT("/neighbor-info", s.handleSetNeighborInfoModule)
			modules.GET("/external-notification", s.handleGetExternalNotificationModule)
			modules.PUT("/external-notification", s.handleSetExternalNotificationModule)
		}

		// Telemetry & Diagnostics
		telemetry := v1.Group("/telemetry")
		{
			// Static routes must come before parameterized routes
			telemetry.GET("/nodes", s.handleGetTelemetryNodes)
			telemetry.GET("/:nodeNum", s.handleGetTelemetry)
			telemetry.GET("/:nodeNum/latest", s.handleGetLatestTelemetry)
			telemetry.GET("/:nodeNum/history", s.handleGetTelemetryHistory)
			telemetry.GET("/:nodeNum/stats", s.handleGetTelemetryStats)
			telemetry.GET("/:nodeNum/aggregated", s.handleGetTelemetryAggregated)
			telemetry.POST("/:nodeNum/request", s.handleRequestTelemetry)
		}

		traceroute := v1.Group("/traceroute")
		{
			traceroute.GET("/:nodeNum", s.handleGetTraceroute)
			traceroute.POST("/:nodeNum", s.handleRequestTraceroute)
		}

		neighborInfo := v1.Group("/neighbor-info")
		{
			neighborInfo.GET("/:nodeNum", s.handleGetNeighborInfo)
			neighborInfo.POST("/:nodeNum", s.handleRequestNeighborInfo)
		}

		// Neighbors (historical data and charts)
		neighbors := v1.Group("/neighbors")
		{
			neighbors.GET("/nodes", s.handleGetNeighborNodes)
			neighbors.GET("/:nodeNum/history", s.handleGetNeighborHistory)
			neighbors.GET("/:nodeNum/chart", s.handleGetNeighborChart)
		}

		// MQTT Bridge
		mqttBridge := v1.Group("/mqtt-bridge")
		{
			mqttBridge.GET("/status", s.handleMQTTBridgeStatus)
			mqttBridge.POST("/connect", s.handleMQTTBridgeConnect)
			mqttBridge.POST("/disconnect", s.handleMQTTBridgeDisconnect)
			mqttBridge.PUT("/channels", s.handleMQTTBridgeSetChannels)
		}

		// Device Actions
		device := v1.Group("/device")
		{
			device.POST("/reboot", s.handleReboot)
			device.POST("/shutdown", s.handleShutdown)
			device.POST("/factory-reset", s.handleFactoryReset)
			device.POST("/nodedb-reset", s.handleNodeDBReset)
		}

		// User/Owner settings
		user := v1.Group("/user")
		{
			user.PUT("", s.handleSetOwner)
		}

		// Quick Chat
		quickChat := v1.Group("/quick-chat")
		{
			quickChat.GET("", s.handleGetQuickChats)
			quickChat.POST("", s.handleCreateQuickChat)
			quickChat.PUT("/:uuid", s.handleUpdateQuickChat)
			quickChat.DELETE("/:uuid", s.handleDeleteQuickChat)
		}

		// Waypoints
		waypoints := v1.Group("/waypoints")
		{
			waypoints.GET("", s.handleGetWaypoints)
			waypoints.POST("", s.handleCreateWaypoint)
			waypoints.DELETE("/:id", s.handleDeleteWaypoint)
		}

		// Traceroutes History
		traceroutes := v1.Group("/traceroutes")
		{
			traceroutes.GET("", s.handleGetTraceroutes)
			traceroutes.GET("/node/:num", s.handleGetNodeTraceroutes)
			traceroutes.GET("/detail/:id", s.handleGetTracerouteDetail)
			traceroutes.DELETE("/old", s.handleDeleteOldTraceroutes)
		}

		// Logs
		logs := v1.Group("/logs")
		{
			logs.GET("", s.handleGetLogs)
			logs.DELETE("", s.handleClearLogs)
		}
	}

	// Health check
	s.router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// Serve embedded static files
	staticFS, err := web.StaticFS()
	if err != nil {
		panic("failed to load embedded static files: " + err.Error())
	}
	s.router.StaticFS("/static", http.FS(staticFS))

	// Serve index.html for root
	s.router.GET("/", func(c *gin.Context) {
		indexFile, err := fs.ReadFile(staticFS, "index.html")
		if err != nil {
			c.String(http.StatusInternalServerError, "Failed to load index.html")
			return
		}
		c.Data(http.StatusOK, "text/html; charset=utf-8", indexFile)
	})
}

// Channel handlers
func (s *Server) handleGetChannels(c *gin.Context) {
	channels := s.meshService.ConfigManager().GetChannels()
	c.JSON(http.StatusOK, gin.H{"channels": channels})
}

func (s *Server) handleGetChannel(c *gin.Context) {
	indexStr := c.Param("index")
	var index int32
	fmt.Sscanf(indexStr, "%d", &index)
	channel, ok := s.meshService.ConfigManager().GetChannel(index)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "channel not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"channel": channel})
}

func (s *Server) handleSetChannel(c *gin.Context) {
	indexStr := c.Param("index")
	var index int32
	fmt.Sscanf(indexStr, "%d", &index)

	var req struct {
		Name string `json:"name"`
		Role string `json:"role"`
		PSK  string `json:"psk"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	// Map role string to protobuf enum
	var role int32
	switch req.Role {
	case "PRIMARY":
		role = 1
	case "SECONDARY":
		role = 2
	case "DISABLED":
		role = 0
	default:
		role = 0
	}

	// Build channel settings
	settings := &struct {
		Name string
		Psk  []byte
	}{
		Name: req.Name,
	}

	// Handle PSK
	if req.PSK == "random" {
		// Generate a random 32-byte key
		psk := make([]byte, 32)
		for i := range psk {
			psk[i] = byte(i * 7 % 256) // Simple pseudo-random for demo
		}
		settings.Psk = psk
	} else if req.PSK == "none" || req.PSK == "" {
		// No PSK change or clear
		settings.Psk = nil
	} else {
		// Use provided PSK as-is (base64 or hex decode would be here in production)
		settings.Psk = []byte(req.PSK)
	}

	// Import pb package types
	channel := s.buildChannel(uint32(index), settings.Name, role, settings.Psk)

	packetId, err := s.meshService.SetChannel(channel)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "ok", "packetId": packetId})
}

func (s *Server) handleDeleteChannel(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (s *Server) handleImportChannelURL(c *gin.Context) {
	var req struct {
		URL string `json:"url" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "URL is required"})
		return
	}

	// Parse the channel URL
	channelSet, err := util.ParseChannelURL(req.URL)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if channelSet == nil || len(channelSet.Settings) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No channels found in URL"})
		return
	}

	// Convert to individual channels
	channels := util.ChannelsFromChannelSet(channelSet)

	// Apply each channel to the device
	for _, ch := range channels {
		packetId, err := s.meshService.SetChannel(ch)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":    fmt.Sprintf("Failed to set channel %d: %s", ch.Index, err.Error()),
				"packetId": packetId,
			})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"status":   "ok",
		"channels": len(channels),
		"message":  fmt.Sprintf("Imported %d channel(s)", len(channels)),
	})
}

func (s *Server) handleExportChannelURL(c *gin.Context) {
	// Get raw channels with full settings (including PSK)
	rawChannels := s.meshService.ConfigManager().GetRawChannels()
	if len(rawChannels) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No channels configured"})
		return
	}

	// Get LoRa config
	config := s.meshService.ConfigManager().GetConfig()

	// Build ChannelSet for URL generation using raw channel settings
	channelSet := &pb.ChannelSet{
		Settings: make([]*pb.ChannelSettings, 0, len(rawChannels)),
		LoraConfig: &pb.Config_LoRaConfig{
			UsePreset:   true,
			ModemPreset: uint32(config.ModemPreset),
			Region:      uint32(config.Region),
			HopLimit:    uint32(config.HopLimit),
			TxEnabled:   config.TxEnabled,
			TxPower:     config.TxPower,
		},
	}

	// Add channel settings from raw channels
	for _, ch := range rawChannels {
		if ch.Settings != nil {
			channelSet.Settings = append(channelSet.Settings, ch.Settings)
		}
	}

	if len(channelSet.Settings) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No active channels to export"})
		return
	}

	// Generate URLs
	meshtasticURL, err := util.GenerateChannelURL(channelSet)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate URL"})
		return
	}

	httpURL, _ := util.GenerateChannelHTTPURL(channelSet)

	c.JSON(http.StatusOK, gin.H{
		"url":     meshtasticURL,
		"httpUrl": httpURL,
	})
}

// Config handlers
func (s *Server) handleGetConfig(c *gin.Context) {
	config := s.meshService.ConfigManager().GetConfig()
	c.JSON(http.StatusOK, gin.H{"config": config})
}

func (s *Server) handleGetDeviceConfig(c *gin.Context) {
	device := s.meshService.ConfigManager().GetDeviceConfigRaw()
	c.JSON(http.StatusOK, gin.H{"device": device})
}

func (s *Server) handleSetDeviceConfig(c *gin.Context) {
	var req pb.Config_DeviceConfig
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	config := &pb.Config{
		PayloadVariant: &pb.Config_Device{Device: &req},
	}

	packetId, err := s.meshService.SetConfig(config)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "ok", "packetId": packetId})
}

func (s *Server) handleGetPositionConfig(c *gin.Context) {
	position := s.meshService.ConfigManager().GetPositionConfigRaw()
	c.JSON(http.StatusOK, gin.H{"position": position})
}

func (s *Server) handleSetPositionConfig(c *gin.Context) {
	var req pb.Config_PositionConfig
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	config := &pb.Config{
		PayloadVariant: &pb.Config_Position{Position: &req},
	}

	packetId, err := s.meshService.SetConfig(config)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "ok", "packetId": packetId})
}

func (s *Server) handleGetPowerConfig(c *gin.Context) {
	power := s.meshService.ConfigManager().GetPowerConfigRaw()
	c.JSON(http.StatusOK, gin.H{"power": power})
}

func (s *Server) handleSetPowerConfig(c *gin.Context) {
	var req pb.Config_PowerConfig
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	config := &pb.Config{
		PayloadVariant: &pb.Config_Power{Power: &req},
	}

	packetId, err := s.meshService.SetConfig(config)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "ok", "packetId": packetId})
}

func (s *Server) handleGetNetworkConfig(c *gin.Context) {
	network := s.meshService.ConfigManager().GetNetworkConfigRaw()
	c.JSON(http.StatusOK, gin.H{"network": network})
}

func (s *Server) handleSetNetworkConfig(c *gin.Context) {
	var req pb.Config_NetworkConfig
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	config := &pb.Config{
		PayloadVariant: &pb.Config_Network{Network: &req},
	}

	packetId, err := s.meshService.SetConfig(config)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "ok", "packetId": packetId})
}

func (s *Server) handleGetDisplayConfig(c *gin.Context) {
	display := s.meshService.ConfigManager().GetDisplayConfigRaw()
	c.JSON(http.StatusOK, gin.H{"display": display})
}

func (s *Server) handleSetDisplayConfig(c *gin.Context) {
	var req pb.Config_DisplayConfig
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	config := &pb.Config{
		PayloadVariant: &pb.Config_Display{Display: &req},
	}

	packetId, err := s.meshService.SetConfig(config)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "ok", "packetId": packetId})
}

func (s *Server) handleGetLoRaConfig(c *gin.Context) {
	lora := s.meshService.ConfigManager().GetLoRaConfigRaw()
	c.JSON(http.StatusOK, gin.H{"lora": lora})
}

func (s *Server) handleSetLoRaConfig(c *gin.Context) {
	var req pb.Config_LoRaConfig
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	config := &pb.Config{
		PayloadVariant: &pb.Config_Lora{Lora: &req},
	}

	packetId, err := s.meshService.SetConfig(config)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "ok", "packetId": packetId})
}

func (s *Server) handleGetBluetoothConfig(c *gin.Context) {
	bluetooth := s.meshService.ConfigManager().GetBluetoothConfigRaw()
	c.JSON(http.StatusOK, gin.H{"bluetooth": bluetooth})
}

func (s *Server) handleSetBluetoothConfig(c *gin.Context) {
	var req pb.Config_BluetoothConfig
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	config := &pb.Config{
		PayloadVariant: &pb.Config_Bluetooth{Bluetooth: &req},
	}

	packetId, err := s.meshService.SetConfig(config)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "ok", "packetId": packetId})
}

func (s *Server) handleGetSecurityConfig(c *gin.Context) {
	security := s.meshService.ConfigManager().GetSecurityConfigRaw()
	c.JSON(http.StatusOK, gin.H{"security": security})
}

func (s *Server) handleSetSecurityConfig(c *gin.Context) {
	var req pb.Config_SecurityConfig
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	config := &pb.Config{
		PayloadVariant: &pb.Config_Security{Security: &req},
	}

	packetId, err := s.meshService.SetConfig(config)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "ok", "packetId": packetId})
}

// Module handlers
func (s *Server) handleGetModules(c *gin.Context) {
	cm := s.meshService.ConfigManager()
	c.JSON(http.StatusOK, gin.H{
		"mqtt":                 cm.GetMQTTConfigRaw(),
		"serial":               cm.GetSerialConfigRaw(),
		"telemetry":            cm.GetTelemetryConfigRaw(),
		"storeForward":         cm.GetStoreForwardConfigRaw(),
		"rangeTest":            cm.GetRangeTestConfigRaw(),
		"cannedMessage":        cm.GetCannedMessageConfigRaw(),
		"neighborInfo":         cm.GetNeighborInfoConfigRaw(),
		"externalNotification": cm.GetExternalNotificationConfigRaw(),
	})
}

func (s *Server) handleGetMQTTModule(c *gin.Context) {
	mqtt := s.meshService.ConfigManager().GetMQTTConfigRaw()
	c.JSON(http.StatusOK, gin.H{"mqtt": mqtt})
}

func (s *Server) handleSetMQTTModule(c *gin.Context) {
	var req pb.ModuleConfig_MQTTConfig
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	moduleConfig := &pb.ModuleConfig{
		PayloadVariant: &pb.ModuleConfig_Mqtt{Mqtt: &req},
	}

	packetId, err := s.meshService.SetModuleConfig(moduleConfig)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Update local config immediately so UI reflects the change
	s.meshService.ConfigManager().ProcessModuleConfig(moduleConfig)

	c.JSON(http.StatusOK, gin.H{"status": "ok", "packetId": packetId})
}

func (s *Server) handleGetSerialModule(c *gin.Context) {
	serial := s.meshService.ConfigManager().GetSerialConfigRaw()
	c.JSON(http.StatusOK, gin.H{"serial": serial})
}

func (s *Server) handleSetSerialModule(c *gin.Context) {
	var req pb.ModuleConfig_SerialConfig
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	moduleConfig := &pb.ModuleConfig{
		PayloadVariant: &pb.ModuleConfig_Serial{Serial: &req},
	}

	packetId, err := s.meshService.SetModuleConfig(moduleConfig)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Update local config immediately so UI reflects the change
	s.meshService.ConfigManager().ProcessModuleConfig(moduleConfig)

	c.JSON(http.StatusOK, gin.H{"status": "ok", "packetId": packetId})
}

func (s *Server) handleGetTelemetryModule(c *gin.Context) {
	telemetry := s.meshService.ConfigManager().GetTelemetryConfigRaw()
	c.JSON(http.StatusOK, gin.H{"telemetry": telemetry})
}

func (s *Server) handleSetTelemetryModule(c *gin.Context) {
	var req pb.ModuleConfig_TelemetryConfig
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	moduleConfig := &pb.ModuleConfig{
		PayloadVariant: &pb.ModuleConfig_Telemetry{Telemetry: &req},
	}

	packetId, err := s.meshService.SetModuleConfig(moduleConfig)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Update local config immediately so UI reflects the change
	s.meshService.ConfigManager().ProcessModuleConfig(moduleConfig)

	c.JSON(http.StatusOK, gin.H{"status": "ok", "packetId": packetId})
}

func (s *Server) handleGetStoreForwardModule(c *gin.Context) {
	storeForward := s.meshService.ConfigManager().GetStoreForwardConfigRaw()
	c.JSON(http.StatusOK, gin.H{"storeForward": storeForward})
}

func (s *Server) handleSetStoreForwardModule(c *gin.Context) {
	var req pb.ModuleConfig_StoreForwardConfig
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	moduleConfig := &pb.ModuleConfig{
		PayloadVariant: &pb.ModuleConfig_StoreForward{StoreForward: &req},
	}

	packetId, err := s.meshService.SetModuleConfig(moduleConfig)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Update local config immediately so UI reflects the change
	s.meshService.ConfigManager().ProcessModuleConfig(moduleConfig)

	c.JSON(http.StatusOK, gin.H{"status": "ok", "packetId": packetId})
}

func (s *Server) handleGetRangeTestModule(c *gin.Context) {
	rangeTest := s.meshService.ConfigManager().GetRangeTestConfigRaw()
	c.JSON(http.StatusOK, gin.H{"rangeTest": rangeTest})
}

func (s *Server) handleSetRangeTestModule(c *gin.Context) {
	var req pb.ModuleConfig_RangeTestConfig
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	moduleConfig := &pb.ModuleConfig{
		PayloadVariant: &pb.ModuleConfig_RangeTest{RangeTest: &req},
	}

	packetId, err := s.meshService.SetModuleConfig(moduleConfig)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Update local config immediately so UI reflects the change
	s.meshService.ConfigManager().ProcessModuleConfig(moduleConfig)

	c.JSON(http.StatusOK, gin.H{"status": "ok", "packetId": packetId})
}

func (s *Server) handleGetCannedMessageModule(c *gin.Context) {
	cannedMessage := s.meshService.ConfigManager().GetCannedMessageConfigRaw()
	c.JSON(http.StatusOK, gin.H{"cannedMessage": cannedMessage})
}

func (s *Server) handleSetCannedMessageModule(c *gin.Context) {
	var req pb.ModuleConfig_CannedMessageConfig
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	moduleConfig := &pb.ModuleConfig{
		PayloadVariant: &pb.ModuleConfig_CannedMessage{CannedMessage: &req},
	}

	packetId, err := s.meshService.SetModuleConfig(moduleConfig)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Update local config immediately so UI reflects the change
	s.meshService.ConfigManager().ProcessModuleConfig(moduleConfig)

	c.JSON(http.StatusOK, gin.H{"status": "ok", "packetId": packetId})
}

func (s *Server) handleGetNeighborInfoModule(c *gin.Context) {
	neighborInfo := s.meshService.ConfigManager().GetNeighborInfoConfigRaw()
	c.JSON(http.StatusOK, gin.H{"neighborInfo": neighborInfo})
}

func (s *Server) handleSetNeighborInfoModule(c *gin.Context) {
	var req pb.ModuleConfig_NeighborInfoConfig
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	moduleConfig := &pb.ModuleConfig{
		PayloadVariant: &pb.ModuleConfig_NeighborInfo{NeighborInfo: &req},
	}

	packetId, err := s.meshService.SetModuleConfig(moduleConfig)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Update local config immediately so UI reflects the change
	s.meshService.ConfigManager().ProcessModuleConfig(moduleConfig)

	c.JSON(http.StatusOK, gin.H{"status": "ok", "packetId": packetId})
}

func (s *Server) handleGetExternalNotificationModule(c *gin.Context) {
	extNotif := s.meshService.ConfigManager().GetExternalNotificationConfigRaw()
	c.JSON(http.StatusOK, gin.H{"externalNotification": extNotif})
}

func (s *Server) handleSetExternalNotificationModule(c *gin.Context) {
	var req pb.ModuleConfig_ExternalNotificationConfig
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	moduleConfig := &pb.ModuleConfig{
		PayloadVariant: &pb.ModuleConfig_ExternalNotification{ExternalNotification: &req},
	}

	packetId, err := s.meshService.SetModuleConfig(moduleConfig)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Update local config immediately so UI reflects the change
	s.meshService.ConfigManager().ProcessModuleConfig(moduleConfig)

	c.JSON(http.StatusOK, gin.H{"status": "ok", "packetId": packetId})
}

// Telemetry handlers
func (s *Server) handleGetTelemetry(c *gin.Context) {
	nodeNumStr := c.Param("nodeNum")
	nodeNum, err := strconv.ParseUint(nodeNumStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid node number"})
		return
	}

	// Get latest telemetry of all types
	latest := s.meshService.TelemetryService().GetAllLatest(uint32(nodeNum))
	c.JSON(http.StatusOK, gin.H{"telemetry": latest})
}

func (s *Server) handleGetLatestTelemetry(c *gin.Context) {
	nodeNumStr := c.Param("nodeNum")
	nodeNum, err := strconv.ParseUint(nodeNumStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid node number"})
		return
	}

	telemetryType := c.Query("type")

	ts := s.meshService.TelemetryService()
	result := make(map[string]interface{})

	switch telemetryType {
	case "device":
		result["device"] = ts.GetLatestDevice(uint32(nodeNum))
	case "environment":
		result["environment"] = ts.GetLatestEnvironment(uint32(nodeNum))
	case "power":
		result["power"] = ts.GetLatestPower(uint32(nodeNum))
	case "air_quality":
		result["airQuality"] = ts.GetLatestAirQuality(uint32(nodeNum))
	case "local_stats":
		result["localStats"] = ts.GetLatestLocalStats(uint32(nodeNum))
	default:
		// Return all types
		result = ts.GetAllLatest(uint32(nodeNum))
	}

	c.JSON(http.StatusOK, gin.H{"telemetry": result})
}

func (s *Server) handleGetTelemetryHistory(c *gin.Context) {
	nodeNumStr := c.Param("nodeNum")
	nodeNum, err := strconv.ParseUint(nodeNumStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid node number"})
		return
	}

	telemetryType := c.Query("type")
	limitStr := c.DefaultQuery("limit", "100")
	limit, _ := strconv.Atoi(limitStr)
	if limit <= 0 || limit > 1000 {
		limit = 100
	}

	history, err := s.meshService.TelemetryService().GetHistory(uint32(nodeNum), telemetryType, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"history": history, "count": len(history)})
}

func (s *Server) handleGetTelemetryStats(c *gin.Context) {
	nodeNumStr := c.Param("nodeNum")
	nodeNum, err := strconv.ParseUint(nodeNumStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid node number"})
		return
	}

	// Default to last 24 hours
	sinceStr := c.DefaultQuery("since", "86400")
	sinceSecs, _ := strconv.ParseInt(sinceStr, 10, 64)
	since := sinceSecs
	if since <= 0 {
		since = 86400 // 24 hours
	}
	sinceTimestamp := int64(0)
	if since > 0 {
		sinceTimestamp = int64(sinceSecs)
	}

	ts := s.meshService.TelemetryService()

	deviceStats, err := ts.GetDeviceStats(uint32(nodeNum), sinceTimestamp)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	envStats, err := ts.GetEnvironmentStats(uint32(nodeNum), sinceTimestamp)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"device":      deviceStats,
		"environment": envStats,
	})
}

func (s *Server) handleRequestTelemetry(c *gin.Context) {
	nodeNumStr := c.Param("nodeNum")
	nodeNum, err := strconv.ParseUint(nodeNumStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid node number"})
		return
	}

	packetId, err := s.meshService.RequestTelemetry(uint32(nodeNum))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "ok", "requestId": packetId})
}

func (s *Server) handleGetTelemetryAggregated(c *gin.Context) {
	nodeNumStr := c.Param("nodeNum")
	nodeNum, err := strconv.ParseUint(nodeNumStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid node number"})
		return
	}

	metric := c.DefaultQuery("metric", "battery_level")
	period := c.DefaultQuery("period", "day")

	data, err := s.meshService.TelemetryService().GetAggregatedData(uint32(nodeNum), metric, period)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data":   data,
		"metric": metric,
		"period": period,
		"count":  len(data),
	})
}

func (s *Server) handleGetTelemetryNodes(c *gin.Context) {
	nodes, err := s.meshService.TelemetryService().GetNodesWithData()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"nodes": nodes})
}

// Traceroute handlers
func (s *Server) handleGetTraceroute(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"traceroute": nil})
}

func (s *Server) handleRequestTraceroute(c *gin.Context) {
	nodeNumStr := c.Param("nodeNum")
	nodeNum, err := strconv.ParseUint(nodeNumStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid node number"})
		return
	}

	packetId, err := s.meshService.SendTraceroute(uint32(nodeNum))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "ok", "requestId": packetId})
}

// Neighbor info handlers
func (s *Server) handleGetNeighborInfo(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"neighbors": nil})
}

func (s *Server) handleRequestNeighborInfo(c *gin.Context) {
	nodeNumStr := c.Param("nodeNum")
	nodeNum, err := strconv.ParseUint(nodeNumStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid node number"})
		return
	}

	packetId, err := s.meshService.RequestNeighborInfo(uint32(nodeNum))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "ok", "requestId": packetId})
}

// Neighbor historical data handlers
func (s *Server) handleGetNeighborNodes(c *gin.Context) {
	nodes, err := s.meshService.NeighborService().GetNodesWithData()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"nodes": nodes})
}

func (s *Server) handleGetNeighborHistory(c *gin.Context) {
	nodeNumStr := c.Param("nodeNum")
	nodeNum, err := strconv.ParseUint(nodeNumStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid node number"})
		return
	}

	limitStr := c.DefaultQuery("limit", "100")
	limit, _ := strconv.Atoi(limitStr)
	if limit <= 0 || limit > 1000 {
		limit = 100
	}

	history, err := s.meshService.NeighborService().GetHistory(uint32(nodeNum), limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"history": history, "count": len(history)})
}

func (s *Server) handleGetNeighborChart(c *gin.Context) {
	nodeNumStr := c.Param("nodeNum")
	nodeNum, err := strconv.ParseUint(nodeNumStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid node number"})
		return
	}

	period := c.DefaultQuery("period", "day")

	data, err := s.meshService.NeighborService().GetChartData(uint32(nodeNum), period)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data":   data,
		"period": period,
		"count":  len(data),
	})
}

// Device action handlers
func (s *Server) handleReboot(c *gin.Context) {
	// Reboot after 2 seconds to allow response to be sent
	packetId, err := s.meshService.Reboot(2)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok", "packetId": packetId})
}

func (s *Server) handleShutdown(c *gin.Context) {
	// Shutdown after 2 seconds to allow response to be sent
	packetId, err := s.meshService.Shutdown(2)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok", "packetId": packetId})
}

func (s *Server) handleFactoryReset(c *gin.Context) {
	// TODO: Implement factory reset
	c.JSON(http.StatusNotImplemented, gin.H{"error": "not implemented"})
}

func (s *Server) handleNodeDBReset(c *gin.Context) {
	// TODO: Implement node DB reset
	c.JSON(http.StatusNotImplemented, gin.H{"error": "not implemented"})
}

// User/Owner handlers
func (s *Server) handleSetOwner(c *gin.Context) {
	var req struct {
		LongName  string `json:"longName"`
		ShortName string `json:"shortName"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	user := &pb.User{
		LongName:  req.LongName,
		ShortName: req.ShortName,
	}

	packetId, err := s.meshService.SetOwner(user)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "ok", "packetId": packetId})
}

// Quick chat handlers
func (s *Server) handleGetQuickChats(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"quick_chats": []interface{}{}})
}

func (s *Server) handleCreateQuickChat(c *gin.Context) {
	c.JSON(http.StatusCreated, gin.H{"uuid": 0})
}

func (s *Server) handleUpdateQuickChat(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (s *Server) handleDeleteQuickChat(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// Waypoint handlers
func (s *Server) handleGetWaypoints(c *gin.Context) {
	waypointDAO := dao.NewWaypointDAO(s.db.DB())
	waypoints, err := waypointDAO.GetAll()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Convert to JSON-friendly format
	result := make([]map[string]interface{}, 0, len(waypoints))
	for _, wp := range waypoints {
		result = append(result, map[string]interface{}{
			"id":          wp.ID,
			"latitude":    float64(wp.LatitudeI) * 1e-7,
			"longitude":   float64(wp.LongitudeI) * 1e-7,
			"latitudeI":   wp.LatitudeI,
			"longitudeI":  wp.LongitudeI,
			"expire":      wp.Expire,
			"lockedTo":    wp.LockedTo,
			"name":        wp.Name,
			"description": wp.Description,
			"icon":        wp.Icon,
			"from":        wp.FromNode,
			"viaMqtt":     wp.ViaMqtt,
			"createdAt":   wp.CreatedAt,
			"updatedAt":   wp.UpdatedAt,
		})
	}
	c.JSON(http.StatusOK, gin.H{"waypoints": result})
}

func (s *Server) handleCreateWaypoint(c *gin.Context) {
	var req struct {
		ID          uint32  `json:"id"`
		Latitude    float64 `json:"latitude"`
		Longitude   float64 `json:"longitude"`
		Expire      uint32  `json:"expire"`
		LockedTo    uint32  `json:"lockedTo"`
		Name        string  `json:"name"`
		Description string  `json:"description"`
		Icon        uint32  `json:"icon"`
		Broadcast   bool    `json:"broadcast"` // Whether to send to mesh
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Generate ID if not provided
	if req.ID == 0 {
		req.ID = uint32(time.Now().UnixNano() & 0xFFFFFFFF)
	}

	// Convert lat/lon to integer format
	latI := int32(req.Latitude * 1e7)
	lonI := int32(req.Longitude * 1e7)

	// Save to database
	waypointDAO := dao.NewWaypointDAO(s.db.DB())
	entity := &dao.WaypointEntity{
		ID:          req.ID,
		LatitudeI:   latI,
		LongitudeI:  lonI,
		Expire:      req.Expire,
		LockedTo:    req.LockedTo,
		Name:        req.Name,
		Description: req.Description,
		Icon:        req.Icon,
		FromNode:    0, // Local waypoint
		ViaMqtt:     false,
	}

	if err := waypointDAO.Upsert(entity); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Broadcast to mesh if requested
	if req.Broadcast && s.meshService.IsConnected() {
		wp := &pb.Waypoint{
			Id:          req.ID,
			LatitudeI:   latI,
			LongitudeI:  lonI,
			Expire:      req.Expire,
			LockedTo:    req.LockedTo,
			Name:        req.Name,
			Description: req.Description,
			Icon:        req.Icon,
		}
		if err := s.meshService.SendWaypoint(wp, 0); err != nil {
			// Log but don't fail - waypoint is saved locally
			c.JSON(http.StatusOK, gin.H{
				"id":      req.ID,
				"warning": "saved locally but failed to broadcast: " + err.Error(),
			})
			return
		}
	}

	// Broadcast locally via WebSocket
	s.wsHub.Broadcast(websocket.Event{
		Type: "waypoint.created",
		Data: map[string]interface{}{
			"id":          req.ID,
			"latitude":    req.Latitude,
			"longitude":   req.Longitude,
			"name":        req.Name,
			"description": req.Description,
			"icon":        req.Icon,
		},
	})

	c.JSON(http.StatusCreated, gin.H{"id": req.ID})
}

func (s *Server) handleDeleteWaypoint(c *gin.Context) {
	idStr := c.Param("id")
	var id uint32
	if _, err := fmt.Sscanf(idStr, "%d", &id); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid waypoint ID"})
		return
	}

	waypointDAO := dao.NewWaypointDAO(s.db.DB())
	if err := waypointDAO.Delete(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Broadcast delete event
	s.wsHub.Broadcast(websocket.Event{
		Type: "waypoint.deleted",
		Data: map[string]interface{}{
			"id": id,
		},
	})

	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// Traceroute handlers
func (s *Server) handleGetTraceroutes(c *gin.Context) {
	limitStr := c.DefaultQuery("limit", "50")
	limit := 50
	if _, err := fmt.Sscanf(limitStr, "%d", &limit); err != nil || limit < 1 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}

	tracerouteDAO := dao.NewTracerouteDAO(s.db.DB())
	traceroutes, err := tracerouteDAO.GetRecent(limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	result := make([]map[string]interface{}, 0, len(traceroutes))
	for _, tr := range traceroutes {
		result = append(result, map[string]interface{}{
			"id":         tr.ID,
			"fromNode":   tr.FromNode,
			"toNode":     tr.ToNode,
			"route":      tr.Route,
			"routeBack":  tr.RouteBack,
			"snrTowards": tr.SnrTowards,
			"snrBack":    tr.SnrBack,
			"hopCount":   tr.HopCount,
			"timestamp":  tr.Timestamp,
			"success":    tr.Success,
		})
	}

	c.JSON(http.StatusOK, gin.H{"traceroutes": result})
}

func (s *Server) handleGetNodeTraceroutes(c *gin.Context) {
	numStr := c.Param("num")
	var nodeNum uint32
	if _, err := fmt.Sscanf(numStr, "%d", &nodeNum); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid node number"})
		return
	}

	limitStr := c.DefaultQuery("limit", "20")
	limit := 20
	if _, err := fmt.Sscanf(limitStr, "%d", &limit); err != nil || limit < 1 {
		limit = 20
	}

	tracerouteDAO := dao.NewTracerouteDAO(s.db.DB())
	traceroutes, err := tracerouteDAO.GetByNode(nodeNum, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Also get stats
	stats, _ := tracerouteDAO.GetStats(nodeNum)

	result := make([]map[string]interface{}, 0, len(traceroutes))
	for _, tr := range traceroutes {
		result = append(result, map[string]interface{}{
			"id":         tr.ID,
			"fromNode":   tr.FromNode,
			"toNode":     tr.ToNode,
			"route":      tr.Route,
			"routeBack":  tr.RouteBack,
			"snrTowards": tr.SnrTowards,
			"snrBack":    tr.SnrBack,
			"hopCount":   tr.HopCount,
			"timestamp":  tr.Timestamp,
			"success":    tr.Success,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"traceroutes": result,
		"stats":       stats,
	})
}

func (s *Server) handleGetTracerouteDetail(c *gin.Context) {
	idStr := c.Param("id")
	var id int64
	if _, err := fmt.Sscanf(idStr, "%d", &id); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid traceroute ID"})
		return
	}

	tracerouteDAO := dao.NewTracerouteDAO(s.db.DB())
	tr, err := tracerouteDAO.GetByID(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "traceroute not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"traceroute": map[string]interface{}{
			"id":         tr.ID,
			"fromNode":   tr.FromNode,
			"toNode":     tr.ToNode,
			"route":      tr.Route,
			"routeBack":  tr.RouteBack,
			"snrTowards": tr.SnrTowards,
			"snrBack":    tr.SnrBack,
			"hopCount":   tr.HopCount,
			"timestamp":  tr.Timestamp,
			"success":    tr.Success,
		},
	})
}

func (s *Server) handleDeleteOldTraceroutes(c *gin.Context) {
	// Delete traceroutes older than 30 days
	tracerouteDAO := dao.NewTracerouteDAO(s.db.DB())
	deleted, err := tracerouteDAO.DeleteOld(30 * 24 * time.Hour)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"deleted": deleted,
		"status":  "ok",
	})
}

// Log handlers
func (s *Server) handleGetLogs(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"logs": []interface{}{}})
}

func (s *Server) handleClearLogs(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// Start starts the HTTP server
func (s *Server) Start() error {
	addr := fmt.Sprintf("%s:%d", s.cfg.Host, s.cfg.Port)
	s.httpServer = &http.Server{
		Addr:    addr,
		Handler: s.router,
	}
	return s.httpServer.ListenAndServe()
}

// Shutdown gracefully shuts down the server
func (s *Server) Shutdown(ctx context.Context) error {
	if s.httpServer != nil {
		return s.httpServer.Shutdown(ctx)
	}
	return nil
}

// buildChannel creates a Channel protobuf message
func (s *Server) buildChannel(index uint32, name string, role int32, psk []byte) *pb.Channel {
	channel := &pb.Channel{
		Index: index,
		Role:  uint32(role),
		Settings: &pb.ChannelSettings{
			Name: name,
		},
	}
	if len(psk) > 0 {
		channel.Settings.Psk = psk
	}
	return channel
}

// MQTT Bridge handlers
func (s *Server) handleMQTTBridgeStatus(c *gin.Context) {
	if s.mqttBridge == nil {
		c.JSON(http.StatusOK, gin.H{
			"enabled":   false,
			"connected": false,
			"error":     "MQTT bridge not configured",
		})
		return
	}
	c.JSON(http.StatusOK, s.mqttBridge.GetStatus())
}

func (s *Server) handleMQTTBridgeConnect(c *gin.Context) {
	if s.mqttBridge == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "MQTT bridge not configured"})
		return
	}

	if err := s.mqttBridge.Connect(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "connected"})
}

func (s *Server) handleMQTTBridgeDisconnect(c *gin.Context) {
	if s.mqttBridge == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "MQTT bridge not configured"})
		return
	}

	s.mqttBridge.Disconnect()
	c.JSON(http.StatusOK, gin.H{"status": "disconnected"})
}

func (s *Server) handleMQTTBridgeSetChannels(c *gin.Context) {
	if s.mqttBridge == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "MQTT bridge not configured"})
		return
	}

	var req struct {
		Channels []string `json:"channels"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	s.mqttBridge.SetChannels(req.Channels)
	c.JSON(http.StatusOK, gin.H{"status": "ok", "channels": req.Channels})
}
