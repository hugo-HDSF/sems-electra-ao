package api

import (
	"time"

	"sems/internal/service"
)

type ConnectRequest struct {
	ConnectorID  string    `json:"connectorId"`
	EVMaxPowerKW float64   `json:"evMaxPowerKW"`
	EVBatteryKWh float64   `json:"evBatteryKWh"`
	EVSoC        float64   `json:"evSoC"`
	Timestamp    time.Time `json:"timestamp"`
}

type ConnectResponse struct {
	SessionID        string  `json:"sessionId"`
	AllocatedPowerKW float64 `json:"allocatedPowerKW"`
}

type DisconnectRequest struct {
	ConnectorID string    `json:"connectorId"`
	Timestamp   time.Time `json:"timestamp"`
}

type StatusResponse struct {
	Status string `json:"status"`
}

type ConfigResponse struct {
	Status    string `json:"status"`
	StationID string `json:"stationId"`
}

type PowerUpdateRequest struct {
	ConnectorID      string    `json:"connectorId"`
	RequestedPowerKW float64   `json:"requestedPowerKW"`
	EVSoC            float64   `json:"evSoC"`
	Timestamp        time.Time `json:"timestamp"`
}

type PowerUpdateResponse struct {
	AllocatedPowerKW float64 `json:"allocatedPowerKW"`
}

type TickRequest struct {
	DurationMinutes int `json:"durationMinutes"`
}

type TickResponse struct {
	AdvancedBy   string                `json:"advancedBy"`
	Timestamp    time.Time             `json:"timestamp"`
	Disconnected []string              `json:"disconnected"`
	Status       service.StationStatus `json:"status"`
}
