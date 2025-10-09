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
	mu                  sync.RWMutex
	cachedStrategies    []SchedulingStrategy
	podFingerprint      string
	strategyFingerprint string
	lastUpdate          time.Time
	ttl                 time.Duration
	valid               bool
	cacheHits           int
	cacheMisses         int
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

// UpdateStrategySnapshot updates the strategy fingerprint
func (c *StrategyCache) UpdateStrategySnapshot(strategies []SchedulingStrategy) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.strategyFingerprint = ComputeStrategyFingerprint(strategies)
}

// SetStrategies stores strategies in cache
func (c *StrategyCache) SetStrategies(strategies []SchedulingStrategy) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.cachedStrategies = strategies
	c.lastUpdate = time.Now()
	c.valid = true
}

// GetStrategiesQuick returns cached strategies without checking pod state
// Relies on Kubernetes Watch to invalidate cache when pods change
func (c *StrategyCache) GetStrategiesQuick(inputStrategies []SchedulingStrategy) []SchedulingStrategy {
	c.mu.RLock()

	// Quick validation checks
	cacheValid := c.valid && len(c.cachedStrategies) > 0
	if cacheValid && time.Since(c.lastUpdate) > c.ttl {
		cacheValid = false
	}

	// Check strategy fingerprint
	if cacheValid {
		currentStrategyFingerprint := ComputeStrategyFingerprint(inputStrategies)
		if currentStrategyFingerprint != c.strategyFingerprint {
			cacheValid = false
		}
	}

	// Return cached copy if valid
	if cacheValid {
		cachedStrategies := make([]SchedulingStrategy, len(c.cachedStrategies))
		copy(cachedStrategies, c.cachedStrategies)
		c.mu.RUnlock()

		// Update hit counter
		c.mu.Lock()
		c.cacheHits++
		c.mu.Unlock()

		return cachedStrategies
	}

	c.mu.RUnlock()

	// Cache miss
	c.mu.Lock()
	c.cacheMisses++
	c.mu.Unlock()

	return nil
}

// GetStrategies returns cached strategies if valid, otherwise returns nil
// This version still checks pod fingerprint for backward compatibility
func (c *StrategyCache) GetStrategies(currentPods []PodInfo, inputStrategies []SchedulingStrategy) []SchedulingStrategy {
	// First, do a quick read-only check
	c.mu.RLock()
	cacheValid := c.valid && len(c.cachedStrategies) > 0
	if cacheValid {
		// Check if cache is expired
		if time.Since(c.lastUpdate) > c.ttl {
			cacheValid = false
		}
	}

	// If valid, check pod fingerprint
	var currentPodFingerprint string
	if cacheValid {
		detector := NewPodChangeDetector()
		currentPodFingerprint = detector.ComputeFingerprint(currentPods)
		if currentPodFingerprint != c.podFingerprint {
			cacheValid = false
		}
	}

	// If still valid, check strategy fingerprint
	var currentStrategyFingerprint string
	if cacheValid {
		currentStrategyFingerprint = ComputeStrategyFingerprint(inputStrategies)
		if currentStrategyFingerprint != c.strategyFingerprint {
			cacheValid = false
		}
	}

	// If still valid, return cached copy
	if cacheValid {
		cachedStrategies := make([]SchedulingStrategy, len(c.cachedStrategies))
		copy(cachedStrategies, c.cachedStrategies)
		c.mu.RUnlock()

		// Update hit counter with separate lock
		c.mu.Lock()
		c.cacheHits++
		c.mu.Unlock()

		return cachedStrategies
	}

	// Cache miss - release read lock and acquire write lock
	c.mu.RUnlock()

	c.mu.Lock()
	defer c.mu.Unlock()

	// Double-check validity after acquiring write lock
	// (another goroutine might have updated cache)
	if c.valid && len(c.cachedStrategies) > 0 {
		if time.Since(c.lastUpdate) <= c.ttl {
			detector := NewPodChangeDetector()
			currentPodFingerprint = detector.ComputeFingerprint(currentPods)
			currentStrategyFingerprint = ComputeStrategyFingerprint(inputStrategies)
			if currentPodFingerprint == c.podFingerprint && currentStrategyFingerprint == c.strategyFingerprint {
				// Cache became valid while we were waiting for lock
				cachedStrategies := make([]SchedulingStrategy, len(c.cachedStrategies))
				copy(cachedStrategies, c.cachedStrategies)
				c.cacheHits++
				return cachedStrategies
			}
		}
	}

	// Definitely a miss
	c.valid = false
	c.cacheMisses++
	return nil
}

// HasPodsChanged checks if pods have changed since last snapshot
func (c *StrategyCache) HasPodsChanged(pods []PodInfo) bool {
	c.mu.RLock()
	detector := NewPodChangeDetector()
	currentFingerprint := detector.ComputeFingerprint(pods)
	lastFingerprint := c.podFingerprint
	c.mu.RUnlock()

	changed := currentFingerprint != lastFingerprint

	if changed {
		// Invalidate cache if pods have changed
		c.mu.Lock()
		c.valid = false
		c.mu.Unlock()
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

// ComputeStrategyFingerprint generates a unique fingerprint for scheduling strategies
// This excludes PID field as PIDs are calculated, not part of input strategy
func ComputeStrategyFingerprint(strategies []SchedulingStrategy) string {
	// Create a deterministic representation excluding calculated PIDs
	type StrategyKey struct {
		Priority      bool
		ExecutionTime uint64
		Selectors     []LabelSelector
		CommandRegex  string
	}

	keys := make([]StrategyKey, len(strategies))
	for i, s := range strategies {
		// Sort selectors for consistency
		selectors := make([]LabelSelector, len(s.Selectors))
		copy(selectors, s.Selectors)
		sort.Slice(selectors, func(i, j int) bool {
			if selectors[i].Key != selectors[j].Key {
				return selectors[i].Key < selectors[j].Key
			}
			return selectors[i].Value < selectors[j].Value
		})

		keys[i] = StrategyKey{
			Priority:      s.Priority,
			ExecutionTime: s.ExecutionTime,
			Selectors:     selectors,
			CommandRegex:  s.CommandRegex,
		}
	}

	// Sort strategies for consistency
	sort.Slice(keys, func(i, j int) bool {
		// First by priority
		if keys[i].Priority != keys[j].Priority {
			return keys[i].Priority
		}
		// Then by execution time
		if keys[i].ExecutionTime != keys[j].ExecutionTime {
			return keys[i].ExecutionTime < keys[j].ExecutionTime
		}
		// Then by command regex
		return keys[i].CommandRegex < keys[j].CommandRegex
	})

	// Compute hash
	data, _ := json.Marshal(keys)
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
// This optimized version avoids calling getPodPidMapping() on cache hits
// Pod changes are detected by Kubernetes Watch mechanism
func GetCachedStrategies(userStrategies []SchedulingStrategy) ([]SchedulingStrategy, bool) {
	// Try to get from cache first (no expensive pod scanning)
	cachedStrategies := strategyCache.GetStrategiesQuick(userStrategies)
	if cachedStrategies != nil {
		log.Printf("Cache hit! Returning cached strategies. Stats: %v", strategyCache.GetStats())
		return cachedStrategies, true
	}

	// Cache miss - need to recalculate
	log.Printf("Cache miss. Recalculating strategies. Stats: %v", strategyCache.GetStats())

	// Now get current pod state (only on cache miss)
	pods, err := getPodPidMapping()
	if err != nil {
		log.Printf("Error getting pod mappings: %v", err)
		return nil, false
	}

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

	// Update cache with both pod and strategy snapshots
	strategyCache.UpdatePodSnapshot(pods)
	strategyCache.UpdateStrategySnapshot(userStrategies)
	strategyCache.SetStrategies(finalStrategies)

	return finalStrategies, false
}
