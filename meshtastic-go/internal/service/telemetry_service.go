package service

import (
	"sync"
	"time"

	"github.com/meshtastic/meshtastic-go/internal/database/dao"
	"github.com/meshtastic/meshtastic-go/internal/websocket"
	"github.com/meshtastic/meshtastic-go/pkg/pb"
	"github.com/rs/zerolog/log"
)

// TelemetryService handles telemetry processing and storage
type TelemetryService struct {
	mu           sync.RWMutex
	telemetryDAO *dao.TelemetryDAO
	wsHub        *websocket.Hub

	// In-memory cache of latest telemetry per node per type
	latestDevice      map[uint32]*pb.DeviceMetrics
	latestEnvironment map[uint32]*pb.EnvironmentMetrics
	latestPower       map[uint32]*pb.PowerMetrics
	latestAirQuality  map[uint32]*pb.AirQualityMetrics
	latestLocalStats  map[uint32]*pb.LocalStats
}

// NewTelemetryService creates a new telemetry service
func NewTelemetryService(telemetryDAO *dao.TelemetryDAO, wsHub *websocket.Hub) *TelemetryService {
	return &TelemetryService{
		telemetryDAO:      telemetryDAO,
		wsHub:             wsHub,
		latestDevice:      make(map[uint32]*pb.DeviceMetrics),
		latestEnvironment: make(map[uint32]*pb.EnvironmentMetrics),
		latestPower:       make(map[uint32]*pb.PowerMetrics),
		latestAirQuality:  make(map[uint32]*pb.AirQualityMetrics),
		latestLocalStats:  make(map[uint32]*pb.LocalStats),
	}
}

// ProcessTelemetry processes incoming telemetry and stores it
func (ts *TelemetryService) ProcessTelemetry(nodeNum uint32, telemetry *pb.Telemetry, viaMqtt bool) {
	if telemetry == nil {
		return
	}

	timestamp := int64(telemetry.Time)
	if timestamp == 0 {
		timestamp = time.Now().Unix()
	}

	ts.mu.Lock()
	defer ts.mu.Unlock()

	// Process based on telemetry type
	switch v := telemetry.Variant.(type) {
	case *pb.Telemetry_DeviceMetrics:
		if v.DeviceMetrics != nil {
			ts.processDeviceMetrics(nodeNum, v.DeviceMetrics, timestamp, viaMqtt)
		}
	case *pb.Telemetry_EnvironmentMetrics:
		if v.EnvironmentMetrics != nil {
			ts.processEnvironmentMetrics(nodeNum, v.EnvironmentMetrics, timestamp, viaMqtt)
		}
	case *pb.Telemetry_PowerMetrics:
		if v.PowerMetrics != nil {
			ts.processPowerMetrics(nodeNum, v.PowerMetrics, timestamp, viaMqtt)
		}
	case *pb.Telemetry_AirQualityMetrics:
		if v.AirQualityMetrics != nil {
			ts.processAirQualityMetrics(nodeNum, v.AirQualityMetrics, timestamp, viaMqtt)
		}
	case *pb.Telemetry_LocalStats:
		if v.LocalStats != nil {
			ts.processLocalStats(nodeNum, v.LocalStats, timestamp, viaMqtt)
		}
	}
}

func (ts *TelemetryService) processDeviceMetrics(nodeNum uint32, dm *pb.DeviceMetrics, timestamp int64, viaMqtt bool) {
	// Merge with existing metrics to preserve data from partial updates
	existing := ts.latestDevice[nodeNum]
	if existing == nil {
		existing = &pb.DeviceMetrics{}
	}
	// Only update non-zero values
	if dm.BatteryLevel != 0 {
		existing.BatteryLevel = dm.BatteryLevel
	}
	if dm.Voltage != 0 {
		existing.Voltage = dm.Voltage
	}
	if dm.ChannelUtilization != 0 {
		existing.ChannelUtilization = dm.ChannelUtilization
	}
	if dm.AirUtilTx != 0 {
		existing.AirUtilTx = dm.AirUtilTx
	}
	if dm.UptimeSeconds != 0 {
		existing.UptimeSeconds = dm.UptimeSeconds
	}
	ts.latestDevice[nodeNum] = existing

	// Store in database (use merged values)
	entity := &dao.TelemetryEntity{
		NodeNum:       nodeNum,
		TelemetryType: dao.TelemetryTypeDevice,
		Timestamp:     timestamp,
		ViaMqtt:       viaMqtt,
	}

	batteryLevel := int32(existing.BatteryLevel)
	entity.BatteryLevel = &batteryLevel
	entity.Voltage = &existing.Voltage
	entity.ChannelUtilization = &existing.ChannelUtilization
	entity.AirUtilTx = &existing.AirUtilTx
	uptimeSeconds := int64(existing.UptimeSeconds)
	entity.UptimeSeconds = &uptimeSeconds

	if err := ts.telemetryDAO.Insert(entity); err != nil {
		log.Warn().Err(err).Uint32("nodeNum", nodeNum).Msg("failed to store device telemetry")
	}

	// Broadcast to WebSocket clients (use merged values)
	ts.wsHub.Broadcast(websocket.Event{
		Type: "telemetry.device",
		Data: map[string]interface{}{
			"nodeNum":            nodeNum,
			"timestamp":          timestamp,
			"batteryLevel":       existing.BatteryLevel,
			"voltage":            existing.Voltage,
			"channelUtilization": existing.ChannelUtilization,
			"airUtilTx":          existing.AirUtilTx,
			"uptimeSeconds":      existing.UptimeSeconds,
			"viaMqtt":            viaMqtt,
		},
	})

	log.Debug().
		Uint32("nodeNum", nodeNum).
		Uint32("battery", existing.BatteryLevel).
		Float32("voltage", existing.Voltage).
		Msg("processed device telemetry")
}

func (ts *TelemetryService) processEnvironmentMetrics(nodeNum uint32, em *pb.EnvironmentMetrics, timestamp int64, viaMqtt bool) {
	// Merge with existing metrics to preserve data from partial updates
	existing := ts.latestEnvironment[nodeNum]
	if existing == nil {
		existing = &pb.EnvironmentMetrics{}
	}
	// Only update non-zero values
	if em.Temperature != 0 {
		existing.Temperature = em.Temperature
	}
	if em.RelativeHumidity != 0 {
		existing.RelativeHumidity = em.RelativeHumidity
	}
	if em.BarometricPressure != 0 {
		existing.BarometricPressure = em.BarometricPressure
	}
	if em.GasResistance != 0 {
		existing.GasResistance = em.GasResistance
	}
	if em.Iaq != 0 {
		existing.Iaq = em.Iaq
	}
	if em.Distance != 0 {
		existing.Distance = em.Distance
	}
	if em.Lux != 0 {
		existing.Lux = em.Lux
	}
	if em.UvLux != 0 {
		existing.UvLux = em.UvLux
	}
	if em.WindSpeed != 0 {
		existing.WindSpeed = em.WindSpeed
	}
	if em.WindDirection != 0 {
		existing.WindDirection = em.WindDirection
	}
	if em.Rainfall != 0 {
		existing.Rainfall = em.Rainfall
	}
	ts.latestEnvironment[nodeNum] = existing

	// Store in database (use merged values)
	entity := &dao.TelemetryEntity{
		NodeNum:       nodeNum,
		TelemetryType: dao.TelemetryTypeEnvironment,
		Timestamp:     timestamp,
		ViaMqtt:       viaMqtt,
	}

	entity.Temperature = &existing.Temperature
	entity.RelativeHumidity = &existing.RelativeHumidity
	entity.BarometricPressure = &existing.BarometricPressure
	entity.GasResistance = &existing.GasResistance
	iaq := int32(existing.Iaq)
	entity.Iaq = &iaq
	entity.Distance = &existing.Distance
	entity.Lux = &existing.Lux
	entity.UvLux = &existing.UvLux
	entity.WindSpeed = &existing.WindSpeed
	windDir := int32(existing.WindDirection)
	entity.WindDirection = &windDir
	entity.Rainfall = &existing.Rainfall

	if err := ts.telemetryDAO.Insert(entity); err != nil {
		log.Warn().Err(err).Uint32("nodeNum", nodeNum).Msg("failed to store environment telemetry")
	}

	// Broadcast to WebSocket clients (use merged values)
	ts.wsHub.Broadcast(websocket.Event{
		Type: "telemetry.environment",
		Data: map[string]interface{}{
			"nodeNum":            nodeNum,
			"timestamp":          timestamp,
			"temperature":        existing.Temperature,
			"relativeHumidity":   existing.RelativeHumidity,
			"barometricPressure": existing.BarometricPressure,
			"gasResistance":      existing.GasResistance,
			"iaq":                existing.Iaq,
			"distance":           existing.Distance,
			"lux":                existing.Lux,
			"uvLux":              existing.UvLux,
			"windSpeed":          existing.WindSpeed,
			"windDirection":      existing.WindDirection,
			"rainfall":           existing.Rainfall,
			"viaMqtt":            viaMqtt,
		},
	})

	log.Debug().
		Uint32("nodeNum", nodeNum).
		Float32("temp", existing.Temperature).
		Float32("humidity", existing.RelativeHumidity).
		Msg("processed environment telemetry")
}

func (ts *TelemetryService) processPowerMetrics(nodeNum uint32, pm *pb.PowerMetrics, timestamp int64, viaMqtt bool) {
	ts.latestPower[nodeNum] = pm

	// Store in database
	entity := &dao.TelemetryEntity{
		NodeNum:       nodeNum,
		TelemetryType: dao.TelemetryTypePower,
		Timestamp:     timestamp,
		ViaMqtt:       viaMqtt,
	}

	entity.Ch1Voltage = &pm.Ch1Voltage
	entity.Ch1Current = &pm.Ch1Current
	entity.Ch2Voltage = &pm.Ch2Voltage
	entity.Ch2Current = &pm.Ch2Current
	entity.Ch3Voltage = &pm.Ch3Voltage
	entity.Ch3Current = &pm.Ch3Current

	if err := ts.telemetryDAO.Insert(entity); err != nil {
		log.Warn().Err(err).Uint32("nodeNum", nodeNum).Msg("failed to store power telemetry")
	}

	// Broadcast to WebSocket clients
	ts.wsHub.Broadcast(websocket.Event{
		Type: "telemetry.power",
		Data: map[string]interface{}{
			"nodeNum":    nodeNum,
			"timestamp":  timestamp,
			"ch1Voltage": pm.Ch1Voltage,
			"ch1Current": pm.Ch1Current,
			"ch2Voltage": pm.Ch2Voltage,
			"ch2Current": pm.Ch2Current,
			"ch3Voltage": pm.Ch3Voltage,
			"ch3Current": pm.Ch3Current,
			"viaMqtt":    viaMqtt,
		},
	})

	log.Debug().
		Uint32("nodeNum", nodeNum).
		Float32("ch1V", pm.Ch1Voltage).
		Msg("processed power telemetry")
}

func (ts *TelemetryService) processAirQualityMetrics(nodeNum uint32, aq *pb.AirQualityMetrics, timestamp int64, viaMqtt bool) {
	ts.latestAirQuality[nodeNum] = aq

	// Store in database
	entity := &dao.TelemetryEntity{
		NodeNum:       nodeNum,
		TelemetryType: dao.TelemetryTypeAirQuality,
		Timestamp:     timestamp,
		ViaMqtt:       viaMqtt,
	}

	pm10 := int32(aq.Pm10Standard)
	pm25 := int32(aq.Pm25Standard)
	pm100 := int32(aq.Pm100Standard)
	co2 := int32(aq.Co2)
	entity.Pm10 = &pm10
	entity.Pm25 = &pm25
	entity.Pm100 = &pm100
	entity.Co2 = &co2

	if err := ts.telemetryDAO.Insert(entity); err != nil {
		log.Warn().Err(err).Uint32("nodeNum", nodeNum).Msg("failed to store air quality telemetry")
	}

	// Broadcast to WebSocket clients
	ts.wsHub.Broadcast(websocket.Event{
		Type: "telemetry.airQuality",
		Data: map[string]interface{}{
			"nodeNum":   nodeNum,
			"timestamp": timestamp,
			"pm10":      aq.Pm10Standard,
			"pm25":      aq.Pm25Standard,
			"pm100":     aq.Pm100Standard,
			"co2":       aq.Co2,
			"viaMqtt":   viaMqtt,
		},
	})

	log.Debug().
		Uint32("nodeNum", nodeNum).
		Uint32("pm25", aq.Pm25Standard).
		Uint32("co2", aq.Co2).
		Msg("processed air quality telemetry")
}

func (ts *TelemetryService) processLocalStats(nodeNum uint32, ls *pb.LocalStats, timestamp int64, viaMqtt bool) {
	ts.latestLocalStats[nodeNum] = ls

	// Store in database (reusing device fields for local stats)
	entity := &dao.TelemetryEntity{
		NodeNum:       nodeNum,
		TelemetryType: dao.TelemetryTypeLocalStats,
		Timestamp:     timestamp,
		ViaMqtt:       viaMqtt,
	}

	uptimeSeconds := int64(ls.UptimeSeconds)
	entity.UptimeSeconds = &uptimeSeconds
	entity.ChannelUtilization = &ls.ChannelUtilization
	entity.AirUtilTx = &ls.AirUtilTx

	if err := ts.telemetryDAO.Insert(entity); err != nil {
		log.Warn().Err(err).Uint32("nodeNum", nodeNum).Msg("failed to store local stats telemetry")
	}

	// Broadcast to WebSocket clients
	ts.wsHub.Broadcast(websocket.Event{
		Type: "telemetry.localStats",
		Data: map[string]interface{}{
			"nodeNum":            nodeNum,
			"timestamp":          timestamp,
			"uptimeSeconds":      ls.UptimeSeconds,
			"channelUtilization": ls.ChannelUtilization,
			"airUtilTx":          ls.AirUtilTx,
			"numPacketsTx":       ls.NumPacketsTx,
			"numPacketsRx":       ls.NumPacketsRx,
			"numPacketsRxBad":    ls.NumPacketsRxBad,
			"numOnlineNodes":     ls.NumOnlineNodes,
			"numTotalNodes":      ls.NumTotalNodes,
			"viaMqtt":            viaMqtt,
		},
	})

	log.Debug().
		Uint32("nodeNum", nodeNum).
		Uint32("uptime", ls.UptimeSeconds).
		Uint32("packetsTx", ls.NumPacketsTx).
		Msg("processed local stats telemetry")
}

// GetLatestDevice returns the latest device metrics for a node
func (ts *TelemetryService) GetLatestDevice(nodeNum uint32) *pb.DeviceMetrics {
	ts.mu.RLock()
	defer ts.mu.RUnlock()
	return ts.latestDevice[nodeNum]
}

// GetLatestEnvironment returns the latest environment metrics for a node
func (ts *TelemetryService) GetLatestEnvironment(nodeNum uint32) *pb.EnvironmentMetrics {
	ts.mu.RLock()
	defer ts.mu.RUnlock()
	return ts.latestEnvironment[nodeNum]
}

// GetLatestPower returns the latest power metrics for a node
func (ts *TelemetryService) GetLatestPower(nodeNum uint32) *pb.PowerMetrics {
	ts.mu.RLock()
	defer ts.mu.RUnlock()
	return ts.latestPower[nodeNum]
}

// GetLatestAirQuality returns the latest air quality metrics for a node
func (ts *TelemetryService) GetLatestAirQuality(nodeNum uint32) *pb.AirQualityMetrics {
	ts.mu.RLock()
	defer ts.mu.RUnlock()
	return ts.latestAirQuality[nodeNum]
}

// GetLatestLocalStats returns the latest local stats for a node
func (ts *TelemetryService) GetLatestLocalStats(nodeNum uint32) *pb.LocalStats {
	ts.mu.RLock()
	defer ts.mu.RUnlock()
	return ts.latestLocalStats[nodeNum]
}

// GetAllLatest returns all latest telemetry for a node
func (ts *TelemetryService) GetAllLatest(nodeNum uint32) map[string]interface{} {
	ts.mu.RLock()
	defer ts.mu.RUnlock()

	result := make(map[string]interface{})
	if dm := ts.latestDevice[nodeNum]; dm != nil {
		result["device"] = dm
	}
	if em := ts.latestEnvironment[nodeNum]; em != nil {
		result["environment"] = em
	}
	if pm := ts.latestPower[nodeNum]; pm != nil {
		result["power"] = pm
	}
	if aq := ts.latestAirQuality[nodeNum]; aq != nil {
		result["airQuality"] = aq
	}
	if ls := ts.latestLocalStats[nodeNum]; ls != nil {
		result["localStats"] = ls
	}
	return result
}

// GetHistory retrieves historical telemetry for a node
func (ts *TelemetryService) GetHistory(nodeNum uint32, telemetryType string, limit int) ([]*dao.TelemetryEntity, error) {
	if telemetryType == "" {
		return ts.telemetryDAO.GetByNodeNum(nodeNum, limit)
	}
	return ts.telemetryDAO.GetByNodeNumAndType(nodeNum, dao.TelemetryType(telemetryType), limit)
}

// GetHistoryInRange retrieves historical telemetry within a time range
func (ts *TelemetryService) GetHistoryInRange(nodeNum uint32, startTime, endTime int64) ([]*dao.TelemetryEntity, error) {
	return ts.telemetryDAO.GetByNodeNumInRange(nodeNum, startTime, endTime)
}

// GetDeviceStats returns device metrics statistics
func (ts *TelemetryService) GetDeviceStats(nodeNum uint32, since int64) (map[string]*dao.TelemetryStats, error) {
	return ts.telemetryDAO.GetDeviceStats(nodeNum, since)
}

// GetEnvironmentStats returns environment metrics statistics
func (ts *TelemetryService) GetEnvironmentStats(nodeNum uint32, since int64) (map[string]*dao.TelemetryStats, error) {
	return ts.telemetryDAO.GetEnvironmentStats(nodeNum, since)
}

// CleanupOldData removes telemetry data older than the specified duration
func (ts *TelemetryService) CleanupOldData(maxAge time.Duration) (int64, error) {
	cutoff := time.Now().Add(-maxAge)
	return ts.telemetryDAO.DeleteOlderThan(cutoff)
}
