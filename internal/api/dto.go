package api

import (
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
