package service

import (
	"io"
	"log/slog"
	"testing"
	"time"

	"sems/internal/domain"
)

func buildTestStationWithBESS() *domain.Station {
	return &domain.Station{
		ID:        "test-station",
		GridLimit: 400,
		EVSEs: []*domain.EVSE{
			{
				ID:       "evse-1",
				MaxPower: 600,
				Connectors: []*domain.Connector{
					{ID: "conn-1a", Status: domain.StatusAvailable},
				},
			},
		},
		BESS: &domain.BESS{
			Capacity:          200, // 200 kWh
			SoC:               0.80, // 80%
			MaxChargePower:    200, // 200 kW
			MaxDischargePower: 200, // 200 kW
			Status:            domain.BESSIdle,
		},
	}
}

func connectEV(station *domain.Station, connID string, req, max float64) {
	for _, evse := range station.EVSEs {
		for _, conn := range evse.Connectors {
			if conn.ID == connID {
				conn.Status = domain.StatusOccupied
				conn.Session = &domain.Session{
					ID:             "ses-" + connID,
					ConnectorID:    connID,
					EVMaxPower:     max,
					RequestedPower: req,
					State:          domain.SessionActive,
				}
			}
		}
	}
}

func newTestController(station *domain.Station) *SiteController {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return NewSiteController(station, logger)
}

func TestBESS_T08_DischargeBoost(t *testing.T) {
	// T08: BESS discharge boosts available power | availablePower = gridLimit + bessDischarge
	s := buildTestStationWithBESS()
	sc := newTestController(s)
	
	// Request 600kW. Grid limit is 400. BESS can discharge 200. Total available = 600.
	connectEV(s, "conn-1a", 600, 600)
	
	sc.reallocate() // test internal reallocate without locks for direct assertions
	
	conn := s.EVSEs[0].Connectors[0]
	if conn.Session.AllocatedPower != 600 {
		t.Errorf("T08: expected 600, got %f", conn.Session.AllocatedPower)
	}
	if s.BESS.Status != domain.BESSDischarging {
		t.Errorf("T08: expected BESS to be discharging")
	}
	if s.BESS.CurrentPower != -200 {
		t.Errorf("T08: expected BESS current power -200, got %f", s.BESS.CurrentPower)
	}
}

func TestBESS_T09_FloorNoDischarge(t *testing.T) {
	// T09: BESS SoC at 10% floor — no discharge | BESS contribution drops to 0
	s := buildTestStationWithBESS()
	s.BESS.SoC = 0.10 // 10% floor
	sc := newTestController(s)
	
	// Request 600kW. Grid limit 400. BESS cannot discharge. Available = 400.
	connectEV(s, "conn-1a", 600, 600)
	
	sc.reallocate()
	
	conn := s.EVSEs[0].Connectors[0]
	if conn.Session.AllocatedPower != 400 {
		t.Errorf("T09: expected 400, got %f", conn.Session.AllocatedPower)
	}
	if s.BESS.Status != domain.BESSIdle {
		t.Errorf("T09: expected BESS to be idle, got %s", s.BESS.Status)
	}
}

func TestBESS_T10_ChargeWithSpare(t *testing.T) {
	// T10: BESS charges when spare grid power exists | BESS SoC increases; charge power <= spare
	s := buildTestStationWithBESS()
	sc := newTestController(s)
	
	// Request 100kW. Grid limit 400. Spare = 300. BESS can charge at max 200.
	connectEV(s, "conn-1a", 100, 100)
	
	sc.reallocate()
	
	if s.BESS.Status != domain.BESSCharging {
		t.Errorf("T10: expected BESS to be charging")
	}
	if s.BESS.CurrentPower != 200 {
		t.Errorf("T10: expected BESS current power 200, got %f", s.BESS.CurrentPower)
	}
}

func TestBESS_T11_DrainOverTime(t *testing.T) {
	// T11: BESS SoC drains over simulated time (Δt)
	// After 1h at -200kW on 200kWh battery: SoC drops by 100% → clamped correctly
	bess := &domain.BESS{
		Capacity:          200,
		SoC:               1.0,
		MaxChargePower:    200,
		MaxDischargePower: 200,
		CurrentPower:      -200,
		Status:            domain.BESSDischarging,
	}
	
	// simulate 30 minutes
	bess.ApplyEnergyDelta(30 * time.Minute)
	
	if bess.SoC != 0.5 { // 100 kWh drained out of 200 kWh = 0.5 loss
		t.Errorf("T11: expected SoC 0.5, got %f", bess.SoC)
	}
	
	// simulate another 1 hour (would drain more than remaining) -> clamped to 0
	bess.ApplyEnergyDelta(1 * time.Hour)
	if bess.SoC != 0 {
		t.Errorf("T11: expected SoC clamped to 0, got %f", bess.SoC)
	}
}

func TestBESS_T12_HitsFloorStopsDischarging(t *testing.T) {
	// T12: BESS hits SoC floor mid-scenario and stops discharging
	s := buildTestStationWithBESS()
	s.BESS.SoC = 0.15 // Start at 15%
	sc := newTestController(s)
	
	connectEV(s, "conn-1a", 600, 600)
	
	// Initially allocates 600, BESS is discharging at -200
	sc.reallocate()
	if s.BESS.CurrentPower != -200 {
		t.Fatalf("expected BESS to discharge initially")
	}
	
	// Fast forward 30 minutes. 200kW * 0.5h = 100kWh. 100 / 200 = 0.5 SoC drain.
	// We were at 0.15. 0.15 - 0.5 = clamped to 0.0.
	s.BESS.ApplyEnergyDelta(30 * time.Minute)
	if s.BESS.SoC != 0.0 {
		t.Fatalf("expected SoC to drain to 0")
	}
	
	// Reallocate! Now SoC is 0 (below floor 0.10). Should stop discharging.
	sc.reallocate()
	
	if s.BESS.Status != domain.BESSIdle {
		t.Errorf("T12: expected BESS to be idle after hitting floor, got %s", s.BESS.Status)
	}
	conn := s.EVSEs[0].Connectors[0]
	if conn.Session.AllocatedPower != 400 {
		t.Errorf("T12: expected allocation to drop to 400, got %f", conn.Session.AllocatedPower)
	}
}

func TestBESS_T13_FormatSoC(t *testing.T) {
	// T13: BESS SoC displayed with 2-digit precision | FormatSoC() returns "60.34%" not "60%"
	bess := &domain.BESS{
		SoC: 0.6034,
	}
	if got := bess.FormatSoC(); got != "60.34%" {
		t.Errorf("T13: expected 60.34%%, got %s", got)
	}
	
	bess.SoC = 0.05
	if got := bess.FormatSoC(); got != "5.00%" {
		t.Errorf("T13: expected 5.00%%, got %s", got)
	}
}
