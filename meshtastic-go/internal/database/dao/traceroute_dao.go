package dao

import (
	"database/sql"
	"encoding/json"
	"time"
)

// TracerouteEntity represents a traceroute record in the database
type TracerouteEntity struct {
	ID         int64
	FromNode   uint32
	ToNode     uint32
	Route      []uint32
	RouteBack  []uint32
	SnrTowards []int32
	SnrBack    []int32
	HopCount   int
	Timestamp  int64
	Success    bool
}

// TracerouteDAO provides data access operations for traceroutes
type TracerouteDAO struct {
	db *sql.DB
}

// NewTracerouteDAO creates a new TracerouteDAO
func NewTracerouteDAO(db *sql.DB) *TracerouteDAO {
	return &TracerouteDAO{db: db}
}

// Insert creates a new traceroute record
func (d *TracerouteDAO) Insert(tr *TracerouteEntity) (int64, error) {
	routeJSON, _ := json.Marshal(tr.Route)
	routeBackJSON, _ := json.Marshal(tr.RouteBack)
	snrTowardsJSON, _ := json.Marshal(tr.SnrTowards)
	snrBackJSON, _ := json.Marshal(tr.SnrBack)

	query := `
		INSERT INTO traceroutes (
			from_node, to_node, route, route_back, snr_towards, snr_back,
			hop_count, timestamp, success
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	result, err := d.db.Exec(query,
		tr.FromNode, tr.ToNode,
		string(routeJSON), string(routeBackJSON),
		string(snrTowardsJSON), string(snrBackJSON),
		tr.HopCount, tr.Timestamp, tr.Success,
	)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

// GetByNodes retrieves traceroutes between two nodes
func (d *TracerouteDAO) GetByNodes(fromNode, toNode uint32, limit int) ([]*TracerouteEntity, error) {
	query := `
		SELECT id, from_node, to_node, route, route_back, snr_towards, snr_back,
			   hop_count, timestamp, success
		FROM traceroutes
		WHERE (from_node = ? AND to_node = ?) OR (from_node = ? AND to_node = ?)
		ORDER BY timestamp DESC
		LIMIT ?
	`
	rows, err := d.db.Query(query, fromNode, toNode, toNode, fromNode, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanTraceroutes(rows)
}

// GetByNode retrieves all traceroutes involving a node
func (d *TracerouteDAO) GetByNode(nodeNum uint32, limit int) ([]*TracerouteEntity, error) {
	query := `
		SELECT id, from_node, to_node, route, route_back, snr_towards, snr_back,
			   hop_count, timestamp, success
		FROM traceroutes
		WHERE from_node = ? OR to_node = ?
		ORDER BY timestamp DESC
		LIMIT ?
	`
	rows, err := d.db.Query(query, nodeNum, nodeNum, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanTraceroutes(rows)
}

// GetRecent retrieves recent traceroutes
func (d *TracerouteDAO) GetRecent(limit int) ([]*TracerouteEntity, error) {
	query := `
		SELECT id, from_node, to_node, route, route_back, snr_towards, snr_back,
			   hop_count, timestamp, success
		FROM traceroutes
		ORDER BY timestamp DESC
		LIMIT ?
	`
	rows, err := d.db.Query(query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanTraceroutes(rows)
}

// GetByID retrieves a traceroute by its ID
func (d *TracerouteDAO) GetByID(id int64) (*TracerouteEntity, error) {
	query := `
		SELECT id, from_node, to_node, route, route_back, snr_towards, snr_back,
			   hop_count, timestamp, success
		FROM traceroutes WHERE id = ?
	`
	var tr TracerouteEntity
	var routeJSON, routeBackJSON, snrTowardsJSON, snrBackJSON string

	err := d.db.QueryRow(query, id).Scan(
		&tr.ID, &tr.FromNode, &tr.ToNode,
		&routeJSON, &routeBackJSON,
		&snrTowardsJSON, &snrBackJSON,
		&tr.HopCount, &tr.Timestamp, &tr.Success,
	)
	if err != nil {
		return nil, err
	}

	json.Unmarshal([]byte(routeJSON), &tr.Route)
	json.Unmarshal([]byte(routeBackJSON), &tr.RouteBack)
	json.Unmarshal([]byte(snrTowardsJSON), &tr.SnrTowards)
	json.Unmarshal([]byte(snrBackJSON), &tr.SnrBack)

	return &tr, nil
}

// DeleteOld removes traceroutes older than the specified duration
func (d *TracerouteDAO) DeleteOld(olderThan time.Duration) (int64, error) {
	cutoff := time.Now().Add(-olderThan).Unix()
	query := `DELETE FROM traceroutes WHERE timestamp < ?`
	result, err := d.db.Exec(query, cutoff)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

// GetStats returns statistics about traceroutes for a node
func (d *TracerouteDAO) GetStats(nodeNum uint32) (map[string]interface{}, error) {
	stats := make(map[string]interface{})

	// Count total traceroutes
	var totalCount int
	err := d.db.QueryRow(`
		SELECT COUNT(*) FROM traceroutes WHERE from_node = ? OR to_node = ?
	`, nodeNum, nodeNum).Scan(&totalCount)
	if err != nil {
		return nil, err
	}
	stats["totalCount"] = totalCount

	// Count unique destinations
	var uniqueDests int
	err = d.db.QueryRow(`
		SELECT COUNT(DISTINCT to_node) FROM traceroutes WHERE from_node = ?
	`, nodeNum).Scan(&uniqueDests)
	if err != nil {
		return nil, err
	}
	stats["uniqueDestinations"] = uniqueDests

	// Average hop count
	var avgHops float64
	err = d.db.QueryRow(`
		SELECT COALESCE(AVG(hop_count), 0) FROM traceroutes WHERE from_node = ?
	`, nodeNum).Scan(&avgHops)
	if err != nil {
		return nil, err
	}
	stats["averageHops"] = avgHops

	return stats, nil
}

func scanTraceroutes(rows *sql.Rows) ([]*TracerouteEntity, error) {
	var traceroutes []*TracerouteEntity

	for rows.Next() {
		var tr TracerouteEntity
		var routeJSON, routeBackJSON, snrTowardsJSON, snrBackJSON string

		if err := rows.Scan(
			&tr.ID, &tr.FromNode, &tr.ToNode,
			&routeJSON, &routeBackJSON,
			&snrTowardsJSON, &snrBackJSON,
			&tr.HopCount, &tr.Timestamp, &tr.Success,
		); err != nil {
			return nil, err
		}

		json.Unmarshal([]byte(routeJSON), &tr.Route)
		json.Unmarshal([]byte(routeBackJSON), &tr.RouteBack)
		json.Unmarshal([]byte(snrTowardsJSON), &tr.SnrTowards)
		json.Unmarshal([]byte(snrBackJSON), &tr.SnrBack)

		traceroutes = append(traceroutes, &tr)
	}

	return traceroutes, rows.Err()
}
