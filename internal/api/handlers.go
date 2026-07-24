package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
	"sems/internal/domain"
)

// routes registers all API endpoints on the Server's router.
func (s *Server) routes() {
	s.router.HandleFunc("POST /api/v1/station/config", s.handleConfig)
	s.router.HandleFunc("POST /api/v1/events/connect", s.handleConnect)
	s.router.HandleFunc("POST /api/v1/events/disconnect", s.handleDisconnect)
	s.router.HandleFunc("POST /api/v1/events/power-update", s.handlePowerUpdate)
	s.router.HandleFunc("POST /api/v1/simulate/tick", s.handleTick)
	s.router.HandleFunc("GET /api/v1/status", s.handleStatus)
	s.router.HandleFunc("GET /api/v1/status/stream", s.handleStatusStream)
	s.router.HandleFunc("GET /api/v1/health", s.handleHealth)

	// Swagger UI static
	s.router.Handle("/swagger/", http.StripPrefix("/swagger/", http.FileServer(http.Dir("./swagger"))))

	// Web Dashboard static
	s.router.Handle("/", http.FileServer(http.Dir("./web")))
}

// handleConfig processes a new station configuration payload.
func (s *Server) handleConfig(w http.ResponseWriter, r *http.Request) {
	var station domain.Station
	if err := json.NewDecoder(r.Body).Decode(&station); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	s.logger.Info("received station config payload", "stationId", station.ID)
	json.NewEncoder(w).Encode(ConfigResponse{
		Status:    "configured",
		StationID: station.ID,
	})
}

// handleConnect processes a new EV connection and returns the allocated power.
func (s *Server) handleConnect(w http.ResponseWriter, r *http.Request) {
	var req ConnectRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	session, err := s.controller.ConnectEV(req.ConnectorID, req.EVMaxPowerKW, req.EVBatteryKWh, req.EVSoC, req.Timestamp)
	if err != nil {
		http.Error(w, err.Error(), http.StatusConflict)
		return
	}

	json.NewEncoder(w).Encode(ConnectResponse{
		SessionID:        session.ID,
		AllocatedPowerKW: session.AllocatedPower,
	})
}

// handleDisconnect processes an EV disconnection.
func (s *Server) handleDisconnect(w http.ResponseWriter, r *http.Request) {
	var req DisconnectRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	err := s.controller.DisconnectEV(req.ConnectorID, req.Timestamp)
	if err != nil {
		http.Error(w, err.Error(), http.StatusConflict)
		return
	}

	json.NewEncoder(w).Encode(StatusResponse{Status: "disconnected"})
}

// handlePowerUpdate processes an EV's updated power request and SoC.
func (s *Server) handlePowerUpdate(w http.ResponseWriter, r *http.Request) {
	var req PowerUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	err := s.controller.UpdatePowerRequest(req.ConnectorID, req.RequestedPowerKW, req.EVSoC, req.Timestamp)
	if err != nil {
		http.Error(w, err.Error(), http.StatusConflict)
		return
	}

	st := s.controller.GetStatus()
	var allocated float64
	for _, sess := range st.Sessions {
		if sess.ConnectorID == req.ConnectorID {
			allocated = sess.AllocatedPowerKW
			break
		}
	}

	json.NewEncoder(w).Encode(PowerUpdateResponse{AllocatedPowerKW: allocated})
}

// handleTick advances the simulation time based on the requested duration.
func (s *Server) handleTick(w http.ResponseWriter, r *http.Request) {
	var req TickRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	dur := time.Duration(req.DurationMinutes) * time.Minute
	res := s.controller.Tick(dur)

	json.NewEncoder(w).Encode(TickResponse{
		AdvancedBy:   dur.String(),
		Timestamp:    res.Timestamp,
		Disconnected: res.Disconnected,
		Status:       res.Status,
	})
}

// handleStatus returns the current snapshot of the station.
func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(s.controller.GetStatus())
}

// handleStatusStream maintains a persistent Server-Sent Events (SSE) connection
// and pushes real-time station updates to the client whenever the state changes.
func (s *Server) handleStatusStream(w http.ResponseWriter, r *http.Request) {
	// Set required headers for SSE
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	// Optional: CORS headers if needed for other frontends
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// Ensure the response writer supports flushing
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
		return
	}

	// Register with the orchestrator to receive updates
	ch := s.controller.Subscribe()
	defer s.controller.Unsubscribe(ch)

	// Listen for updates or client disconnection
	for {
		select {
		case status := <-ch:
			// Encode the status object to JSON
			jsonBytes, err := json.Marshal(status)
			if err != nil {
				s.logger.Error("Failed to marshal SSE status", "error", err)
				continue
			}
			
			// Write the standard SSE payload format: "data: {json}\n\n"
			fmt.Fprintf(w, "data: %s\n\n", jsonBytes)
			flusher.Flush()

		case <-r.Context().Done():
			// The client closed the connection (e.g. closed the browser tab)
			s.logger.Info("SSE client disconnected")
			return
		}
	}
}

// handleHealth returns a simple 200 OK for health checks.
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	json.NewEncoder(w).Encode(StatusResponse{Status: "ok"})
}
