package domain

import (
	"context"
)

// Service defines the interface for the service layer
type Service interface {
	// VerifyAndGenerateToken verifies the provided public key and generates a JWT token if valid
	VerifyAndGenerateToken(ctx context.Context, publicKey string) (string, error)
	// GetAllPodInfos retrieves all pod information by scanning the /proc filesystem
	GetAllPodInfos(ctx context.Context) ([]*PodInfo, error)
	// SaveBSSMetrics saves the provided BSS metrics data
	SaveBSSMetrics(ctx context.Context, bssMetrics *BssData) error
	// GetBSSMetrics retrieves the latest BSS metrics data
	GetBSSMetrics(ctx context.Context) (*BssData, error)
	// SaveSchedulingStrategy saves the provided scheduling strategies
	SaveSchedulingStrategy(ctx context.Context, strategy []*SchedulingStrategy) error
	// FindCurrentUsingSchedulingStrategiesWithPID finds the current scheduling strategies being used and their associated PIDs
	FindCurrentUsingSchedulingStrategiesWithPID(ctx context.Context) ([]*SchedulingStrategy, bool, error)
	// GetStrategyCacheStats returns statistics about the strategy cache
	GetStrategyCacheStats() map[string]any
}
