package api

import (
	"errors"
	"time"

	"sems/internal/service"
)

// ConnectRequest defines the payload for an EV connection event.
type ConnectRequest struct {
	ConnectorID  string    `json:"connectorId"`
	EVMaxPowerKW float64   `json:"evMaxPowerKW"`
	EVBatteryKWh float64   `json:"evBatteryKWh"`
	EVSoC        float64   `json:"evSoC"`
	Timestamp    time.Time `json:"timestamp"`
}

// Validate checks the structural integrity of the connect request.
func (r *ConnectRequest) Validate() error {
	if r.ConnectorID == "" {
		return errors.New("connectorId is required")
	}
	if r.EVMaxPowerKW <= 0 {
		return errors.New("evMaxPowerKW must be strictly positive")
	}
	if r.EVSoC < 0.0 || r.EVSoC > 1.0 {
		return errors.New("evSoC must be between 0.0 and 1.0")
	}
	return nil
}

// ConnectResponse represents the result of a successful connection.
type ConnectResponse struct {
	SessionID        string  `json:"sessionId"`
	AllocatedPowerKW float64 `json:"allocatedPowerKW"`
}

// DisconnectRequest defines the payload for disconnecting an EV.
type DisconnectRequest struct {
	ConnectorID string    `json:"connectorId"`
	Timestamp   time.Time `json:"timestamp"`
}

// StatusResponse represents a generic status message.
type StatusResponse struct {
	Status string `json:"status"`
}

// ConfigResponse represents the result of a station configuration update.
type ConfigResponse struct {
	Status    string `json:"status"`
	StationID string `json:"stationId"`
}

// PowerUpdateRequest defines the payload for updating an EV's requested power and SoC.
type PowerUpdateRequest struct {
	ConnectorID      string    `json:"connectorId"`
	RequestedPowerKW float64   `json:"requestedPowerKW"`
	EVSoC            float64   `json:"evSoC"`
	Timestamp        time.Time `json:"timestamp"`
}

// Validate checks the structural integrity of the power update request.
func (r *PowerUpdateRequest) Validate() error {
	if r.ConnectorID == "" {
		return errors.New("connectorId is required")
	}
	if r.RequestedPowerKW < 0 {
		return errors.New("requestedPowerKW cannot be negative")
	}
	if r.EVSoC < 0.0 || r.EVSoC > 1.0 {
		return errors.New("evSoC must be between 0.0 and 1.0")
	}
	return nil
}

// PowerUpdateResponse represents the result of a power update, containing the newly allocated power.
type PowerUpdateResponse struct {
	AllocatedPowerKW float64 `json:"allocatedPowerKW"`
}

// TickRequest defines the payload for advancing the simulation time.
type TickRequest struct {
	DurationMinutes int `json:"durationMinutes"`
}

// TickResponse represents the result of a simulation tick, containing the advanced time and disconnected EVs.
type TickResponse struct {
	AdvancedBy   string                `json:"advancedBy"`
	Timestamp    time.Time             `json:"timestamp"`
	Disconnected []string              `json:"disconnected"`
	Status       service.StationStatus `json:"status"`
}
