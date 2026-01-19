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
		Key:   "app",
		Value: "decisionmaker",
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

func (svc *Service) DeleteScheduleStrategy(ctx context.Context, operator *domain.Claims, strategyID string) error {
	strategyObjID, err := bson.ObjectIDFromHex(strategyID)
	if err != nil {
		return errors.WithMessagef(err, "invalid strategy ID %s", strategyID)
	}

	operatorID, err := operator.GetBsonObjectUID()
	if err != nil {
		return errors.WithMessagef(err, "invalid operator ID %s", operator.UID)
	}

	// Check if strategy exists and belongs to the operator
	queryOpt := &domain.QueryStrategyOptions{
		IDs:        []bson.ObjectID{strategyObjID},
		CreatorIDs: []bson.ObjectID{operatorID},
	}
	err = svc.Repo.QueryStrategies(ctx, queryOpt)
	if err != nil {
		return err
	}
	if len(queryOpt.Result) == 0 {
		return errs.NewHTTPStatusError(http.StatusNotFound, "strategy not found or you don't have permission to delete it", nil)
	}

	// Delete associated intents first
	err = svc.Repo.DeleteIntentsByStrategyID(ctx, strategyObjID)
	if err != nil {
		return fmt.Errorf("delete intents by strategy ID: %w", err)
	}

	// Delete the strategy
	err = svc.Repo.DeleteStrategy(ctx, strategyObjID)
	if err != nil {
		return fmt.Errorf("delete strategy: %w", err)
	}

	logger.Logger(ctx).Info().Msgf("deleted strategy %s and its associated intents", strategyID)
	return nil
}

func (svc *Service) DeleteScheduleIntents(ctx context.Context, operator *domain.Claims, intentIDs []string) error {
	if len(intentIDs) == 0 {
		return nil
	}

	operatorID, err := operator.GetBsonObjectUID()
	if err != nil {
		return errors.WithMessagef(err, "invalid operator ID %s", operator.UID)
	}

	intentObjIDs := make([]bson.ObjectID, 0, len(intentIDs))
	for _, id := range intentIDs {
		objID, err := bson.ObjectIDFromHex(id)
		if err != nil {
			return errors.WithMessagef(err, "invalid intent ID %s", id)
		}
		intentObjIDs = append(intentObjIDs, objID)
	}

	// Check if intents exist and belong to the operator
	queryOpt := &domain.QueryIntentOptions{
		IDs:        intentObjIDs,
		CreatorIDs: []bson.ObjectID{operatorID},
	}
	err = svc.Repo.QueryIntents(ctx, queryOpt)
	if err != nil {
		return err
	}
	if len(queryOpt.Result) != len(intentIDs) {
		return errs.NewHTTPStatusError(http.StatusNotFound, "one or more intents not found or you don't have permission to delete them", nil)
	}

	// Delete the intents
	err = svc.Repo.DeleteIntents(ctx, intentObjIDs)
	if err != nil {
		return fmt.Errorf("delete intents: %w", err)
	}

	logger.Logger(ctx).Info().Msgf("deleted %d intents", len(intentIDs))
	return nil
}
