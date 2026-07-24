package main

import (
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"

	"sems/internal/api"
	"sems/internal/config"
	"sems/internal/service"
)

func main() {
	configPath := flag.String("config", "configs/example_station.json", "Path to station config JSON")
	port := flag.Int("port", 8080, "HTTP server port")
	devMode := flag.Bool("dev", false, "Enable development mode (text logging)")
	flag.Parse()

	var handler slog.Handler
	if *devMode {
		handler = slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug})
	} else {
		handler = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})
	}
	logger := slog.New(handler)
	slog.SetDefault(logger)

	logger.Info("Starting SEMS", "config", *configPath, "port", *port)

	station, err := config.LoadStation(*configPath)
	if err != nil {
		logger.Error("Failed to load config", "error", err)
		os.Exit(1)
	}

	siteController := service.NewSiteController(station, logger)
	server := api.NewServer(siteController, logger)

	addr := fmt.Sprintf(":%d", *port)
	logger.Info("Listening for requests", "addr", addr)
	if err := http.ListenAndServe(addr, server); err != nil {
		logger.Error("HTTP server failed", "error", err)
		os.Exit(1)
	}
}
