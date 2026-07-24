package api

import (
	"log/slog"
	"net/http"
	"time"

	"sems/internal/service"
)

type Server struct {
	controller *service.SiteController
	logger     *slog.Logger
	router     *http.ServeMux
}

func NewServer(controller *service.SiteController, logger *slog.Logger) *Server {
	s := &Server{
		controller: controller,
		logger:     logger,
		router:     http.NewServeMux(),
	}
	s.routes()
	return s
}



func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	s.router.ServeHTTP(w, r)
	s.logger.Info("HTTP Request", "method", r.Method, "path", r.URL.Path, "duration", time.Since(start).String())
}
