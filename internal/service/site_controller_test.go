package service

import (
	"testing"
	"time"
)

func TestSimulation_T14_TickAdvancesSoC(t *testing.T) {
	s := buildTestStationWithBESS()
	sc := newTestController(s)

	ts := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
	// 600kW request, grid 400 + BESS 200 = 600
	sc.ConnectEV("conn-1a", 600, 100, 0.20, ts) // 100kWh battery, starts at 20% SoC

	// Tick 5 minutes = 1/12 hour.
	// power: 600kW. Energy = 600 * 1/12 = 50kWh.
	// SoC increase = 50kWh / 100kWh = 0.50.
	// New SoC = 0.20 + 0.50 = 0.70.
	res := sc.Tick(5 * time.Minute)

	if len(res.Disconnected) != 0 {
		t.Fatalf("expected 0 disconnected, got %d", len(res.Disconnected))
	}

	status := sc.GetStatus()
	if len(status.Sessions) != 1 {
		t.Fatalf("expected 1 session")
	}

	if status.Sessions[0].EVSoC != 0.70 {
		t.Errorf("T14: expected 0.70 SoC, got %f", status.Sessions[0].EVSoC)
	}
}

func TestSimulation_T15_AutoDisconnectAt100(t *testing.T) {
	s := buildTestStationWithBESS()
	sc := newTestController(s)

	ts := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
	sc.ConnectEV("conn-1a", 600, 100, 0.90, ts) // 90% SoC

	// Need 10% (10kWh) to reach 100%. At 600kW, it takes 1 minute.
	// Tick 2 minutes to ensure it overshoots and disconnects.
	res := sc.Tick(2 * time.Minute)

	if len(res.Disconnected) != 1 || res.Disconnected[0] != "conn-1a" {
		t.Errorf("T15: expected conn-1a to be disconnected, got %v", res.Disconnected)
	}

	status := sc.GetStatus()
	if len(status.Sessions) != 0 {
		t.Errorf("T15: expected 0 sessions, got %d", len(status.Sessions))
	}
}

func TestSimulation_T16_FullChargeCycle(t *testing.T) {
	s := buildTestStationWithBESS()
	sc := newTestController(s)

	ts := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
	sc.ConnectEV("conn-1a", 150, 50, 0.20, ts)

	// 50kWh battery, from 20% to 100% needs 80% = 40kWh.
	// 40kWh / 150kW = 0.266 hours = 16 minutes.

	// tick 10 mins
	sc.Tick(10 * time.Minute)
	status := sc.GetStatus()
	if len(status.Sessions) != 1 {
		t.Fatalf("expected 1 session")
	}

	// tick another 10 mins (total 20, past 16)
	res := sc.Tick(10 * time.Minute)
	if len(res.Disconnected) != 1 {
		t.Errorf("T16: expected disconnect on full charge cycle")
	}
}

func TestSimulation_T17_SimultaneousBESSandEV(t *testing.T) {
	s := buildTestStationWithBESS() // BESS at 80% (160kWh)
	sc := newTestController(s)

	ts := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
	sc.ConnectEV("conn-1a", 600, 100, 0.20, ts)

	// tick 6 minutes (0.1 hours)
	// EV power: 600kW. EV energy: 60kWh. SoC change: +0.60 (now 0.80)
	// BESS discharges at 200kW. BESS energy: 20kWh. SoC change: -0.10 (now 0.70)
	sc.Tick(6 * time.Minute)

	status := sc.GetStatus()

	// Use small delta for float comparison
	evSoC := status.Sessions[0].EVSoC
	if evSoC < 0.79 || evSoC > 0.81 {
		t.Errorf("T17: expected EV SoC ~0.80, got %f", evSoC)
	}

	bessSoC := status.BESS.SoC
	if bessSoC < 0.69 || bessSoC > 0.71 {
		t.Errorf("T17: expected BESS SoC ~0.70, got %f", bessSoC)
	}
}
