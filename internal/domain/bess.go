package domain

import (
	"fmt"
	"time"
)

// BESS represents a Battery Energy Storage System.
type BESS struct {
	Capacity          float64 // kWh — total storage capacity
	SoC               float64 // 0.0–1.0 — displayed with 2-digit precision (e.g. 0.6034 → "60.34%")
	MaxChargePower    float64 // kW
	MaxDischargePower float64 // kW
	CurrentPower      float64 // kW — current charge/discharge rate (positive=charging, negative=discharging)
	Status            BESSStatus
}

// ApplyEnergyDelta settles BESS energy based on elapsed time.
// Called BEFORE processing each new event (lazy evaluation).
// Between events, CurrentPower is constant, so energy = power × time.
func (b *BESS) ApplyEnergyDelta(dt time.Duration) {
	if b == nil || dt <= 0 {
		return
	}
	hours := dt.Hours()
	energyKWh := b.CurrentPower * hours
	b.SoC += energyKWh / b.Capacity
	b.SoC = clamp(b.SoC, 0.0, 1.0)
}

// AvailableDischargePower returns how much discharge power the BESS can provide.
// Returns 0 if BESS is nil or SoC is at/below the 10% floor.
func (b *BESS) AvailableDischargePower() float64 {
	if b == nil {
		return 0
	}
	if b.SoC <= 0.10 {
		return 0
	}
	return b.MaxDischargePower
}

// ShouldCharge returns true if the BESS can accept charge from spare grid power.
func (b *BESS) ShouldCharge(spareGridPower float64) bool {
	if b == nil {
		return false
	}
	return b.SoC < 1.0 && spareGridPower > 0
}

// FormatSoC returns SoC as a 2-digit precision percentage string (e.g. "60.34%")
func (b *BESS) FormatSoC() string {
	if b == nil {
		return "N/A"
	}
	return fmt.Sprintf("%.2f%%", b.SoC*100)
}

// BESSStatus defines the operational states of the battery.
type BESSStatus string

const (
	BESSIdle        BESSStatus = "idle"
	BESSCharging    BESSStatus = "charging"
	BESSDischarging BESSStatus = "discharging"
)
