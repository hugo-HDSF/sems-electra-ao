package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"sems/internal/config"
)

func TestLoadStation(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "station.json")
	
	configData := []byte(`{
	  "id": "station-01",
	  "gridLimitKW": 400,
	  "evses": [
		{
		  "id": "evse-1",
		  "maxPowerKW": 300,
		  "connectors": [
			{ "id": "conn-1a", "type": "CCS_DC" },
			{ "id": "conn-1b", "type": "CCS_DC" }
		  ]
		}
	  ],
	  "bess": {
		"capacityKWh": 200,
		"initialSoC": 0.80,
		"maxChargePowerKW": 200,
		"maxDischargePowerKW": 200
	  }
	}`)
	
	if err := os.WriteFile(configPath, configData, 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	station, err := config.LoadStation(configPath)
	if err != nil {
		t.Fatalf("LoadStation failed: %v", err)
	}

	if station.ID != "station-01" {
		t.Errorf("expected station ID 'station-01', got %s", station.ID)
	}
	if station.GridLimit != 400 {
		t.Errorf("expected grid limit 400, got %f", station.GridLimit)
	}
	if len(station.EVSEs) != 1 {
		t.Errorf("expected 1 EVSE, got %d", len(station.EVSEs))
	} else {
		evse := station.EVSEs[0]
		if evse.ID != "evse-1" || evse.MaxPower != 300 {
			t.Errorf("unexpected EVSE details")
		}
		if len(evse.Connectors) != 2 {
			t.Errorf("expected 2 connectors, got %d", len(evse.Connectors))
		}
	}
	
	if station.BESS == nil {
		t.Errorf("expected BESS to be parsed, got nil")
	} else {
		if station.BESS.Capacity != 200 || station.BESS.SoC != 0.8 {
			t.Errorf("unexpected BESS details")
		}
	}
}
