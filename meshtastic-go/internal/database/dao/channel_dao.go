package dao

import (
	"database/sql"
	"fmt"
)

// ChannelEntity represents a channel record in the database
type ChannelEntity struct {
	Index     int32
	MyNodeNum uint32
	Role      int32
	Settings  []byte // Serialized protobuf ChannelSettings
}

// ChannelDAO provides data access operations for channels
type ChannelDAO struct {
	db *sql.DB
}

// NewChannelDAO creates a new ChannelDAO
func NewChannelDAO(db *sql.DB) *ChannelDAO {
	return &ChannelDAO{db: db}
}

// Upsert inserts or updates a channel record
func (d *ChannelDAO) Upsert(ch *ChannelEntity) error {
	query := `
		INSERT INTO channels (idx, myNodeNum, role, settings)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(idx, myNodeNum) DO UPDATE SET
			role = excluded.role,
			settings = excluded.settings
	`
	_, err := d.db.Exec(query, ch.Index, ch.MyNodeNum, ch.Role, ch.Settings)
	return err
}

// Insert creates a new channel record
func (d *ChannelDAO) Insert(ch *ChannelEntity) error {
	query := `
		INSERT INTO channels (idx, myNodeNum, role, settings)
		VALUES (?, ?, ?, ?)
	`
	_, err := d.db.Exec(query, ch.Index, ch.MyNodeNum, ch.Role, ch.Settings)
	return err
}

// Update updates an existing channel record
func (d *ChannelDAO) Update(ch *ChannelEntity) error {
	query := `
		UPDATE channels SET role = ?, settings = ?
		WHERE idx = ? AND myNodeNum = ?
	`
	result, err := d.db.Exec(query, ch.Role, ch.Settings, ch.Index, ch.MyNodeNum)
	if err != nil {
		return err
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("channel %d not found for node %d", ch.Index, ch.MyNodeNum)
	}
	return nil
}

// Delete removes a channel by index and node number
func (d *ChannelDAO) Delete(index int32, myNodeNum uint32) error {
	result, err := d.db.Exec("DELETE FROM channels WHERE idx = ? AND myNodeNum = ?", index, myNodeNum)
	if err != nil {
		return err
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("channel %d not found for node %d", index, myNodeNum)
	}
	return nil
}

// DeleteByNodeNum deletes all channels for a node
func (d *ChannelDAO) DeleteByNodeNum(myNodeNum uint32) (int64, error) {
	result, err := d.db.Exec("DELETE FROM channels WHERE myNodeNum = ?", myNodeNum)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

// GetByIndex retrieves a channel by index and node number
func (d *ChannelDAO) GetByIndex(index int32, myNodeNum uint32) (*ChannelEntity, error) {
	query := `
		SELECT idx, myNodeNum, role, settings
		FROM channels WHERE idx = ? AND myNodeNum = ?
	`
	row := d.db.QueryRow(query, index, myNodeNum)
	return scanChannel(row)
}

// GetByNodeNum retrieves all channels for a node
func (d *ChannelDAO) GetByNodeNum(myNodeNum uint32) ([]*ChannelEntity, error) {
	query := `
		SELECT idx, myNodeNum, role, settings
		FROM channels WHERE myNodeNum = ?
		ORDER BY idx ASC
	`
	return d.queryChannels(query, myNodeNum)
}

// GetActive retrieves all active (non-disabled) channels for a node
func (d *ChannelDAO) GetActive(myNodeNum uint32) ([]*ChannelEntity, error) {
	query := `
		SELECT idx, myNodeNum, role, settings
		FROM channels WHERE myNodeNum = ? AND role > 0
		ORDER BY idx ASC
	`
	return d.queryChannels(query, myNodeNum)
}

// GetPrimary retrieves the primary channel for a node
func (d *ChannelDAO) GetPrimary(myNodeNum uint32) (*ChannelEntity, error) {
	query := `
		SELECT idx, myNodeNum, role, settings
		FROM channels WHERE myNodeNum = ? AND role = 1
		LIMIT 1
	`
	row := d.db.QueryRow(query, myNodeNum)
	return scanChannel(row)
}

// Count returns the number of channels for a node
func (d *ChannelDAO) Count(myNodeNum uint32) (int, error) {
	var count int
	err := d.db.QueryRow("SELECT COUNT(*) FROM channels WHERE myNodeNum = ?", myNodeNum).Scan(&count)
	return count, err
}

// CountActive returns the number of active channels for a node
func (d *ChannelDAO) CountActive(myNodeNum uint32) (int, error) {
	var count int
	err := d.db.QueryRow("SELECT COUNT(*) FROM channels WHERE myNodeNum = ? AND role > 0", myNodeNum).Scan(&count)
	return count, err
}

// Helper functions

func (d *ChannelDAO) queryChannels(query string, args ...interface{}) ([]*ChannelEntity, error) {
	rows, err := d.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var channels []*ChannelEntity
	for rows.Next() {
		ch := &ChannelEntity{}
		err := rows.Scan(&ch.Index, &ch.MyNodeNum, &ch.Role, &ch.Settings)
		if err != nil {
			return nil, err
		}
		channels = append(channels, ch)
	}
	return channels, rows.Err()
}

func scanChannel(row *sql.Row) (*ChannelEntity, error) {
	ch := &ChannelEntity{}
	err := row.Scan(&ch.Index, &ch.MyNodeNum, &ch.Role, &ch.Settings)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return ch, nil
}
