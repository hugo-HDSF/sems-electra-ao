package domain_test

import (
	"testing"

	"sems/internal/domain"
)

func buildTestStation() *domain.Station {
	return &domain.Station{
		ID:        "test-station",
		GridLimit: 400,
		EVSEs: []*domain.EVSE{
			{
				ID:       "evse-1",
				MaxPower: 300,
				Connectors: []*domain.Connector{
					{ID: "conn-1a", Status: domain.StatusAvailable},
					{ID: "conn-1b", Status: domain.StatusAvailable},
				},
			},
			{
				ID:       "evse-2",
				MaxPower: 300,
				Connectors: []*domain.Connector{
					{ID: "conn-2a", Status: domain.StatusAvailable},
					{ID: "conn-2b", Status: domain.StatusAvailable},
				},
			},
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

func TestAllocation_T01_SingleEV(t *testing.T) {
	s := buildTestStation()
	connectEV(s, "conn-1a", 250, 250)

	allocs := domain.Allocate(s)
	if allocs["conn-1a"] != 250 {
		t.Errorf("T01: expected 250, got %f", allocs["conn-1a"])
	}
}

func TestAllocation_T02_TwoEVs_UnderLimit(t *testing.T) {
	s := buildTestStation()
	connectEV(s, "conn-1a", 150, 150)
	connectEV(s, "conn-2a", 150, 150)

	allocs := domain.Allocate(s)
	if allocs["conn-1a"] != 150 || allocs["conn-2a"] != 150 {
		t.Errorf("T02: expected 150/150, got %f / %f", allocs["conn-1a"], allocs["conn-2a"])
	}
}

func TestAllocation_T03_TwoEVs_OverLimit(t *testing.T) {
	s := buildTestStation()
	connectEV(s, "conn-1a", 250, 250)
	connectEV(s, "conn-2a", 250, 250)

	allocs := domain.Allocate(s)
	// Grid is 400. 250+250 = 500. Proportional: 200 each.
	if allocs["conn-1a"] != 200 || allocs["conn-2a"] != 200 {
		t.Errorf("T03: expected 200/200, got %f / %f", allocs["conn-1a"], allocs["conn-2a"])
	}
}

func TestAllocation_T04_EVDisconnects(t *testing.T) {
	s := buildTestStation()
	connectEV(s, "conn-1a", 250, 250)
	connectEV(s, "conn-2a", 250, 250)

	// conn-2a disconnects
	s.EVSEs[1].Connectors[0].Session = nil
	s.EVSEs[1].Connectors[0].Status = domain.StatusAvailable

	allocs := domain.Allocate(s)
	if allocs["conn-1a"] != 250 {
		t.Errorf("T04: expected 250, got %f", allocs["conn-1a"])
	}
}

func TestAllocation_T05_EVUpdatesPower(t *testing.T) {
	s := buildTestStation()
	connectEV(s, "conn-1a", 250, 250)
	connectEV(s, "conn-2a", 250, 250)

	// conn-2a reduces demand to 100
	s.EVSEs[1].Connectors[0].Session.RequestedPower = 100

	allocs := domain.Allocate(s)
	// Grid = 400. Total demand = 350. Both get what they want.
	if allocs["conn-1a"] != 250 || allocs["conn-2a"] != 100 {
		t.Errorf("T05: expected 250/100, got %f / %f", allocs["conn-1a"], allocs["conn-2a"])
	}
}

func TestAllocation_T06_EVSESharing(t *testing.T) {
	s := buildTestStation()
	// Two connectors on same EVSE request 200 each. EVSE limit is 300.
	connectEV(s, "conn-1a", 200, 200)
	connectEV(s, "conn-1b", 200, 200)

	allocs := domain.Allocate(s)
	// EVSE 1 gets 300 total (it requested 400). Connectors share 300 proportionally -> 150 each.
	if allocs["conn-1a"] != 150 || allocs["conn-1b"] != 150 {
		t.Errorf("T06: expected 150/150, got %f / %f", allocs["conn-1a"], allocs["conn-1b"])
	}
}

func TestAllocation_T07_VehicleLimit(t *testing.T) {
	s := buildTestStation()
	// Request 250, but max is 100
	connectEV(s, "conn-1a", 250, 100)

	allocs := domain.Allocate(s)
	if allocs["conn-1a"] != 100 {
		t.Errorf("T07: expected 100, got %f", allocs["conn-1a"])
	}
}
