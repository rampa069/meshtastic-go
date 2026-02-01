package dao

import (
	"database/sql"
	"fmt"
	"time"
)

// NodeEntity represents a node record in the database
type NodeEntity struct {
	Num              uint32
	LongName         string
	ShortName        string
	Latitude         float64
	Longitude        float64
	SNR              float64
	RSSI             int32
	LastHeard        int64
	Channel          int32
	ViaMqtt          bool
	HopsAway         int32
	IsFavorite       bool
	IsIgnored        bool
	IsMuted          bool
	Notes            string
	// Serialized protobuf data
	User             []byte
	Position         []byte
	DeviceMetrics    []byte
	EnvironmentMetrics []byte
	PowerMetrics     []byte
	PublicKey        []byte
}

// NodeDAO provides data access operations for nodes
type NodeDAO struct {
	db *sql.DB
}

// NewNodeDAO creates a new NodeDAO
func NewNodeDAO(db *sql.DB) *NodeDAO {
	return &NodeDAO{db: db}
}

// Insert creates a new node record
func (d *NodeDAO) Insert(node *NodeEntity) error {
	query := `
		INSERT INTO nodes (
			num, long_name, short_name, latitude, longitude, snr, rssi,
			last_heard, channel, via_mqtt, hops_away, is_favorite, is_ignored,
			is_muted, notes, user, position, device_metrics, environment_metrics,
			power_metrics, public_key
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	_, err := d.db.Exec(query,
		node.Num, node.LongName, node.ShortName, node.Latitude, node.Longitude,
		node.SNR, node.RSSI, node.LastHeard, node.Channel, boolToInt(node.ViaMqtt),
		node.HopsAway, boolToInt(node.IsFavorite), boolToInt(node.IsIgnored),
		boolToInt(node.IsMuted), node.Notes, node.User, node.Position,
		node.DeviceMetrics, node.EnvironmentMetrics, node.PowerMetrics, node.PublicKey,
	)
	return err
}

// Update updates an existing node record
func (d *NodeDAO) Update(node *NodeEntity) error {
	query := `
		UPDATE nodes SET
			long_name = ?, short_name = ?, latitude = ?, longitude = ?,
			snr = ?, rssi = ?, last_heard = ?, channel = ?, via_mqtt = ?,
			hops_away = ?, is_favorite = ?, is_ignored = ?, is_muted = ?,
			notes = ?, user = ?, position = ?, device_metrics = ?,
			environment_metrics = ?, power_metrics = ?, public_key = ?
		WHERE num = ?
	`
	result, err := d.db.Exec(query,
		node.LongName, node.ShortName, node.Latitude, node.Longitude,
		node.SNR, node.RSSI, node.LastHeard, node.Channel, boolToInt(node.ViaMqtt),
		node.HopsAway, boolToInt(node.IsFavorite), boolToInt(node.IsIgnored),
		boolToInt(node.IsMuted), node.Notes, node.User, node.Position,
		node.DeviceMetrics, node.EnvironmentMetrics, node.PowerMetrics, node.PublicKey,
		node.Num,
	)
	if err != nil {
		return err
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("node %d not found", node.Num)
	}
	return nil
}

// Upsert inserts or updates a node record
func (d *NodeDAO) Upsert(node *NodeEntity) error {
	query := `
		INSERT INTO nodes (
			num, long_name, short_name, latitude, longitude, snr, rssi,
			last_heard, channel, via_mqtt, hops_away, is_favorite, is_ignored,
			is_muted, notes, user, position, device_metrics, environment_metrics,
			power_metrics, public_key
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(num) DO UPDATE SET
			long_name = COALESCE(NULLIF(excluded.long_name, ''), long_name),
			short_name = COALESCE(NULLIF(excluded.short_name, ''), short_name),
			latitude = CASE WHEN excluded.latitude != 0 THEN excluded.latitude ELSE latitude END,
			longitude = CASE WHEN excluded.longitude != 0 THEN excluded.longitude ELSE longitude END,
			snr = excluded.snr,
			rssi = excluded.rssi,
			last_heard = excluded.last_heard,
			channel = excluded.channel,
			via_mqtt = excluded.via_mqtt,
			hops_away = excluded.hops_away,
			user = COALESCE(excluded.user, user),
			position = COALESCE(excluded.position, position),
			device_metrics = COALESCE(excluded.device_metrics, device_metrics),
			environment_metrics = COALESCE(excluded.environment_metrics, environment_metrics),
			power_metrics = COALESCE(excluded.power_metrics, power_metrics),
			public_key = COALESCE(excluded.public_key, public_key)
	`
	_, err := d.db.Exec(query,
		node.Num, node.LongName, node.ShortName, node.Latitude, node.Longitude,
		node.SNR, node.RSSI, node.LastHeard, node.Channel, boolToInt(node.ViaMqtt),
		node.HopsAway, boolToInt(node.IsFavorite), boolToInt(node.IsIgnored),
		boolToInt(node.IsMuted), node.Notes, node.User, node.Position,
		node.DeviceMetrics, node.EnvironmentMetrics, node.PowerMetrics, node.PublicKey,
	)
	return err
}

// Delete removes a node by its number
func (d *NodeDAO) Delete(num uint32) error {
	result, err := d.db.Exec("DELETE FROM nodes WHERE num = ?", num)
	if err != nil {
		return err
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("node %d not found", num)
	}
	return nil
}

// GetByNum retrieves a node by its number
func (d *NodeDAO) GetByNum(num uint32) (*NodeEntity, error) {
	query := `
		SELECT num, long_name, short_name, latitude, longitude, snr, rssi,
			last_heard, channel, via_mqtt, hops_away, is_favorite, is_ignored,
			is_muted, notes, user, position, device_metrics, environment_metrics,
			power_metrics, public_key
		FROM nodes WHERE num = ?
	`
	row := d.db.QueryRow(query, num)
	return scanNode(row)
}

// GetAll retrieves all nodes
func (d *NodeDAO) GetAll() ([]*NodeEntity, error) {
	query := `
		SELECT num, long_name, short_name, latitude, longitude, snr, rssi,
			last_heard, channel, via_mqtt, hops_away, is_favorite, is_ignored,
			is_muted, notes, user, position, device_metrics, environment_metrics,
			power_metrics, public_key
		FROM nodes ORDER BY last_heard DESC
	`
	return d.queryNodes(query)
}

// GetFavorites retrieves all favorite nodes
func (d *NodeDAO) GetFavorites() ([]*NodeEntity, error) {
	query := `
		SELECT num, long_name, short_name, latitude, longitude, snr, rssi,
			last_heard, channel, via_mqtt, hops_away, is_favorite, is_ignored,
			is_muted, notes, user, position, device_metrics, environment_metrics,
			power_metrics, public_key
		FROM nodes WHERE is_favorite = 1 ORDER BY last_heard DESC
	`
	return d.queryNodes(query)
}

// GetRecent retrieves nodes heard within the given duration
func (d *NodeDAO) GetRecent(duration time.Duration) ([]*NodeEntity, error) {
	cutoff := time.Now().Add(-duration).Unix()
	query := `
		SELECT num, long_name, short_name, latitude, longitude, snr, rssi,
			last_heard, channel, via_mqtt, hops_away, is_favorite, is_ignored,
			is_muted, notes, user, position, device_metrics, environment_metrics,
			power_metrics, public_key
		FROM nodes WHERE last_heard >= ? ORDER BY last_heard DESC
	`
	return d.queryNodes(query, cutoff)
}

// GetWithPosition retrieves nodes that have position data
func (d *NodeDAO) GetWithPosition() ([]*NodeEntity, error) {
	query := `
		SELECT num, long_name, short_name, latitude, longitude, snr, rssi,
			last_heard, channel, via_mqtt, hops_away, is_favorite, is_ignored,
			is_muted, notes, user, position, device_metrics, environment_metrics,
			power_metrics, public_key
		FROM nodes WHERE latitude != 0 AND longitude != 0 ORDER BY last_heard DESC
	`
	return d.queryNodes(query)
}

// GetDirect retrieves nodes with direct connection (hops_away = 0)
func (d *NodeDAO) GetDirect() ([]*NodeEntity, error) {
	query := `
		SELECT num, long_name, short_name, latitude, longitude, snr, rssi,
			last_heard, channel, via_mqtt, hops_away, is_favorite, is_ignored,
			is_muted, notes, user, position, device_metrics, environment_metrics,
			power_metrics, public_key
		FROM nodes WHERE hops_away = 0 ORDER BY last_heard DESC
	`
	return d.queryNodes(query)
}

// Search finds nodes matching the query in long_name or short_name
func (d *NodeDAO) Search(query string) ([]*NodeEntity, error) {
	searchPattern := "%" + query + "%"
	sqlQuery := `
		SELECT num, long_name, short_name, latitude, longitude, snr, rssi,
			last_heard, channel, via_mqtt, hops_away, is_favorite, is_ignored,
			is_muted, notes, user, position, device_metrics, environment_metrics,
			power_metrics, public_key
		FROM nodes
		WHERE long_name LIKE ? OR short_name LIKE ?
		ORDER BY last_heard DESC
	`
	return d.queryNodes(sqlQuery, searchPattern, searchPattern)
}

// Count returns the total number of nodes
func (d *NodeDAO) Count() (int, error) {
	var count int
	err := d.db.QueryRow("SELECT COUNT(*) FROM nodes").Scan(&count)
	return count, err
}

// SetFavorite updates the favorite status of a node
func (d *NodeDAO) SetFavorite(num uint32, favorite bool) error {
	_, err := d.db.Exec("UPDATE nodes SET is_favorite = ? WHERE num = ?", boolToInt(favorite), num)
	return err
}

// SetIgnored updates the ignored status of a node
func (d *NodeDAO) SetIgnored(num uint32, ignored bool) error {
	_, err := d.db.Exec("UPDATE nodes SET is_ignored = ? WHERE num = ?", boolToInt(ignored), num)
	return err
}

// SetMuted updates the muted status of a node
func (d *NodeDAO) SetMuted(num uint32, muted bool) error {
	_, err := d.db.Exec("UPDATE nodes SET is_muted = ? WHERE num = ?", boolToInt(muted), num)
	return err
}

// SetNotes updates the notes for a node
func (d *NodeDAO) SetNotes(num uint32, notes string) error {
	_, err := d.db.Exec("UPDATE nodes SET notes = ? WHERE num = ?", notes, num)
	return err
}

// UpdatePosition updates just the position fields
func (d *NodeDAO) UpdatePosition(num uint32, lat, lon float64, positionBlob []byte) error {
	_, err := d.db.Exec(
		"UPDATE nodes SET latitude = ?, longitude = ?, position = ?, last_heard = ? WHERE num = ?",
		lat, lon, positionBlob, time.Now().Unix(), num,
	)
	return err
}

// UpdateLastHeard updates the last_heard timestamp
func (d *NodeDAO) UpdateLastHeard(num uint32) error {
	_, err := d.db.Exec("UPDATE nodes SET last_heard = ? WHERE num = ?", time.Now().Unix(), num)
	return err
}

// DeleteOlderThan removes nodes not heard since the given time
func (d *NodeDAO) DeleteOlderThan(cutoff time.Time) (int64, error) {
	result, err := d.db.Exec("DELETE FROM nodes WHERE last_heard < ? AND is_favorite = 0", cutoff.Unix())
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

// Helper functions

func (d *NodeDAO) queryNodes(query string, args ...interface{}) ([]*NodeEntity, error) {
	rows, err := d.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var nodes []*NodeEntity
	for rows.Next() {
		node, err := scanNodeRows(rows)
		if err != nil {
			return nil, err
		}
		nodes = append(nodes, node)
	}
	return nodes, rows.Err()
}

func scanNode(row *sql.Row) (*NodeEntity, error) {
	node := &NodeEntity{}
	var viaMqtt, isFavorite, isIgnored, isMuted int
	err := row.Scan(
		&node.Num, &node.LongName, &node.ShortName, &node.Latitude, &node.Longitude,
		&node.SNR, &node.RSSI, &node.LastHeard, &node.Channel, &viaMqtt,
		&node.HopsAway, &isFavorite, &isIgnored, &isMuted, &node.Notes,
		&node.User, &node.Position, &node.DeviceMetrics, &node.EnvironmentMetrics,
		&node.PowerMetrics, &node.PublicKey,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	node.ViaMqtt = viaMqtt == 1
	node.IsFavorite = isFavorite == 1
	node.IsIgnored = isIgnored == 1
	node.IsMuted = isMuted == 1
	return node, nil
}

func scanNodeRows(rows *sql.Rows) (*NodeEntity, error) {
	node := &NodeEntity{}
	var viaMqtt, isFavorite, isIgnored, isMuted int
	err := rows.Scan(
		&node.Num, &node.LongName, &node.ShortName, &node.Latitude, &node.Longitude,
		&node.SNR, &node.RSSI, &node.LastHeard, &node.Channel, &viaMqtt,
		&node.HopsAway, &isFavorite, &isIgnored, &isMuted, &node.Notes,
		&node.User, &node.Position, &node.DeviceMetrics, &node.EnvironmentMetrics,
		&node.PowerMetrics, &node.PublicKey,
	)
	if err != nil {
		return nil, err
	}
	node.ViaMqtt = viaMqtt == 1
	node.IsFavorite = isFavorite == 1
	node.IsIgnored = isIgnored == 1
	node.IsMuted = isMuted == 1
	return node, nil
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
