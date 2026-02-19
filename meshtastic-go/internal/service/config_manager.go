package service

import (
	"sync"

	"github.com/meshtastic/meshtastic-go/internal/websocket"
	"github.com/meshtastic/meshtastic-go/pkg/pb"
	"github.com/rs/zerolog/log"
)

// Channel represents a mesh channel
type Channel struct {
	Index    int32  `json:"index"`
	Name     string `json:"name"`
	Role     string `json:"role"`
	PSK      string `json:"psk,omitempty"` // Hidden for security in JSON
	Settings *ChannelSettings `json:"settings,omitempty"`
	// Raw protobuf settings for URL generation (not exposed in JSON)
	RawSettings *pb.ChannelSettings `json:"-"`
}

// ChannelSettings contains channel-specific settings
type ChannelSettings struct {
	UplinkEnabled   bool            `json:"uplinkEnabled,omitempty"`
	DownlinkEnabled bool            `json:"downlinkEnabled,omitempty"`
	ModuleSettings  map[string]bool `json:"moduleSettings,omitempty"`
}

// DeviceConfig represents the device configuration
type DeviceConfig struct {
	// Device info
	FirmwareVersion string `json:"firmwareVersion"`
	HardwareModel   string `json:"hardwareModel"`
	HasWifi         bool   `json:"hasWifi"`
	HasBluetooth    bool   `json:"hasBluetooth"`
	HasEthernet     bool   `json:"hasEthernet"`
	CanShutdown     bool   `json:"canShutdown"`

	// Device config
	Role                   int32  `json:"role"`
	SerialEnabled          bool   `json:"serialEnabled"`
	DebugLogEnabled        bool   `json:"debugLogEnabled"`
	NodeInfoBroadcastSecs  int32  `json:"nodeInfoBroadcastSecs"`
	DoubleTapAsButtonPress bool   `json:"doubleTapAsButtonPress"`
	LedHeartbeatDisabled   bool   `json:"ledHeartbeatDisabled"`
	Tzdef                  string `json:"tzdef"`

	// LoRa config
	Region            int32   `json:"region"`
	ModemPreset       int32   `json:"modemPreset"`
	HopLimit          int32   `json:"hopLimit"`
	TxEnabled         bool    `json:"txEnabled"`
	TxPower           int32   `json:"txPower"`
	ChannelNum        int32   `json:"channelNum"`
	OverrideDutyCycle bool    `json:"overrideDutyCycle"`
	FrequencyOffset   float32 `json:"frequencyOffset"`

	// Position config
	GpsEnabled                     bool  `json:"gpsEnabled"`
	PositionBroadcastSecs          int32 `json:"positionBroadcastSecs"`
	FixedPosition                  bool  `json:"fixedPosition"`
	PositionBroadcastSmartEnabled  bool  `json:"positionBroadcastSmartEnabled"`
	GpsUpdateInterval              int32 `json:"gpsUpdateInterval"`
	BroadcastSmartMinimumDistance  int32 `json:"broadcastSmartMinimumDistance"`
	BroadcastSmartMinimumIntervalSecs int32 `json:"broadcastSmartMinimumIntervalSecs"`

	// Power config
	IsPowerSaving              bool  `json:"isPowerSaving"`
	OnBatteryShutdownAfterSecs int32 `json:"onBatteryShutdownAfterSecs"`
	WaitBluetoothSecs          int32 `json:"waitBluetoothSecs"`
	LsSecs                     int32 `json:"lsSecs"`
	MinWakeSecs                int32 `json:"minWakeSecs"`

	// Display config
	ScreenOnSecs           int32 `json:"screenOnSecs"`
	GpsFormat              int32 `json:"gpsFormat"`
	AutoScreenCarouselSecs int32 `json:"autoScreenCarouselSecs"`
	FlipScreen             bool  `json:"flipScreen"`
	CompassNorthTop        bool  `json:"compassNorthTop"`
	Units                  int32 `json:"units"`
	WakeOnTapOrMotion      bool  `json:"wakeOnTapOrMotion"`

	// Bluetooth config
	BluetoothEnabled  bool  `json:"bluetoothEnabled"`
	BluetoothMode     int32 `json:"bluetoothMode"`
	BluetoothFixedPin int32 `json:"bluetoothFixedPin"`

	// Network config
	WifiEnabled bool   `json:"wifiEnabled"`
	WifiSsid    string `json:"wifiSsid"`
	NtpServer   string `json:"ntpServer"`
	EthEnabled  bool   `json:"ethEnabled"`
}

// ConfigManager manages device configuration and channels
type ConfigManager struct {
	mu       sync.RWMutex
	wsHub    *websocket.Hub
	channels map[int32]*Channel
	config   *DeviceConfig
	metadata *pb.DeviceMetadata

	// Raw protobuf configs
	deviceConfig    *pb.Config_DeviceConfig
	positionConfig  *pb.Config_PositionConfig
	powerConfig     *pb.Config_PowerConfig
	networkConfig   *pb.Config_NetworkConfig
	displayConfig   *pb.Config_DisplayConfig
	loraConfig      *pb.Config_LoRaConfig
	bluetoothConfig *pb.Config_BluetoothConfig
	securityConfig  *pb.Config_SecurityConfig

	// Raw module configs
	mqttConfig                 *pb.ModuleConfig_MQTTConfig
	serialConfig               *pb.ModuleConfig_SerialConfig
	externalNotificationConfig *pb.ModuleConfig_ExternalNotificationConfig
	storeForwardConfig         *pb.ModuleConfig_StoreForwardConfig
	rangeTestConfig            *pb.ModuleConfig_RangeTestConfig
	telemetryConfig            *pb.ModuleConfig_TelemetryConfig
	cannedMessageConfig        *pb.ModuleConfig_CannedMessageConfig
	neighborInfoConfig         *pb.ModuleConfig_NeighborInfoConfig
}

// NewConfigManager creates a new config manager
func NewConfigManager(wsHub *websocket.Hub) *ConfigManager {
	return &ConfigManager{
		wsHub:    wsHub,
		channels: make(map[int32]*Channel),
		config:   &DeviceConfig{},
	}
}

// GetDeviceConfigRaw returns the raw device config
func (cm *ConfigManager) GetDeviceConfigRaw() *pb.Config_DeviceConfig {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return cm.deviceConfig
}

// GetPositionConfigRaw returns the raw position config
func (cm *ConfigManager) GetPositionConfigRaw() *pb.Config_PositionConfig {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return cm.positionConfig
}

// GetPowerConfigRaw returns the raw power config
func (cm *ConfigManager) GetPowerConfigRaw() *pb.Config_PowerConfig {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return cm.powerConfig
}

// GetNetworkConfigRaw returns the raw network config
func (cm *ConfigManager) GetNetworkConfigRaw() *pb.Config_NetworkConfig {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return cm.networkConfig
}

// GetDisplayConfigRaw returns the raw display config
func (cm *ConfigManager) GetDisplayConfigRaw() *pb.Config_DisplayConfig {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return cm.displayConfig
}

// GetLoRaConfigRaw returns the raw LoRa config
func (cm *ConfigManager) GetLoRaConfigRaw() *pb.Config_LoRaConfig {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return cm.loraConfig
}

// GetBluetoothConfigRaw returns the raw Bluetooth config
func (cm *ConfigManager) GetBluetoothConfigRaw() *pb.Config_BluetoothConfig {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return cm.bluetoothConfig
}

// GetSecurityConfigRaw returns the raw security config
func (cm *ConfigManager) GetSecurityConfigRaw() *pb.Config_SecurityConfig {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return cm.securityConfig
}

// Module config getters

// GetMQTTConfigRaw returns the raw MQTT config
func (cm *ConfigManager) GetMQTTConfigRaw() *pb.ModuleConfig_MQTTConfig {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return cm.mqttConfig
}

// GetSerialConfigRaw returns the raw serial config
func (cm *ConfigManager) GetSerialConfigRaw() *pb.ModuleConfig_SerialConfig {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return cm.serialConfig
}

// GetExternalNotificationConfigRaw returns the raw external notification config
func (cm *ConfigManager) GetExternalNotificationConfigRaw() *pb.ModuleConfig_ExternalNotificationConfig {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return cm.externalNotificationConfig
}

// GetStoreForwardConfigRaw returns the raw store forward config
func (cm *ConfigManager) GetStoreForwardConfigRaw() *pb.ModuleConfig_StoreForwardConfig {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return cm.storeForwardConfig
}

// GetRangeTestConfigRaw returns the raw range test config
func (cm *ConfigManager) GetRangeTestConfigRaw() *pb.ModuleConfig_RangeTestConfig {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return cm.rangeTestConfig
}

// GetTelemetryConfigRaw returns the raw telemetry config
func (cm *ConfigManager) GetTelemetryConfigRaw() *pb.ModuleConfig_TelemetryConfig {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return cm.telemetryConfig
}

// GetCannedMessageConfigRaw returns the raw canned message config
func (cm *ConfigManager) GetCannedMessageConfigRaw() *pb.ModuleConfig_CannedMessageConfig {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return cm.cannedMessageConfig
}

// GetNeighborInfoConfigRaw returns the raw neighbor info config
func (cm *ConfigManager) GetNeighborInfoConfigRaw() *pb.ModuleConfig_NeighborInfoConfig {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return cm.neighborInfoConfig
}

// GetChannels returns all channels
func (cm *ConfigManager) GetChannels() []*Channel {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	channels := make([]*Channel, 0, len(cm.channels))
	for _, ch := range cm.channels {
		channels = append(channels, ch)
	}
	return channels
}

// GetChannel returns a channel by index
func (cm *ConfigManager) GetChannel(index int32) (*Channel, bool) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	ch, ok := cm.channels[index]
	return ch, ok
}

// GetConfig returns the device configuration
func (cm *ConfigManager) GetConfig() *DeviceConfig {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return cm.config
}

// ProcessChannel processes a channel message from the device
func (cm *ConfigManager) ProcessChannel(channel *pb.Channel) {
	if channel == nil {
		return
	}

	cm.mu.Lock()
	defer cm.mu.Unlock()

	index := int32(channel.Index)

	ch := &Channel{
		Index:       index,
		Role:        channelRoleToString(pb.Channel_Role(channel.Role)),
		RawSettings: channel.Settings, // Store raw settings for URL generation
	}

	if channel.Settings != nil {
		ch.Name = channel.Settings.Name
		// Don't expose PSK for security in JSON
		if len(channel.Settings.Psk) > 0 {
			ch.PSK = "[hidden]"
		}
		ch.Settings = &ChannelSettings{
			UplinkEnabled:   channel.Settings.UplinkEnabled,
			DownlinkEnabled: channel.Settings.DownlinkEnabled,
		}
	}

	cm.channels[index] = ch

	log.Info().
		Int32("index", index).
		Str("name", ch.Name).
		Str("role", ch.Role).
		Msg("processed channel")

	// Broadcast update
	cm.wsHub.Broadcast(websocket.Event{
		Type: "channel.updated",
		Data: map[string]interface{}{
			"channel": ch,
		},
	})
}

// GetRawChannels returns the raw protobuf channels for URL generation
func (cm *ConfigManager) GetRawChannels() []*pb.Channel {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	channels := make([]*pb.Channel, 0, len(cm.channels))
	for _, ch := range cm.channels {
		if ch.RawSettings != nil && ch.Role != "DISABLED" {
			channels = append(channels, &pb.Channel{
				Index:    uint32(ch.Index),
				Settings: ch.RawSettings,
				Role:     uint32(channelRoleFromString(ch.Role)),
			})
		}
	}
	return channels
}

func channelRoleFromString(role string) pb.Channel_Role {
	switch role {
	case "PRIMARY":
		return pb.Channel_PRIMARY
	case "SECONDARY":
		return pb.Channel_SECONDARY
	default:
		return pb.Channel_DISABLED
	}
}

// ProcessConfig processes a config message from the device
func (cm *ConfigManager) ProcessConfig(config *pb.Config) {
	if config == nil {
		return
	}

	cm.mu.Lock()
	defer cm.mu.Unlock()

	configType := ""

	// Process based on config type and store raw config
	if device := config.GetDevice(); device != nil {
		cm.deviceConfig = device
		cm.config.Role = int32(device.Role)
		cm.config.SerialEnabled = device.SerialEnabled
		cm.config.DebugLogEnabled = device.DebugLogEnabled
		cm.config.NodeInfoBroadcastSecs = int32(device.NodeInfoBroadcastSecs)
		cm.config.DoubleTapAsButtonPress = device.DoubleTapAsButtonPress
		cm.config.LedHeartbeatDisabled = device.LedHeartbeatDisabled
		cm.config.Tzdef = device.Tzdef
		configType = "device"
		log.Debug().Int32("role", cm.config.Role).Msg("processed device config")
	}

	if lora := config.GetLora(); lora != nil {
		cm.loraConfig = lora
		cm.config.Region = int32(lora.Region)
		cm.config.ModemPreset = int32(lora.ModemPreset)
		cm.config.HopLimit = int32(lora.HopLimit)
		cm.config.TxEnabled = lora.TxEnabled
		cm.config.TxPower = lora.TxPower
		cm.config.ChannelNum = int32(lora.ChannelNum)
		cm.config.OverrideDutyCycle = lora.OverrideDutyCycle
		cm.config.FrequencyOffset = lora.FrequencyOffset
		configType = "lora"
		log.Debug().Int32("region", cm.config.Region).Int32("modem", cm.config.ModemPreset).Msg("processed lora config")
	}

	if position := config.GetPosition(); position != nil {
		cm.positionConfig = position
		cm.config.GpsEnabled = position.GpsEnabled
		cm.config.PositionBroadcastSecs = int32(position.PositionBroadcastSecs)
		cm.config.FixedPosition = position.FixedPosition
		cm.config.PositionBroadcastSmartEnabled = position.PositionBroadcastSmartEnabled
		cm.config.GpsUpdateInterval = int32(position.GpsUpdateInterval)
		cm.config.BroadcastSmartMinimumDistance = int32(position.BroadcastSmartMinimumDistance)
		cm.config.BroadcastSmartMinimumIntervalSecs = int32(position.BroadcastSmartMinimumIntervalSecs)
		configType = "position"
		log.Debug().Bool("gpsEnabled", cm.config.GpsEnabled).Msg("processed position config")
	}

	if power := config.GetPower(); power != nil {
		cm.powerConfig = power
		cm.config.IsPowerSaving = power.IsPowerSaving
		cm.config.OnBatteryShutdownAfterSecs = int32(power.OnBatteryShutdownAfterSecs)
		cm.config.WaitBluetoothSecs = int32(power.WaitBluetoothSecs)
		cm.config.LsSecs = int32(power.LsSecs)
		cm.config.MinWakeSecs = int32(power.MinWakeSecs)
		configType = "power"
		log.Debug().Bool("powerSaving", cm.config.IsPowerSaving).Msg("processed power config")
	}

	if display := config.GetDisplay(); display != nil {
		cm.displayConfig = display
		cm.config.ScreenOnSecs = int32(display.ScreenOnSecs)
		cm.config.GpsFormat = int32(display.GpsFormat)
		cm.config.AutoScreenCarouselSecs = int32(display.AutoScreenCarouselSecs)
		cm.config.FlipScreen = display.FlipScreen
		cm.config.CompassNorthTop = display.CompassNorthTop
		cm.config.Units = int32(display.Units)
		cm.config.WakeOnTapOrMotion = display.WakeOnTapOrMotion
		configType = "display"
		log.Debug().Int32("screenOnSecs", cm.config.ScreenOnSecs).Msg("processed display config")
	}

	if bluetooth := config.GetBluetooth(); bluetooth != nil {
		cm.bluetoothConfig = bluetooth
		cm.config.BluetoothEnabled = bluetooth.Enabled
		cm.config.BluetoothMode = int32(bluetooth.Mode)
		cm.config.BluetoothFixedPin = int32(bluetooth.FixedPin)
		configType = "bluetooth"
		log.Debug().Bool("btEnabled", cm.config.BluetoothEnabled).Msg("processed bluetooth config")
	}

	if network := config.GetNetwork(); network != nil {
		cm.networkConfig = network
		cm.config.WifiEnabled = network.WifiEnabled
		cm.config.WifiSsid = network.WifiSsid
		cm.config.NtpServer = network.NtpServer
		cm.config.EthEnabled = network.EthEnabled
		configType = "network"
		log.Debug().Bool("wifiEnabled", network.WifiEnabled).Msg("processed network config")
	}

	if security := config.GetSecurity(); security != nil {
		cm.securityConfig = security
		configType = "security"
		log.Debug().Bool("adminChannelEnabled", security.AdminChannelEnabled).Msg("processed security config")
	}

	// Broadcast config update
	cm.wsHub.Broadcast(websocket.Event{
		Type: "config.updated",
		Data: map[string]interface{}{
			"configType": configType,
			"config":     cm.config,
		},
	})
}

// ProcessModuleConfig processes a module config message from the device
func (cm *ConfigManager) ProcessModuleConfig(moduleConfig *pb.ModuleConfig) {
	if moduleConfig == nil {
		return
	}

	cm.mu.Lock()
	defer cm.mu.Unlock()

	moduleType := ""

	if mqtt := moduleConfig.GetMqtt(); mqtt != nil {
		cm.mqttConfig = mqtt
		moduleType = "mqtt"
		log.Debug().Bool("enabled", mqtt.Enabled).Msg("processed MQTT module config")
	}

	if serial := moduleConfig.GetSerial(); serial != nil {
		cm.serialConfig = serial
		moduleType = "serial"
		log.Debug().Bool("enabled", serial.Enabled).Msg("processed serial module config")
	}

	if extNotif := moduleConfig.GetExternalNotification(); extNotif != nil {
		cm.externalNotificationConfig = extNotif
		moduleType = "externalNotification"
		log.Debug().Bool("enabled", extNotif.Enabled).Msg("processed external notification module config")
	}

	if storeForward := moduleConfig.GetStoreForward(); storeForward != nil {
		cm.storeForwardConfig = storeForward
		moduleType = "storeForward"
		log.Debug().Bool("enabled", storeForward.Enabled).Msg("processed store forward module config")
	}

	if rangeTest := moduleConfig.GetRangeTest(); rangeTest != nil {
		cm.rangeTestConfig = rangeTest
		moduleType = "rangeTest"
		log.Debug().Bool("enabled", rangeTest.Enabled).Msg("processed range test module config")
	}

	if telemetry := moduleConfig.GetTelemetry(); telemetry != nil {
		cm.telemetryConfig = telemetry
		moduleType = "telemetry"
		log.Debug().Uint32("deviceInterval", telemetry.DeviceUpdateInterval).Msg("processed telemetry module config")
	}

	if cannedMsg := moduleConfig.GetCannedMessage(); cannedMsg != nil {
		cm.cannedMessageConfig = cannedMsg
		moduleType = "cannedMessage"
		log.Debug().Bool("enabled", cannedMsg.Enabled).Msg("processed canned message module config")
	}

	if neighborInfo := moduleConfig.GetNeighborInfo(); neighborInfo != nil {
		cm.neighborInfoConfig = neighborInfo
		moduleType = "neighborInfo"
		log.Debug().Bool("enabled", neighborInfo.Enabled).Msg("processed neighbor info module config")
	}

	// Broadcast module config update
	cm.wsHub.Broadcast(websocket.Event{
		Type: "moduleConfig.updated",
		Data: map[string]interface{}{
			"moduleType": moduleType,
		},
	})
}

// ProcessMetadata processes device metadata
func (cm *ConfigManager) ProcessMetadata(metadata *pb.DeviceMetadata) {
	if metadata == nil {
		return
	}

	cm.mu.Lock()
	defer cm.mu.Unlock()

	cm.metadata = metadata
	cm.config.FirmwareVersion = metadata.FirmwareVersion
	cm.config.HardwareModel = metadata.HwModel.String()
	cm.config.HasWifi = metadata.HasWifi
	cm.config.HasBluetooth = metadata.HasBluetooth
	cm.config.HasEthernet = metadata.HasEthernet
	cm.config.CanShutdown = metadata.CanShutdown

	log.Info().
		Str("firmware", cm.config.FirmwareVersion).
		Str("hardware", cm.config.HardwareModel).
		Msg("processed device metadata")

	// Broadcast metadata update
	cm.wsHub.Broadcast(websocket.Event{
		Type: "metadata.updated",
		Data: map[string]interface{}{
			"metadata": map[string]interface{}{
				"firmwareVersion": cm.config.FirmwareVersion,
				"hardwareModel":   cm.config.HardwareModel,
				"hasWifi":         cm.config.HasWifi,
				"hasBluetooth":    cm.config.HasBluetooth,
				"hasEthernet":     cm.config.HasEthernet,
			},
		},
	})
}

// ClearConfig clears all stored configuration (on disconnect)
func (cm *ConfigManager) ClearConfig() {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	cm.channels = make(map[int32]*Channel)
	cm.config = &DeviceConfig{}
	cm.metadata = nil
}

func channelRoleToString(role pb.Channel_Role) string {
	switch role {
	case pb.Channel_DISABLED:
		return "DISABLED"
	case pb.Channel_PRIMARY:
		return "PRIMARY"
	case pb.Channel_SECONDARY:
		return "SECONDARY"
	default:
		return "UNKNOWN"
	}
}

