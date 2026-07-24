package config

import (
	"encoding/json"
	"os"

	"sems/internal/domain"
)

type stationConfig struct {
	ID          string        `json:"id"`
	GridLimitKW float64       `json:"gridLimitKW"`
	EVSEs       []evseConfig  `json:"evses"`
	BESS        *bessConfig   `json:"bess,omitempty"`
}

type evseConfig struct {
	ID         string            `json:"id"`
	MaxPowerKW float64           `json:"maxPowerKW"`
	Connectors []connectorConfig `json:"connectors"`
}

type connectorConfig struct {
	ID   string `json:"id"`
	Type string `json:"type"`
}

type bessConfig struct {
	CapacityKWh         float64 `json:"capacityKWh"`
	InitialSoC          float64 `json:"initialSoC"`
	MaxChargePowerKW    float64 `json:"maxChargePowerKW"`
	MaxDischargePowerKW float64 `json:"maxDischargePowerKW"`
}

// LoadStation loads a Station from a JSON configuration file.
func LoadStation(path string) (*domain.Station, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cfg stationConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	station := &domain.Station{
		ID:        cfg.ID,
		GridLimit: cfg.GridLimitKW,
		EVSEs:     make([]*domain.EVSE, 0, len(cfg.EVSEs)),
	}

	for _, eCfg := range cfg.EVSEs {
		evse := &domain.EVSE{
			ID:         eCfg.ID,
			MaxPower:   eCfg.MaxPowerKW,
			Connectors: make([]*domain.Connector, 0, len(eCfg.Connectors)),
		}
		for _, cCfg := range eCfg.Connectors {
			evse.Connectors = append(evse.Connectors, &domain.Connector{
				ID:     cCfg.ID,
				Type:   domain.ConnectorType(cCfg.Type),
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
