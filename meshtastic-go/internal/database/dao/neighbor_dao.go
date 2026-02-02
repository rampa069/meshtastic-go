package dao

import (
	"database/sql"
	"time"
)

// NeighborHistoryEntity represents a single neighbor observation
type NeighborHistoryEntity struct {
	ID              int64
	NodeNum         uint32
	NeighborNodeNum uint32
	SNR             float32
	Timestamp       int64
}

// NeighborSnapshotEntity represents an aggregated snapshot of neighbor info
type NeighborSnapshotEntity struct {
	ID            int64
	NodeNum       uint32
	NeighborCount int
	AvgSNR        float64
	Timestamp     int64
}

// NeighborAggregatePoint represents aggregated data for a time period
type NeighborAggregatePoint struct {
	Timestamp     int64   `json:"timestamp"`
	AvgCount      float64 `json:"avgCount"`
	MaxCount      int     `json:"maxCount"`
	MinCount      int     `json:"minCount"`
	AvgSNR        float64 `json:"avgSnr"`
	DataPoints    int     `json:"dataPoints"`
}

// NeighborDAO provides data access operations for neighbor info
type NeighborDAO struct {
	db *sql.DB
}

// NewNeighborDAO creates a new NeighborDAO
func NewNeighborDAO(db *sql.DB) *NeighborDAO {
	return &NeighborDAO{db: db}
}

// InsertHistory inserts a single neighbor history record
func (d *NeighborDAO) InsertHistory(entity *NeighborHistoryEntity) error {
	query := `
		INSERT INTO neighbor_history (node_num, neighbor_node_num, snr, timestamp)
		VALUES (?, ?, ?, ?)
	`
	result, err := d.db.Exec(query, entity.NodeNum, entity.NeighborNodeNum, entity.SNR, entity.Timestamp)
	if err != nil {
		return err
	}
	id, err := result.LastInsertId()
	if err == nil {
		entity.ID = id
	}
	return err
}

// InsertSnapshot inserts a neighbor snapshot record
func (d *NeighborDAO) InsertSnapshot(entity *NeighborSnapshotEntity) error {
	query := `
		INSERT INTO neighbor_snapshots (node_num, neighbor_count, avg_snr, timestamp)
		VALUES (?, ?, ?, ?)
	`
	result, err := d.db.Exec(query, entity.NodeNum, entity.NeighborCount, entity.AvgSNR, entity.Timestamp)
	if err != nil {
		return err
	}
	id, err := result.LastInsertId()
	if err == nil {
		entity.ID = id
	}
	return err
}

// GetHistoryByNodeNum retrieves neighbor history for a specific node
func (d *NeighborDAO) GetHistoryByNodeNum(nodeNum uint32, limit int) ([]*NeighborHistoryEntity, error) {
	query := `
		SELECT id, node_num, neighbor_node_num, snr, timestamp
		FROM neighbor_history
		WHERE node_num = ?
		ORDER BY timestamp DESC
		LIMIT ?
	`
	rows, err := d.db.Query(query, nodeNum, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []*NeighborHistoryEntity
	for rows.Next() {
		entity := &NeighborHistoryEntity{}
		if err := rows.Scan(&entity.ID, &entity.NodeNum, &entity.NeighborNodeNum, &entity.SNR, &entity.Timestamp); err != nil {
			return nil, err
		}
		result = append(result, entity)
	}
	return result, rows.Err()
}

// GetSnapshotsAggregated retrieves aggregated snapshot data for charting
func (d *NeighborDAO) GetSnapshotsAggregated(nodeNum uint32, startTime, endTime int64, periodSeconds int64) ([]*NeighborAggregatePoint, error) {
	query := `
		SELECT
			(timestamp / ?) * ? as period_start,
			AVG(neighbor_count) as avg_count,
			MAX(neighbor_count) as max_count,
			MIN(neighbor_count) as min_count,
			AVG(avg_snr) as avg_snr,
			COUNT(*) as data_points
		FROM neighbor_snapshots
		WHERE node_num = ? AND timestamp >= ? AND timestamp <= ?
		GROUP BY period_start
		ORDER BY period_start ASC
	`
	rows, err := d.db.Query(query, periodSeconds, periodSeconds, nodeNum, startTime, endTime)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []*NeighborAggregatePoint
	for rows.Next() {
		point := &NeighborAggregatePoint{}
		var avgSNR sql.NullFloat64
		if err := rows.Scan(&point.Timestamp, &point.AvgCount, &point.MaxCount, &point.MinCount, &avgSNR, &point.DataPoints); err != nil {
			return nil, err
		}
		if avgSNR.Valid {
			point.AvgSNR = avgSNR.Float64
		}
		result = append(result, point)
	}
	return result, rows.Err()
}

// GetNodesWithNeighborInfo returns all node numbers that have neighbor data
func (d *NeighborDAO) GetNodesWithNeighborInfo() ([]uint32, error) {
	query := `
		SELECT DISTINCT node_num
		FROM neighbor_snapshots
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

// DeleteOlderThan removes neighbor data older than the given cutoff
func (d *NeighborDAO) DeleteOlderThan(cutoff time.Time) (int64, error) {
	cutoffUnix := cutoff.Unix()

	// Delete from history
	result1, err := d.db.Exec("DELETE FROM neighbor_history WHERE timestamp < ?", cutoffUnix)
	if err != nil {
		return 0, err
	}
	deleted1, _ := result1.RowsAffected()

	// Delete from snapshots
	result2, err := d.db.Exec("DELETE FROM neighbor_snapshots WHERE timestamp < ?", cutoffUnix)
	if err != nil {
		return deleted1, err
	}
	deleted2, _ := result2.RowsAffected()

	return deleted1 + deleted2, nil
}

// CountHistory returns the total number of neighbor history records
func (d *NeighborDAO) CountHistory() (int, error) {
	var count int
	err := d.db.QueryRow("SELECT COUNT(*) FROM neighbor_history").Scan(&count)
	return count, err
}

// CountSnapshots returns the total number of neighbor snapshot records
func (d *NeighborDAO) CountSnapshots() (int, error) {
	var count int
	err := d.db.QueryRow("SELECT COUNT(*) FROM neighbor_snapshots").Scan(&count)
	return count, err
}
