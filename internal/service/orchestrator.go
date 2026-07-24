package service

import (
	"errors"
	"fmt"
	"time"

	"sems/internal/domain"
)

var (
	ErrConnectorNotFound = errors.New("connector not found")
	ErrConnectorOccupied = errors.New("connector is occupied")
	ErrConnectorAvailable = errors.New("connector is available, no session to update")
)

// advanceTime settles BESS energy based on the elapsed time since the last event.
// It is called BEFORE processing any new event to ensure energy calculations are accurate.
func (sc *SiteController) advanceTime(ts time.Time) {
	if sc.lastTimestamp.IsZero() {
		sc.lastTimestamp = ts
		return
	}
	dt := ts.Sub(sc.lastTimestamp)
	if dt > 0 && sc.station.BESS != nil {
		sc.station.BESS.ApplyEnergyDelta(dt)
	}
	sc.lastTimestamp = ts
}

// findConnector locates a connector by its ID across all EVSEs in the station.
// It returns an error if the connector does not exist.
func (sc *SiteController) findConnector(id string) (*domain.Connector, error) {
	for _, evse := range sc.station.EVSEs {
		for _, conn := range evse.Connectors {
			if conn.ID == id {
				return conn, nil
			}
		}
	}
	return nil, ErrConnectorNotFound
}

// ConnectEV creates a new charging session for an EV connecting to the specified connector.
// The initial requested power is implicitly set to the EV's max power.
func (sc *SiteController) ConnectEV(connectorID string, evMaxPower, evBatteryKWh, evSoC float64, ts time.Time) (*domain.Session, error) {
	sc.mu.Lock()
	defer sc.mu.Unlock()

	sc.advanceTime(ts)

	conn, err := sc.findConnector(connectorID)
	if err != nil {
		return nil, err
	}
	if conn.Status != domain.StatusAvailable {
		return nil, ErrConnectorOccupied
	}

	sessionID := fmt.Sprintf("ses-%s-%d", connectorID, ts.UnixNano())
	session := &domain.Session{
		ID:             sessionID,
		ConnectorID:    connectorID,
		EVMaxPower:     evMaxPower,
		EVBatteryKWh:   evBatteryKWh,
		EVSoC:          evSoC,
		RequestedPower: evMaxPower, // Implicitly evMaxPower on connect
		State:          domain.SessionActive,
		StartedAt:      ts,
	}

	conn.Session = session
	conn.Status = domain.StatusOccupied

	sc.reallocate()

	sc.logger.Info("EV connected", "connectorId", connectorID, "sessionId", sessionID, "allocated", conn.Session.AllocatedPower)
	return session, nil
}

// DisconnectEV removes an active charging session from the specified connector.
func (sc *SiteController) DisconnectEV(connectorID string, ts time.Time) error {
	sc.mu.Lock()
	defer sc.mu.Unlock()

	sc.advanceTime(ts)

	conn, err := sc.findConnector(connectorID)
	if err != nil {
		return err
	}
	if conn.Status != domain.StatusOccupied {
		return ErrConnectorAvailable
	}

	conn.Session.State = domain.SessionFinished
	conn.Session = nil
	conn.Status = domain.StatusAvailable

	sc.reallocate()
	sc.logger.Info("EV disconnected", "connectorId", connectorID)
	return nil
}

// UpdatePowerRequest updates the requested power and current SoC for an active charging session.
// This is typically called when the EV's charging curve demands less power as it fills up.
func (sc *SiteController) UpdatePowerRequest(connectorID string, requestedKW, evSoC float64, ts time.Time) error {
	sc.mu.Lock()
	defer sc.mu.Unlock()

	sc.advanceTime(ts)

	conn, err := sc.findConnector(connectorID)
	if err != nil {
		return err
	}
	if conn.Status != domain.StatusOccupied || conn.Session == nil {
		return ErrConnectorAvailable
	}

	conn.Session.RequestedPower = requestedKW
	conn.Session.EVSoC = evSoC

	sc.reallocate()
	sc.logger.Info("Power request updated", "connectorId", connectorID, "requested", requestedKW, "allocated", conn.Session.AllocatedPower)
	return nil
}

// TickResult — returned by Tick() with simulation state after advancement
type TickResult struct {
	AdvancedBy   time.Duration
	Timestamp    time.Time
	Disconnected []string
	Status       StationStatus
}

// Tick simulates the passage of time for the station, updating EV SoCs based on power delivered.
// It auto-disconnects EVs that reach 100% SoC and returns the simulation results.
func (sc *SiteController) Tick(duration time.Duration) TickResult {
	sc.mu.Lock()
	defer sc.mu.Unlock()

	ts := sc.lastTimestamp.Add(duration)
	sc.advanceTime(ts)

	var disconnected []string
	for _, evse := range sc.station.EVSEs {
		for _, conn := range evse.Connectors {
			if conn.Session != nil && conn.Session.State == domain.SessionActive {
				conn.Session.UpdateSoC(duration)
				if conn.Session.IsFull() {
					conn.Session.State = domain.SessionFinished
					conn.Session = nil
					conn.Status = domain.StatusAvailable
					disconnected = append(disconnected, conn.ID)
					sc.logger.Info("EV auto-disconnected (100% SoC)", "connectorId", conn.ID)
				}
			}
		}
	}

	sc.reallocate()

	return TickResult{
		AdvancedBy:   duration,
		Timestamp:    ts,
		Disconnected: disconnected,
		Status:       sc.getStatusLocked(),
	}
}

// GetStatus returns a snapshot of the current station status, including all active sessions and BESS state.
func (sc *SiteController) GetStatus() StationStatus {
	sc.mu.RLock()
	defer sc.mu.RUnlock()
	return sc.getStatusLocked()
}

// getStatusLocked is the internal implementation of GetStatus that assumes the caller holds the read lock.
// It constructs the DTOs for the current state.
func (sc *SiteController) getStatusLocked() StationStatus {
	var totalAllocated float64
	sessions := []SessionStatus{}
	var evses []EVSEStatus
	
	for _, evse := range sc.station.EVSEs {
		var conns []ConnectorStatus
		for _, conn := range evse.Connectors {
			var sess *SessionStatus
			if conn.Session != nil && conn.Session.State == domain.SessionActive {
				totalAllocated += conn.Session.AllocatedPower
				sess = &SessionStatus{
					ConnectorID:      conn.ID,
					EVSoC:            conn.Session.EVSoC,
					EVSoCPercent:     fmt.Sprintf("%.2f%%", conn.Session.EVSoC*100),
					AllocatedPowerKW: conn.Session.AllocatedPower,
					EVBatteryKWh:     conn.Session.EVBatteryKWh,
				}
				sessions = append(sessions, *sess)
			}
			conns = append(conns, ConnectorStatus{
				ID:      conn.ID,
				Type:    string(conn.Type),
				Status:  string(conn.Status),
				Session: sess,
			})
		}
		evses = append(evses, EVSEStatus{
			ID:         evse.ID,
			MaxPowerKW: evse.MaxPower,
			Connectors: conns,
		})
	}

	var bessStatus *BESSStatus
	if sc.station.BESS != nil {
		bessStatus = &BESSStatus{
			SoC:                 sc.station.BESS.SoC,
			SoCPercent:          sc.station.BESS.FormatSoC(),
			CurrentPowerKW:      sc.station.BESS.CurrentPower,
			MaxChargePowerKW:    sc.station.BESS.MaxChargePower,
			MaxDischargePowerKW: sc.station.BESS.MaxDischargePower,
			Status:              string(sc.station.BESS.Status),
			CapacityKWh:         sc.station.BESS.Capacity,
		}
	}

	availablePower := sc.station.GridLimit
	if sc.station.BESS != nil {
		availablePower += sc.station.BESS.AvailableDischargePower()
	}

	return StationStatus{
		StationID:          sc.station.ID,
		GridLimitKW:        sc.station.GridLimit,
		TotalAllocatedKW:   totalAllocated,
		AvailablePowerKW:   availablePower,
		LastEventTimestamp: sc.lastTimestamp,
		BESS:               bessStatus,
		Sessions:           sessions,
	}
}

type StationStatus struct {
	StationID          string
	GridLimitKW        float64
	TotalAllocatedKW   float64
	AvailablePowerKW   float64
	LastEventTimestamp time.Time
	BESS               *BESSStatus
	Sessions           []SessionStatus
	EVSEs              []EVSEStatus
}

type EVSEStatus struct {
	ID         string
	MaxPowerKW float64
	Connectors []ConnectorStatus
}

type ConnectorStatus struct {
	ID      string
	Type    string
	Status  string
	Session *SessionStatus
}

type BESSStatus struct {
	SoC                 float64
	SoCPercent          string
	CurrentPowerKW      float64
	MaxChargePowerKW    float64
	MaxDischargePowerKW float64
	Status              string
	CapacityKWh         float64
}

type SessionStatus struct {
	ConnectorID      string
	EVSoC            float64
	EVSoCPercent     string
	AllocatedPowerKW float64
	EVBatteryKWh     float64
}
