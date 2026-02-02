package service

import (
	"sync"
	"time"

	"github.com/meshtastic/meshtastic-go/internal/database/dao"
	"github.com/meshtastic/meshtastic-go/internal/websocket"
	"github.com/meshtastic/meshtastic-go/pkg/pb"
	"github.com/rs/zerolog/log"
)

// NeighborService handles neighbor info processing and storage
type NeighborService struct {
	mu          sync.RWMutex
	neighborDAO *dao.NeighborDAO
	wsHub       *websocket.Hub
}

// NewNeighborService creates a new neighbor service
func NewNeighborService(neighborDAO *dao.NeighborDAO, wsHub *websocket.Hub) *NeighborService {
	return &NeighborService{
		neighborDAO: neighborDAO,
		wsHub:       wsHub,
	}
}

// ProcessNeighborInfo processes incoming neighbor info and stores it in the database
func (ns *NeighborService) ProcessNeighborInfo(nodeNum uint32, neighborInfo *pb.NeighborInfo) {
	if neighborInfo == nil {
		return
	}

	ns.mu.Lock()
	defer ns.mu.Unlock()

	timestamp := time.Now().Unix()
	neighbors := neighborInfo.Neighbors

	// Calculate average SNR
	var totalSNR float64
	var snrCount int
	for _, n := range neighbors {
		totalSNR += float64(n.Snr)
		snrCount++
	}

	avgSNR := float64(0)
	if snrCount > 0 {
		avgSNR = totalSNR / float64(snrCount)
	}

	// Insert individual neighbor history records
	for _, n := range neighbors {
		entity := &dao.NeighborHistoryEntity{
			NodeNum:         nodeNum,
			NeighborNodeNum: n.NodeId,
			SNR:             n.Snr,
			Timestamp:       timestamp,
		}
		if err := ns.neighborDAO.InsertHistory(entity); err != nil {
			log.Warn().Err(err).Uint32("nodeNum", nodeNum).Msg("failed to store neighbor history")
		}
	}

	// Insert snapshot
	snapshot := &dao.NeighborSnapshotEntity{
		NodeNum:       nodeNum,
		NeighborCount: len(neighbors),
		AvgSNR:        avgSNR,
		Timestamp:     timestamp,
	}
	if err := ns.neighborDAO.InsertSnapshot(snapshot); err != nil {
		log.Warn().Err(err).Uint32("nodeNum", nodeNum).Msg("failed to store neighbor snapshot")
	}

	log.Debug().
		Uint32("nodeNum", nodeNum).
		Int("neighborCount", len(neighbors)).
		Float64("avgSNR", avgSNR).
		Msg("processed neighbor info")
}

// GetHistory retrieves neighbor history for a node
func (ns *NeighborService) GetHistory(nodeNum uint32, limit int) ([]*dao.NeighborHistoryEntity, error) {
	return ns.neighborDAO.GetHistoryByNodeNum(nodeNum, limit)
}

// GetChartData retrieves aggregated neighbor data for charting
// period: "day" (24h by hour), "month" (30d by day), "year" (12m by week)
func (ns *NeighborService) GetChartData(nodeNum uint32, period string) ([]*dao.NeighborAggregatePoint, error) {
	now := time.Now().Unix()
	var startTime int64
	var periodSeconds int64

	switch period {
	case "day":
		startTime = now - 24*60*60        // Last 24 hours
		periodSeconds = 60 * 60           // 1 hour periods
	case "month":
		startTime = now - 30*24*60*60     // Last 30 days
		periodSeconds = 24 * 60 * 60      // 1 day periods
	case "year":
		startTime = now - 365*24*60*60    // Last 12 months
		periodSeconds = 7 * 24 * 60 * 60  // 1 week periods
	default:
		startTime = now - 24*60*60
		periodSeconds = 60 * 60
	}

	return ns.neighborDAO.GetSnapshotsAggregated(nodeNum, startTime, now, periodSeconds)
}

// GetNodesWithData returns all nodes that have neighbor data
func (ns *NeighborService) GetNodesWithData() ([]uint32, error) {
	return ns.neighborDAO.GetNodesWithNeighborInfo()
}

// CleanupOldData removes neighbor data older than the specified duration
func (ns *NeighborService) CleanupOldData(maxAge time.Duration) (int64, error) {
	cutoff := time.Now().Add(-maxAge)
	return ns.neighborDAO.DeleteOlderThan(cutoff)
}
