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
// based on the total demand of the connectors on each EVSE, redistributing excess power.
func computeSiteShares(station *Station, demands map[string]float64, available float64) map[string]float64 {
	evseDemands := make(map[string]float64)
	evseLimits := make(map[string]float64)
	for _, evse := range station.EVSEs {
		var sum float64
		for _, conn := range evse.Connectors {
			sum += demands[conn.ID]
		}
		if sum > 0 {
			evseDemands[evse.ID] = sum
			evseLimits[evse.ID] = evse.MaxPower
		}
	}

	return proportionalSplitWithLimits(evseDemands, evseLimits, available)
}

// computeConnectorAllocations distributes the EVSE-level power allocation
// proportionally among the connectors on that EVSE, redistributing excess power.
func computeConnectorAllocations(station *Station, evseShares map[string]float64, demands map[string]float64) map[string]float64 {
	allocations := make(map[string]float64)

	for _, evse := range station.EVSEs {
		share, ok := evseShares[evse.ID]
		if !ok || share <= 0 {
			continue
		}

		connDemands := make(map[string]float64)
		connLimits := make(map[string]float64)
		for _, conn := range evse.Connectors {
			if d, ok := demands[conn.ID]; ok {
				connDemands[conn.ID] = d
				limit := d
				if conn.Session != nil {
					limit = min(limit, conn.Session.EVMaxPower)
				}
				connLimits[conn.ID] = limit
			}
		}

		if len(connDemands) == 0 {
			continue
		}

		connShares := proportionalSplitWithLimits(connDemands, connLimits, share)

		for _, conn := range evse.Connectors {
			if connShare, ok := connShares[conn.ID]; ok {
				allocations[conn.ID] = connShare
			}
		}
	}

	return allocations
}

// proportionalSplitWithLimits splits a budget among participants based on their requests,
// capping each participant at their respective limit, and redistributing any excess budget.
func proportionalSplitWithLimits(requests map[string]float64, limits map[string]float64, budget float64) map[string]float64 {
	shares := make(map[string]float64)
	
	// Create the initial pool of active participants that still need power
	active := make(map[string]bool)
	for k := range requests {
		active[k] = true
	}

	// Continuously redistribute the budget as long as we have active participants and power to give
	for len(active) > 0 && budget > 0.001 {
		var totalActiveReq float64
		for id := range active {
			totalActiveReq += requests[id]
		}

		if totalActiveReq <= 0 {
			break
		}

		var anyCapped bool
		for id := range active {
			req := requests[id]
			
			// Calculate this participant's mathematical fair share of the remaining budget
			fairShare := (req / totalActiveReq) * budget
			limit := limits[id]
			
			// If the fair share exceeds physical limits, cap the participant,
			// subtract their usage from the budget, and remove them from the active pool
			if fairShare >= limit || fairShare >= req {
				actualShare := min(limit, req)
				shares[id] += actualShare
				budget -= actualShare
				delete(active, id)
				anyCapped = true
			}
		}

		if !anyCapped {
			// No limits hit this round, meaning the remaining budget can be perfectly 
			// distributed. Grant everyone their exact fair share and break the loop.
			for id := range active {
				req := requests[id]
				fairShare := (req / totalActiveReq) * budget
				shares[id] += fairShare
			}
			break
		}
	}
	return shares
}
