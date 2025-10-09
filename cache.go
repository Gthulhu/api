package main

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"log"
	"sort"
	"sync"
	"time"

	apiv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/watch"
)

// StrategyCache manages caching of scheduling strategies
type StrategyCache struct {
	mu               sync.RWMutex
	cachedStrategies []SchedulingStrategy
	podFingerprint   string
	lastUpdate       time.Time
	ttl              time.Duration
	valid            bool
	cacheHits        int
	cacheMisses      int
}

// NewStrategyCache creates a new strategy cache with default TTL
func NewStrategyCache() *StrategyCache {
	return &StrategyCache{
		ttl:   5 * time.Minute, // Default TTL
		valid: false,
	}
}

// NewStrategyCacheWithTTL creates a cache with custom TTL
func NewStrategyCacheWithTTL(ttl time.Duration) *StrategyCache {
	return &StrategyCache{
		ttl:   ttl,
		valid: false,
	}
}

// UpdatePodSnapshot updates the pod fingerprint
func (c *StrategyCache) UpdatePodSnapshot(pods []PodInfo) {
	c.mu.Lock()
	defer c.mu.Unlock()

	detector := NewPodChangeDetector()
	c.podFingerprint = detector.ComputeFingerprint(pods)
}

// SetStrategies stores strategies in cache
func (c *StrategyCache) SetStrategies(strategies []SchedulingStrategy) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.cachedStrategies = strategies
	c.lastUpdate = time.Now()
	c.valid = true
}

// GetStrategies returns cached strategies if valid, otherwise returns nil
func (c *StrategyCache) GetStrategies(currentPods []PodInfo) []SchedulingStrategy {
	c.mu.RLock()

	// Check if cache is expired
	if time.Since(c.lastUpdate) > c.ttl {
		c.mu.RUnlock()
		c.mu.Lock()
		defer c.mu.Unlock()
		c.valid = false
		c.cacheMisses++
		return nil
	}

	// Check if pods have changed
	detector := NewPodChangeDetector()
	currentFingerprint := detector.ComputeFingerprint(currentPods)

	if currentFingerprint != c.podFingerprint {
		c.mu.RUnlock()
		c.mu.Lock()
		defer c.mu.Unlock()
		c.valid = false
		c.cacheMisses++
		return nil
	}

	if c.valid && len(c.cachedStrategies) > 0 {
		// Cache hit - increment counter and return copy
		cachedStrategies := make([]SchedulingStrategy, len(c.cachedStrategies))
		copy(cachedStrategies, c.cachedStrategies)

		c.mu.RUnlock()
		c.mu.Lock()
		c.cacheHits++
		c.mu.Unlock()

		return cachedStrategies
	}

	c.mu.RUnlock()
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cacheMisses++
	return nil
}

// HasPodsChanged checks if pods have changed since last snapshot
func (c *StrategyCache) HasPodsChanged(pods []PodInfo) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	detector := NewPodChangeDetector()
	currentFingerprint := detector.ComputeFingerprint(pods)
	changed := currentFingerprint != c.podFingerprint

	if changed {
		// Invalidate cache if pods have changed
		c.mu.RUnlock()
		c.mu.Lock()
		c.valid = false
		c.mu.Unlock()
		c.mu.RLock()
	}

	return changed
}

// IsValid returns whether cache is valid
func (c *StrategyCache) IsValid() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// Check TTL
	if time.Since(c.lastUpdate) > c.ttl {
		return false
	}

	return c.valid
}

// Invalidate marks cache as invalid
func (c *StrategyCache) Invalidate() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.valid = false
}

// GetCacheHits returns number of cache hits
func (c *StrategyCache) GetCacheHits() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.cacheHits
}

// GetCacheMisses returns number of cache misses
func (c *StrategyCache) GetCacheMisses() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.cacheMisses
}

// GetStats returns cache statistics
func (c *StrategyCache) GetStats() map[string]interface{} {
	c.mu.RLock()
	defer c.mu.RUnlock()

	hitRate := float64(0)
	if c.cacheHits+c.cacheMisses > 0 {
		hitRate = float64(c.cacheHits) / float64(c.cacheHits+c.cacheMisses) * 100
	}

	return map[string]interface{}{
		"hits":        c.cacheHits,
		"misses":      c.cacheMisses,
		"hit_rate":    fmt.Sprintf("%.2f%%", hitRate),
		"valid":       c.valid,
		"last_update": c.lastUpdate.Format(time.RFC3339),
		"ttl_seconds": c.ttl.Seconds(),
	}
}

// PodChangeDetector computes fingerprints for pod states
type PodChangeDetector struct{}

// NewPodChangeDetector creates a new pod change detector
func NewPodChangeDetector() *PodChangeDetector {
	return &PodChangeDetector{}
}

// PodFingerprint represents essential pod information for change detection
type PodFingerprint struct {
	UID       string
	Processes []ProcessFingerprint
}

// ProcessFingerprint represents essential process information
type ProcessFingerprint struct {
	PID     int
	Command string
}

// ComputeFingerprint generates a unique fingerprint for pod state
func (d *PodChangeDetector) ComputeFingerprint(pods []PodInfo) string {
	// Create a deterministic representation
	fingerprints := make([]PodFingerprint, len(pods))

	for i, pod := range pods {
		processes := make([]ProcessFingerprint, len(pod.Processes))
		for j, proc := range pod.Processes {
			processes[j] = ProcessFingerprint{
				PID:     proc.PID,
				Command: proc.Command,
			}
		}

		// Sort processes by PID for consistency
		sort.Slice(processes, func(i, j int) bool {
			return processes[i].PID < processes[j].PID
		})

		fingerprints[i] = PodFingerprint{
			UID:       pod.PodUID,
			Processes: processes,
		}
	}

	// Sort pods by UID for consistency
	sort.Slice(fingerprints, func(i, j int) bool {
		return fingerprints[i].UID < fingerprints[j].UID
	})

	// Compute hash
	data, _ := json.Marshal(fingerprints)
	hash := sha256.Sum256(data)
	return fmt.Sprintf("%x", hash)
}

// PodEvent represents a Kubernetes pod event
type PodEvent struct {
	Type string
	Pod  apiv1.Pod
}

// PodWatcher watches for Kubernetes pod changes
type PodWatcher struct {
	mu              sync.RWMutex
	changeCallbacks []func()
	stopChan        chan struct{}
	running         bool
}

// NewPodWatcher creates a new pod watcher
func NewPodWatcher() *PodWatcher {
	return &PodWatcher{
		changeCallbacks: make([]func(), 0),
		stopChan:        make(chan struct{}),
	}
}

// OnPodChange registers a callback for pod changes
func (w *PodWatcher) OnPodChange(callback func()) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.changeCallbacks = append(w.changeCallbacks, callback)
}

// SimulateEvent simulates a pod event (for testing)
func (w *PodWatcher) SimulateEvent(event PodEvent) {
	w.notifyCallbacks()
}

// notifyCallbacks calls all registered callbacks
func (w *PodWatcher) notifyCallbacks() {
	w.mu.RLock()
	callbacks := make([]func(), len(w.changeCallbacks))
	copy(callbacks, w.changeCallbacks)
	w.mu.RUnlock()

	for _, callback := range callbacks {
		callback()
	}
}

// Start begins watching for pod changes
func (w *PodWatcher) Start() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.running {
		return fmt.Errorf("watcher already running")
	}

	w.running = true

	// In production, this would use Kubernetes watch API
	go w.watchLoop()

	return nil
}

// Stop stops the watcher
func (w *PodWatcher) Stop() {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.running {
		close(w.stopChan)
		w.running = false
	}
}

// watchLoop is the main watch loop
func (w *PodWatcher) watchLoop() {
	// In production, this would set up Kubernetes watch
	for {
		select {
		case <-w.stopChan:
			return
		default:
			// Would process Kubernetes events here
		}
	}
}

// WatchKubernetesPods watches Kubernetes pods for changes
func WatchKubernetesPods(watcher watch.Interface, cache *StrategyCache) {
	for event := range watcher.ResultChan() {
		switch event.Type {
		case watch.Added, watch.Modified, watch.Deleted:
			// Pod state has changed, invalidate cache
			cache.Invalidate()
			log.Printf("Pod event detected: %v, cache invalidated", event.Type)
		}
	}
}

// Global cache instance
var strategyCache = NewStrategyCache()

// GetCachedStrategies returns cached strategies or recalculates if needed
func GetCachedStrategies(userStrategies []SchedulingStrategy) ([]SchedulingStrategy, bool) {
	// Get current pod state
	pods, err := getPodPidMapping()
	if err != nil {
		log.Printf("Error getting pod mappings: %v", err)
		return nil, false
	}

	// Try to get from cache
	cachedStrategies := strategyCache.GetStrategies(pods)
	if cachedStrategies != nil {
		log.Printf("Cache hit! Returning cached strategies. Stats: %v", strategyCache.GetStats())
		return cachedStrategies, true
	}

	// Cache miss - need to recalculate
	log.Printf("Cache miss. Recalculating strategies. Stats: %v", strategyCache.GetStats())

	// Recalculate strategies
	var finalStrategies []SchedulingStrategy
	for _, strategy := range userStrategies {
		if len(strategy.Selectors) > 0 {
			matchedPIDs, err := findPIDsByStrategy(strategy)
			if err != nil {
				log.Printf("Error finding PIDs for strategy: %v", err)
				continue
			}

			for _, pid := range matchedPIDs {
				finalStrategies = append(finalStrategies, SchedulingStrategy{
					Priority:      strategy.Priority,
					ExecutionTime: strategy.ExecutionTime,
					PID:           pid,
				})
			}
		} else if strategy.PID != 0 {
			finalStrategies = append(finalStrategies, strategy)
		}
	}

	// Update cache
	strategyCache.UpdatePodSnapshot(pods)
	strategyCache.SetStrategies(finalStrategies)

	return finalStrategies, false
}