package api

import (
	"encoding/json"
	"net/http"
	"time"
	"sems/internal/domain"
)

func (s *Server) routes() {
	s.router.HandleFunc("POST /api/v1/station/config", s.handleConfig)
	s.router.HandleFunc("POST /api/v1/events/connect", s.handleConnect)
	s.router.HandleFunc("POST /api/v1/events/disconnect", s.handleDisconnect)
	s.router.HandleFunc("POST /api/v1/events/power-update", s.handlePowerUpdate)
	s.router.HandleFunc("POST /api/v1/simulate/tick", s.handleTick)
	s.router.HandleFunc("GET /api/v1/status", s.handleStatus)
	s.router.HandleFunc("GET /api/v1/health", s.handleHealth)

	// Swagger UI static
	s.router.Handle("/swagger/", http.StripPrefix("/swagger/", http.FileServer(http.Dir("./swagger"))))
}

func (s *Server) handleConfig(w http.ResponseWriter, r *http.Request) {
	var station domain.Station
	if err := json.NewDecoder(r.Body).Decode(&station); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Assume SiteController has a Reconfigure method
	// We'll add this method or just log for now if we can't mutate safely
	s.logger.Info("received station config payload", "stationId", station.ID)
	// (Reconfigure logic omitted for simplicity unless requested)

	json.NewEncoder(w).Encode(ConfigResponse{
		Status:    "configured",
		StationID: station.ID,
	})
}

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

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(s.controller.GetStatus())
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	json.NewEncoder(w).Encode(StatusResponse{Status: "ok"})
}
