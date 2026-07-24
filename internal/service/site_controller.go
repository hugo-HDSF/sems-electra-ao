package service

import (
	"sync"
	"time"
	"log/slog"
	
	"sems/internal/domain"
)

// SiteController orchestrates the charging station's state, managing time, power allocation, and SSE subscriptions.
type SiteController struct {
	station       *domain.Station
	lastTimestamp time.Time
	mu            sync.RWMutex
	logger        *slog.Logger

	subscribers map[chan StationStatus]struct{}
	subMu       sync.Mutex
}

func NewSiteController(station *domain.Station, logger *slog.Logger) *SiteController {
	return &SiteController{
		station: station,
		logger:  logger,
	}
}

// computeSparePower calculates remaining grid power after total consumption
func (sc *SiteController) computeSparePower() float64 {
	var consumed float64
	for _, evse := range sc.station.EVSEs {
		for _, conn := range evse.Connectors {
			if conn.Session != nil && conn.Session.State == domain.SessionActive {
				consumed += conn.Session.AllocatedPower
			}
		}
	}
	spare := sc.station.GridLimit - consumed
	if spare < 0 {
		return 0
	}
	return spare
}

// Reconfigure dynamically hot-swaps the station configuration and resets the simulation time.
// It acquires the full write lock to prevent race conditions during reallocation.
func (sc *SiteController) Reconfigure(station *domain.Station) {
	sc.mu.Lock()
	defer sc.mu.Unlock()

	sc.station = station
	sc.lastTimestamp = time.Time{} // Reset time
	
	sc.logger.Info("Station dynamically reconfigured", "stationId", station.ID)
	
	sc.reallocate()
	sc.broadcast()
}

// demandExceedsGrid returns true if total requested power exceeds grid limit
func (sc *SiteController) demandExceedsGrid() bool {
	var requested float64
	for _, evse := range sc.station.EVSEs {
		for _, conn := range evse.Connectors {
			if conn.Session != nil && conn.Session.State == domain.SessionActive && !conn.Session.IsFull() {
				requested += conn.Session.RequestedPower
			}
		}
	}
	return requested > sc.station.GridLimit
}

// setBESSCharging configures the BESS to charge using spare power
func (sc *SiteController) setBESSCharging(spare float64) {
	bess := sc.station.BESS
	chargePower := spare
	if chargePower > bess.MaxChargePower {
		chargePower = bess.MaxChargePower
	}
	bess.CurrentPower = chargePower // positive for charging
	bess.Status = domain.BESSCharging
}

// setBESSDischarging configures the BESS to discharge
func (sc *SiteController) setBESSDischarging() {
	bess := sc.station.BESS
	if bess.AvailableDischargePower() > 0 {
		bess.CurrentPower = -bess.MaxDischargePower // negative for discharging
		bess.Status = domain.BESSDischarging
	} else {
		sc.setBESSIdle()
	}
}

// setBESSIdle configures the BESS to be idle
func (sc *SiteController) setBESSIdle() {
	bess := sc.station.BESS
	bess.CurrentPower = 0
	bess.Status = domain.BESSIdle
}

// updateBESSDirection sets BESS state for next interval
func (sc *SiteController) updateBESSDirection() {
	sparePower := sc.computeSparePower()
	bess := sc.station.BESS
	if bess == nil {
		return
	}

	if bess.ShouldCharge(sparePower) {
		sc.setBESSCharging(sparePower)
	} else if sc.demandExceedsGrid() {
		sc.setBESSDischarging()
	} else {
		sc.setBESSIdle()
	}
}

// Reallocate triggers the domain allocation logic and applies it
// (Part of Phase 3/4 integration)
func (sc *SiteController) reallocate() {
	allocs := domain.Allocate(sc.station)
	for _, evse := range sc.station.EVSEs {
		for _, conn := range evse.Connectors {
			if conn.Session != nil && conn.Session.State == domain.SessionActive {
				if alloc, ok := allocs[conn.ID]; ok {
					conn.Session.AllocatedPower = alloc
				} else {
					conn.Session.AllocatedPower = 0
				}
			}
		}
	}
	sc.updateBESSDirection()
}
