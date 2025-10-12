package main

import (
	"reflect"
	"testing"
	"time"

	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TestStrategyCache_ShouldReturnCachedWhenNoChanges tests that cache returns stored strategies
// when there are no pod changes and no strategy changes
func TestStrategyCache_ShouldReturnCachedWhenNoChanges(t *testing.T) {
	// Arrange
	cache := NewStrategyCache()
	initialPods := []PodInfo{
		{PodUID: "pod1", Processes: []PodProcess{{PID: 100, Command: "test"}}},
		{PodUID: "pod2", Processes: []PodProcess{{PID: 200, Command: "test"}}},
	}
	inputStrategies := []SchedulingStrategy{
		{Priority: true, ExecutionTime: 1000, Selectors: []LabelSelector{{Key: "app", Value: "test"}}},
	}
	initialStrategies := []SchedulingStrategy{
		{Priority: true, ExecutionTime: 1000, PID: 100},
		{Priority: false, ExecutionTime: 2000, PID: 200},
	}

	// Act - Set up cache with initial data
	cache.updatePodSnapshot(initialPods)
	cache.updateStrategySnapshot(inputStrategies)
	cache.setStrategies(initialStrategies)

	// First call should return from cache (cache hit)
	firstResult := cache.getStrategies(initialPods, inputStrategies)

	// Second call with same pods and strategies should also return from cache (another cache hit)
	secondResult := cache.getStrategies(initialPods, inputStrategies)

	// Assert
	if !reflect.DeepEqual(firstResult, secondResult) {
		t.Error("Expected same strategies from cache when pods and strategies haven't changed")
	}

	// Both calls should be cache hits since we set strategies before calling GetStrategies
	if cache.getCacheHits() != 2 {
		t.Errorf("Expected 2 cache hits, got %d", cache.getCacheHits())
	}
}

// TestStrategyCache_ShouldInvalidateOnNewPod tests that cache invalidates when new pod is added
func TestStrategyCache_ShouldInvalidateOnNewPod(t *testing.T) {
	// Arrange
	cache := NewStrategyCache()
	initialPods := []PodInfo{
		{PodUID: "pod1", Processes: []PodProcess{{PID: 100, Command: "test"}}},
	}
	updatedPods := []PodInfo{
		{PodUID: "pod1", Processes: []PodProcess{{PID: 100, Command: "test"}}},
		{PodUID: "pod2", Processes: []PodProcess{{PID: 200, Command: "test"}}}, // New pod
	}
	inputStrategies := []SchedulingStrategy{
		{Priority: true, ExecutionTime: 1000, Selectors: []LabelSelector{{Key: "app", Value: "test"}}},
	}

	// Act
	cache.updatePodSnapshot(initialPods)
	cache.updateStrategySnapshot(inputStrategies)
	cache.setStrategies([]SchedulingStrategy{{Priority: true, ExecutionTime: 1000, PID: 100}})
	_ = cache.getStrategies(initialPods, inputStrategies)

	// Should detect change and invalidate
	hasChanged := cache.hasPodsChanged(updatedPods)

	// Assert
	if !hasChanged {
		t.Error("Expected cache to detect new pod addition")
	}

	if cache.isValid() {
		t.Error("Expected cache to be invalidated after pod change")
	}
}

// TestStrategyCache_ShouldInvalidateOnPodRestart tests cache invalidation on pod restart
func TestStrategyCache_ShouldInvalidateOnPodRestart(t *testing.T) {
	// Arrange
	cache := NewStrategyCache()
	initialPods := []PodInfo{
		{PodUID: "pod1", Processes: []PodProcess{{PID: 100, Command: "test"}}},
	}
	// Same pod UID but different PID (restart scenario)
	restartedPods := []PodInfo{
		{PodUID: "pod1", Processes: []PodProcess{{PID: 150, Command: "test"}}},
	}
	inputStrategies := []SchedulingStrategy{
		{Priority: true, ExecutionTime: 1000, Selectors: []LabelSelector{{Key: "app", Value: "test"}}},
	}

	// Act
	cache.updatePodSnapshot(initialPods)
	cache.updateStrategySnapshot(inputStrategies)
	cache.setStrategies([]SchedulingStrategy{{Priority: true, ExecutionTime: 1000, PID: 100}})
	_ = cache.getStrategies(initialPods, inputStrategies)

	hasChanged := cache.hasPodsChanged(restartedPods)

	// Assert
	if !hasChanged {
		t.Error("Expected cache to detect pod restart (PID change)")
	}

	if cache.isValid() {
		t.Error("Expected cache to be invalidated after pod restart")
	}
}

// TestStrategyCache_ShouldNotInvalidateOnIrrelevantChanges tests that cache stays valid
// when changes don't affect scheduling
func TestStrategyCache_ShouldNotInvalidateOnIrrelevantChanges(t *testing.T) {
	// Arrange
	cache := NewStrategyCache()
	initialPods := []PodInfo{
		{PodUID: "pod1", Processes: []PodProcess{{PID: 100, Command: "test"}}},
		{PodUID: "pod2", Processes: []PodProcess{{PID: 200, Command: "other"}}},
	}
	// Same pods, just different order
	reorderedPods := []PodInfo{
		{PodUID: "pod2", Processes: []PodProcess{{PID: 200, Command: "other"}}},
		{PodUID: "pod1", Processes: []PodProcess{{PID: 100, Command: "test"}}},
	}
	inputStrategies := []SchedulingStrategy{
		{Priority: true, ExecutionTime: 1000, Selectors: []LabelSelector{{Key: "app", Value: "test"}}},
	}

	// Act
	cache.updatePodSnapshot(initialPods)
	cache.updateStrategySnapshot(inputStrategies)
	cache.setStrategies([]SchedulingStrategy{{Priority: true, ExecutionTime: 1000, PID: 100}})
	_ = cache.getStrategies(initialPods, inputStrategies)

	hasChanged := cache.hasPodsChanged(reorderedPods)

	// Assert
	if hasChanged {
		t.Error("Expected cache to remain valid when only order changes")
	}

	if !cache.isValid() {
		t.Error("Expected cache to stay valid for irrelevant changes")
	}
}

// TestStrategyCache_ShouldExpireAfterTTL tests cache expiration
func TestStrategyCache_ShouldExpireAfterTTL(t *testing.T) {
	// Arrange
	cache := NewStrategyCacheWithTTL(100 * time.Millisecond) // Short TTL for testing
	pods := []PodInfo{
		{PodUID: "pod1", Processes: []PodProcess{{PID: 100, Command: "test"}}},
	}
	inputStrategies := []SchedulingStrategy{
		{Priority: true, ExecutionTime: 1000, Selectors: []LabelSelector{{Key: "app", Value: "test"}}},
	}

	// Act
	cache.updatePodSnapshot(pods)
	cache.updateStrategySnapshot(inputStrategies)
	cache.setStrategies([]SchedulingStrategy{{Priority: true, ExecutionTime: 1000, PID: 100}})
	_ = cache.getStrategies(pods, inputStrategies)

	// Wait for TTL to expire
	time.Sleep(150 * time.Millisecond)

	// Assert
	if cache.isValid() {
		t.Error("Expected cache to expire after TTL")
	}
}

// TestStrategyCache_ShouldInvalidateOnStrategyChange tests that cache invalidates when strategies change
func TestStrategyCache_ShouldInvalidateOnStrategyChange(t *testing.T) {
	// Arrange
	cache := NewStrategyCache()
	pods := []PodInfo{
		{PodUID: "pod1", Processes: []PodProcess{{PID: 100, Command: "test"}}},
	}
	initialStrategies := []SchedulingStrategy{
		{Priority: true, ExecutionTime: 1000, Selectors: []LabelSelector{{Key: "app", Value: "test"}}},
	}
	updatedStrategies := []SchedulingStrategy{
		{Priority: false, ExecutionTime: 2000, Selectors: []LabelSelector{{Key: "app", Value: "prod"}}}, // Changed strategy
	}

	// Act
	cache.updatePodSnapshot(pods)
	cache.updateStrategySnapshot(initialStrategies)
	cache.setStrategies([]SchedulingStrategy{{Priority: true, ExecutionTime: 1000, PID: 100}})

	// First call with initial strategies - should hit cache
	firstResult := cache.getStrategies(pods, initialStrategies)
	if firstResult == nil {
		t.Error("Expected cache hit with initial strategies")
	}

	// Second call with changed strategies - should miss cache
	secondResult := cache.getStrategies(pods, updatedStrategies)
	if secondResult != nil {
		t.Error("Expected cache miss when strategies changed")
	}

	// Assert
	if cache.getCacheHits() != 1 {
		t.Errorf("Expected 1 cache hit, got %d", cache.getCacheHits())
	}
	if cache.getCacheMisses() != 1 {
		t.Errorf("Expected 1 cache miss, got %d", cache.getCacheMisses())
	}
}

// TestPodChangeDetector_ComputeFingerprint tests pod fingerprint computation
func TestPodChangeDetector_ComputeFingerprint(t *testing.T) {
	// Arrange
	detector := NewPodChangeDetector()
	pods1 := []PodInfo{
		{PodUID: "pod1", Processes: []PodProcess{{PID: 100, Command: "test"}}},
	}
	pods2 := []PodInfo{
		{PodUID: "pod1", Processes: []PodProcess{{PID: 100, Command: "test"}}},
	}
	pods3 := []PodInfo{
		{PodUID: "pod1", Processes: []PodProcess{{PID: 200, Command: "test"}}}, // Different PID
	}

	// Act
	fingerprint1 := detector.ComputeFingerprint(pods1)
	fingerprint2 := detector.ComputeFingerprint(pods2)
	fingerprint3 := detector.ComputeFingerprint(pods3)

	// Assert
	if fingerprint1 != fingerprint2 {
		t.Error("Expected same fingerprint for identical pod states")
	}

	if fingerprint1 == fingerprint3 {
		t.Error("Expected different fingerprint for different PIDs")
	}
}

// TestKubernetesPodWatcher_ShouldDetectPodEvents tests Kubernetes pod watcher
func TestKubernetesPodWatcher_ShouldDetectPodEvents(t *testing.T) {
	// This test would require mock Kubernetes client
	// For now, we'll define the interface

	// Arrange
	watcher := NewPodWatcher()
	changeDetected := false

	// Register callback for pod changes
	watcher.OnPodChange(func() {
		changeDetected = true
	})

	// Act - Simulate pod event
	watcher.SimulateEvent(PodEvent{
		Type: "ADDED",
		Pod: apiv1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				UID: "new-pod",
			},
		},
	})

	// Assert
	if !changeDetected {
		t.Error("Expected watcher to detect pod addition event")
	}
}

// TestIntegration_CacheWithRealAPI tests the complete flow
func TestIntegration_CacheWithRealAPI(t *testing.T) {
	// This would be an integration test combining all components
	t.Run("Should use cache when no changes", func(t *testing.T) {
		// Test the full flow with cache
	})

	t.Run("Should recalculate on pod changes", func(t *testing.T) {
		// Test cache invalidation and recalculation
	})
}
