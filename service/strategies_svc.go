package service

import (
	"context"
	"fmt"
	"regexp"
	"sync/atomic"

	"github.com/Gthulhu/api/cache"
	"github.com/Gthulhu/api/domain"
	"github.com/Gthulhu/api/util"
)

var (
	latestSchedulingStrategyData atomic.Value
)

// SaveSchedulingStrategy saves the provided scheduling strategies and invalidates the cache
func (svc *Service) SaveSchedulingStrategy(ctx context.Context, strategy []*domain.SchedulingStrategy) error {
	latestSchedulingStrategyData.Store(strategy)
	svc.StrategyCache.Invalidate()
	return nil
}

// FindCurrentUsingSchedulingStrategies finds the current scheduling strategies being used
func (svc *Service) FindCurrentUsingSchedulingStrategiesWithPID(ctx context.Context) ([]*domain.SchedulingStrategy, bool, error) {
	data := latestSchedulingStrategyData.Load()
	if data != nil {
		strategies, ok := data.([]*domain.SchedulingStrategy)
		if ok {
			return svc.FindSchedulingStrategiesWithPID(ctx, procDir, strategies)
		}
	}

	return []*domain.SchedulingStrategy{}, false, nil
}

// GetStrategyCacheStats returns statistics about the strategy cache
func (svc *Service) GetStrategyCacheStats() map[string]any {
	stats := svc.StrategyCache.GetStats()
	return stats
}

// FindSchedulingStrategiesWithPID finds scheduling strategies with associated PIDs
func (svc *Service) FindSchedulingStrategiesWithPID(ctx context.Context, rootDir string, usingStrategies []*domain.SchedulingStrategy) ([]*domain.SchedulingStrategy, bool, error) {
	cachedStrategies := svc.StrategyCache.GetStrategiesQuick(usingStrategies)
	if cachedStrategies != nil {
		return cachedStrategies, true, nil
	}

	// Recalculate strategies
	pods, err := svc.FindPodInfoFrom(ctx, rootDir)
	if err != nil {
		return nil, false, fmt.Errorf("failed to get pod-pid mappings: %v", err)
	}

	var finalStrategies []*domain.SchedulingStrategy
	for _, strategy := range usingStrategies {
		if len(strategy.Selectors) > 0 {
			matchedPIDs, err := svc.findPIDsByStrategy(ctx, pods, strategy)
			if err != nil {
				util.GetLogger().Error("Error finding PIDs for strategy", util.LogErrAttr(err))
				continue
			}

			for _, pid := range matchedPIDs {
				finalStrategies = append(finalStrategies, &domain.SchedulingStrategy{
					Priority:      strategy.Priority,
					ExecutionTime: strategy.ExecutionTime,
					Selectors:     strategy.Selectors,
					PID:           pid,
				})
			}
		} else if strategy.PID != 0 {
			finalStrategies = append(finalStrategies, strategy)
		}
	}
	// Update cache with both pod and strategy snapshots
	svc.StrategyCache.UpdatePodSnapshot(pods)
	svc.StrategyCache.UpdateStrategySnapshot(usingStrategies)
	svc.StrategyCache.SetStrategies(finalStrategies)
	return finalStrategies, false, nil
}

// findPIDsByStrategy finds PIDs that match the given scheduling strategy
func (svc *Service) findPIDsByStrategy(ctx context.Context, pods []*domain.PodInfo, strategy *domain.SchedulingStrategy) ([]int, error) {
	var matchedPIDs []int

	// Set default regex if empty
	if strategy.CommandRegex == "" {
		strategy.CommandRegex = ".*"
	}

	// Compile regex and add to cache
	compiledRegex, err := regexp.Compile(strategy.CommandRegex)
	if err != nil {
		return nil, fmt.Errorf("invalid regex pattern '%s': %v", strategy.CommandRegex, err)
	}

	for _, pod := range pods {
		podSpec, ok := cache.GetKubernetesPod(pod.PodUID)
		if !ok {
			podSpecTemp, err := svc.K8sAdapter.GetPodByPodUID(ctx, pod.PodUID)
			if err != nil {
				return nil, err
			}
			podSpec = podSpecTemp
			cache.SetKubernetesPodCache(pod.PodUID, podSpec)
		}
		labels := podSpec.Labels
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
				if compiledRegex.MatchString(process.Command) {
					matchedPIDs = append(matchedPIDs, process.PID)
				}
			}
		}
	}

	return matchedPIDs, nil
}
