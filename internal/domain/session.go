package domain

import "time"

// Session represents a charging event between one EV and one Connector.
type Session struct {
	ID             string
	ConnectorID    string
	EVMaxPower     float64 // kW — vehicle hardware limit
	EVBatteryKWh   float64 // kWh — EV battery capacity (for SoC computation)
	EVSoC          float64 // 0.0–1.0 — last reported State of Charge
	RequestedPower float64 // kW — current EV request (decreases as SoC rises)
	AllocatedPower float64 // kW — what Site Controller assigned
	State          SessionState
	StartedAt      time.Time
}

// UpdateSoC advances the EV's SoC based on power delivered over Δt.
// energy (kWh) = allocatedPower (kW) × hours
// ΔSoC = energy / batteryCapacity
func (s *Session) UpdateSoC(dt time.Duration) {
	if s == nil || dt <= 0 || s.EVBatteryKWh <= 0 {
		return
	}
	hours := dt.Hours()
	energyKWh := s.AllocatedPower * hours
	s.EVSoC += energyKWh / s.EVBatteryKWh
	s.EVSoC = clamp(s.EVSoC, 0.0, 1.0)
}

// IsFull returns true when the EV has reached 100% SoC.
func (s *Session) IsFull() bool {
	return s != nil && s.EVSoC >= 1.0
}

// SessionState defines the operational states of a charging session.
type SessionState string

const (
	SessionActive   SessionState = "active"
	SessionFinished SessionState = "finished"
)
