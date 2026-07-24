package domain

// Station — top-level aggregate, loaded from config
type Station struct {
	ID        string
	GridLimit float64 // kW — max import from grid
	EVSEs     []*EVSE
	BESS      *BESS // nil if no BESS
}

// EVSE — Electric Vehicle Supply Equipment (physical charger hardware)
type EVSE struct {
	ID         string
	MaxPower   float64 // kW — EVSE hardware limit
	Connectors []*Connector
}

// Connector — physical plug (CCS, Type2)
type Connector struct {
	ID      string
	Type    ConnectorType
	EVSEID  string
	Status  ConnectorStatus
	Session *Session // nil if no active session
}

// ConnectorType — physical connector standard
type ConnectorType string

const (
	ConnectorCCS_DC   ConnectorType = "CCS_DC"
	ConnectorType2_AC ConnectorType = "TYPE_2_AC"
)

// ConnectorStatus
type ConnectorStatus string

const (
	StatusAvailable ConnectorStatus = "available"
	StatusOccupied  ConnectorStatus = "occupied"
	StatusFaulted   ConnectorStatus = "faulted"
)
