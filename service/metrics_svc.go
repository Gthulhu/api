package service

import (
	"context"
	"errors"
	"sync/atomic"

	"github.com/Gthulhu/api/domain"
)

var (
	latestBssData atomic.Value
	ErrNoBssData  = errors.New("no BSS metrics data available")
)

// SaveBSSMetrics saves the provided BSS metrics data
func (svc *Service) SaveBSSMetrics(ctx context.Context, bssMetrics *domain.BssData) error {
	latestBssData.Store(bssMetrics)
	return nil
}

// GetBSSMetrics retrieves the latest BSS metrics data
func (svc *Service) GetBSSMetrics(ctx context.Context) (*domain.BssData, error) {
	data := latestBssData.Load()
	if data != nil {
		bssData, ok := data.(*domain.BssData)
		if ok {
			return bssData, nil
		}
	}
	return &domain.BssData{}, ErrNoBssData
}
