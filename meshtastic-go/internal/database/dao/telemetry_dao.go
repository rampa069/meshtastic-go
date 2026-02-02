package dao

import (
	"database/sql"
	"time"
)

// TelemetryType identifies the type of telemetry
type TelemetryType string

const (
	TelemetryTypeDevice      TelemetryType = "device"
	TelemetryTypeEnvironment TelemetryType = "environment"
	TelemetryTypePower       TelemetryType = "power"
	TelemetryTypeAirQuality  TelemetryType = "air_quality"
	TelemetryTypeLocalStats  TelemetryType = "local_stats"
	TelemetryTypeHealth      TelemetryType = "health"
)

// TelemetryEntity represents a telemetry record in the database
type TelemetryEntity struct {
	ID            int64
	NodeNum       uint32
	TelemetryType TelemetryType
	Timestamp     int64
	ViaMqtt       bool

	// Device metrics
	BatteryLevel       *int32
	Voltage            *float32
	ChannelUtilization *float32
	AirUtilTx          *float32
	UptimeSeconds      *int64

	// Environment metrics
	Temperature        *float32
	RelativeHumidity   *float32
	BarometricPressure *float32
	GasResistance      *float32
	Iaq                *int32
	Distance           *float32
	Lux                *float32
	UvLux              *float32
	WindSpeed          *float32
	WindDirection      *int32
	Rainfall           *float32

	// Power metrics
	Ch1Voltage *float32
	Ch1Current *float32
	Ch2Voltage *float32
	Ch2Current *float32
	Ch3Voltage *float32
	Ch3Current *float32

	// Air quality metrics
	Pm10  *int32
	Pm25  *int32
	Pm100 *int32
	Co2   *int32

	// Raw protobuf data
	RawData []byte
}

// TelemetryStats contains statistics for a telemetry field
type TelemetryStats struct {
	NodeNum   uint32        `json:"nodeNum"`
	Type      TelemetryType `json:"type"`
	Field     string        `json:"field"`
	Min       float64       `json:"min"`
	Max       float64       `json:"max"`
	Avg       float64       `json:"avg"`
	Count     int           `json:"count"`
	FirstTime int64         `json:"firstTime"`
	LastTime  int64         `json:"lastTime"`
}

// TelemetryAggregatePoint represents aggregated telemetry data for a time period
type TelemetryAggregatePoint struct {
	Timestamp int64   `json:"timestamp"`
	Avg       float64 `json:"avg"`
	Min       float64 `json:"min"`
	Max       float64 `json:"max"`
	Count     int     `json:"count"`
}

// TelemetryDAO provides data access operations for telemetry
type TelemetryDAO struct {
	db *sql.DB
}

// NewTelemetryDAO creates a new TelemetryDAO
func NewTelemetryDAO(db *sql.DB) *TelemetryDAO {
	return &TelemetryDAO{db: db}
}

// Insert creates a new telemetry record
func (d *TelemetryDAO) Insert(t *TelemetryEntity) error {
	query := `
		INSERT INTO telemetry (
			node_num, telemetry_type, timestamp, via_mqtt,
			battery_level, voltage, channel_utilization, air_util_tx, uptime_seconds,
			temperature, relative_humidity, barometric_pressure, gas_resistance, iaq,
			distance, lux, uv_lux, wind_speed, wind_direction, rainfall,
			ch1_voltage, ch1_current, ch2_voltage, ch2_current, ch3_voltage, ch3_current,
			pm10, pm25, pm100, co2, raw_data
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	result, err := d.db.Exec(query,
		t.NodeNum, t.TelemetryType, t.Timestamp, boolToInt(t.ViaMqtt),
		t.BatteryLevel, t.Voltage, t.ChannelUtilization, t.AirUtilTx, t.UptimeSeconds,
		t.Temperature, t.RelativeHumidity, t.BarometricPressure, t.GasResistance, t.Iaq,
		t.Distance, t.Lux, t.UvLux, t.WindSpeed, t.WindDirection, t.Rainfall,
		t.Ch1Voltage, t.Ch1Current, t.Ch2Voltage, t.Ch2Current, t.Ch3Voltage, t.Ch3Current,
		t.Pm10, t.Pm25, t.Pm100, t.Co2, t.RawData,
	)
	if err != nil {
		return err
	}
	id, err := result.LastInsertId()
	if err == nil {
		t.ID = id
	}
	return err
}

// GetByNodeNum retrieves all telemetry for a node
func (d *TelemetryDAO) GetByNodeNum(nodeNum uint32, limit int) ([]*TelemetryEntity, error) {
	query := `
		SELECT id, node_num, telemetry_type, timestamp, via_mqtt,
			battery_level, voltage, channel_utilization, air_util_tx, uptime_seconds,
			temperature, relative_humidity, barometric_pressure, gas_resistance, iaq,
			distance, lux, uv_lux, wind_speed, wind_direction, rainfall,
			ch1_voltage, ch1_current, ch2_voltage, ch2_current, ch3_voltage, ch3_current,
			pm10, pm25, pm100, co2, raw_data
		FROM telemetry WHERE node_num = ? ORDER BY timestamp DESC LIMIT ?
	`
	return d.queryTelemetry(query, nodeNum, limit)
}

// GetByNodeNumAndType retrieves telemetry for a node filtered by type
func (d *TelemetryDAO) GetByNodeNumAndType(nodeNum uint32, telemetryType TelemetryType, limit int) ([]*TelemetryEntity, error) {
	query := `
		SELECT id, node_num, telemetry_type, timestamp, via_mqtt,
			battery_level, voltage, channel_utilization, air_util_tx, uptime_seconds,
			temperature, relative_humidity, barometric_pressure, gas_resistance, iaq,
			distance, lux, uv_lux, wind_speed, wind_direction, rainfall,
			ch1_voltage, ch1_current, ch2_voltage, ch2_current, ch3_voltage, ch3_current,
			pm10, pm25, pm100, co2, raw_data
		FROM telemetry WHERE node_num = ? AND telemetry_type = ? ORDER BY timestamp DESC LIMIT ?
	`
	return d.queryTelemetry(query, nodeNum, telemetryType, limit)
}

// GetByNodeNumInRange retrieves telemetry for a node within a time range
func (d *TelemetryDAO) GetByNodeNumInRange(nodeNum uint32, startTime, endTime int64) ([]*TelemetryEntity, error) {
	query := `
		SELECT id, node_num, telemetry_type, timestamp, via_mqtt,
			battery_level, voltage, channel_utilization, air_util_tx, uptime_seconds,
			temperature, relative_humidity, barometric_pressure, gas_resistance, iaq,
			distance, lux, uv_lux, wind_speed, wind_direction, rainfall,
			ch1_voltage, ch1_current, ch2_voltage, ch2_current, ch3_voltage, ch3_current,
			pm10, pm25, pm100, co2, raw_data
		FROM telemetry WHERE node_num = ? AND timestamp >= ? AND timestamp <= ? ORDER BY timestamp ASC
	`
	return d.queryTelemetry(query, nodeNum, startTime, endTime)
}

// GetLatestByNode retrieves the most recent telemetry of each type for a node
func (d *TelemetryDAO) GetLatestByNode(nodeNum uint32) (map[TelemetryType]*TelemetryEntity, error) {
	query := `
		SELECT id, node_num, telemetry_type, timestamp, via_mqtt,
			battery_level, voltage, channel_utilization, air_util_tx, uptime_seconds,
			temperature, relative_humidity, barometric_pressure, gas_resistance, iaq,
			distance, lux, uv_lux, wind_speed, wind_direction, rainfall,
			ch1_voltage, ch1_current, ch2_voltage, ch2_current, ch3_voltage, ch3_current,
			pm10, pm25, pm100, co2, raw_data
		FROM telemetry t1
		WHERE node_num = ? AND timestamp = (
			SELECT MAX(timestamp) FROM telemetry t2
			WHERE t2.node_num = t1.node_num AND t2.telemetry_type = t1.telemetry_type
		)
	`
	telemetryList, err := d.queryTelemetry(query, nodeNum)
	if err != nil {
		return nil, err
	}

	result := make(map[TelemetryType]*TelemetryEntity)
	for _, t := range telemetryList {
		result[t.TelemetryType] = t
	}
	return result, nil
}

// GetStats calculates statistics for a specific field
func (d *TelemetryDAO) GetStats(nodeNum uint32, telemetryType TelemetryType, field string, since int64) (*TelemetryStats, error) {
	query := `
		SELECT
			MIN(` + field + `) as min_val,
			MAX(` + field + `) as max_val,
			AVG(` + field + `) as avg_val,
			COUNT(*) as count,
			MIN(timestamp) as first_time,
			MAX(timestamp) as last_time
		FROM telemetry
		WHERE node_num = ? AND telemetry_type = ? AND timestamp >= ? AND ` + field + ` IS NOT NULL
	`

	stats := &TelemetryStats{
		NodeNum: nodeNum,
		Type:    telemetryType,
		Field:   field,
	}

	var minVal, maxVal, avgVal sql.NullFloat64
	var count int
	var firstTime, lastTime sql.NullInt64

	err := d.db.QueryRow(query, nodeNum, telemetryType, since).Scan(
		&minVal, &maxVal, &avgVal, &count, &firstTime, &lastTime,
	)
	if err != nil {
		return nil, err
	}

	if minVal.Valid {
		stats.Min = minVal.Float64
	}
	if maxVal.Valid {
		stats.Max = maxVal.Float64
	}
	if avgVal.Valid {
		stats.Avg = avgVal.Float64
	}
	stats.Count = count
	if firstTime.Valid {
		stats.FirstTime = firstTime.Int64
	}
	if lastTime.Valid {
		stats.LastTime = lastTime.Int64
	}

	return stats, nil
}

// GetDeviceStats returns device metrics statistics
func (d *TelemetryDAO) GetDeviceStats(nodeNum uint32, since int64) (map[string]*TelemetryStats, error) {
	fields := []string{"battery_level", "voltage", "channel_utilization", "air_util_tx"}
	result := make(map[string]*TelemetryStats)

	for _, field := range fields {
		stats, err := d.GetStats(nodeNum, TelemetryTypeDevice, field, since)
		if err != nil {
			return nil, err
		}
		if stats.Count > 0 {
			result[field] = stats
		}
	}
	return result, nil
}

// GetEnvironmentStats returns environment metrics statistics
func (d *TelemetryDAO) GetEnvironmentStats(nodeNum uint32, since int64) (map[string]*TelemetryStats, error) {
	fields := []string{"temperature", "relative_humidity", "barometric_pressure"}
	result := make(map[string]*TelemetryStats)

	for _, field := range fields {
		stats, err := d.GetStats(nodeNum, TelemetryTypeEnvironment, field, since)
		if err != nil {
			return nil, err
		}
		if stats.Count > 0 {
			result[field] = stats
		}
	}
	return result, nil
}

// GetAggregatedByPeriod retrieves aggregated telemetry data grouped by time period
// Supported fields: battery_level, voltage, channel_utilization, air_util_tx, uptime_seconds,
// temperature, relative_humidity, barometric_pressure, etc.
func (d *TelemetryDAO) GetAggregatedByPeriod(
	nodeNum uint32,
	telemetryType TelemetryType,
	field string,
	startTime, endTime int64,
	periodSeconds int64,
) ([]*TelemetryAggregatePoint, error) {
	// Validate field name to prevent SQL injection (only allow known columns)
	validFields := map[string]bool{
		"battery_level": true, "voltage": true, "channel_utilization": true,
		"air_util_tx": true, "uptime_seconds": true, "temperature": true,
		"relative_humidity": true, "barometric_pressure": true, "gas_resistance": true,
		"iaq": true, "distance": true, "lux": true, "uv_lux": true,
		"wind_speed": true, "wind_direction": true, "rainfall": true,
		"ch1_voltage": true, "ch1_current": true, "ch2_voltage": true,
		"ch2_current": true, "ch3_voltage": true, "ch3_current": true,
		"pm10": true, "pm25": true, "pm100": true, "co2": true,
	}
	if !validFields[field] {
		return nil, nil // Invalid field, return empty
	}

	query := `
		SELECT
			(timestamp / ?) * ? as period_start,
			AVG(` + field + `) as avg_val,
			MIN(` + field + `) as min_val,
			MAX(` + field + `) as max_val,
			COUNT(*) as count
		FROM telemetry
		WHERE node_num = ? AND telemetry_type = ? AND timestamp >= ? AND timestamp <= ? AND ` + field + ` IS NOT NULL
		GROUP BY period_start
		ORDER BY period_start ASC
	`

	rows, err := d.db.Query(query, periodSeconds, periodSeconds, nodeNum, telemetryType, startTime, endTime)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []*TelemetryAggregatePoint
	for rows.Next() {
		point := &TelemetryAggregatePoint{}
		var avgVal, minVal, maxVal sql.NullFloat64
		if err := rows.Scan(&point.Timestamp, &avgVal, &minVal, &maxVal, &point.Count); err != nil {
			return nil, err
		}
		if avgVal.Valid {
			point.Avg = avgVal.Float64
		}
		if minVal.Valid {
			point.Min = minVal.Float64
		}
		if maxVal.Valid {
			point.Max = maxVal.Float64
		}
		result = append(result, point)
	}
	return result, rows.Err()
}

// GetNodesWithTelemetry returns all node numbers that have telemetry data
func (d *TelemetryDAO) GetNodesWithTelemetry() ([]uint32, error) {
	query := `
		SELECT DISTINCT node_num
		FROM telemetry
		ORDER BY node_num
	`
	rows, err := d.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []uint32
	for rows.Next() {
		var nodeNum uint32
		if err := rows.Scan(&nodeNum); err != nil {
			return nil, err
		}
		result = append(result, nodeNum)
	}
	return result, rows.Err()
}

// DeleteOlderThan removes telemetry records older than the given timestamp
func (d *TelemetryDAO) DeleteOlderThan(cutoff time.Time) (int64, error) {
	result, err := d.db.Exec("DELETE FROM telemetry WHERE timestamp < ?", cutoff.Unix())
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

// DeleteByNodeNum removes all telemetry for a node
func (d *TelemetryDAO) DeleteByNodeNum(nodeNum uint32) (int64, error) {
	result, err := d.db.Exec("DELETE FROM telemetry WHERE node_num = ?", nodeNum)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

// Count returns the total number of telemetry records
func (d *TelemetryDAO) Count() (int, error) {
	var count int
	err := d.db.QueryRow("SELECT COUNT(*) FROM telemetry").Scan(&count)
	return count, err
}

// CountByNode returns the number of telemetry records for a node
func (d *TelemetryDAO) CountByNode(nodeNum uint32) (int, error) {
	var count int
	err := d.db.QueryRow("SELECT COUNT(*) FROM telemetry WHERE node_num = ?", nodeNum).Scan(&count)
	return count, err
}

// Helper functions

func (d *TelemetryDAO) queryTelemetry(query string, args ...interface{}) ([]*TelemetryEntity, error) {
	rows, err := d.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var telemetryList []*TelemetryEntity
	for rows.Next() {
		t, err := scanTelemetryRow(rows)
		if err != nil {
			return nil, err
		}
		telemetryList = append(telemetryList, t)
	}
	return telemetryList, rows.Err()
}

func scanTelemetryRow(rows *sql.Rows) (*TelemetryEntity, error) {
	t := &TelemetryEntity{}
	var viaMqtt int
	var telemetryType string

	err := rows.Scan(
		&t.ID, &t.NodeNum, &telemetryType, &t.Timestamp, &viaMqtt,
		&t.BatteryLevel, &t.Voltage, &t.ChannelUtilization, &t.AirUtilTx, &t.UptimeSeconds,
		&t.Temperature, &t.RelativeHumidity, &t.BarometricPressure, &t.GasResistance, &t.Iaq,
		&t.Distance, &t.Lux, &t.UvLux, &t.WindSpeed, &t.WindDirection, &t.Rainfall,
		&t.Ch1Voltage, &t.Ch1Current, &t.Ch2Voltage, &t.Ch2Current, &t.Ch3Voltage, &t.Ch3Current,
		&t.Pm10, &t.Pm25, &t.Pm100, &t.Co2, &t.RawData,
	)
	if err != nil {
		return nil, err
	}

	t.ViaMqtt = viaMqtt == 1
	t.TelemetryType = TelemetryType(telemetryType)
	return t, nil
}
