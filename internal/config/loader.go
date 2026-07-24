package config

import (
	"encoding/json"
	"os"

	"sems/internal/domain"
)

// stationConfig represents the JSON structure for the entire station.
type stationConfig struct {
	ID          string        `json:"id"`
	GridLimitKW float64       `json:"gridLimitKW"`
	EVSEs       []evseConfig  `json:"evses"`
	BESS        *bessConfig   `json:"bess,omitempty"`
}

// evseConfig represents the JSON structure for an EVSE.
type evseConfig struct {
	ID         string            `json:"id"`
	MaxPowerKW float64           `json:"maxPowerKW"`
	Connectors []connectorConfig `json:"connectors"`
}

// connectorConfig represents the JSON structure for a connector.
type connectorConfig struct {
	ID   string `json:"id"`
	Type string `json:"type"`
}

// bessConfig represents the JSON structure for the battery storage system.
type bessConfig struct {
	CapacityKWh         float64 `json:"capacityKWh"`
	InitialSoC          float64 `json:"initialSoC"`
	MaxChargePowerKW    float64 `json:"maxChargePowerKW"`
	MaxDischargePowerKW float64 `json:"maxDischargePowerKW"`
}

// LoadStation loads a Station from a JSON configuration file.
func LoadStation(path string) (*domain.Station, error) {
	// Read and parse the raw JSON payload
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cfg stationConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	// Map the JSON DTOs into the strict core Domain entities
	station := &domain.Station{
		ID:        cfg.ID,
		GridLimit: cfg.GridLimitKW,
		EVSEs:     make([]*domain.EVSE, 0, len(cfg.EVSEs)),
	}

	// Initialize the nested hardware hierarchy (EVSEs -> Connectors)
	for _, evseCfg := range cfg.EVSEs {
		evse := &domain.EVSE{
			ID:         evseCfg.ID,
			MaxPower:   evseCfg.MaxPowerKW,
			Connectors: make([]*domain.Connector, 0, len(evseCfg.Connectors)),
		}
		for _, connCfg := range evseCfg.Connectors {
			evse.Connectors = append(evse.Connectors, &domain.Connector{
				ID:     connCfg.ID,
				Type:   domain.ConnectorType(connCfg.Type),
				EVSEID: evse.ID,
				Status: domain.StatusAvailable,
			})
		}
		station.EVSEs = append(station.EVSEs, evse)
	}

	if cfg.BESS != nil {
		station.BESS = &domain.BESS{
			Capacity:          cfg.BESS.CapacityKWh,
			SoC:               cfg.BESS.InitialSoC,
			MaxChargePower:    cfg.BESS.MaxChargePowerKW,
			MaxDischargePower: cfg.BESS.MaxDischargePowerKW,
			Status:            domain.BESSIdle,
		}
	}

	return station, nil
}
