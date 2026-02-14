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
	expectedRootsByNode := buildExpectedIntentRootsByNode(queryOpt.Result)
	emptyRootHash := util.BuildMerkleTree(nil).Hash

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
		expectedRoot := expectedRootsByNode[dm.NodeID]
		if expectedRoot == "" {
			expectedRoot = emptyRootHash
		}
		if rootHash != expectedRoot {
			logger.Logger(ctx).Warn().Msgf("intent merkle mismatch for dm %s: expected=%s actual=%s", dm, expectedRoot, rootHash)
		}
	}
	return nil
}

func sortScheduleIntentsByKey(intents []*domain.ScheduleIntent) []*domain.ScheduleIntent {
	normalized := normalizeScheduleIntents(intents)
	results := make([]*domain.ScheduleIntent, 0, len(normalized))
	results = append(results, normalized...)
	sort.Slice(results, func(i, j int) bool {
		return scheduleIntentSortKey(results[i]) < scheduleIntentSortKey(results[j])
	})
	return results
}

func normalizeScheduleIntents(intents []*domain.ScheduleIntent) []*domain.ScheduleIntent {
	results := make([]*domain.ScheduleIntent, 0, len(intents))
	for _, intent := range intents {
		if intent == nil {
			continue
		}
		results = append(results, intent)
	}
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
	labels := make([]string, 0, len(intent.PodLabels))
	for key, value := range intent.PodLabels {
		labels = append(labels, key+"="+value)
	}
	sort.Strings(labels)
	serialized := strings.Join([]string{
		"podName=" + intent.PodName,
		"podID=" + intent.PodID,
		"nodeID=" + intent.NodeID,
		"k8sNamespace=" + intent.K8sNamespace,
		"commandRegex=" + intent.CommandRegex,
		"priority=" + strconv.Itoa(intent.Priority),
		"executionTime=" + strconv.FormatInt(intent.ExecutionTime, 10),
		"podLabels=" + strings.Join(labels, ","),
	}, "|")
	return util.HashStringSHA256Hex(serialized)
}

func buildExpectedIntentRootsByNode(intents []*domain.ScheduleIntent) map[string]string {
	byNode := make(map[string][]*domain.ScheduleIntent)
	for _, intent := range normalizeScheduleIntents(intents) {
		byNode[intent.NodeID] = append(byNode[intent.NodeID], intent)
	}
	roots := make(map[string]string, len(byNode))
	for nodeID, nodeIntents := range byNode {
		roots[nodeID] = buildScheduleIntentMerkleRoot(nodeIntents)
	}
	return roots
}

func buildScheduleIntentMerkleRoot(intents []*domain.ScheduleIntent) string {
	leafHashes := make([]string, 0, len(intents))
	sortedIntents := sortScheduleIntentsByKey(intents)
	for _, intent := range sortedIntents {
		leafHashes = append(leafHashes, hashScheduleIntent(intent))
	}
	return util.BuildMerkleTree(leafHashes).Hash
}
