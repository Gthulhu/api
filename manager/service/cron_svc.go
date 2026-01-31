package service

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/Gthulhu/api/manager/domain"
	"github.com/Gthulhu/api/pkg/logger"
	"github.com/Gthulhu/api/pkg/util"
)

func (svc *Service) CheckDMIntents(ctx context.Context) error {
	if svc.K8SAdapter == nil {
		return domain.ErrNoClient
	}

	dmLabel := domain.LabelSelector{
		Key:   "app",
		Value: "decisionmaker",
	}
	dmQueryOpt := &domain.QueryDecisionMakerPodsOptions{
		DecisionMakerLabel: dmLabel,
	}
	dms, err := svc.K8SAdapter.QueryDecisionMakerPods(ctx, dmQueryOpt)
	if err != nil {
		return err
	}
	if len(dms) == 0 {
		logger.Logger(ctx).Warn().Msg("no decision maker pods found for intents check")
		return nil
	}

	queryOpt := &domain.QueryIntentOptions{}
	if err := svc.Repo.QueryIntents(ctx, queryOpt); err != nil {
		return err
	}

	leafHashes := make([]string, 0, len(queryOpt.Result))
	sortedIntents := sortScheduleIntentsByKey(queryOpt.Result)
	for _, intent := range sortedIntents {
		leafHashes = append(leafHashes, hashScheduleIntent(intent))
	}
	root := util.BuildMerkleTree(leafHashes)
	expectedRoot := ""
	if root != nil {
		expectedRoot = root.Hash
	}

	for _, dm := range dms {
		if dm.State != domain.NodeStateOnline {
			continue
		}
		if svc.DMAdapter == nil {
			return fmt.Errorf("decision maker adapter is nil")
		}
		rootHash, err := svc.DMAdapter.GetIntentMerkleRoot(ctx, dm)
		if err != nil {
			logger.Logger(ctx).Warn().Err(err).Msgf("failed to get merkle root from dm %s", dm)
			continue
		}
		if rootHash != expectedRoot {
			logger.Logger(ctx).Warn().Msgf("intent merkle mismatch for dm %s: expected=%s actual=%s", dm, expectedRoot, rootHash)
		}
	}
	return nil
}

func sortScheduleIntentsByKey(intents []*domain.ScheduleIntent) []*domain.ScheduleIntent {
	results := make([]*domain.ScheduleIntent, 0, len(intents))
	results = append(results, intents...)
	sort.Slice(results, func(i, j int) bool {
		return scheduleIntentSortKey(results[i]) < scheduleIntentSortKey(results[j])
	})
	return results
}

func scheduleIntentSortKey(intent *domain.ScheduleIntent) string {
	labels := make([]string, 0, len(intent.PodLabels))
	for key, value := range intent.PodLabels {
		labels = append(labels, key+"="+value)
	}
	sort.Strings(labels)
	return strings.Join([]string{
		intent.PodName,
		intent.PodID,
		intent.NodeID,
		intent.K8sNamespace,
		intent.CommandRegex,
		strconv.Itoa(intent.Priority),
		strconv.FormatInt(intent.ExecutionTime, 10),
		strings.Join(labels, ","),
	}, "|")
}

func hashScheduleIntent(intent *domain.ScheduleIntent) string {
	return util.HashStringSHA256Hex(scheduleIntentSortKey(intent))
}
