package api

import (
	"log/slog"
	"net/http"
	"time"

	"sems/internal/service"
)

// Server handles all HTTP REST endpoints and SSE streams.
type Server struct {
	controller *service.SiteController
	logger     *slog.Logger
	router     *http.ServeMux
}

// NewServer creates a new API Server instance with registered routes.
func NewServer(controller *service.SiteController, logger *slog.Logger) *Server {
	s := &Server{
		controller: controller,
		logger:     logger,
		router:     http.NewServeMux(),
	}
	s.routes()
	return s
}



// ServeHTTP implements the http.Handler interface and adds request logging.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	s.router.ServeHTTP(w, r)
	s.logger.Info("HTTP Request", "method", r.Method, "path", r.URL.Path, "duration", time.Since(start).String())
}
