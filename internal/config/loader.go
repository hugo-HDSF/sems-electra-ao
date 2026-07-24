package config

import (
	"encoding/json"
	"fmt"
	"os"

	"sems/internal/domain"
)

// StationConfig represents the JSON structure for the entire station.
type StationConfig struct {
	ID          string        `json:"id"`
	GridLimitKW float64       `json:"gridLimitKW"`
	EVSEs       []EVSEConfig  `json:"evses"`
	BESS        *BESSConfig   `json:"bess,omitempty"`
}

// EVSEConfig represents the JSON structure for an EVSE.
type EVSEConfig struct {
	ID         string            `json:"id"`
	MaxPowerKW float64           `json:"maxPowerKW"`
	Connectors []ConnectorConfig `json:"connectors"`
}

// ConnectorConfig represents the JSON structure for a connector.
type ConnectorConfig struct {
	ID   string `json:"id"`
	Type string `json:"type"`
}

// BESSConfig represents the JSON structure for the battery storage system.
type BESSConfig struct {
	CapacityKWh         float64 `json:"capacityKWh"`
	InitialSoC          float64 `json:"initialSoC"`
	MaxChargePowerKW    float64 `json:"maxChargePowerKW"`
	MaxDischargePowerKW float64 `json:"maxDischargePowerKW"`
}

// Validate checks the structural integrity of the station configuration.
func (c *StationConfig) Validate() error {
	if c.GridLimitKW < 0 {
		return fmt.Errorf("GridLimitKW cannot be negative")
	}
	for _, evse := range c.EVSEs {
		if err := evse.Validate(); err != nil {
			return err
		}
	}
	if c.BESS != nil {
		if err := c.BESS.Validate(); err != nil {
			return err
		}
	}
	return nil
}

// Validate checks the EVSE configuration.
func (e *EVSEConfig) Validate() error {
	if e.MaxPowerKW <= 0 {
		return fmt.Errorf("EVSE %s max power must be greater than 0", e.ID)
	}
	return nil
}

// Validate checks the BESS configuration.
func (b *BESSConfig) Validate() error {
	if b.CapacityKWh <= 0 {
		return fmt.Errorf("BESS capacity must be greater than 0")
	}
	if b.InitialSoC < 0.0 || b.InitialSoC > 1.0 {
		return fmt.Errorf("BESS InitialSoC must be between 0.0 and 1.0")
	}
	return nil
}

// LoadStation loads a Station from a JSON configuration file.
func LoadStation(path string) (*domain.Station, error) {
	// Read and parse the raw JSON payload
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cfg StationConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid station configuration: %w", err)
	}

	return cfg.ToDomain(), nil
}

// ToDomain maps the JSON DTOs into the strict core Domain entities.
func (c *StationConfig) ToDomain() *domain.Station {
	station := &domain.Station{
		ID:        c.ID,
		GridLimit: c.GridLimitKW,
		EVSEs:     make([]*domain.EVSE, 0, len(c.EVSEs)),
	}

	for _, evseCfg := range c.EVSEs {
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

	if c.BESS != nil {
		station.BESS = &domain.BESS{
			Capacity:          c.BESS.CapacityKWh,
			SoC:               c.BESS.InitialSoC,
			MaxChargePower:    c.BESS.MaxChargePowerKW,
			MaxDischargePower: c.BESS.MaxDischargePowerKW,
			Status:            domain.BESSIdle,
		}
	}

	return station
}
