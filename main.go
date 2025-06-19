package main

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/mux"
)

// BssData represents the metrics data structure
type BssData struct {
	Usersched_pid        uint32 `json:"usersched_pid"`        // The PID of the userspace scheduler
	Nr_queued            uint64 `json:"nr_queued"`            // Number of tasks queued in the userspace scheduler
	Nr_scheduled         uint64 `json:"nr_scheduled"`         // Number of tasks scheduled by the userspace scheduler
	Nr_running           uint64 `json:"nr_running"`           // Number of tasks currently running in the userspace scheduler
	Nr_online_cpus       uint64 `json:"nr_online_cpus"`       // Number of online CPUs in the system
	Nr_user_dispatches   uint64 `json:"nr_user_dispatches"`   // Number of user-space dispatches
	Nr_kernel_dispatches uint64 `json:"nr_kernel_dispatches"` // Number of kernel-space dispatches
	Nr_cancel_dispatches uint64 `json:"nr_cancel_dispatches"` // Number of cancelled dispatches
	Nr_bounce_dispatches uint64 `json:"nr_bounce_dispatches"` // Number of bounce dispatches
	Nr_failed_dispatches uint64 `json:"nr_failed_dispatches"` // Number of failed dispatches
	Nr_sched_congested   uint64 `json:"nr_sched_congested"`   // Number of times the scheduler was congested
}

// MetricsResponse represents the response structure
type MetricsResponse struct {
	Success   bool   `json:"success"`
	Message   string `json:"message"`
	Timestamp string `json:"timestamp"`
}

// ErrorResponse represents error response structure
type ErrorResponse struct {
	Success bool   `json:"success"`
	Error   string `json:"error"`
}

// MetricsHandler handles incoming metrics data
func MetricsHandler(w http.ResponseWriter, r *http.Request) {
	// Set response headers
	w.Header().Set("Content-Type", "application/json")

	// Only accept POST requests
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(ErrorResponse{
			Success: false,
			Error:   "Only POST method is allowed",
		})
		return
	}

	// Parse JSON body
	var bssData BssData
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&bssData); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(ErrorResponse{
			Success: false,
			Error:   "Invalid JSON format: " + err.Error(),
		})
		return
	}

	// Log received metrics
	log.Printf("Received metrics from PID %d:", bssData.Usersched_pid)
	log.Printf("  Queued tasks: %d", bssData.Nr_queued)
	log.Printf("  Scheduled tasks: %d", bssData.Nr_scheduled)
	log.Printf("  Running tasks: %d", bssData.Nr_running)
	log.Printf("  Online CPUs: %d", bssData.Nr_online_cpus)
	log.Printf("  User dispatches: %d", bssData.Nr_user_dispatches)
	log.Printf("  Kernel dispatches: %d", bssData.Nr_kernel_dispatches)
	log.Printf("  Cancel dispatches: %d", bssData.Nr_cancel_dispatches)
	log.Printf("  Bounce dispatches: %d", bssData.Nr_bounce_dispatches)
	log.Printf("  Failed dispatches: %d", bssData.Nr_failed_dispatches)
	log.Printf("  Scheduler congested: %d", bssData.Nr_sched_congested)

	// TODO: Here you can add logic to store metrics in database or process them
	// For now, we just acknowledge receipt

	// Send success response
	response := MetricsResponse{
		Success:   true,
		Message:   "Metrics received successfully",
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// HealthHandler provides a health check endpoint
func HealthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	response := map[string]interface{}{
		"status":    "healthy",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"service":   "BSS Metrics API Server",
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// CORS middleware
func enableCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// Logging middleware
func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		log.Printf("%s %s %s", r.Method, r.RequestURI, r.RemoteAddr)
		next.ServeHTTP(w, r)
		log.Printf("Request completed in %v", time.Since(start))
	})
}

func main() {
	// Create router
	r := mux.NewRouter()

	// Apply middleware
	r.Use(loggingMiddleware)
	r.Use(enableCORS)

	// Define routes
	r.HandleFunc("/api/v1/metrics", MetricsHandler).Methods("POST", "OPTIONS")
	r.HandleFunc("/health", HealthHandler).Methods("GET")
	r.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		response := map[string]string{
			"message":   "BSS Metrics API Server",
			"version":   "1.0.0",
			"endpoints": "/api/v1/metrics (POST), /health (GET)",
		}
		json.NewEncoder(w).Encode(response)
	}).Methods("GET")

	// Server configuration
	port := ":8080"
	log.Printf("Starting BSS Metrics API Server on port %s", port)
	log.Printf("Endpoints:")
	log.Printf("  POST /api/v1/metrics - Submit metrics data")
	log.Printf("  GET  /health         - Health check")
	log.Printf("  GET  /               - API information")

	// Start server
	srv := &http.Server{
		Handler:      r,
		Addr:         port,
		WriteTimeout: 15 * time.Second,
		ReadTimeout:  15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	log.Fatal(srv.ListenAndServe())
}
