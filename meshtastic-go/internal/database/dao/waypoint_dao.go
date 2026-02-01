package dao

import (
	"database/sql"
	"time"
)

// WaypointEntity represents a waypoint record in the database
type WaypointEntity struct {
	ID          uint32
	LatitudeI   int32
	LongitudeI  int32
	Expire      uint32
	LockedTo    uint32
	Name        string
	Description string
	Icon        uint32
	FromNode    uint32
	ViaMqtt     bool
	CreatedAt   int64
	UpdatedAt   int64
}

// WaypointDAO provides data access operations for waypoints
type WaypointDAO struct {
	db *sql.DB
}

// NewWaypointDAO creates a new WaypointDAO
func NewWaypointDAO(db *sql.DB) *WaypointDAO {
	return &WaypointDAO{db: db}
}

// Insert creates a new waypoint record
func (d *WaypointDAO) Insert(wp *WaypointEntity) error {
	query := `
		INSERT INTO waypoints (
			id, latitude_i, longitude_i, expire, locked_to, name,
			description, icon, from_node, via_mqtt, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	now := time.Now().Unix()
	_, err := d.db.Exec(query,
		wp.ID, wp.LatitudeI, wp.LongitudeI, wp.Expire, wp.LockedTo,
		wp.Name, wp.Description, wp.Icon, wp.FromNode, wp.ViaMqtt,
		now, now,
	)
	return err
}

// Upsert creates or updates a waypoint record
func (d *WaypointDAO) Upsert(wp *WaypointEntity) error {
	query := `
		INSERT INTO waypoints (
			id, latitude_i, longitude_i, expire, locked_to, name,
			description, icon, from_node, via_mqtt, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			latitude_i = excluded.latitude_i,
			longitude_i = excluded.longitude_i,
			expire = excluded.expire,
			locked_to = excluded.locked_to,
			name = excluded.name,
			description = excluded.description,
			icon = excluded.icon,
			from_node = excluded.from_node,
			via_mqtt = excluded.via_mqtt,
			updated_at = excluded.updated_at
	`
	now := time.Now().Unix()
	_, err := d.db.Exec(query,
		wp.ID, wp.LatitudeI, wp.LongitudeI, wp.Expire, wp.LockedTo,
		wp.Name, wp.Description, wp.Icon, wp.FromNode, wp.ViaMqtt,
		now, now,
	)
	return err
}

// GetByID retrieves a waypoint by its ID
func (d *WaypointDAO) GetByID(id uint32) (*WaypointEntity, error) {
	query := `
		SELECT id, latitude_i, longitude_i, expire, locked_to, name,
			   description, icon, from_node, via_mqtt, created_at, updated_at
		FROM waypoints WHERE id = ?
	`
	var wp WaypointEntity
	err := d.db.QueryRow(query, id).Scan(
		&wp.ID, &wp.LatitudeI, &wp.LongitudeI, &wp.Expire, &wp.LockedTo,
		&wp.Name, &wp.Description, &wp.Icon, &wp.FromNode, &wp.ViaMqtt,
		&wp.CreatedAt, &wp.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &wp, nil
}

// GetAll retrieves all non-expired waypoints
func (d *WaypointDAO) GetAll() ([]*WaypointEntity, error) {
	query := `
		SELECT id, latitude_i, longitude_i, expire, locked_to, name,
			   description, icon, from_node, via_mqtt, created_at, updated_at
		FROM waypoints
		WHERE expire = 0 OR expire > ?
		ORDER BY created_at DESC
	`
	now := time.Now().Unix()
	rows, err := d.db.Query(query, now)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var waypoints []*WaypointEntity
	for rows.Next() {
		var wp WaypointEntity
		if err := rows.Scan(
			&wp.ID, &wp.LatitudeI, &wp.LongitudeI, &wp.Expire, &wp.LockedTo,
			&wp.Name, &wp.Description, &wp.Icon, &wp.FromNode, &wp.ViaMqtt,
			&wp.CreatedAt, &wp.UpdatedAt,
		); err != nil {
			return nil, err
		}
		waypoints = append(waypoints, &wp)
	}
	return waypoints, rows.Err()
}

// Delete removes a waypoint by ID
func (d *WaypointDAO) Delete(id uint32) error {
	query := `DELETE FROM waypoints WHERE id = ?`
	_, err := d.db.Exec(query, id)
	return err
}

// DeleteExpired removes all expired waypoints
func (d *WaypointDAO) DeleteExpired() (int64, error) {
	query := `DELETE FROM waypoints WHERE expire > 0 AND expire < ?`
	result, err := d.db.Exec(query, time.Now().Unix())
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}
