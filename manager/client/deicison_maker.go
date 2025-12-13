package client

import (
	"context"
	"errors"
	"net/http"

	"github.com/Gthulhu/api/manager/domain"
)

func NewDecisionMakerClient() domain.DecisionMakerAdapter {
	return &DecisionMakerClient{}
}

type DecisionMakerClient struct {
	http.Client
}

func (dm DecisionMakerClient) SendSchedulingIntent(ctx context.Context, decisionMaker *domain.DecisionMakerPod, intents []*domain.ScheduleIntent) error {
	// TODO: Implementation of sending scheduling intents to the decision maker pod
	return errors.New("not implemented")
}
