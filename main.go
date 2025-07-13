package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/mux"
)

// BssData represents the metrics data structure
type BssData struct {
	Usersched_last_run_at uint64 `json:"usersched_last_run_at"` // The PID of the userspace scheduler
	Nr_queued             uint64 `json:"nr_queued"`             // Number of tasks queued in the userspace scheduler
	Nr_scheduled          uint64 `json:"nr_scheduled"`          // Number of tasks scheduled by the userspace scheduler
	Nr_running            uint64 `json:"nr_running"`            // Number of tasks currently running in the userspace scheduler
	Nr_online_cpus        uint64 `json:"nr_online_cpus"`        // Number of online CPUs in the system
	Nr_user_dispatches    uint64 `json:"nr_user_dispatches"`    // Number of user-space dispatches
	Nr_kernel_dispatches  uint64 `json:"nr_kernel_dispatches"`  // Number of kernel-space dispatches
	Nr_cancel_dispatches  uint64 `json:"nr_cancel_dispatches"`  // Number of cancelled dispatches
	Nr_bounce_dispatches  uint64 `json:"nr_bounce_dispatches"`  // Number of bounce dispatches
	Nr_failed_dispatches  uint64 `json:"nr_failed_dispatches"`  // Number of failed dispatches
	Nr_sched_congested    uint64 `json:"nr_sched_congested"`    // Number of times the scheduler was congested
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

// PodProcess represents a process information within a pod
type PodProcess struct {
	PID     int    `json:"pid"`
	Command string `json:"command"`
	PPID    int    `json:"ppid,omitempty"`
}

// PodInfo represents pod information with associated processes
type PodInfo struct {
	PodName     string       `json:"pod_name"`
	Namespace   string       `json:"namespace"`
	PodUID      string       `json:"pod_uid"`
	ContainerID string       `json:"container_id,omitempty"`
	Processes   []PodProcess `json:"processes"`
}

// PodPidResponse represents the response structure for pod-pid mapping
type PodPidResponse struct {
	Success   bool      `json:"success"`
	Message   string    `json:"message"`
	Timestamp string    `json:"timestamp"`
	Pods      []PodInfo `json:"pods"`
}

// getPodInfoFromCgroup extracts pod information from cgroup path
func getPodInfoFromCgroup(cgroupPath string) (string, string, string, error) {
	// Parse cgroup path to extract pod information
	// Format: /kubepods/burstable/pod<pod-uid>/<container-id>
	// or: /kubepods/pod<pod-uid>/<container-id>
	parts := strings.Split(cgroupPath, "/")

	var podUID, containerID string
	for i, part := range parts {
		if strings.HasPrefix(part, "pod") {
			podUID = strings.TrimPrefix(part, "pod")
			podUID = strings.ReplaceAll(podUID, "_", "-")
			if i+1 < len(parts) {
				containerID = parts[i+1]
			}
			break
		}
	}

	if podUID == "" {
		return "", "", "", fmt.Errorf("pod UID not found in cgroup path")
	}

	return podUID, containerID, "", nil
}

// getProcessInfo reads process information from /proc/<pid>/
func getProcessInfo(pid int) (PodProcess, error) {
	process := PodProcess{PID: pid}

	// Read command from /proc/<pid>/comm
	commPath := fmt.Sprintf("/proc/%d/comm", pid)
	if data, err := os.ReadFile(commPath); err == nil {
		process.Command = strings.TrimSpace(string(data))
	}

	// Read PPID from /proc/<pid>/stat
	statPath := fmt.Sprintf("/proc/%d/stat", pid)
	if data, err := os.ReadFile(statPath); err == nil {
		fields := strings.Fields(string(data))
		if len(fields) >= 4 {
			if ppid, err := strconv.Atoi(fields[3]); err == nil {
				process.PPID = ppid
			}
		}
	}

	return process, nil
}

// getPodPidMapping scans the system to find pod-pid mappings
func getPodPidMapping() ([]PodInfo, error) {
	podMap := make(map[string]*PodInfo)

	// Walk through /proc to find all processes
	procDir := "/proc"
	entries, err := os.ReadDir(procDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read /proc directory: %v", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		// Check if directory name is a PID (numeric)
		pid, err := strconv.Atoi(entry.Name())
		if err != nil {
			continue
		}

		// Read cgroup information for this process
		cgroupPath := fmt.Sprintf("/proc/%d/cgroup", pid)
		file, err := os.Open(cgroupPath)
		if err != nil {
			continue
		}

		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			line := scanner.Text()
			// Look for kubepods in cgroup hierarchy
			if strings.Contains(line, "kubepods") {
				parts := strings.Split(line, ":")
				if len(parts) >= 3 {
					cgroupHierarchy := parts[2]

					// Extract pod information
					podUID, containerID, _, err := getPodInfoFromCgroup(cgroupHierarchy)
					if err != nil {
						continue
					}

					// Get process information
					process, err := getProcessInfo(pid)
					if err != nil {
						continue
					}

					// Create or update pod info
					if podInfo, exists := podMap[podUID]; exists {
						podInfo.Processes = append(podInfo.Processes, process)
						if containerID != "" && podInfo.ContainerID == "" {
							podInfo.ContainerID = containerID
						}
					} else {
						podMap[podUID] = &PodInfo{
							PodUID:      podUID,
							ContainerID: containerID,
							Processes:   []PodProcess{process},
						}
					}
				}
				break
			}
		}
		file.Close()
	}

	// Convert map to slice
	var pods []PodInfo
	for _, podInfo := range podMap {
		pods = append(pods, *podInfo)
	}

	return pods, nil
}

// PodPidHandler provides pod-pid mapping information
func PodPidHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Only accept GET requests
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(ErrorResponse{
			Success: false,
			Error:   "Only GET method is allowed",
		})
		return
	}

	// Get pod-pid mappings
	pods, err := getPodPidMapping()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(ErrorResponse{
			Success: false,
			Error:   "Failed to get pod-pid mappings: " + err.Error(),
		})
		return
	}

	// Log the request
	log.Printf("Pod-PID mapping requested: found %d pods with processes", len(pods))

	// Send success response
	response := PodPidResponse{
		Success:   true,
		Message:   "Pod-PID mappings retrieved successfully",
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Pods:      pods,
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
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
	log.Printf("  UserSched last run at %d:", bssData.Usersched_last_run_at)
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
	r.HandleFunc("/api/v1/pods/pids", PodPidHandler).Methods("GET", "OPTIONS")
	r.HandleFunc("/health", HealthHandler).Methods("GET")
	r.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		response := map[string]string{
			"message":   "BSS Metrics API Server",
			"version":   "1.0.0",
			"endpoints": "/api/v1/metrics (POST), /api/v1/pods/pids (GET), /health (GET)",
		}
		json.NewEncoder(w).Encode(response)
	}).Methods("GET")

	// Server configuration
	port := ":8080"
	log.Printf("Starting BSS Metrics API Server on port %s", port)
	log.Printf("Endpoints:")
	log.Printf("  POST /api/v1/metrics   - Submit metrics data")
	log.Printf("  GET  /api/v1/pods/pids - Get pod-PID mappings")
	log.Printf("  GET  /health           - Health check")
	log.Printf("  GET  /                 - API information")

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
