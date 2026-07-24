package api

import (
	"log/slog"
	"net/http"

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
	s.router.ServeHTTP(w, r)
}
