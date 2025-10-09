package main

import (
	"bufio"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"log"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
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

// GetMetricsResponse represents the response structure for getting current metrics
type GetMetricsResponse struct {
	Success          bool     `json:"success"`
	Message          string   `json:"message"`
	Timestamp        string   `json:"timestamp"`
	Data             *BssData `json:"data,omitempty"`
	MetricsTimestamp string   `json:"metrics_timestamp,omitempty"`
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

// LabelSelector represents a key-value pair for pod label selection
type LabelSelector struct {
	Key   string `json:"key"`   // Label key
	Value string `json:"value"` // Label value
}

// SchedulingStrategy represents a strategy for process scheduling
type SchedulingStrategy struct {
	Priority      bool            `json:"priority"`                // If true, set vtime to minimum vtime
	ExecutionTime uint64          `json:"execution_time"`          // Time slice for this process in nanoseconds
	PID           int             `json:"pid,omitempty"`           // Process ID to apply this strategy to
	Selectors     []LabelSelector `json:"selectors,omitempty"`     // Label selectors to match pods
	CommandRegex  string          `json:"command_regex,omitempty"` // Regex to match process command
}

// SchedulingStrategiesResponse represents the response structure for scheduling strategies
type SchedulingStrategiesResponse struct {
	Success    bool                 `json:"success"`
	Message    string               `json:"message"`
	Timestamp  string               `json:"timestamp"`
	Scheduling []SchedulingStrategy `json:"scheduling"`
}

// StrategyRequest represents the request structure for setting scheduling strategies
type StrategyRequest struct {
	Strategies []SchedulingStrategy `json:"strategies"`
}

// TokenRequest represents the request structure for JWT token generation
type TokenRequest struct {
	PublicKey string `json:"public_key"` // PEM encoded public key
}

// TokenResponse represents the response structure for JWT token generation
type TokenResponse struct {
	Success   bool   `json:"success"`
	Message   string `json:"message"`
	Timestamp string `json:"timestamp"`
	Token     string `json:"token,omitempty"`
}

// Claims represents JWT token claims
type Claims struct {
	ClientID string `json:"client_id"`
	jwt.RegisteredClaims
}

var (
	strategiesMutex sync.RWMutex
	userStrategies  []SchedulingStrategy
	jwtPrivateKey   *rsa.PrivateKey
	jwtConfig       JWTConfig

	// Latest metrics data
	metricsMutex     sync.RWMutex
	latestMetrics    *BssData
	metricsTimestamp time.Time

	// Regex compilation cache
	regexCacheMu sync.RWMutex
	regexCache   = make(map[string]*regexp.Regexp)
)

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
		defer file.Close()

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
		if err := json.NewEncoder(w).Encode(ErrorResponse{
			Success: false,
			Error:   "Only GET method is allowed",
		}); err != nil {
			log.Printf("Error encoding response: %v", err)
		}
		return
	}

	// Get pod-pid mappings
	pods, err := getPodPidMapping()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		if err := json.NewEncoder(w).Encode(ErrorResponse{
			Success: false,
			Error:   "Failed to get pod-pid mappings: " + err.Error(),
		}); err != nil {
			log.Printf("Error encoding response: %v", err)
		}
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
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("Error encoding response: %v", err)
	}
}

// MetricsHandler handles incoming metrics data
func MetricsHandler(w http.ResponseWriter, r *http.Request) {
	// Set response headers
	w.Header().Set("Content-Type", "application/json")

	// Only accept POST requests
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		if err := json.NewEncoder(w).Encode(ErrorResponse{
			Success: false,
			Error:   "Only POST method is allowed",
		}); err != nil {
			log.Printf("Error encoding response: %v", err)
		}
		return
	}

	// Parse JSON body
	var bssData BssData
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&bssData); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		if err := json.NewEncoder(w).Encode(ErrorResponse{
			Success: false,
			Error:   "Invalid JSON format: " + err.Error(),
		}); err != nil {
			log.Printf("Error encoding response: %v", err)
		}
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

	// Store latest metrics data
	metricsMutex.Lock()
	latestMetrics = &bssData
	metricsTimestamp = time.Now()
	metricsMutex.Unlock()

	// TODO: Here you can add logic to store metrics in database or process them
	// For now, we just acknowledge receipt

	// Send success response
	response := MetricsResponse{
		Success:   true,
		Message:   "Metrics received successfully",
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}

	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("Error encoding response: %v", err)
	}
}

// GetMetricsHandler provides current metrics data
func GetMetricsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Only accept GET requests
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		if err := json.NewEncoder(w).Encode(ErrorResponse{
			Success: false,
			Error:   "Only GET method is allowed",
		}); err != nil {
			log.Printf("Error encoding response: %v", err)
		}
		return
	}

	metricsMutex.RLock()
	currentMetrics := latestMetrics
	currentTimestamp := metricsTimestamp
	metricsMutex.RUnlock()

	var response GetMetricsResponse
	if currentMetrics != nil {
		response = GetMetricsResponse{
			Success:          true,
			Message:          "Current metrics retrieved successfully",
			Timestamp:        time.Now().UTC().Format(time.RFC3339),
			Data:             currentMetrics,
			MetricsTimestamp: currentTimestamp.UTC().Format(time.RFC3339),
		}
	} else {
		response = GetMetricsResponse{
			Success:   false,
			Message:   "No metrics data available yet",
			Timestamp: time.Now().UTC().Format(time.RFC3339),
		}
	}

	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("Error encoding response: %v", err)
	}
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
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("Error encoding response: %v", err)
	}
}

// GetSchedulingStrategiesHandler provides scheduling strategies for Gthulhu with caching
func GetSchedulingStrategiesHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Only accept GET requests
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		if err := json.NewEncoder(w).Encode(ErrorResponse{
			Success: false,
			Error:   "Only GET method is allowed",
		}); err != nil {
			log.Printf("Error encoding response: %v", err)
		}
		return
	}

	// Get configured strategies
	var configuredStrategies []SchedulingStrategy
	strategiesMutex.RLock()
	if len(userStrategies) > 0 {
		configuredStrategies = append(configuredStrategies, userStrategies...)
	}
	strategiesMutex.RUnlock()

	// Try to get cached strategies
	finalStrategies, fromCache := GetCachedStrategies(configuredStrategies)

	// If not from cache, strategies were recalculated in GetCachedStrategies
	var message string
	if fromCache {
		message = "Scheduling strategies retrieved from cache"
	} else {
		message = "Scheduling strategies recalculated due to pod changes"
	}

	// Log the request with cache info
	log.Printf("Scheduling strategies requested by Gthulhu, generated %d strategies (from cache: %v)",
		len(finalStrategies), fromCache)

	// Add cache statistics to response
	cacheStats := strategyCache.GetStats()

	// Send success response with cache info
	response := SchedulingStrategiesResponse{
		Success:    true,
		Message:    message,
		Timestamp:  time.Now().UTC().Format(time.RFC3339),
		Scheduling: finalStrategies,
	}

	// Add cache stats as header for debugging
	w.Header().Set("X-Cache-Hit", fmt.Sprintf("%v", fromCache))
	w.Header().Set("X-Cache-Stats", fmt.Sprintf("hits=%d,misses=%d,hit_rate=%v",
		cacheStats["hits"], cacheStats["misses"], cacheStats["hit_rate"]))

	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("Error encoding response: %v", err)
	}
}

// SaveStrategiesHandler handles requests to save new scheduling strategies
func SaveStrategiesHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Only accept POST requests
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		if err := json.NewEncoder(w).Encode(ErrorResponse{
			Success: false,
			Error:   "Only POST method is allowed",
		}); err != nil {
			log.Printf("Error encoding response: %v", err)
		}
		return
	}

	// Parse JSON body
	var request StrategyRequest
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&request); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		if err := json.NewEncoder(w).Encode(ErrorResponse{
			Success: false,
			Error:   "Invalid JSON format: " + err.Error(),
		}); err != nil {
			log.Printf("Error encoding response: %v", err)
		}
		return
	}

	// Validate strategies
	if len(request.Strategies) == 0 {
		w.WriteHeader(http.StatusBadRequest)
		if err := json.NewEncoder(w).Encode(ErrorResponse{
			Success: false,
			Error:   "No strategies provided",
		}); err != nil {
			log.Printf("Error encoding response: %v", err)
		}
		return
	}

	strategiesMutex.Lock()
	userStrategies = request.Strategies
	strategiesMutex.Unlock()

	// Invalidate cache since strategies have changed
	strategyCache.Invalidate()
	log.Printf("Strategy cache invalidated due to configuration change")

	// Log received strategies
	log.Printf("Received %d new scheduling strategies", len(request.Strategies))
	for i, strategy := range request.Strategies {
		log.Printf("Strategy %d: Priority=%v, ExecutionTime=%d", i+1, strategy.Priority, strategy.ExecutionTime)
		if len(strategy.Selectors) > 0 {
			for j, selector := range strategy.Selectors {
				log.Printf("  Selector %d: %s=%s", j+1, selector.Key, selector.Value)
			}
		}
		if strategy.PID != 0 {
			log.Printf("  PID: %d", strategy.PID)
		}
	}

	// Send success response
	response := MetricsResponse{
		Success:   true,
		Message:   "Scheduling strategies saved successfully",
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}

	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("Error encoding response: %v", err)
	}
}

var cmdOptions CommandLineOptions

func getPodLabels(podUID string) (map[string]string, error) {
	pod, err := getKubernetesPod(podUID, cmdOptions)
	if err != nil {
		return nil, err
	}
	return pod.Labels, nil
}

func findPIDsByStrategy(strategy SchedulingStrategy) ([]int, error) {
	var matchedPIDs []int

	pods, err := getPodPidMapping()
	if err != nil {
		return nil, fmt.Errorf("failed to get pod-pid mappings: %v", err)
	}

	// Set default regex if empty
	if strategy.CommandRegex == "" {
		strategy.CommandRegex = ".*"
	}

	// Get or compile regex pattern (with caching)
	regexCacheMu.RLock()
	regex, exists := regexCache[strategy.CommandRegex]
	regexCacheMu.RUnlock()

	if !exists {
		// Compile regex and add to cache
		compiledRegex, err := regexp.Compile(strategy.CommandRegex)
		if err != nil {
			return nil, fmt.Errorf("invalid regex pattern '%s': %v", strategy.CommandRegex, err)
		}

		regexCacheMu.Lock()
		regexCache[strategy.CommandRegex] = compiledRegex
		regexCacheMu.Unlock()

		regex = compiledRegex
		log.Printf("Compiled and cached regex pattern: %s", strategy.CommandRegex)
	}

	for _, pod := range pods {
		podSpec, err := getKubernetesPod(pod.PodUID, cmdOptions)
		if err != nil {
			return nil, err
		}
		labels := podSpec.Labels
		log.Printf("Pod %s labels: %v", pod.PodUID, labels)

		matches := true
		for _, selector := range strategy.Selectors {
			value, exists := labels[selector.Key]
			if !exists || value != selector.Value {
				matches = false
				break
			}
		}

		if matches {
			// Use cached regex for all process matching
			for _, process := range pod.Processes {
				if regex.MatchString(process.Command) {
					matchedPIDs = append(matchedPIDs, process.PID)
				}
			}
		}
	}

	return matchedPIDs, nil
}

// loadPrivateKey loads RSA private key from PEM file
func loadPrivateKey(keyPath string) (*rsa.PrivateKey, error) {
	keyData, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read private key file: %v", err)
	}

	block, _ := pem.Decode(keyData)
	if block == nil {
		return nil, fmt.Errorf("failed to decode PEM block containing private key")
	}

	key, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		// Try PKCS8 format
		keyInterface, err := x509.ParsePKCS8PrivateKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("failed to parse private key: %v", err)
		}
		var ok bool
		key, ok = keyInterface.(*rsa.PrivateKey)
		if !ok {
			return nil, fmt.Errorf("private key is not RSA")
		}
	}

	return key, nil
}

// generatePrivateKey generates a new RSA private key and saves it to file
func generatePrivateKey(keyPath string) (*rsa.PrivateKey, error) {
	// Generate RSA private key
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, fmt.Errorf("failed to generate private key: %v", err)
	}

	// Create directory if it doesn't exist
	if err := os.MkdirAll(strings.TrimSuffix(keyPath, "/private_key.pem"), 0755); err != nil {
		log.Printf("Warning: failed to create directory for private key: %v", err)
	}

	// Save private key to file
	keyBytes := x509.MarshalPKCS1PrivateKey(privateKey)
	keyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: keyBytes,
	})

	if err := os.WriteFile(keyPath, keyPEM, 0600); err != nil {
		log.Printf("Warning: failed to save private key to file: %v", err)
	} else {
		log.Printf("Generated and saved new private key to %s", keyPath)
	}

	return privateKey, nil
}

// initJWT initializes JWT configuration and private key
func initJWT(config JWTConfig) error {
	jwtConfig = config

	// Try to load existing private key
	key, err := loadPrivateKey(config.PrivateKeyPath)
	if err != nil {
		log.Printf("Failed to load private key from %s: %v", config.PrivateKeyPath, err)
		log.Printf("Generating new private key...")

		// Generate new private key
		key, err = generatePrivateKey(config.PrivateKeyPath)
		if err != nil {
			return fmt.Errorf("failed to generate private key: %v", err)
		}
	} else {
		log.Printf("Loaded private key from %s", config.PrivateKeyPath)
	}

	jwtPrivateKey = key
	return nil
}

// verifyPublicKey verifies if the provided public key matches our private key
func verifyPublicKey(publicKeyPEM string) error {
	block, _ := pem.Decode([]byte(publicKeyPEM))
	if block == nil {
		return fmt.Errorf("failed to decode PEM block containing public key")
	}

	publicKey, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return fmt.Errorf("failed to parse public key: %v", err)
	}

	rsaPublicKey, ok := publicKey.(*rsa.PublicKey)
	if !ok {
		return fmt.Errorf("public key is not RSA")
	}

	// Compare public key with our private key's public key
	if !rsaPublicKey.Equal(&jwtPrivateKey.PublicKey) {
		return fmt.Errorf("public key does not match server's private key")
	}

	return nil
}

// generateJWT generates a JWT token for authenticated client
func generateJWT(clientID string) (string, error) {
	claims := Claims{
		ClientID: clientID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Duration(jwtConfig.TokenDuration) * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
			Issuer:    "bss-api-server",
			Subject:   clientID,
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	return token.SignedString(jwtPrivateKey)
}

// validateJWT validates a JWT token and returns the claims
func validateJWT(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return &jwtPrivateKey.PublicKey, nil
	})

	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(*Claims); ok && token.Valid {
		return claims, nil
	}

	return nil, fmt.Errorf("invalid token")
}

// TokenHandler handles JWT token generation requests
func TokenHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Only accept POST requests
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		if err := json.NewEncoder(w).Encode(ErrorResponse{
			Success: false,
			Error:   "Only POST method is allowed",
		}); err != nil {
			log.Printf("Error encoding response: %v", err)
		}
		return
	}

	// Parse JSON body
	var request TokenRequest
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&request); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		if err := json.NewEncoder(w).Encode(ErrorResponse{
			Success: false,
			Error:   "Invalid JSON format: " + err.Error(),
		}); err != nil {
			log.Printf("Error encoding response: %v", err)
		}
		return
	}

	// Verify public key
	if err := verifyPublicKey(request.PublicKey); err != nil {
		w.WriteHeader(http.StatusUnauthorized)
		if err := json.NewEncoder(w).Encode(ErrorResponse{
			Success: false,
			Error:   "Public key verification failed: " + err.Error(),
		}); err != nil {
			log.Printf("Error encoding response: %v", err)
		}
		return
	}

	// Generate client ID from public key hash (simplified)
	clientID := fmt.Sprintf("client_%d", time.Now().Unix())

	// Generate JWT token
	token, err := generateJWT(clientID)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		if err := json.NewEncoder(w).Encode(ErrorResponse{
			Success: false,
			Error:   "Failed to generate token: " + err.Error(),
		}); err != nil {
			log.Printf("Error encoding response: %v", err)
		}
		return
	}

	log.Printf("Generated JWT token for client %s", clientID)

	// Send success response with token
	response := TokenResponse{
		Success:   true,
		Message:   "Token generated successfully",
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Token:     token,
	}

	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("Error encoding response: %v", err)
	}
}

// jwtAuthMiddleware validates JWT token for protected endpoints
func jwtAuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Skip auth for OPTIONS requests, health check, root endpoint, token endpoint, and static files
		if r.Method == "OPTIONS" ||
			r.URL.Path == "/health" ||
			r.URL.Path == "/" ||
			r.URL.Path == "/api/v1/auth/token" ||
			strings.HasPrefix(r.URL.Path, "/static/") {
			next.ServeHTTP(w, r)
			return
		}

		// Extract token from Authorization header
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			if err := json.NewEncoder(w).Encode(ErrorResponse{
				Success: false,
				Error:   "Authorization header is required",
			}); err != nil {
				log.Printf("Error encoding response: %v", err)
			}
			return
		}

		// Check Bearer token format
		const bearerSchema = "Bearer "
		if !strings.HasPrefix(authHeader, bearerSchema) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			if err := json.NewEncoder(w).Encode(ErrorResponse{
				Success: false,
				Error:   "Authorization header must start with 'Bearer '",
			}); err != nil {
				log.Printf("Error encoding response: %v", err)
			}
			return
		}

		tokenString := authHeader[len(bearerSchema):]

		// Validate JWT token
		claims, err := validateJWT(tokenString)
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			if err := json.NewEncoder(w).Encode(ErrorResponse{
				Success: false,
				Error:   "Invalid or expired token: " + err.Error(),
			}); err != nil {
				log.Printf("Error encoding response: %v", err)
			}
			return
		}

		log.Printf("Authenticated request from client: %s", claims.ClientID)
		next.ServeHTTP(w, r)
	})
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
	// Parse command line options
	cmdOptions = ParseCommandLineOptions()
	PrintCommandLineOptions(cmdOptions)

	// Initialize Kubernetes client
	if err := initKubernetesClient(cmdOptions); err != nil {
		log.Printf("Warning: Failed to initialize Kubernetes client: %v", err)
		log.Printf("Pod label information will be based on mock data")
	} else {
		// Start Kubernetes pod watcher for cache invalidation
		if err := StartPodWatcher(strategyCache); err != nil {
			log.Printf("Warning: Failed to start pod watcher: %v", err)
			log.Printf("Cache will not be automatically invalidated on pod changes")
		} else {
			log.Println("Kubernetes pod watcher started successfully")
		}
	}

	// Load configuration
	config, err := LoadConfig(cmdOptions.ConfigPath)
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Initialize JWT
	if err := initJWT(config.JWT); err != nil {
		log.Fatalf("Failed to initialize JWT: %v", err)
	}

	// If port is specified in command line, override the port in config file
	port := config.Server.Port
	if cmdOptions.Port != "" {
		port = cmdOptions.Port
		log.Printf("Using port from command line: %s", port)
	}

	strategiesMutex.Lock()
	if len(config.Strategies.Default) > 0 {
		userStrategies = config.Strategies.Default
		log.Printf("Loaded %d default strategies from config", len(userStrategies))
	}
	strategiesMutex.Unlock()

	// Create router
	r := mux.NewRouter()

	// Apply middleware
	r.Use(loggingMiddleware)
	r.Use(enableCORS)
	r.Use(jwtAuthMiddleware) // Add JWT authentication middleware

	// Define routes
	r.HandleFunc("/api/v1/auth/token", TokenHandler).Methods("POST", "OPTIONS")
	r.HandleFunc("/api/v1/metrics", MetricsHandler).Methods("POST", "OPTIONS")
	r.HandleFunc("/api/v1/metrics", GetMetricsHandler).Methods("GET", "OPTIONS")
	r.HandleFunc("/api/v1/pods/pids", PodPidHandler).Methods("GET", "OPTIONS")
	r.HandleFunc("/api/v1/scheduling/strategies", GetSchedulingStrategiesHandler).Methods("GET", "OPTIONS")
	r.HandleFunc("/api/v1/scheduling/strategies", SaveStrategiesHandler).Methods("POST", "OPTIONS")
	r.HandleFunc("/health", HealthHandler).Methods("GET")

	// Static file server for frontend
	staticFS := http.FileServer(http.Dir("./static/"))
	r.PathPrefix("/static/").Handler(http.StripPrefix("/static/", staticFS)).Methods("GET")

	// Root endpoint redirects to frontend
	r.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			http.Redirect(w, r, "/static/", http.StatusMovedPermanently)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		response := map[string]string{
			"message":   "BSS Metrics API Server",
			"version":   "1.0.0",
			"endpoints": "/api/v1/auth/token (POST), /api/v1/metrics (POST), /api/v1/pods/pids (GET), /api/v1/scheduling/strategies (GET, POST), /health (GET), /static/ (Frontend)",
		}
		if err := json.NewEncoder(w).Encode(response); err != nil {
			log.Printf("Error encoding response: %v", err)
		}
	}).Methods("GET")

	// Server configuration
	serverPort := port
	log.Printf("Starting BSS Metrics API Server on port %s", serverPort)
	log.Printf("Endpoints:")
	log.Printf("  POST /api/v1/auth/token              - Generate JWT token")
	log.Printf("  POST /api/v1/metrics                - Submit metrics data")
	log.Printf("  GET  /api/v1/metrics                - Get current metrics")
	log.Printf("  GET  /api/v1/pods/pids              - Get pod-PID mappings")
	log.Printf("  GET  /api/v1/scheduling/strategies  - Get scheduling strategies")
	log.Printf("  POST /api/v1/scheduling/strategies  - Save scheduling strategies")
	log.Printf("  GET  /health                        - Health check")
	log.Printf("  GET  /static/                       - Frontend web interface")
	log.Printf("  GET  /                              - Redirect to frontend")
	log.Printf("JWT Authentication: Enabled (Token duration: %d hours)", jwtConfig.TokenDuration)

	// Start server
	srv := &http.Server{
		Handler:      r,
		Addr:         serverPort,
		WriteTimeout: time.Duration(config.Server.WriteTimeout) * time.Second,
		ReadTimeout:  time.Duration(config.Server.ReadTimeout) * time.Second,
		IdleTimeout:  time.Duration(config.Server.IdleTimeout) * time.Second,
	}

	log.Fatal(srv.ListenAndServe())
}
