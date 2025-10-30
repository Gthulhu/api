package rest

import (
	"net/http"
	"path/filepath"
	"runtime"

	"github.com/Gthulhu/api/util"
	"github.com/gorilla/mux"
)

func SetupRoutes(route *mux.Router, h *Handler) {

	route.Use(loggingMiddleware)
	route.Use(enableCORS)
	route.Use(getJwtAuthMiddleware(h.jwtPrivateKey)) // Add JWT authentication middleware

	route.HandleFunc("/api/v1/auth/token", h.GenTokenHandler).Methods("POST", "OPTIONS")

	route.HandleFunc("/api/v1/metrics", h.SaveMetricsHandler).Methods("POST", "OPTIONS")
	route.HandleFunc("/api/v1/metrics", h.GetMetricsHandler).Methods("GET", "OPTIONS")

	route.HandleFunc("/api/v1/pods/pids", h.GetPodPidHandler).Methods("GET", "OPTIONS")

	route.HandleFunc("/api/v1/scheduling/strategies", h.GetSchedulingStrategiesHandler).Methods("GET", "OPTIONS")
	route.HandleFunc("/api/v1/scheduling/strategies", h.SaveSchedulingStrategiesHandler).Methods("POST", "OPTIONS")

	route.HandleFunc("/health", h.HealthCheck).Methods("GET")
	route.HandleFunc("/", h.Version).Methods("GET")

	setupStaticRoutes(route)

	logger := util.GetLogger()
	logger.Info("Endpoints:")
	logger.Info("  POST /api/v1/auth/token              - Generate JWT token")
	logger.Info("  POST /api/v1/metrics                - Submit metrics data")
	logger.Info("  GET  /api/v1/metrics                - Get current metrics")
	logger.Info("  GET  /api/v1/pods/pids              - Get pod-PID mappings")
	logger.Info("  GET  /api/v1/scheduling/strategies  - Get scheduling strategies")
	logger.Info("  POST /api/v1/scheduling/strategies  - Save scheduling strategies")
	logger.Info("  GET  /health                        - Health check")
	logger.Info("  GET  /static/                       - Frontend web interface")
	logger.Info("  GET  /                              - Redirect to frontend")
}

func setupStaticRoutes(route *mux.Router) {
	_, f, _, _ := runtime.Caller(0)
	staticFolder := filepath.Join(filepath.Dir(f), "../", "static")

	staticFS := http.FileServer(http.Dir(staticFolder))
	route.PathPrefix("/static/").Handler(http.StripPrefix("/static/", staticFS)).Methods("GET")
}
