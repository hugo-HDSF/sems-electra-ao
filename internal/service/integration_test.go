package service

import (
	"testing"
	"time"

	"sems/internal/domain"
)

// TestIntegration_T18_EndToEnd tests an end-to-end simulation from empty station -> full capacity -> BESS drain -> EVs disconnect
func TestIntegration_T18_EndToEnd(t *testing.T) {
	s := &domain.Station{
		ID:        "test-station",
		GridLimit: 1000, // 1000 kW grid
		EVSEs: []*domain.EVSE{
			{
				ID:       "evse-1",
				MaxPower: 600,
				Connectors: []*domain.Connector{
					{ID: "conn-1a", Status: domain.StatusAvailable},
					{ID: "conn-1b", Status: domain.StatusAvailable},
				},
			},
			{
				ID:       "evse-2",
				MaxPower: 600,
				Connectors: []*domain.Connector{
					{ID: "conn-2a", Status: domain.StatusAvailable},
				},
			},
		},
		BESS: &domain.BESS{
			Capacity:          500, // 500 kWh
			SoC:               1.0, // 100%
			MaxChargePower:    200,
			MaxDischargePower: 200,
			Status:            domain.BESSIdle,
		},
	}
	sc := newTestController(s)

	ts := time.Date(2024, 1, 15, 8, 0, 0, 0, time.UTC)

	// Step 1: Connect EVs requesting a total of 1200 kW (600 each)
	// Grid is 1000, BESS can discharge 200. Total available = 1200.
	// Each should get 1200 / 2 = 600 kW.
	sc.ConnectEV("conn-1a", 600, 100, 0.20, ts)
	sc.ConnectEV("conn-2a", 600, 100, 0.20, ts)

	status := sc.GetStatus()
	if status.TotalAllocatedKW != 1200 {
		t.Fatalf("expected 1200 kW allocated, got %f", status.TotalAllocatedKW)
	}

	// BESS discharging at 200kW for 10 minutes drains 33.33kWh. 
	// EVs charging at 600kW for 10 minutes adds 100kWh.
	// Wait, 100kWh battery needs 80kWh to reach 100% from 20%.
	// At 600kW, it takes 80 / 600 = 0.133 hours = 8 minutes!
	
	// Let's tick 5 minutes.
	// BESS: 200kW * 5min = 16.67kWh drained. 
	res := sc.Tick(5 * time.Minute)
	
	status = sc.GetStatus()
	if len(res.Disconnected) != 0 {
		t.Fatalf("expected 0 disconnected at 5 minutes")
	}

	// Step 3: Tick another 5 minutes (Total 10 mins > 8 mins). They should disconnect.
	res = sc.Tick(5 * time.Minute)
	if len(res.Disconnected) != 2 {
		t.Fatalf("expected 2 disconnected EVs, got %d", len(res.Disconnected))
	}
	
	// Since all disconnected, spare power = 1000kW. BESS can charge at 200kW.
	status = sc.GetStatus()
	if status.BESS.Status != "charging" {
		t.Fatalf("expected BESS to charge after EVs leave, got %s", status.BESS.Status)
	}
}

// TestIntegration_T19_EdgeCase_SimultaneousHit tests when BESS hits floor exactly as EV hits 100% SoC
func TestIntegration_T19_EdgeCase_SimultaneousHit(t *testing.T) {
	s := &domain.Station{
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
			Capacity:          100, // 100 kWh
			SoC:               0.60, // 60%
			MaxChargePower:    200,
			MaxDischargePower: 200,
			Status:            domain.BESSIdle,
		},
	}
	sc := newTestController(s)

	ts := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
	// EV needs 50kWh to reach full (100 * 0.5)
	sc.ConnectEV("conn-1a", 600, 100, 0.50, ts)
	
	// Allocation: Grid 400 + BESS 200 = 600.
	// BESS discharging at 200kW.
	// BESS has 50kWh available before hitting 10% floor (50% of 100kWh).
	// EV is charging at 600kW. 
	// Wait, to hit both exactly at the same time:
	// Both need to take exactly 15 minutes!
	// BESS 200kW * 15m (0.25h) = 50kWh. BESS hits 10% floor.
	// EV 600kW * 15m (0.25h) = 150kWh. But EV only needs 50kWh!
	// So EV will hit full in 5 minutes (600kW * 5m = 50kWh).
	
	// Let's adjust EV so it needs exactly 150kWh.
	// We'll disconnect and reconnect.
	sc.DisconnectEV("conn-1a", ts)
	
	// EV needs 150kWh. Battery = 300kWh. Starts at 50%.
	sc.ConnectEV("conn-1a", 600, 300, 0.50, ts)

	// Tick exactly 15 minutes.
	res := sc.Tick(15 * time.Minute)

	// Both events happen: EV disconnects, and BESS hits floor.
	if len(res.Disconnected) != 1 {
		t.Fatalf("expected EV to disconnect")
	}

	status := sc.GetStatus()
	if status.BESS.SoC > 0.1001 || status.BESS.SoC < 0.0999 { // Float precision check
		t.Errorf("expected BESS SoC to hit 10%% floor, got %f", status.BESS.SoC)
	}
	
	// After EV leaves, BESS should start charging since spare power is 400.
	if status.BESS.Status != "charging" {
		t.Errorf("expected BESS to start charging, got %s", status.BESS.Status)
	}
}

// TestIntegration_T20_Performance tests that 1000 ticks process in under 100ms
func TestIntegration_T20_Performance(t *testing.T) {
	s := buildTestStationWithBESS()
	sc := newTestController(s)

	ts := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
	sc.ConnectEV("conn-1a", 600, 100, 0.20, ts)

	start := time.Now()

	for i := 0; i < 1000; i++ {
		// Use small ticks so it doesn't auto-disconnect immediately
		// Or reconnect if it disconnects
		res := sc.Tick(1 * time.Second)
		if len(res.Disconnected) > 0 {
			sc.ConnectEV("conn-1a", 600, 100, 0.20, ts.Add(time.Duration(i)*time.Second))
		}
	}

	elapsed := time.Since(start)
	if elapsed > 100*time.Millisecond {
		t.Errorf("T20: performance test failed: took %v (expected < 100ms)", elapsed)
	}
}
