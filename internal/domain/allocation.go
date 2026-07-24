package domain

// Allocate calculates the power allocated to each active connector based on the
// available station power, EVSE hardware limits, connector/vehicle limits, and requested power.
func Allocate(station *Station) map[string]float64 {
	demands := gatherDemands(station)
	if len(demands) == 0 {
		return map[string]float64{}
	}

	available := computeAvailablePower(station)
	evseShares := computeSiteShares(station, demands, available)
	return computeConnectorAllocations(station, evseShares, demands)
}

// gatherDemands collects the current requested power from all active sessions.
func gatherDemands(station *Station) map[string]float64 {
	demands := make(map[string]float64)
	for _, evse := range station.EVSEs {
		for _, conn := range evse.Connectors {
			if conn.Session != nil && conn.Session.State == SessionActive && !conn.Session.IsFull() {
				demands[conn.ID] = conn.Session.RequestedPower
			}
		}
	}
	return demands
}

// computeAvailablePower determines the total power available at the station level.
func computeAvailablePower(station *Station) float64 {
	available := station.GridLimit
	if station.BESS != nil {
		available += station.BESS.AvailableDischargePower()
	}
	return available
}

// computeSiteShares allocates the available station power proportionally among EVSEs
// based on the total demand of the connectors on each EVSE.
func computeSiteShares(station *Station, demands map[string]float64, available float64) map[string]float64 {
	evseDemands := make(map[string]float64)
	for _, evse := range station.EVSEs {
		var sum float64
		for _, conn := range evse.Connectors {
			sum += demands[conn.ID]
		}
		if sum > 0 {
			evseDemands[evse.ID] = sum
		}
	}

	rawShares := proportionalSplit(evseDemands, available)

	// Cap each EVSE's share to its MaxPower
	evseShares := make(map[string]float64)
	for _, evse := range station.EVSEs {
		if share, ok := rawShares[evse.ID]; ok {
			evseShares[evse.ID] = min(share, evse.MaxPower)
		}
	}

	return evseShares
}

// computeConnectorAllocations distributes the EVSE-level power allocation
// proportionally among the connectors on that EVSE.
func computeConnectorAllocations(station *Station, evseShares map[string]float64, demands map[string]float64) map[string]float64 {
	allocations := make(map[string]float64)

	for _, evse := range station.EVSEs {
		share, ok := evseShares[evse.ID]
		if !ok || share <= 0 {
			continue
		}

		connDemands := make(map[string]float64)
		for _, conn := range evse.Connectors {
			if d, ok := demands[conn.ID]; ok {
				connDemands[conn.ID] = d
			}
		}

		if len(connDemands) == 0 {
			continue
		}

		connShares := proportionalSplit(connDemands, share)

		for _, conn := range evse.Connectors {
			if connShare, ok := connShares[conn.ID]; ok {
				allocations[conn.ID] = capAtLimits(connShare, conn.Session)
			}
		}
	}

	return allocations
}

// proportionalSplit splits a budget among participants based on their requests.
func proportionalSplit(requests map[string]float64, budget float64) map[string]float64 {
	shares := make(map[string]float64)
	var totalRequest float64
	for _, req := range requests {
		totalRequest += req
	}

	if totalRequest == 0 || budget == 0 {
		return shares
	}

	if totalRequest <= budget {
		for id, req := range requests {
			shares[id] = req
		}
		return shares
	}

	for id, req := range requests {
		shares[id] = (req / totalRequest) * budget
	}
	return shares
}

// capAtLimits ensures the final allocation does not exceed the session constraints.
func capAtLimits(allocated float64, session *Session) float64 {
	if session == nil {
		return 0
	}
	v := min(allocated, session.EVMaxPower)
	v = min(v, session.RequestedPower)
	return v
}
