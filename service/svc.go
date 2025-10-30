package service

import (
	"context"
	"crypto/rsa"

	"github.com/Gthulhu/api/adapter/kubernetes"
	"github.com/Gthulhu/api/cache"
	"github.com/Gthulhu/api/config"
	"github.com/Gthulhu/api/domain"
)

// Params holds the parameters for creating a new Service
type Params struct {
	K8sAdapter    kubernetes.K8sAdapter
	StrategyCache *cache.StrategyCache
	JWTPrivateKey *rsa.PrivateKey
	Config        *config.Config
}

// NewService creates a new Service instance
func NewService(ctx context.Context, params Params) (*Service, error) {
	svc := &Service{
		K8sAdapter:    params.K8sAdapter,
		StrategyCache: params.StrategyCache,
		jwtPrivateKey: params.JWTPrivateKey,
		config:        params.Config,
	}

	if len(params.Config.Strategies.Default) > 0 {
		defaultStrategies := []*domain.SchedulingStrategy{}
		for _, strat := range params.Config.Strategies.Default {
			ds := domain.SchedulingStrategy{
				Priority:      strat.Priority,
				ExecutionTime: strat.ExecutionTime,
				PID:           strat.PID,
				CommandRegex:  strat.CommandRegex,
				Selectors:     []domain.LabelSelector{},
			}
			for _, sel := range strat.Selectors {
				ds.Selectors = append(ds.Selectors, domain.LabelSelector{
					Key:   sel.Key,
					Value: sel.Value,
				})
			}
			defaultStrategies = append(defaultStrategies, &ds)
		}
		err := svc.SaveSchedulingStrategy(context.Background(), defaultStrategies)
		if err != nil {
			return nil, err
		}
	}

	return svc, nil
}

// Service represents the main service structure
type Service struct {
	kubernetes.K8sAdapter
	*cache.StrategyCache
	jwtPrivateKey *rsa.PrivateKey
	config        *config.Config
}
