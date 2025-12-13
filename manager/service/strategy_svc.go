package service

import (
	"context"
	"fmt"
	"net/http"

	"github.com/Gthulhu/api/manager/domain"
	"github.com/Gthulhu/api/manager/errs"
	"github.com/Gthulhu/api/pkg/logger"
	"github.com/pkg/errors"
	"go.mongodb.org/mongo-driver/v2/bson"
)

func (svc *Service) CreateScheduleStrategy(ctx context.Context, operator *domain.Claims, strategy *domain.ScheduleStrategy) error {
	operatorID, err := operator.GetBsonObjectUID()
	if err != nil {
		return errors.WithMessagef(err, "invalid operator ID %s", operator.UID)
	}
	queryOpt := &domain.QueryPodsOptions{
		K8SNamespace:   strategy.K8sNamespace,
		LabelSelectors: strategy.LabelSelectors,
		CommandRegex:   strategy.CommandRegex,
	}
	pods, err := svc.K8SAdapter.QueryPods(ctx, queryOpt)
	if err != nil {
		return err
	}
	if len(pods) == 0 {
		return errs.NewHTTPStatusError(http.StatusNotFound, "no pods match the strategy criteria", fmt.Errorf("no pods found for the given namespaces and label selectors, opts:%+v", queryOpt))
	}

	logger.Logger(ctx).Debug().Msgf("found %d pods matching the strategy criteria", len(pods))

	strategy.BaseEntity = domain.NewBaseEntity(&operatorID, &operatorID)

	intents := make([]*domain.ScheduleIntent, 0, len(pods))
	nodeIDsMap := make(map[string]struct{})
	nodeIDs := make([]string, 0)
	for _, pod := range pods {
		intent := domain.NewScheduleIntent(strategy, pod)
		intents = append(intents, &intent)
		if _, exists := nodeIDsMap[pod.NodeID]; !exists {
			nodeIDsMap[pod.NodeID] = struct{}{}
			nodeIDs = append(nodeIDs, pod.NodeID)
		}
	}

	err = svc.Repo.InsertStrategyAndIntents(ctx, strategy, intents)
	if err != nil {
		return fmt.Errorf("insert strategy and intents into repository: %w", err)
	}

	dmLabel := domain.LabelSelector{
		Key:   "role",
		Value: "decision-maker",
	}

	dmQueryOpt := &domain.QueryDecisionMakerPodsOptions{
		DecisionMakerLabel: dmLabel,
		NodeIDs:            nodeIDs,
	}
	dms, err := svc.K8SAdapter.QueryDecisionMakerPods(ctx, dmQueryOpt)
	if err != nil {
		return err
	}
	if len(dms) == 0 {
		logger.Logger(ctx).Warn().Msgf("no decision maker pods found for scheduling intents, opts:%+v", dmQueryOpt)
		return nil
	}

	logger.Logger(ctx).Debug().Msgf("found %d decision maker pods for scheduling intents", len(dms))

	nodeIDIntentsMap := make(map[string][]*domain.ScheduleIntent)
	nodeIDIntentIDsMap := make(map[string][]bson.ObjectID)
	nodeIDDMap := make(map[string]*domain.DecisionMakerPod)
	for _, dmPod := range dms {
		for _, intent := range intents {
			if intent.NodeID == dmPod.NodeID {
				nodeIDIntentIDsMap[dmPod.Host] = append(nodeIDIntentIDsMap[dmPod.Host], intent.ID)
				nodeIDIntentsMap[dmPod.Host] = append(nodeIDIntentsMap[dmPod.Host], intent)
				nodeIDDMap[dmPod.Host] = dmPod
			}
		}
	}
	for host, intents := range nodeIDIntentsMap {
		dmPod := nodeIDDMap[host]
		err = svc.DMAdapter.SendSchedulingIntent(ctx, dmPod, intents)
		if err != nil {
			return fmt.Errorf("send scheduling intents to decision maker %s: %w", host, err)
		}
		err = svc.Repo.BatchUpdateIntentsState(ctx, nodeIDIntentIDsMap[host], domain.IntentStateSent)
		if err != nil {
			return fmt.Errorf("insert strategy and intents into repository: %w", err)
		}
		logger.Logger(ctx).Info().Msgf("sent %d scheduling intents to decision maker %s", len(intents), host)
	}
	return nil
}

func (svc *Service) ListScheduleStrategies(ctx context.Context, filterOpts *domain.QueryStrategyOptions) error {
	return svc.Repo.QueryStrategies(ctx, filterOpts)
}

func (svc *Service) ListScheduleIntents(ctx context.Context, filterOpts *domain.QueryIntentOptions) error {
	return svc.Repo.QueryIntents(ctx, filterOpts)
}
