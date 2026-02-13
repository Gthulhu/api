package service

import (
	"context"
	"errors"
	"testing"

	"github.com/Gthulhu/api/manager/domain"
	"github.com/Gthulhu/api/pkg/util"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestCheckDMIntentsNoK8SAdapter(t *testing.T) {
	svc := &Service{}

	err := svc.CheckDMIntents(context.Background())
	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrNoClient)
}

func TestCheckDMIntentsQueryDecisionMakerPodsError(t *testing.T) {
	ctx := context.Background()
	mockK8S := domain.NewMockK8SAdapter(t)
	expectedErr := errors.New("query dms failed")
	mockK8S.EXPECT().
		QueryDecisionMakerPods(mock.Anything, mock.Anything).
		Run(func(_ context.Context, opt *domain.QueryDecisionMakerPodsOptions) {
			require.NotNil(t, opt)
			assert.Equal(t, "app", opt.DecisionMakerLabel.Key)
			assert.Equal(t, "decisionmaker", opt.DecisionMakerLabel.Value)
		}).
		Return(nil, expectedErr).
		Once()

	svc := &Service{
		K8SAdapter: mockK8S,
	}
	err := svc.CheckDMIntents(ctx)
	require.Error(t, err)
	assert.ErrorIs(t, err, expectedErr)
}

func TestCheckDMIntentsNoDecisionMakerPods(t *testing.T) {
	ctx := context.Background()
	mockK8S := domain.NewMockK8SAdapter(t)
	mockK8S.EXPECT().
		QueryDecisionMakerPods(mock.Anything, mock.Anything).
		Return([]*domain.DecisionMakerPod{}, nil).
		Once()

	svc := &Service{
		K8SAdapter: mockK8S,
		// Repo intentionally left nil. If code regresses and tries to query intents,
		// this test will fail via nil dereference.
	}

	err := svc.CheckDMIntents(ctx)
	require.NoError(t, err)
}

func TestCheckDMIntentsDMAdapterNilForOnlineNode(t *testing.T) {
	ctx := context.Background()
	mockK8S := domain.NewMockK8SAdapter(t)
	mockRepo := domain.NewMockRepository(t)
	dm := &domain.DecisionMakerPod{
		NodeID: "node-online",
		Host:   "127.0.0.1",
		Port:   8080,
		State:  domain.NodeStateOnline,
	}

	mockK8S.EXPECT().
		QueryDecisionMakerPods(mock.Anything, mock.Anything).
		Return([]*domain.DecisionMakerPod{dm}, nil).
		Once()
	mockRepo.EXPECT().
		QueryIntents(mock.Anything, mock.Anything).
		Run(func(_ context.Context, opt *domain.QueryIntentOptions) {
			opt.Result = []*domain.ScheduleIntent{}
		}).
		Return(nil).
		Once()

	svc := &Service{
		K8SAdapter: mockK8S,
		Repo:       mockRepo,
		DMAdapter:  nil,
	}
	err := svc.CheckDMIntents(ctx)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "decision maker adapter is nil")
}

func TestCheckDMIntentsHappyPathOnlineOnly(t *testing.T) {
	ctx := context.Background()
	mockK8S := domain.NewMockK8SAdapter(t)
	mockRepo := domain.NewMockRepository(t)
	mockDM := domain.NewMockDecisionMakerAdapter(t)

	onlineDM := &domain.DecisionMakerPod{
		NodeID: "node-online",
		Host:   "127.0.0.1",
		Port:   8080,
		State:  domain.NodeStateOnline,
	}
	offlineDM := &domain.DecisionMakerPod{
		NodeID: "node-offline",
		Host:   "127.0.0.1",
		Port:   8081,
		State:  domain.NodeStateOffline,
	}
	intents := []*domain.ScheduleIntent{
		{
			PodName:       "pod-b",
			PodID:         "pod-id-b",
			NodeID:        "node-b",
			K8sNamespace:  "ns-b",
			CommandRegex:  "busybox",
			Priority:      1,
			ExecutionTime: 22,
			PodLabels: map[string]string{
				"k2": "v2",
			},
		},
		{
			PodName:       "pod-a",
			PodID:         "pod-id-a",
			NodeID:        "node-a",
			K8sNamespace:  "ns-a",
			CommandRegex:  "nginx",
			Priority:      0,
			ExecutionTime: 11,
			PodLabels: map[string]string{
				"k1": "v1",
			},
		},
	}
	expectedRoot := buildExpectedScheduleIntentRootHash(intents)

	mockK8S.EXPECT().
		QueryDecisionMakerPods(mock.Anything, mock.Anything).
		Return([]*domain.DecisionMakerPod{onlineDM, offlineDM}, nil).
		Once()
	mockRepo.EXPECT().
		QueryIntents(mock.Anything, mock.Anything).
		Run(func(_ context.Context, opt *domain.QueryIntentOptions) {
			opt.Result = intents
		}).
		Return(nil).
		Once()
	mockDM.EXPECT().
		GetIntentMerkleRoot(mock.Anything, onlineDM).
		Return(expectedRoot, nil).
		Once()

	svc := &Service{
		K8SAdapter: mockK8S,
		Repo:       mockRepo,
		DMAdapter:  mockDM,
	}

	err := svc.CheckDMIntents(ctx)
	require.NoError(t, err)
}

func TestSortScheduleIntentsByKeyAndHashDeterministic(t *testing.T) {
	intentA := &domain.ScheduleIntent{
		PodName:       "pod-a",
		PodID:         "pod-id-a",
		NodeID:        "node-a",
		K8sNamespace:  "default",
		CommandRegex:  "nginx",
		Priority:      1,
		ExecutionTime: 10,
		PodLabels: map[string]string{
			"b": "2",
			"a": "1",
		},
	}
	intentB := &domain.ScheduleIntent{
		PodName:       "pod-b",
		PodID:         "pod-id-b",
		NodeID:        "node-b",
		K8sNamespace:  "kube-system",
		CommandRegex:  "busybox",
		Priority:      0,
		ExecutionTime: 20,
		PodLabels: map[string]string{
			"k1": "v1",
		},
	}
	sorted := sortScheduleIntentsByKey([]*domain.ScheduleIntent{intentB, intentA})

	require.Len(t, sorted, 2)
	assert.Equal(t, intentA.PodName, sorted[0].PodName)
	assert.Equal(t, intentB.PodName, sorted[1].PodName)
	assert.Equal(t, hashScheduleIntent(intentA), hashScheduleIntent(&domain.ScheduleIntent{
		PodName:       intentA.PodName,
		PodID:         intentA.PodID,
		NodeID:        intentA.NodeID,
		K8sNamespace:  intentA.K8sNamespace,
		CommandRegex:  intentA.CommandRegex,
		Priority:      intentA.Priority,
		ExecutionTime: intentA.ExecutionTime,
		PodLabels: map[string]string{
			"a": "1",
			"b": "2",
		},
	}))
}

func buildExpectedScheduleIntentRootHash(intents []*domain.ScheduleIntent) string {
	leafHashes := make([]string, 0, len(intents))
	sorted := sortScheduleIntentsByKey(intents)
	for _, intent := range sorted {
		leafHashes = append(leafHashes, hashScheduleIntent(intent))
	}
	root := util.BuildMerkleTree(leafHashes)
	if root == nil {
		return ""
	}
	return root.Hash
}
